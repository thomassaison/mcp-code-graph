package main

import (
	"flag"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"net/http"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomassaison/mcp-code-graph/internal/debug"
	"github.com/thomassaison/mcp-code-graph/internal/embedding"
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

	// Index the project first (before starting MCP protocol)
	if err := server.IndexProject(); err != nil {
		log.Fatalf("Failed to index project: %v", err)
	}

	// Log startup info to stderr (stdout is used for MCP protocol)
	slog.Info("MCP Code Graph server starting")
	slog.Info("project", "path", projectPath)
	slog.Info("database", "path", dbPath)
	slog.Info("indexed", "functions", server.Graph().NodeCount())

	// Start web server if configured
	if webAddr := os.Getenv("MCP_CODE_GRAPH_WEB"); webAddr != "" {
		go func() {
			webHandler := web.NewHandler(server.Graph())
			slog.Info("Starting web server", "address", webAddr)
			if err := http.ListenAndServe(webAddr, webHandler); err != nil {
				slog.Error("Web server error", "error", err)
			}
		}()
	}

	// Create MCP server with capabilities
	mcpSrv := mcpserver.NewMCPServer(
		"mcp-code-graph",
		version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(true, true),
	)

	// Register tools and resources
	server.RegisterTools(mcpSrv)
	server.RegisterResources(mcpSrv)

	// Start serving MCP protocol over stdio
	if err := mcpserver.ServeStdio(mcpSrv); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
