package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/embedding"
	"github.com/thomassaison/mcp-code-graph/internal/indexer"
	"github.com/thomassaison/mcp-code-graph/internal/llm"
	"github.com/thomassaison/mcp-code-graph/internal/mcp"
	"github.com/thomassaison/mcp-code-graph/internal/web"
)

// version is injected at build time via ldflags
var version = "dev"

func main() {
	flag.Parse()

	// Parse debug config
	debugLevel := 0
	if v := os.Getenv("MCP_CODE_GRAPH_DEBUG"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			debugLevel = n
		}
	}
	debugFile := os.Getenv("MCP_CODE_GRAPH_DEBUG_FILE")
	if err := debug.Setup(debugLevel, debugFile, nil); err != nil {
		log.Printf("warning: failed to setup debug logger: %v", err)
	}

	// Auto-detect project directory from current working directory
	projectPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Determine database directory
	// Check MCP_CODE_GRAPH_DIR env var, otherwise use project-local directory
	dbDir := os.Getenv("MCP_CODE_GRAPH_DIR")
	if dbDir == "" {
		dbDir = filepath.Join(projectPath, ".mcp-code-graph")
	}

	// Ensure database directory exists
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Database path (without extension - server adds suffixes)
	dbPath := filepath.Join(dbDir, "db")

	// Parse embedding config
	embeddingCfg, err := embedding.ParseConfig(os.Getenv("EMBEDDING_CONFIG"))
	if err != nil {
		log.Printf("warning: failed to parse embedding config: %v", err)
	}

	// Parse LLM config
	llmCfg, err := llm.ParseConfig(os.Getenv("LLM_CONFIG"))
	if err != nil {
		log.Printf("warning: failed to parse LLM config: %v", err)
	}

	// Create server
	server, err := mcp.NewServer(&mcp.Config{
		DBPath:      dbPath,
		ProjectPath: projectPath,
		Embedding:   embeddingCfg,
		LLM:         llmCfg,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Load persisted graph data synchronously — fast (<1s), makes tools usable immediately
	server.LoadGraph()
	slog.Info("MCP Code Graph server starting",
		"project", projectPath,
		"loaded_nodes", server.Graph().NodeCount(),
	)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create and start MCP server FIRST so it can respond to initialize immediately.
	mcpSrv := mcpserver.NewMCPServer(
		"mcp-code-graph",
		version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(true, true),
	)

	// Register tools and resources
	server.RegisterTools(mcpSrv)
	server.RegisterResources(mcpSrv)

	// Start MCP protocol on stdio (must be first so the client doesn't timeout).
	// When the client disconnects (stdin closes), trigger graceful shutdown.
	go func() {
		if err := mcpserver.ServeStdio(mcpSrv); err != nil {
			slog.Error("MCP server error", "error", err)
		}
		// Client disconnected — trigger shutdown
		sigChan <- syscall.SIGTERM
	}()

	// Reindex project in the background. Tools work immediately with loaded data;
	// the graph's RWMutex handles concurrent reads/writes safely.
	server.PrepareAsyncIndex()
	var (
		watcher   *indexer.Watcher
		indexDone = make(chan struct{})
	)
	go func() {
		defer close(indexDone)

		if err := server.IndexProject(); err != nil {
			slog.Error("Failed to index project", "error", err)
			return
		}

		server.MarkReady()
		slog.Info("Index complete", "functions", server.Graph().NodeCount())

		// Start filesystem watcher for incremental re-indexing
		var watcherErr error
		watcher, watcherErr = server.WatchProject(500 * time.Millisecond)
		if watcherErr != nil {
			slog.Warn("Failed to create file watcher (incremental updates disabled)", "error", watcherErr)
		} else {
			if err := watcher.Watch(projectPath); err != nil {
				slog.Warn("Failed to start file watcher", "error", err)
				watcher.Close()
				watcher = nil
			} else {
				slog.Info("Watching for file changes", "path", projectPath)
			}
		}

		// Generate summaries in background (non-critical)
		if err := server.GenerateSummaries(context.Background()); err != nil {
			slog.Warn("failed to generate summaries", "error", err)
		}
	}()

	// Start web server if configured
	var httpSrv *http.Server
	if webAddr := os.Getenv("MCP_CODE_GRAPH_WEB"); webAddr != "" {
		modulePath := readModulePath(projectPath)
		webHandler := web.NewHandler(server.Graph(), modulePath)
		httpSrv = &http.Server{Addr: webAddr, Handler: webHandler}
		go func() {
			slog.Info("Starting web server", "address", webAddr)
			if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
				slog.Error("Web server error", "error", err)
			}
		}()
	}

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Shutting down...")

	// Wait for background indexing to finish so we don't race on the DB
	<-indexDone

	// Graceful shutdown
	if httpSrv != nil {
		if err := httpSrv.Shutdown(context.Background()); err != nil {
			slog.Error("Web server shutdown error", "error", err)
		}
	}

	// Close watcher if indexing already set it up
	if watcher != nil {
		watcher.Close()
	}

	server.Close()
	slog.Info("Server stopped")
}

func readModulePath(projectPath string) string {
	data, err := os.ReadFile(filepath.Join(projectPath, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}
