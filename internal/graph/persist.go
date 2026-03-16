package graph

import (
	"database/sql"
	"encoding/json"
	"log/slog"

	_ "modernc.org/sqlite"
)

type Persister struct {
	dbPath string
	db     *sql.DB
}

func NewPersister(dbPath string) (*Persister, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Create schema and indexes (idempotent).
	// Migration: drop legacy edges table that lacks UNIQUE constraint.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			package TEXT NOT NULL,
			name TEXT NOT NULL,
			file TEXT NOT NULL,
			line INTEGER NOT NULL,
			column INTEGER,
			signature TEXT,
			docstring TEXT,
			summary TEXT,
			metadata TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package);
		CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
		CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
	`); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Migrate edges table: if it exists without the UNIQUE constraint, recreate it.
	if err := migrateEdgesTable(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Persister{
		dbPath: dbPath,
		db:     db,
	}, nil
}

// migrateEdgesTable ensures the edges table has a UNIQUE constraint on (from_id, to_id, type).
// If the table exists without the constraint (legacy schema), it is recreated.
func migrateEdgesTable(db *sql.DB) error {
	// Check if edges table exists
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='edges'").Scan(&tableName)
	if err == sql.ErrNoRows {
		// Table doesn't exist yet, create with UNIQUE constraint
		_, err := db.Exec(`
			CREATE TABLE edges (
				from_id TEXT NOT NULL,
				to_id TEXT NOT NULL,
				type TEXT NOT NULL,
				metadata TEXT,
				UNIQUE(from_id, to_id, type)
			);
			CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
			CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
			CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);
		`)
		return err
	}
	if err != nil {
		return err
	}

	// Table exists — check if it has the legacy 'id' column (AUTOINCREMENT schema)
	var hasIDColumn bool
	rows, err := db.Query("PRAGMA table_info(edges)")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return err
		}
		if name == "id" {
			hasIDColumn = true
			break
		}
	}

	if hasIDColumn {
		// Legacy schema detected — migrate
		slog.Info("migrating edges table to add UNIQUE constraint")
		_, err := db.Exec(`
			DROP TABLE IF EXISTS edges;
			CREATE TABLE edges (
				from_id TEXT NOT NULL,
				to_id TEXT NOT NULL,
				type TEXT NOT NULL,
				metadata TEXT,
				UNIQUE(from_id, to_id, type)
			);
			CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
			CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
			CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type);
		`)
		return err
	}

	return nil
}

func (p *Persister) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *Persister) Save(g *Graph) error {
	slog.Debug("graph save started", "db", p.dbPath)

	// Snapshot the graph under a read lock to avoid concurrent map access.
	nodes := g.AllNodes()
	edges := g.AllEdges()

	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec("DELETE FROM edges")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM nodes")
	if err != nil {
		return err
	}

	for _, node := range nodes {
		var summaryJSON, metadataJSON []byte
		if node.Summary != nil {
			summaryJSON, err = json.Marshal(node.Summary)
			if err != nil {
				return err
			}
		}
		if node.Metadata != nil {
			metadataJSON, err = json.Marshal(node.Metadata)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
			INSERT INTO nodes (id, type, package, name, file, line, column, signature, docstring, summary, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, node.ID, node.Type, node.Package, node.Name, node.File, node.Line, node.Column,
			node.Signature, node.Docstring, string(summaryJSON), string(metadataJSON))
		if err != nil {
			return err
		}
	}

	for _, edge := range edges {
		var metadataJSON []byte
		if edge.Metadata != nil {
			metadataJSON, err = json.Marshal(edge.Metadata)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(`
			INSERT OR IGNORE INTO edges (from_id, to_id, type, metadata)
			VALUES (?, ?, ?, ?)
		`, edge.From, edge.To, edge.Type, string(metadataJSON))
		if err != nil {
			return err
		}
	}

	slog.Debug("graph save complete", "nodes", len(nodes), "db", p.dbPath)
	return tx.Commit()
}

func (p *Persister) Load(g *Graph) error {
	slog.Debug("graph load started", "db", p.dbPath)

	rows, err := p.db.Query(`
		SELECT id, type, package, name, file, line, column, signature, docstring, summary, metadata
		FROM nodes
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		node := &Node{}
		var summaryJSON, metadataJSON sql.NullString
		err = rows.Scan(&node.ID, &node.Type, &node.Package, &node.Name, &node.File, &node.Line, &node.Column,
			&node.Signature, &node.Docstring, &summaryJSON, &metadataJSON)
		if err != nil {
			return err
		}

		if summaryJSON.Valid && summaryJSON.String != "" {
			node.Summary = &Summary{}
			if err := json.Unmarshal([]byte(summaryJSON.String), node.Summary); err != nil {
				return err
			}
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			node.Metadata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataJSON.String), &node.Metadata); err != nil {
				return err
			}
		}

		g.AddNode(node)
	}

	edgeRows, err := p.db.Query(`
		SELECT from_id, to_id, type, metadata
		FROM edges
	`)
	if err != nil {
		return err
	}
	defer func() { _ = edgeRows.Close() }()

	for edgeRows.Next() {
		edge := &Edge{}
		var metadataJSON sql.NullString
		err = edgeRows.Scan(&edge.From, &edge.To, &edge.Type, &metadataJSON)
		if err != nil {
			return err
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			edge.Metadata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataJSON.String), &edge.Metadata); err != nil {
				return err
			}
		}

		g.AddEdge(edge)
	}

	slog.Debug("graph load complete", "db", p.dbPath)
	return nil
}
