package vector

import (
	"database/sql"
	"fmt"
	stdmath "math"
	"sort"
	"sync"

	"github.com/thomassaison/mcp-code-graph/internal/math"
	_ "modernc.org/sqlite"
)

type SearchResult struct {
	NodeID string
	Text   string
	Score  float32
}

type cacheEntry struct {
	text      string
	embedding []float32
}

type Store struct {
	dbPath string
	db     *sql.DB

	mu    sync.RWMutex
	cache map[string]cacheEntry
}

func NewStore(dbPath string) (*Store, error) {
	store := &Store{
		dbPath: dbPath,
		cache:  make(map[string]cacheEntry),
	}
	if err := store.open(); err != nil {
		return nil, err
	}
	if err := store.initTables(); err != nil {
		return nil, err
	}
	if err := store.loadCache(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) open() error {
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	s.db = db
	return nil
}

func (s *Store) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS embeddings (
			node_id TEXT PRIMARY KEY,
			text TEXT NOT NULL,
			embedding BLOB
		);
		CREATE INDEX IF NOT EXISTS idx_embeddings_node ON embeddings(node_id);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

func (s *Store) loadCache() error {
	rows, err := s.db.Query(`SELECT node_id, text, embedding FROM embeddings`)
	if err != nil {
		return fmt.Errorf("load cache: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var nodeID, text string
		var embeddingBytes []byte
		if err := rows.Scan(&nodeID, &text, &embeddingBytes); err != nil {
			continue
		}

		embedding := bytesToFloat32(embeddingBytes)
		s.cache[nodeID] = cacheEntry{
			text:      text,
			embedding: embedding,
		}
	}

	return nil
}

func bytesToFloat32(b []byte) []float32 {
	embedding := make([]float32, len(b)/4)
	for i := range embedding {
		bits := uint32(b[i*4]) |
			uint32(b[i*4+1])<<8 |
			uint32(b[i*4+2])<<16 |
			uint32(b[i*4+3])<<24
		embedding[i] = stdmath.Float32frombits(bits)
	}
	return embedding
}

func float32ToBytes(f []float32) []byte {
	b := make([]byte, len(f)*4)
	for i, v := range f {
		bits := stdmath.Float32bits(v)
		b[i*4] = byte(bits)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return b
}

func (s *Store) Insert(nodeID, text string, embedding []float32) error {
	embeddingBytes := float32ToBytes(embedding)

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO embeddings (node_id, text, embedding)
		VALUES (?, ?, ?)
	`, nodeID, text, embeddingBytes)
	if err != nil {
		return err
	}

	// Update cache
	s.mu.Lock()
	s.cache[nodeID] = cacheEntry{
		text:      text,
		embedding: embedding,
	}
	s.mu.Unlock()

	return nil
}

func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult
	for nodeID, entry := range s.cache {
		score := math.CosineSimilarity(query, entry.embedding)
		results = append(results, SearchResult{
			NodeID: nodeID,
			Text:   entry.text,
			Score:  score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Store) GetEmbedding(nodeID string) ([]float32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.cache[nodeID]
	if !ok {
		return nil, fmt.Errorf("embedding not found for node %s", nodeID)
	}

	// Return a copy to prevent mutation
	embedding := make([]float32, len(entry.embedding))
	copy(embedding, entry.embedding)
	return embedding, nil
}
