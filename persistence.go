package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// PersistenceDB wraps the SQLite database used to store app state.
type PersistenceDB struct {
	db      *sql.DB
	dataDir string // directory where parquet snapshots are written
}

func newPersistenceDB() (*PersistenceDB, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("finding config dir: %w", err)
	}
	appDir := filepath.Join(configDir, "sql.garden")
	dataDir := filepath.Join(appDir, "data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	dbPath := filepath.Join(appDir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening state db: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	return &PersistenceDB{db: db, dataDir: dataDir}, nil
}

func migrateSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS canvas_state (
			key         TEXT PRIMARY KEY,
			nodes_json  TEXT NOT NULL,
			updated_at  INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);

		CREATE TABLE IF NOT EXISTS table_data (
			table_name   TEXT PRIMARY KEY,
			parquet_path TEXT NOT NULL,
			updated_at   INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);

		CREATE TABLE IF NOT EXISTS saved_connections (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			type        TEXT NOT NULL,
			dsn         TEXT,
			read_only   INTEGER NOT NULL DEFAULT 1,
			created_at  INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);

		CREATE TABLE IF NOT EXISTS settings (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
		);
	`)
	return err
}

// ── App settings ──────────────────────────────────────────────────────────────

func (p *PersistenceDB) saveSetting(key, value string) error {
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES (?, ?, strftime('%s','now'))`,
		key, value,
	)
	return err
}

func (p *PersistenceDB) getSetting(key string) (string, error) {
	var value string
	err := p.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (p *PersistenceDB) close() {
	if p.db != nil {
		p.db.Close()
	}
}

// ── Canvas state ──────────────────────────────────────────────────────────────

func (p *PersistenceDB) saveCanvasState(nodesJSON string) error {
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO canvas_state (key, nodes_json, updated_at)
		 VALUES ('v1', ?, strftime('%s','now'))`,
		nodesJSON,
	)
	return err
}

// loadCanvasState returns the saved JSON, or "" if nothing has been saved yet.
func (p *PersistenceDB) loadCanvasState() (string, error) {
	var js string
	err := p.db.QueryRow(`SELECT nodes_json FROM canvas_state WHERE key = 'v1'`).Scan(&js)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return js, err
}

// ── Table data (parquet files) ────────────────────────────────────────────────

// parquetPath returns the canonical on-disk path for a table's snapshot.
func (p *PersistenceDB) parquetPath(tableName string) string {
	// Replace path-separator characters so the name is safe to use as a filename.
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, tableName)
	return filepath.Join(p.dataDir, safe+".parquet")
}

func (p *PersistenceDB) recordTableData(tableName, path string) error {
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO table_data (table_name, parquet_path, updated_at)
		 VALUES (?, ?, strftime('%s','now'))`,
		tableName, path,
	)
	return err
}

// getTableDataPath returns the parquet path for a table, or "" if not recorded.
func (p *PersistenceDB) getTableDataPath(tableName string) (string, error) {
	var path string
	err := p.db.QueryRow(`SELECT parquet_path FROM table_data WHERE table_name = ?`, tableName).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return path, err
}

// ── Saved connections ─────────────────────────────────────────────────────────

// ConnectionRecord is persisted to SQLite and exposed to the frontend.
type ConnectionRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`      // "postgres" | "sqlite" | "duckdb" | "mysql"
	DSN       string `json:"dsn"`
	ReadOnly  bool   `json:"readOnly"`
	CreatedAt int64  `json:"createdAt"` // Unix seconds
}

func (p *PersistenceDB) saveConnection(conn ConnectionRecord) (ConnectionRecord, error) {
	if conn.ID == "" {
		conn.ID = fmt.Sprintf("conn_%d", time.Now().UnixNano())
	}
	if conn.CreatedAt == 0 {
		conn.CreatedAt = time.Now().Unix()
	}
	_, err := p.db.Exec(
		`INSERT OR REPLACE INTO saved_connections (id, name, type, dsn, read_only, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		conn.ID, conn.Name, conn.Type, conn.DSN, boolToInt(conn.ReadOnly), conn.CreatedAt,
	)
	return conn, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (p *PersistenceDB) listConnections() ([]ConnectionRecord, error) {
	rows, err := p.db.Query(
		`SELECT id, name, type, dsn, read_only, created_at
		 FROM saved_connections ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conns []ConnectionRecord
	for rows.Next() {
		var c ConnectionRecord
		var readOnly int
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.DSN, &readOnly, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.ReadOnly = readOnly != 0
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

func (p *PersistenceDB) getConnection(id string) (ConnectionRecord, error) {
	var c ConnectionRecord
	var readOnly int
	err := p.db.QueryRow(
		`SELECT id, name, type, dsn, read_only, created_at
		 FROM saved_connections WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Type, &c.DSN, &readOnly, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return ConnectionRecord{}, fmt.Errorf("connection %q not found", id)
	}
	c.ReadOnly = readOnly != 0
	return c, err
}

func (p *PersistenceDB) deleteConnection(id string) error {
	_, err := p.db.Exec(`DELETE FROM saved_connections WHERE id = ?`, id)
	return err
}

// removeTableData deletes the SQLite record and returns the parquet path so the
// caller can remove the file.
func (p *PersistenceDB) removeTableData(tableName string) (string, error) {
	var path string
	err := p.db.QueryRow(`SELECT parquet_path FROM table_data WHERE table_name = ?`, tableName).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	_, err = p.db.Exec(`DELETE FROM table_data WHERE table_name = ?`, tableName)
	return path, err
}
