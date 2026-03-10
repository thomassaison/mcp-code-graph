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
		db.Close()
		return nil, err
	}

	return &Persister{
		dbPath: dbPath,
		db:     db,
	}, nil
}

func (p *Persister) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

func (p *Persister) Save(g *Graph) error {
	slog.Debug("graph save started", "db", p.dbPath)

	_, err := p.db.Exec(`
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
		)
	`)
	if err != nil {
		return err
	}

	_, err = p.db.Exec(`
		CREATE TABLE IF NOT EXISTS edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			type TEXT NOT NULL,
			metadata TEXT
		)
	`)
	if err != nil {
		return err
	}

	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM edges")
	if err != nil {
		return err
	}

	_, err = tx.Exec("DELETE FROM nodes")
	if err != nil {
		return err
	}

	for _, node := range g.nodes {
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

	for _, edges := range g.edges {
		for _, edge := range edges {
			var metadataJSON []byte
			if edge.Metadata != nil {
				metadataJSON, err = json.Marshal(edge.Metadata)
				if err != nil {
					return err
				}
			}

			_, err = tx.Exec(`
				INSERT INTO edges (from_id, to_id, type, metadata)
				VALUES (?, ?, ?, ?)
			`, edge.From, edge.To, edge.Type, string(metadataJSON))
			if err != nil {
				return err
			}
		}
	}

	slog.Debug("graph save complete", "nodes", len(g.nodes), "db", p.dbPath)
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
	defer rows.Close()

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
	defer edgeRows.Close()

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
