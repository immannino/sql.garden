package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// mcpNodeEntry is a lightweight record of a node added via MCP/AI this session,
// kept in memory so list_canvas_nodes doesn't depend on the frontend auto-save.
type mcpNodeEntry struct {
	ID   string
	Name string
	Kind string // "query" | "chart" | "markdown" | "table"
}

// App holds all application state. Every exported method becomes a callable
// binding in the frontend via the Wails runtime.
type App struct {
	ctx             context.Context
	duck            *sql.DB
	persist         *PersistenceDB
	mcpSrv          *mcpServer
	mcpNodeRegistry sync.Map // id → mcpNodeEntry
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

	pdb, err := newPersistenceDB()
	if err != nil {
		// Non-fatal: app runs fine without persistence (data won't survive restart).
		fmt.Printf("sql.garden: persistence unavailable: %v\n", err)
	} else {
		a.persist = pdb
	}

	go a.startMCPServer()

	runtime.MenuSetApplicationMenu(ctx, a.buildMenu())
	runtime.MenuUpdateApplicationMenu(ctx)
}

func (a *App) emit(event string) func(*menu.CallbackData) {
	return func(_ *menu.CallbackData) { runtime.EventsEmit(a.ctx, event) }
}

func (a *App) buildMenu() *menu.Menu {
	m := menu.NewMenu()

	// ── File ──────────────────────────────────────────────────────────────────
	file := m.AddSubmenu("File")
	file.AddText("Add Query", keys.Combo("q", keys.CmdOrCtrlKey, keys.OptionOrAltKey), a.emit("menu:add-query"))
	file.AddText("Add Chart", keys.Combo("c", keys.CmdOrCtrlKey, keys.OptionOrAltKey), a.emit("menu:add-chart"))
	file.AddText("Add Note", keys.Combo("n", keys.CmdOrCtrlKey, keys.OptionOrAltKey), a.emit("menu:add-note"))
	file.AddText("Add Section", keys.Combo("s", keys.CmdOrCtrlKey, keys.OptionOrAltKey), a.emit("menu:add-section"))
	file.AddSeparator()
	file.AddText("Import…", keys.CmdOrCtrl("i"), a.emit("menu:import"))

	// ── Edit ─────────────────────────────────────────────────────────────────
	// nil callbacks let macOS route these through the WKWebView responder chain
	// so standard clipboard shortcuts work inside text fields and code editors.
	edit := m.AddSubmenu("Edit")
	edit.AddText("Undo", keys.CmdOrCtrl("z"), nil)
	edit.AddText("Redo", keys.Combo("z", keys.CmdOrCtrlKey, keys.ShiftKey), nil)
	edit.AddSeparator()
	edit.AddText("Cut", keys.CmdOrCtrl("x"), nil)
	edit.AddText("Copy", keys.CmdOrCtrl("c"), nil)
	edit.AddText("Paste", keys.CmdOrCtrl("v"), nil)
	edit.AddSeparator()
	edit.AddText("Select All", keys.CmdOrCtrl("a"), nil)

	// ── View ──────────────────────────────────────────────────────────────────
	view := m.AddSubmenu("View")
	view.AddText("Fit View", keys.Combo("f", keys.CmdOrCtrlKey, keys.ShiftKey), a.emit("menu:fit-view"))
	view.AddSeparator()
	view.AddText("Zoom In", keys.CmdOrCtrl("="), a.emit("menu:zoom-in"))
	view.AddText("Zoom Out", keys.CmdOrCtrl("-"), a.emit("menu:zoom-out"))
	view.AddText("Actual Size", keys.CmdOrCtrl("0"), a.emit("menu:zoom-reset"))
	view.AddSeparator()
	view.AddText("Toggle Layers", keys.Combo("l", keys.CmdOrCtrlKey, keys.ShiftKey), a.emit("menu:toggle-layers"))
	view.AddText("Toggle Query Panel", keys.Combo("p", keys.CmdOrCtrlKey, keys.ShiftKey), a.emit("menu:toggle-query"))

	// ── Help ──────────────────────────────────────────────────────────────────
	help := m.AddSubmenu("Help")
	help.AddText("Keyboard Shortcuts", keys.CmdOrCtrl("/"), a.emit("menu:shortcuts"))

	return m
}

func (a *App) shutdown(_ context.Context) {
	if a.mcpSrv != nil && a.mcpSrv.srv != nil {
		a.mcpSrv.srv.Close()
	}
	if a.persist != nil {
		a.persist.close()
	}
	if a.duck != nil {
		a.duck.Close()
	}
}

func (a *App) GetMCPPort() int { return MCPPort }

// ── Persistence ───────────────────────────────────────────────────────────────

// SaveCanvasState replaces the saved canvas node list with the given JSON blob.
func (a *App) SaveCanvasState(nodesJSON string) error {
	if a.persist == nil {
		return nil
	}
	return a.persist.saveCanvasState(nodesJSON)
}

// LoadCanvasState returns the previously saved canvas JSON, or "" on first run.
func (a *App) LoadCanvasState() (string, error) {
	if a.persist == nil {
		return "", nil
	}
	return a.persist.loadCanvasState()
}

// SaveTableData copies a DuckDB table to a parquet file and records its path.
func (a *App) SaveTableData(tableName string) error {
	if a.persist == nil {
		return nil
	}
	path := a.persist.parquetPath(tableName)
	safeTable := escapeDoubleQuote(tableName)
	safePath := escapeSingleQuote(path)
	if _, err := a.duck.ExecContext(a.ctx,
		fmt.Sprintf(`COPY "%s" TO '%s' (FORMAT PARQUET)`, safeTable, safePath),
	); err != nil {
		return err
	}
	return a.persist.recordTableData(tableName, path)
}

// GetTableDataPath returns the parquet file path for a table, or "" if unknown.
func (a *App) GetTableDataPath(tableName string) (string, error) {
	if a.persist == nil {
		return "", nil
	}
	return a.persist.getTableDataPath(tableName)
}

// ── Connection profiles ───────────────────────────────────────────────────────

// SaveConnection creates or updates a saved connection profile.
func (a *App) SaveConnection(conn ConnectionRecord) (ConnectionRecord, error) {
	if a.persist == nil {
		return conn, nil
	}
	return a.persist.saveConnection(conn)
}

// ListConnections returns all saved connection profiles ordered by creation time.
func (a *App) ListConnections() ([]ConnectionRecord, error) {
	if a.persist == nil {
		return nil, nil
	}
	return a.persist.listConnections()
}

// DeleteConnection removes a saved connection profile by ID.
func (a *App) DeleteConnection(id string) error {
	if a.persist == nil {
		return nil
	}
	return a.persist.deleteConnection(id)
}

// AutoConnectResult is returned per-connection by AutoConnectAll.
type AutoConnectResult struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Alias string `json:"alias"`
	Error string `json:"error,omitempty"`
}

// AutoConnectAll tries to connect every saved connection on startup.
// Failures are reported per-entry rather than aborting the whole list.
func (a *App) AutoConnectAll() []AutoConnectResult {
	if a.persist == nil {
		return nil
	}
	conns, err := a.persist.listConnections()
	if err != nil || len(conns) == 0 {
		return nil
	}
	results := make([]AutoConnectResult, 0, len(conns))
	for _, conn := range conns {
		alias, err := a.ConnectSaved(conn.ID)
		r := AutoConnectResult{ID: conn.ID, Name: conn.Name, Alias: alias}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// ── SQL Views ─────────────────────────────────────────────────────────────────

// CreateView creates or replaces a named DuckDB VIEW over the given SQL.
func (a *App) CreateView(name, sql string) error {
	_, err := a.duck.ExecContext(a.ctx,
		fmt.Sprintf(`CREATE OR REPLACE VIEW "%s" AS %s`, escapeDoubleQuote(name), sql))
	return err
}

// DropView removes a named VIEW if it exists.
func (a *App) DropView(name string) error {
	_, err := a.duck.ExecContext(a.ctx,
		fmt.Sprintf(`DROP VIEW IF EXISTS "%s"`, escapeDoubleQuote(name)))
	return err
}

// ConnectSaved loads any required extension and ATTACHes the saved connection.
// Returns the alias used so the frontend can reference it in SQL.
func (a *App) ConnectSaved(id string) (string, error) {
	if a.persist == nil {
		return "", fmt.Errorf("persistence not available")
	}
	conn, err := a.persist.getConnection(id)
	if err != nil {
		return "", err
	}

	switch strings.ToLower(conn.Type) {
	case "postgres":
		if err := a.LoadExtension("postgres"); err != nil {
			return "", fmt.Errorf("loading postgres extension: %w", err)
		}
	case "mysql":
		if err := a.LoadExtension("mysql"); err != nil {
			return "", fmt.Errorf("loading mysql extension: %w", err)
		}
	}

	alias := sanitizeAlias(conn.Name)
	dbType := strings.ToUpper(conn.Type)
	if dbType == "DUCKDB" {
		dbType = "" // native attach, no TYPE clause
	}

	dsn := conn.DSN
	if strings.EqualFold(conn.Type, "postgres") {
		dsn = postgresURLToKV(dsn)
	}

	// Best-effort detach before re-attaching. Ignore "not found" and "already
	// exists" — AttachDatabase handles the latter if DETACH couldn't run.
	_, _ = a.duck.ExecContext(a.ctx, fmt.Sprintf(`DETACH "%s"`, escapeDoubleQuote(alias)))

	if err := a.AttachDatabase(AttachOptions{
		Alias:    alias,
		DSN:      dsn,
		Type:     dbType,
		ReadOnly: conn.ReadOnly,
	}); err != nil {
		return "", err
	}
	return alias, nil
}

// DisconnectSaved DETACHes a previously connected saved connection.
func (a *App) DisconnectSaved(id string) error {
	if a.persist == nil {
		return nil
	}
	conn, err := a.persist.getConnection(id)
	if err != nil {
		return err
	}
	return a.DetachDatabase(sanitizeAlias(conn.Name))
}

// ── Schema explorer ───────────────────────────────────────────────────────────

type SchemaInfo struct {
	Name string `json:"name"`
}

type TableInfo struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "table" or "view"
}

type ColumnMeta struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

func (a *App) GetDatabaseSchemas(alias string) ([]SchemaInfo, error) {
	rows, err := a.duck.QueryContext(a.ctx,
		`SELECT schema_name FROM duckdb_schemas()
		 WHERE database_name = ? AND internal = false
		 ORDER BY schema_name`,
		alias,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SchemaInfo
	for rows.Next() {
		var s SchemaInfo
		if err := rows.Scan(&s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (a *App) GetSchemaTables(alias, schema string) ([]TableInfo, error) {
	rows, err := a.duck.QueryContext(a.ctx,
		`SELECT table_name AS name, 'table' AS kind FROM duckdb_tables()
		 WHERE database_name = ? AND schema_name = ? AND internal = false
		 UNION ALL
		 SELECT view_name AS name, 'view' AS kind FROM duckdb_views()
		 WHERE database_name = ? AND schema_name = ? AND internal = false
		 ORDER BY kind, name`,
		alias, schema, alias, schema,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Kind); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (a *App) GetTableColumns(alias, schema, table string) ([]ColumnMeta, error) {
	rows, err := a.duck.QueryContext(a.ctx,
		`SELECT column_name, data_type, is_nullable::BOOLEAN
		 FROM duckdb_columns()
		 WHERE database_name = ? AND schema_name = ? AND table_name = ?
		 ORDER BY column_index`,
		alias, schema, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ColumnMeta
	for rows.Next() {
		var c ColumnMeta
		if err := rows.Scan(&c.Name, &c.Type, &c.Nullable); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// sanitizeAlias converts a connection name to a valid DuckDB identifier.
func sanitizeAlias(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, strings.ToLower(name))
	if result == "" {
		return "connection"
	}
	if result[0] >= '0' && result[0] <= '9' {
		result = "db_" + result
	}
	return result
}

// ── App settings ─────────────────────────────────────────────────────────────

type AppSettings struct {
	Theme string `json:"theme"` // "dark" | "light" | "system"
}

const appSettingsKey = "app_settings"

func (a *App) GetAppSettings() (AppSettings, error) {
	if a.persist == nil {
		return AppSettings{Theme: "system"}, nil
	}
	raw, err := a.persist.getSetting(appSettingsKey)
	if err != nil || raw == "" {
		return AppSettings{Theme: "system"}, nil
	}
	var s AppSettings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return AppSettings{Theme: "system"}, nil
	}
	return s, nil
}

func (a *App) SaveAppSettings(s AppSettings) error {
	if a.persist == nil {
		return nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return a.persist.saveSetting(appSettingsKey, string(raw))
}

const appVersion = "v0.0.0-alpha.5"

// GetAppVersion returns the current application version string.
func (a *App) GetAppVersion() string { return appVersion }

// UpdateInfo is the result of CheckForUpdate.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseURL     string `json:"releaseURL"`
}

// CheckForUpdate queries the GitHub releases API and returns version info.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	const repo = "immannino/sql.garden"
	info := UpdateInfo{
		CurrentVersion: appVersion,
		ReleaseURL:     "https://github.com/" + repo + "/releases",
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet,
		"https://api.github.com/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases published yet
		return info, nil
	}
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return info, err
	}

	latest := strings.TrimPrefix(payload.TagName, "v")
	info.LatestVersion = latest
	if payload.HTMLURL != "" {
		info.ReleaseURL = payload.HTMLURL
	}
	info.HasUpdate = isNewerVersion(latest, strings.TrimPrefix(appVersion, "v"))
	return info, nil
}

// MCPConfigStatus describes whether the Claude Desktop MCP config already
// contains the sql-garden entry, and where the config file lives.
type MCPConfigStatus struct {
	Found      bool   `json:"found"`
	Path       string `json:"path"`
	Configured bool   `json:"configured"`
}

// GetMCPConfigStatus detects the Claude Desktop config file and reports
// whether sql-garden is already registered in it.
func (a *App) GetMCPConfigStatus() MCPConfigStatus {
	path, err := claudeDesktopConfigPath()
	if err != nil {
		return MCPConfigStatus{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPConfigStatus{Found: true, Path: path}
	}
	return MCPConfigStatus{
		Found:      true,
		Path:       path,
		Configured: strings.Contains(string(data), "sql-garden"),
	}
}

// WriteMCPToClaudeDesktop merges the sql-garden MCP entry into the Claude
// Desktop config file, creating it if it does not yet exist.
func (a *App) WriteMCPToClaudeDesktop() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	path, pathErr := claudeDesktopConfigPath()
	if pathErr != nil {
		// File doesn't exist yet — create at the standard macOS location.
		path = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var cfg map[string]interface{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	servers, _ := cfg["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}
	servers["sql-garden"] = map[string]interface{}{
		"url": fmt.Sprintf("http://127.0.0.1:%d/mcp", MCPPort),
	}
	cfg["mcpServers"] = servers

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func claudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
		filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json"),
		filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),
	}
	for _, p := range candidates {
		if p == filepath.Join("", "Claude", "claude_desktop_config.json") {
			continue // skip empty APPDATA
		}
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("claude_desktop_config.json not found")
}

// isNewerVersion returns true if candidate is strictly newer than current.
// Compares major.minor.patch numerically; ignores pre-release suffixes.
func isNewerVersion(candidate, current string) bool {
	parse := func(v string) [3]int {
		// strip pre-release suffix (e.g. "-beta")
		if i := strings.IndexByte(v, '-'); i >= 0 {
			v = v[:i]
		}
		parts := strings.SplitN(v, ".", 3)
		var out [3]int
		for i, p := range parts {
			if i >= 3 {
				break
			}
			n, _ := strconv.Atoi(p)
			out[i] = n
		}
		return out
	}
	c, cur := parse(candidate), parse(current)
	for i := range c {
		if c[i] > cur[i] {
			return true
		}
		if c[i] < cur[i] {
			return false
		}
	}
	return false
}

// DeleteTableData removes a table's parquet snapshot and its SQLite record.
func (a *App) DeleteTableData(tableName string) error {
	if a.persist == nil {
		return nil
	}
	path, err := a.persist.removeTableData(tableName)
	if err != nil {
		return err
	}
	if path != "" {
		os.Remove(path) //nolint:errcheck
	}
	return nil
}

// ── Types shared with the frontend ───────────────────────────────────────────

type QueryResult struct {
	Columns     []string         `json:"columns"`
	ColumnTypes []string         `json:"columnTypes"`
	Rows        []map[string]any `json:"rows"`
	RowCount    int              `json:"rowCount"`
	DurationMs  float64          `json:"durationMs"`
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

	result := make([]map[string]any, 0)
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

	colTypes, _ := rows.ColumnTypes()
	typeNames := make([]string, len(colTypes))
	for i, ct := range colTypes {
		typeNames[i] = ct.DatabaseTypeName()
	}

	return QueryResult{
		Columns:     cols,
		ColumnTypes: typeNames,
		Rows:        result,
		RowCount:    len(result),
		DurationMs:  float64(time.Since(start).Microseconds()) / 1000,
	}, nil
}

// ClipboardGet returns the current clipboard text content.
func (a *App) ClipboardGet() string {
	text, _ := runtime.ClipboardGetText(a.ctx)
	return text
}

// ClipboardSet writes text to the system clipboard.
func (a *App) ClipboardSet(text string) {
	_ = runtime.ClipboardSetText(a.ctx, text)
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
		var notnull bool
		var dflt any
		var pk bool
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:       name,
			Type:       typ,
			PrimaryKey: pk,
			Nullable:   !notnull,
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
// postgresURLToKV converts a postgres:// or postgresql:// URL into libpq key=value
// format, which is what DuckDB's postgres extension reliably accepts.
// DSNs already in key=value format are returned unchanged.
func postgresURLToKV(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "postgres" && scheme != "postgresql" {
		return dsn
	}

	var kv []string
	if h := u.Hostname(); h != "" {
		kv = append(kv, "host="+pgKVVal(h))
	}
	if p := u.Port(); p != "" {
		kv = append(kv, "port="+p)
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		kv = append(kv, "dbname="+pgKVVal(db))
	}
	if u.User != nil {
		if user := u.User.Username(); user != "" {
			kv = append(kv, "user="+pgKVVal(user))
		}
		if pass, ok := u.User.Password(); ok {
			kv = append(kv, "password="+pgKVVal(pass))
		}
	}
	for k, vals := range u.Query() {
		if len(vals) > 0 {
			kv = append(kv, k+"="+pgKVVal(vals[0]))
		}
	}
	return strings.Join(kv, " ")
}

// pgKVVal quotes a libpq key=value value if it contains whitespace or quotes.
// Single quotes and backslashes are backslash-escaped per the libpq spec.
func pgKVVal(s string) string {
	if !strings.ContainsAny(s, " \t'\\") {
		return s
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

// DuckDB can ATTACH external databases — Postgres, MySQL, SQLite — natively.
// The frontend passes a connection string; we attach it under a user-chosen alias.

type AttachOptions struct {
	Alias    string `json:"alias"` // name used in SQL, e.g. "prod"
	DSN      string `json:"dsn"`   // e.g. "dbname=mydb host=localhost"
	Type     string `json:"type"`  // "postgres", "sqlite", "mysql", ""
	ReadOnly bool   `json:"readOnly"`
}

// AttachDatabase ATTACHes an external database into DuckDB so its tables are
// queryable alongside local DuckDB tables.
func (a *App) AttachDatabase(opts AttachOptions) error {
	// Build options clause — DuckDB requires all options inside one (…) block.
	var optParts []string
	if opts.Type != "" {
		optParts = append(optParts, "TYPE "+opts.Type)
	}
	if opts.ReadOnly {
		optParts = append(optParts, "READ_ONLY")
	}
	optClause := ""
	if len(optParts) > 0 {
		optClause = " (" + strings.Join(optParts, ", ") + ")"
	}
	stmt := fmt.Sprintf(
		"ATTACH '%s' AS \"%s\"%s",
		escapeSingleQuote(opts.DSN),
		escapeDoubleQuote(opts.Alias),
		optClause,
	)
	_, err := a.duck.ExecContext(a.ctx, stmt)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		// The alias is already attached (e.g. DETACH couldn't run because of open
		// pool connections, or AutoConnectAll ran twice on hot-reload). The database
		// is accessible, so treat this as success.
		return nil
	}
	return err
}

// DetachDatabase removes an attached database. Returns nil if the alias was
// never attached — DETACH IF EXISTS is not supported in this DuckDB version.
func (a *App) DetachDatabase(alias string) error {
	_, err := a.duck.ExecContext(a.ctx,
		fmt.Sprintf(`DETACH "%s"`, escapeDoubleQuote(alias)),
	)
	if err != nil && (strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "does not exist") ||
		strings.Contains(err.Error(), "not open")) {
		return nil
	}
	return err
}

// ListAttachedDatabases returns the databases currently attached to DuckDB.
func (a *App) ListAttachedDatabases() (QueryResult, error) {
	return a.Query("SELECT database_name, path, type FROM duckdb_databases() WHERE database_name != 'memory' ORDER BY database_name")
}

// ── File import ──────────────────────────────────────────────────────────────
// The browser can read a File's bytes; we receive them as base64, write to a
// temp file on the real filesystem, and let DuckDB read it natively.

func (a *App) ImportFileFromBase64(b64data, origFilename, tableName string) error {
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return fmt.Errorf("decoding file data: %w", err)
	}

	ext := filepath.Ext(origFilename)
	tmp, err := os.CreateTemp("", "sqgarden_*"+ext)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	readFn := readFnForPath(tmpPath, origFilename)
	safeTable := escapeDoubleQuote(tableName)
	_, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(`CREATE TABLE "%s" AS SELECT * FROM %s`, safeTable, readFn))
	return err
}

// ImportFromPath reads a file already on the local filesystem directly into DuckDB.
func (a *App) ImportFromPath(filePath, tableName string) error {
	readFn := readFnForPath(filePath, filePath)
	safeTable := escapeDoubleQuote(tableName)
	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`CREATE TABLE "%s" AS SELECT * FROM %s`, safeTable, readFn,
	)); err != nil {
		if !isCsvPath(filePath) {
			return err
		}
		// Retry with all_varchar=true for the same DuckDB POINT-type false-positive.
		_, err2 := a.duck.ExecContext(a.ctx, fmt.Sprintf(
			`CREATE TABLE "%s" AS SELECT * FROM read_csv_auto('%s', all_varchar=true)`,
			safeTable, escapeSingleQuote(filePath),
		))
		return err2
	}
	return nil
}

// ImportFromUrl downloads a file over HTTP/HTTPS into a temp file and imports it.
// The file extension is inferred from the URL path; Content-Type is used as a
// fallback. This avoids the DuckDB httpfs extension entirely for plain HTTP URLs.
func (a *App) ImportFromUrl(rawURL, tableName string) error {
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "sql.garden/1.0")
	req.Header.Set("Accept", "text/csv,application/octet-stream,application/json,*/*")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") {
		return fmt.Errorf("server returned HTML instead of data (Content-Type: %s) — the URL may require login or JavaScript", ct)
	}

	// Derive extension from the URL path before query string / fragment.
	// Fall back to Content-Type when the path has no recognisable extension.
	urlPath := rawURL
	if i := strings.IndexAny(rawURL, "?#"); i >= 0 {
		urlPath = rawURL[:i]
	}
	ext := strings.ToLower(filepath.Ext(urlPath))
	if ext == "" || ext == "." {
		switch {
		case strings.Contains(ct, "parquet"):
			ext = ".parquet"
		case strings.Contains(ct, "json"):
			ext = ".json"
		default:
			ext = ".csv"
		}
	}

	tmp, err := os.CreateTemp("", "sqgarden_url_*"+ext)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("downloading: %w", err)
	}
	tmp.Close()

	readFn := readFnForPath(tmpPath, tmpPath)
	safeTable := escapeDoubleQuote(tableName)
	if _, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`CREATE TABLE "%s" AS SELECT * FROM %s`, safeTable, readFn,
	)); err != nil && isCsvPath(tmpPath) {
		// DuckDB's auto_detect sometimes misidentifies "(lat, lon)" coordinate
		// strings as PostgreSQL POINT types, which require the postgres extension.
		// Retry with all_varchar=true to skip type inference entirely.
		_, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(
			`CREATE TABLE "%s" AS SELECT * FROM read_csv_auto('%s', all_varchar=true)`,
			safeTable, escapeSingleQuote(tmpPath),
		))
	}
	return err
}

// ImportSqliteFromPath imports all user tables from a SQLite file already on disk.
// prefix is prepended to each table name (e.g. "crm_" → "crm_users"). Empty = no prefix.
// Returns the DuckDB table names created.
func (a *App) ImportSqliteFromPath(filePath, prefix string) ([]string, error) {
	alias := fmt.Sprintf("_sqg_%d", time.Now().UnixNano())
	safePath := escapeSingleQuote(filePath)
	safeAlias := escapeDoubleQuote(alias)

	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`ATTACH '%s' AS "%s" (TYPE SQLITE, READ_ONLY)`, safePath, safeAlias,
	)); err != nil {
		return nil, fmt.Errorf("attaching SQLite DB: %w", err)
	}
	defer a.duck.ExecContext(a.ctx, fmt.Sprintf(`DETACH "%s"`, safeAlias)) //nolint:errcheck

	tableRows, err := a.duck.QueryContext(a.ctx, fmt.Sprintf(
		`SELECT table_name FROM information_schema.tables WHERE table_catalog = '%s' AND table_schema = 'main' ORDER BY table_name`,
		escapeSingleQuote(alias),
	))
	if err != nil {
		return nil, fmt.Errorf("listing SQLite tables: %w", err)
	}
	var srcTables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			tableRows.Close()
			return nil, err
		}
		srcTables = append(srcTables, name)
	}
	tableRows.Close()
	if err := tableRows.Err(); err != nil {
		return nil, err
	}

	var created []string
	for _, tbl := range srcTables {
		base := tbl
		if prefix != "" {
			base = prefix + tbl
		}
		target, err := a.uniqueTableName(base)
		if err != nil {
			continue
		}
		src := fmt.Sprintf(`"%s"."%s"`, safeAlias, escapeDoubleQuote(tbl))
		if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(
			`CREATE TABLE "%s" AS SELECT * FROM %s`, escapeDoubleQuote(target), src,
		)); err != nil {
			continue
		}
		created = append(created, target)
	}
	return created, nil
}

// OpenFileDialog shows the native file picker for a single file.
func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open data file",
		Filters: []runtime.FileFilter{
			{DisplayName: "Data files", Pattern: "*.csv;*.tsv;*.parquet;*.json;*.jsonl;*.sqlite;*.db"},
		},
	})
}

// OpenMultipleFilesDialog shows the native file picker allowing multiple selection.
func (a *App) OpenMultipleFilesDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open data files",
		Filters: []runtime.FileFilter{
			{DisplayName: "Data files", Pattern: "*.csv;*.tsv;*.parquet;*.json;*.jsonl;*.sqlite;*.db"},
		},
	})
}

// SaveFileWithDialog shows the native save-file dialog pre-filled with the
// suggested filename, then writes content to the chosen path.
// Returns the path written to, or an empty string if the user cancelled.
func (a *App) SaveFileWithDialog(suggestedName, content string) (string, error) {
	ext := strings.TrimPrefix(filepath.Ext(suggestedName), ".")

	filterMap := map[string]runtime.FileFilter{
		"csv":  {DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		"tsv":  {DisplayName: "TSV (*.tsv)", Pattern: "*.tsv"},
		"json": {DisplayName: "JSON (*.json)", Pattern: "*.json"},
		"md":   {DisplayName: "Markdown (*.md)", Pattern: "*.md"},
	}
	filters := []runtime.FileFilter{}
	if f, ok := filterMap[ext]; ok {
		filters = append(filters, f)
	}
	filters = append(filters, runtime.FileFilter{DisplayName: "All files", Pattern: "*.*"})

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save file",
		DefaultFilename: suggestedName,
		Filters:         filters,
	})
	if err != nil || path == "" {
		return "", err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}
	return path, nil
}

// ImportSqliteFromBase64 imports all user tables from a SQLite database file.
// Native DuckDB (non-WASM) can ATTACH SQLite files directly.
// Returns the DuckDB table names that were created.
func (a *App) ImportSqliteFromBase64(b64data, origFilename string) ([]string, error) {
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return nil, fmt.Errorf("decoding file data: %w", err)
	}

	ext := filepath.Ext(origFilename)
	tmp, err := os.CreateTemp("", "sqgarden_*"+ext)
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()

	alias := fmt.Sprintf("_sqg_%d", time.Now().UnixNano())
	safePath := escapeSingleQuote(tmpPath)
	safeAlias := escapeDoubleQuote(alias)

	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`ATTACH '%s' AS "%s" (TYPE SQLITE, READ_ONLY)`, safePath, safeAlias,
	)); err != nil {
		return nil, fmt.Errorf("attaching SQLite DB: %w", err)
	}
	defer a.duck.ExecContext(a.ctx, fmt.Sprintf(`DETACH "%s"`, safeAlias)) //nolint:errcheck

	// List user tables from the attached database
	tableRows, err := a.duck.QueryContext(a.ctx, fmt.Sprintf(
		`SELECT table_name FROM information_schema.tables WHERE table_catalog = '%s' AND table_schema = 'main' ORDER BY table_name`,
		escapeSingleQuote(alias),
	))
	if err != nil {
		return nil, fmt.Errorf("listing SQLite tables: %w", err)
	}
	var srcTables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			tableRows.Close()
			return nil, err
		}
		srcTables = append(srcTables, name)
	}
	tableRows.Close()
	if err := tableRows.Err(); err != nil {
		return nil, err
	}

	var created []string
	for _, tbl := range srcTables {
		target, err := a.uniqueTableName(tbl)
		if err != nil {
			continue
		}
		src := fmt.Sprintf(`"%s"."%s"`, safeAlias, escapeDoubleQuote(tbl))
		if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(
			`CREATE TABLE "%s" AS SELECT * FROM %s`, escapeDoubleQuote(target), src,
		)); err != nil {
			continue
		}
		created = append(created, target)
	}
	return created, nil
}

// uniqueTableName returns a name not already used in the main DuckDB schema.
func (a *App) uniqueTableName(base string) (string, error) {
	name := base
	for i := 1; i <= 100; i++ {
		rows, err := a.duck.QueryContext(a.ctx, fmt.Sprintf(
			`SELECT 1 FROM information_schema.tables WHERE table_schema = 'main' AND table_name = '%s' LIMIT 1`,
			escapeSingleQuote(name),
		))
		if err != nil {
			return "", err
		}
		exists := rows.Next()
		rows.Close()
		if !exists {
			return name, nil
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
	return "", fmt.Errorf("could not find unique name for %q after 100 attempts", base)
}

// CopyTableToParquet serialises a DuckDB table to Parquet and returns it
// base64-encoded. Used by usePersistence to save table data to IndexedDB
// (same as the web version) until the SQLite persistence layer is wired up.
func (a *App) CopyTableToParquet(tableName string) (string, error) {
	tmp, err := os.CreateTemp("", "sqgarden_export_*.parquet")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	safeTable := escapeDoubleQuote(tableName)
	safePath := escapeSingleQuote(tmpPath)
	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(`COPY "%s" TO '%s' (FORMAT PARQUET)`, safeTable, safePath)); err != nil {
		return "", err
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
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

func isCsvPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".tsv", ".txt", "":
		return true
	}
	return false
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

// readFnForPath returns the DuckDB read function call for a given file path,
// using the original filename's extension to pick the right reader.
func readFnForPath(tmpPath, origName string) string {
	ext := filepath.Ext(origName)
	safePath := escapeSingleQuote(tmpPath)
	switch ext {
	case ".parquet":
		return fmt.Sprintf("read_parquet('%s')", safePath)
	case ".json":
		return fmt.Sprintf("read_json_auto('%s')", safePath)
	case ".jsonl", ".ndjson":
		return fmt.Sprintf("read_json_auto('%s', format='newline_delimited')", safePath)
	case ".tsv":
		return fmt.Sprintf("read_csv_auto('%s', delim='\t')", safePath)
	default: // .csv, .txt, and anything else
		return fmt.Sprintf("read_csv_auto('%s')", safePath)
	}
}
