package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// App holds all application state. Every exported method becomes a callable
// binding in the frontend via the Wails runtime.
type App struct {
	ctx  context.Context
	duck *sql.DB
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	db, err := sql.Open("duckdb", "")
	if err != nil {
		panic(fmt.Sprintf("failed to open DuckDB: %v", err))
	}
	db.SetMaxOpenConns(1) // DuckDB is single-writer
	a.duck = db
}

func (a *App) shutdown(_ context.Context) {
	if a.duck != nil {
		a.duck.Close()
	}
}

// ── Types shared with the frontend ───────────────────────────────────────────

type QueryResult struct {
	Columns  []string         `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"rowCount"`
	DurationMs float64        `json:"durationMs"`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	PrimaryKey bool   `json:"primaryKey"`
	Nullable   bool   `json:"nullable"`
}

// ── Core query methods ────────────────────────────────────────────────────────

// Query executes a SELECT and returns typed rows.
func (a *App) Query(query string) (QueryResult, error) {
	start := time.Now()
	rows, err := a.duck.QueryContext(a.ctx, query)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return QueryResult{}, err
	}

	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return QueryResult{}, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeValue(vals[i])
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, err
	}

	return QueryResult{
		Columns:    cols,
		Rows:       result,
		RowCount:   len(result),
		DurationMs: float64(time.Since(start).Microseconds()) / 1000,
	}, nil
}

// Exec runs a statement that returns no rows (CREATE, INSERT, DROP, etc.).
func (a *App) Exec(query string) error {
	_, err := a.duck.ExecContext(a.ctx, query)
	return err
}

// GetTableInfo returns column metadata for a table (mirrors PRAGMA table_info).
func (a *App) GetTableInfo(tableName string) ([]ColumnInfo, error) {
	rows, err := a.duck.QueryContext(a.ctx,
		fmt.Sprintf("PRAGMA table_info('%s')", escapeSingleQuote(tableName)),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:       name,
			Type:       typ,
			PrimaryKey: pk == 1,
			Nullable:   notnull == 0,
		})
	}
	return cols, rows.Err()
}

// LoadExtension installs and loads a DuckDB extension by name.
func (a *App) LoadExtension(name string) error {
	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf("LOAD '%s'", name)); err != nil {
		if _, err2 := a.duck.ExecContext(a.ctx, fmt.Sprintf("INSTALL '%s'", name)); err2 != nil {
			return err2
		}
		if _, err2 := a.duck.ExecContext(a.ctx, fmt.Sprintf("LOAD '%s'", name)); err2 != nil {
			return err2
		}
	}
	return nil
}

// ── External database connections ─────────────────────────────────────────────
// DuckDB can ATTACH external databases — Postgres, MySQL, SQLite — natively.
// The frontend passes a connection string; we attach it under a user-chosen alias.

type AttachOptions struct {
	Alias    string `json:"alias"`    // name used in SQL, e.g. "prod"
	DSN      string `json:"dsn"`      // e.g. "dbname=mydb host=localhost"
	Type     string `json:"type"`     // "postgres", "sqlite", "mysql", ""
	ReadOnly bool   `json:"readOnly"`
}

// AttachDatabase ATTACHes an external database into DuckDB so its tables are
// queryable alongside local DuckDB tables.
func (a *App) AttachDatabase(opts AttachOptions) error {
	typeClause := ""
	if opts.Type != "" {
		typeClause = fmt.Sprintf(" (TYPE %s)", opts.Type)
	}
	readOnlyClause := ""
	if opts.ReadOnly {
		readOnlyClause = " READ_ONLY"
	}
	stmt := fmt.Sprintf(
		"ATTACH '%s' AS \"%s\"%s%s",
		escapeSingleQuote(opts.DSN),
		escapeDoubleQuote(opts.Alias),
		typeClause,
		readOnlyClause,
	)
	_, err := a.duck.ExecContext(a.ctx, stmt)
	return err
}

// DetachDatabase removes an attached database.
func (a *App) DetachDatabase(alias string) error {
	_, err := a.duck.ExecContext(a.ctx,
		fmt.Sprintf("DETACH \"%s\"", escapeDoubleQuote(alias)),
	)
	return err
}

// ListAttachedDatabases returns the databases currently attached to DuckDB.
func (a *App) ListAttachedDatabases() (QueryResult, error) {
	return a.Query("SELECT database_name, path, type FROM duckdb_databases() WHERE database_name != 'memory' ORDER BY database_name")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func normalizeValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		// Try JSON first, fall back to string
		var j any
		if json.Unmarshal(val, &j) == nil {
			return j
		}
		return string(val)
	case time.Time:
		return val.Format(time.RFC3339Nano)
	default:
		return val
	}
}

func escapeSingleQuote(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			result = append(result, '\'', '\'')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}

func escapeDoubleQuote(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			result = append(result, '"', '"')
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}
