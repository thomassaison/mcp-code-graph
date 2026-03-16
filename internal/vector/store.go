// anthropic/claude-sonnet-4-6
package vector

import (
	"container/heap"
	"database/sql"
	"fmt"
	stdmath "math"
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
	summaryText      string
	codeText         string
	summaryEmbedding []float32
	codeEmbedding    []float32
	summaryNorm      float32
	codeNorm         float32
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
	if err := store.migrateIfNeeded(); err != nil {
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

func (s *Store) migrateIfNeeded() error {
	rows, err := s.db.Query(`PRAGMA table_info(embeddings)`)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var dfltValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if name == "summary_embedding" {
			return nil // already on new schema
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info: %w", err)
	}

	// Old schema or no table — drop and let initTables recreate
	_, err = s.db.Exec(`DROP TABLE IF EXISTS embeddings`)
	return err
}

func (s *Store) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS embeddings (
			node_id           TEXT PRIMARY KEY,
			summary_text      TEXT,
			code_text         TEXT,
			summary_embedding BLOB,
			code_embedding    BLOB
		);
		CREATE INDEX IF NOT EXISTS idx_embeddings_node ON embeddings(node_id);
	`)
	if err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

func (s *Store) loadCache() error {
	rows, err := s.db.Query(`SELECT node_id, summary_text, code_text, summary_embedding, code_embedding FROM embeddings`)
	if err != nil {
		return fmt.Errorf("load cache: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var nodeID, summaryText, codeText string
		var summaryBytes, codeBytes []byte
		if err := rows.Scan(&nodeID, &summaryText, &codeText, &summaryBytes, &codeBytes); err != nil {
			continue
		}
		summaryEmb := nullableBytesToFloat32(summaryBytes)
		codeEmb := nullableBytesToFloat32(codeBytes)
		s.cache[nodeID] = cacheEntry{
			summaryText:      summaryText,
			codeText:         codeText,
			summaryEmbedding: summaryEmb,
			codeEmbedding:    codeEmb,
			summaryNorm:      math.L2Norm(summaryEmb),
			codeNorm:         math.L2Norm(codeEmb),
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate embeddings: %w", err)
	}
	return nil
}

func nullableBytesToFloat32(b []byte) []float32 {
	if b == nil {
		return nil
	}
	return bytesToFloat32(b)
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

// Insert stores both embeddings for a node. Either embedding slice may be nil
// (the corresponding column is set to NULL). If a row already exists for
// nodeID, it is fully replaced.
func (s *Store) Insert(
	nodeID string,
	summaryText string, summaryEmb []float32,
	codeText string, codeEmb []float32,
) error {
	var summaryBytes []byte
	if summaryEmb != nil {
		summaryBytes = float32ToBytes(summaryEmb)
	}
	var codeBytes []byte
	if codeEmb != nil {
		codeBytes = float32ToBytes(codeEmb)
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO embeddings (node_id, summary_text, code_text, summary_embedding, code_embedding)
		VALUES (?, ?, ?, ?, ?)
	`, nodeID, summaryText, codeText, summaryBytes, codeBytes)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.cache[nodeID] = cacheEntry{
		summaryText:      summaryText,
		codeText:         codeText,
		summaryEmbedding: summaryEmb,
		codeEmbedding:    codeEmb,
		summaryNorm:      math.L2Norm(summaryEmb),
		codeNorm:         math.L2Norm(codeEmb),
	}
	s.mu.Unlock()

	return nil
}

// weightedScore computes the combined similarity score for a cache entry using
// pre-computed norms and a single query norm to avoid redundant sqrt calls.
// Returns 0 if the entry has no usable embeddings.
func weightedScore(query []float32, queryNorm float32, entry cacheEntry) float32 {
	hasSummary := entry.summaryEmbedding != nil && entry.summaryNorm > 0
	hasCode := entry.codeEmbedding != nil && entry.codeNorm > 0

	cosineFast := func(emb []float32, embNorm float32) float32 {
		if queryNorm == 0 {
			return 0
		}
		dot := math.DotProduct(query, emb)
		return dot / (queryNorm * embNorm)
	}

	switch {
	case hasSummary && hasCode:
		return 0.6*cosineFast(entry.summaryEmbedding, entry.summaryNorm) +
			0.4*cosineFast(entry.codeEmbedding, entry.codeNorm)
	case hasSummary:
		return cosineFast(entry.summaryEmbedding, entry.summaryNorm)
	case hasCode:
		return cosineFast(entry.codeEmbedding, entry.codeNorm)
	default:
		return 0
	}
}

// resultHeap is a min-heap of SearchResult ordered by Score.
// It is used to track the top-K results without sorting the full slice.
type resultHeap []SearchResult

func (h resultHeap) Len() int            { return len(h) }
func (h resultHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score } // min at root
func (h resultHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *resultHeap) Push(x interface{}) { *h = append(*h, x.(SearchResult)) }
func (h *resultHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// topK uses a min-heap of size limit to select the top-K results by score
// without sorting the full candidate slice.
func topK(candidates []SearchResult, limit int) []SearchResult {
	if limit <= 0 {
		return nil
	}
	h := &resultHeap{}
	heap.Init(h)
	for _, r := range candidates {
		if h.Len() < limit {
			heap.Push(h, r)
		} else if r.Score > (*h)[0].Score {
			heap.Pop(h)
			heap.Push(h, r)
		}
	}
	// Extract in descending order
	result := make([]SearchResult, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(SearchResult)
	}
	return result
}

func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryNorm := math.L2Norm(query)
	var candidates []SearchResult
	for nodeID, entry := range s.cache {
		score := weightedScore(query, queryNorm, entry)
		if score == 0 {
			continue
		}
		candidates = append(candidates, SearchResult{
			NodeID: nodeID,
			Text:   entry.summaryText,
			Score:  score,
		})
	}

	return topK(candidates, limit), nil
}

// HasEmbeddings reports whether summary and code embeddings exist for nodeID.
// Reads from the in-memory cache only.
func (s *Store) HasEmbeddings(nodeID string) (hasSummary, hasCode bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[nodeID]
	if !ok {
		return false, false
	}
	return entry.summaryEmbedding != nil, entry.codeEmbedding != nil
}

// ScoreNodes ranks the provided nodeIDs by weighted cosine similarity
// against query. Only nodes present in the cache are scored.
func (s *Store) ScoreNodes(query []float32, nodeIDs []string, limit int) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodeSet := make(map[string]struct{}, len(nodeIDs))
	for _, id := range nodeIDs {
		nodeSet[id] = struct{}{}
	}

	queryNorm := math.L2Norm(query)
	var candidates []SearchResult
	for nodeID, entry := range s.cache {
		if _, ok := nodeSet[nodeID]; !ok {
			continue
		}
		score := weightedScore(query, queryNorm, entry)
		if score == 0 {
			continue
		}
		candidates = append(candidates, SearchResult{
			NodeID: nodeID,
			Text:   entry.summaryText,
			Score:  score,
		})
	}

	return topK(candidates, limit)
}

// Delete removes the embedding for nodeID from both the SQLite DB and the in-memory cache.
// It is a no-op if nodeID does not exist.
func (s *Store) Delete(nodeID string) error {
	_, err := s.db.Exec(`DELETE FROM embeddings WHERE node_id = ?`, nodeID)
	if err != nil {
		return fmt.Errorf("delete embedding: %w", err)
	}

	s.mu.Lock()
	delete(s.cache, nodeID)
	s.mu.Unlock()

	return nil
}

func (s *Store) Close() {
	if s.db != nil {
		_ = s.db.Close()
	}
}
