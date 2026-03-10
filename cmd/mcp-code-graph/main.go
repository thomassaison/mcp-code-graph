package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/thomassaison/mcp-code-graph/internal/embedding"
	"github.com/thomassaison/mcp-code-graph/internal/mcp"
)

// version is injected at build time via ldflags
var version = "dev"

func main() {
	llmModel := flag.String("model", "", "LLM model for summaries (empty = mock)")
	flag.Parse()

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

	// Create server
	server, err := mcp.NewServer(&mcp.Config{
		DBPath:      dbPath,
		ProjectPath: projectPath,
		LLMModel:    *llmModel,
		Embedding:   embeddingCfg,
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Index the project first (before starting MCP protocol)
	if err := server.IndexProject(); err != nil {
		log.Fatalf("Failed to index project: %v", err)
	}

	// Log startup info to stderr (stdout is used for MCP protocol)
	log.Printf("MCP Code Graph server starting")
	log.Printf("Project: %s", projectPath)
	log.Printf("Database: %s", dbPath)
	log.Printf("Functions indexed: %d", server.Graph().NodeCount())

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
