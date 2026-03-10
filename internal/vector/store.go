package vector

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	_ "modernc.org/sqlite"
)

type SearchResult struct {
	NodeID string
	Text   string
	Score  float32
}

type Store struct {
	dbPath string
	db     *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	store := &Store{dbPath: dbPath}
	if err := store.open(); err != nil {
		return nil, err
	}
	if err := store.initTables(); err != nil {
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

func (s *Store) Insert(nodeID, text string, embedding []float32) error {
	embeddingBytes := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := math.Float32bits(v)
		embeddingBytes[i*4] = byte(bits)
		embeddingBytes[i*4+1] = byte(bits >> 8)
		embeddingBytes[i*4+2] = byte(bits >> 16)
		embeddingBytes[i*4+3] = byte(bits >> 24)
	}

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO embeddings (node_id, text, embedding)
		VALUES (?, ?, ?)
	`, nodeID, text, embeddingBytes)
	return err
}

func (s *Store) Search(query []float32, limit int) ([]SearchResult, error) {
	rows, err := s.db.Query(`SELECT node_id, text, embedding FROM embeddings`)
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var nodeID, text string
		var embeddingBytes []byte
		if err := rows.Scan(&nodeID, &text, &embeddingBytes); err != nil {
			continue
		}

		embedding := make([]float32, len(embeddingBytes)/4)
		for i := range embedding {
			bits := uint32(embeddingBytes[i*4]) |
				uint32(embeddingBytes[i*4+1])<<8 |
				uint32(embeddingBytes[i*4+2])<<16 |
				uint32(embeddingBytes[i*4+3])<<24
			embedding[i] = math.Float32frombits(bits)
		}

		score := cosineSimilarity(query, embedding)
		results = append(results, SearchResult{
			NodeID: nodeID,
			Text:   text,
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
	var embeddingBytes []byte
	err := s.db.QueryRow(`SELECT embedding FROM embeddings WHERE node_id = ?`, nodeID).Scan(&embeddingBytes)
	if err != nil {
		return nil, fmt.Errorf("get embedding: %w", err)
	}

	embedding := make([]float32, len(embeddingBytes)/4)
	for i := range embedding {
		bits := uint32(embeddingBytes[i*4]) |
			uint32(embeddingBytes[i*4+1])<<8 |
			uint32(embeddingBytes[i*4+2])<<16 |
			uint32(embeddingBytes[i*4+3])<<24
		embedding[i] = math.Float32frombits(bits)
	}

	return embedding, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt32(normA) * sqrt32(normB))
}

func sqrt32(x float32) float32 {
	z := x
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}
