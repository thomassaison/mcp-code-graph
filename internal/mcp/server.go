package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/thomas-saison/mcp-code-graph/internal/graph"
	"github.com/thomas-saison/mcp-code-graph/internal/indexer"
	goparser "github.com/thomas-saison/mcp-code-graph/internal/parser/go"
	"github.com/thomas-saison/mcp-code-graph/internal/summary"
	"github.com/thomas-saison/mcp-code-graph/internal/vector"
)

type Config struct {
	DBPath      string
	ProjectPath string
	LLMModel    string
}

type Server struct {
	graph     *graph.Graph
	vector    *vector.Store
	indexer   *indexer.Indexer
	summary   *summary.Generator
	persister *graph.Persister
	parser    *goparser.GoParser
	config    *Config
}

func NewServer(cfg *Config) (*Server, error) {
	gr := graph.New()

	vecStore, err := vector.NewStore(cfg.DBPath + ".vec.db")
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	p := goparser.New()
	idx := indexer.New(gr, p)
	persister := graph.NewPersister(cfg.DBPath + ".graph.db")

	var gen *summary.Generator
	if cfg.LLMModel != "" {
		gen = summary.NewGenerator(&summary.MockProvider{}, cfg.LLMModel)
	} else {
		gen = summary.NewGenerator(&summary.MockProvider{}, "mock")
	}

	return &Server{
		graph:     gr,
		vector:    vecStore,
		indexer:   idx,
		summary:   gen,
		persister: persister,
		parser:    p,
		config:    cfg,
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	// Load existing graph data (ignore errors - file may not exist yet)
	if err := s.persister.Load(s.graph); err != nil {
		// Non-critical: first run or corrupted file, will rebuild
	}

	if err := s.indexer.IndexModule(s.config.ProjectPath); err != nil {
		return fmt.Errorf("index project: %w", err)
	}

	// Generate summaries (ignore errors - non-critical for basic operation)
	if err := s.summary.GenerateAll(ctx, s.graph); err != nil {
		// Non-critical: summaries can be regenerated later
	}

	if err := s.persister.Save(s.graph); err != nil {
		return fmt.Errorf("save graph: %w", err)
	}

	return nil
}

func (s *Server) Close() {
	if err := s.persister.Save(s.graph); err != nil {
		log.Printf("failed to save graph on close: %v", err)
	}
	s.vector.Close()
}

func (s *Server) Graph() *graph.Graph {
	return s.graph
}
