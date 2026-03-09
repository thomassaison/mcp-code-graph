package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/thomas-saison/mcp-code-graph/internal/mcp"
)

func main() {
	projectPath := flag.String("project", ".", "Path to the Go project to index")
	dbPath := flag.String("db", ".mcp-code-graph/db.sqlite", "Path to the database file")
	llmModel := flag.String("model", "gpt-4o-mini", "LLM model for summaries")
	flag.Parse()

	dbDir := filepath.Dir(*dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating DB directory: %v\n", err)
		os.Exit(1)
	}

	server, err := mcp.NewServer(&mcp.Config{
		DBPath:      *dbPath,
		ProjectPath: *projectPath,
		LLMModel:    *llmModel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := server.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("MCP Code Graph server started\n")
	fmt.Printf("Project: %s\n", *projectPath)
	fmt.Printf("Database: %s\n", *dbPath)
	fmt.Printf("Functions indexed: %d\n", server.Graph().NodeCount())

	<-ctx.Done()
}
