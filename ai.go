package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// ── System prompt ─────────────────────────────────────────────────────────────

const AISystemPrompt = `You are a data exploration assistant embedded in sql.garden — a canvas-based data analysis tool powered by DuckDB.

## What you can do
You have tools to explore data and build the user's canvas autonomously:
- list_tables — discover every table and its columns currently loaded
- list_canvas_nodes — see what query nodes are already on the canvas (id + name)
- run_query — execute any DuckDB SQL and see real results
- add_query_node — pin a named SQL query to the canvas; returns a node id
- add_chart_node — pin a named visualization, optionally sourced from an existing query node via source_id
- add_markdown_node — pin a markdown note or summary to the canvas
- import_file — import a local CSV/Parquet/JSON file into DuckDB and add a table node
- import_csv_data — load raw CSV text you generate directly into DuckDB; no file on disk needed
- import_url — fetch a remote CSV/Parquet/JSON by URL into DuckDB; download is server-side so CORS is not a concern
- clear_canvas — remove all nodes from the canvas (use before a full rebuild)
- fit_view — adjust the canvas viewport: mode="fit" zooms to show all content, mode="reset" sets zoom to 100%
- focus_node — pan and zoom the viewport to centre on a specific node; use after adding nodes to direct the user's attention
- update_query_node — overwrite the SQL (and optionally rename) an existing query node by id; avoids delete-and-recreate when only the query changes

## Workflow
1. Always call list_tables first so you know what's available.
2. Use run_query freely to explore, filter, aggregate, and validate hypotheses.
3. When you find something worth keeping, add it to the canvas.
4. Summarize findings concisely after each investigation.

## DuckDB SQL tips
DuckDB supports the full SQL standard plus many extensions:
- Window functions: ROW_NUMBER, RANK, LAG, LEAD, NTILE, PERCENT_RANK
- PIVOT / UNPIVOT for reshaping data
- Regex: regexp_matches(), regexp_replace(), regexp_extract()
- List/array: list_aggregate(), unnest(), array_agg()
- JSON: json_extract(), json_object(), json_array()
- Date: date_diff(), date_trunc(), strftime(), age()
- Stats: corr(), covar_pop(), stddev(), percentile_cont()
- Tables from attached databases: alias.schema.table (e.g. prod.public.orders)

## Style
- Be concise — show results, don't narrate every step.
- If a query fails, fix and retry; don't ask the user to debug.
- Add nodes for findings that are genuinely useful to revisit.
- Format numbers in SELECT output (ROUND, printf).`

// ── Public types (exposed via Wails) ──────────────────────────────────────────

type AISettings struct {
	Enabled    bool   `json:"enabled"`
	Provider   string `json:"provider"`   // "anthropic" | "openai"
	APIKey     string `json:"apiKey"`
	Model      string `json:"model"`
	UserPrompt string `json:"userPrompt"`
}

type AIChatMessage struct {
	Role    string `json:"role"`    // "user" | "assistant"
	Content string `json:"content"`
}

// CanvasColumn is a lightweight column descriptor sent to the frontend.
type CanvasColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type CanvasAction struct {
	Type        string         `json:"type"`             // "query"|"chart"|"markdown"|"table"|"clear"
	NodeID      string         `json:"nodeId,omitempty"` // stable id returned to Claude so it can reference this node
	Name        string         `json:"name"`
	SQL         string         `json:"sql,omitempty"`
	Content     string         `json:"content,omitempty"`   // markdown body
	SourceID    string         `json:"sourceId,omitempty"`  // id of a query node whose data this chart uses
	ChartType   string         `json:"chartType,omitempty"` // "barY"|"barX"|"lineY"|"areaY"|"dot"|"cell"|"pie"|"donut"|"number"|"boolean"|"conditional"
	XColumn     string         `json:"xColumn,omitempty"`
	YColumn     string         `json:"yColumn,omitempty"`
	ColorColumn string         `json:"colorColumn,omitempty"`
	LabelColumn string         `json:"labelColumn,omitempty"`
	Columns     []CanvasColumn `json:"columns,omitempty"`  // table import: column schema
	RowCount    int64          `json:"rowCount,omitempty"` // table import: row count
	// Explicit canvas position — used by sample datasets for designed layouts.
	// When HasPosition is false the frontend falls back to auto-placement.
	HasPosition bool    `json:"hasPosition,omitempty"`
	X           float64 `json:"x,omitempty"`
	Y           float64 `json:"y,omitempty"`
	// Explicit size — used by resize_node and move_node tools.
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

type AIResponse struct {
	Content       string         `json:"content"`
	CanvasActions []CanvasAction `json:"canvasActions"`
}

// ── Settings persistence ──────────────────────────────────────────────────────

const aiSettingsKey = "ai_settings"

func (a *App) GetAISettings() (AISettings, error) {
	if a.persist == nil {
		return defaultAISettings(), nil
	}
	raw, err := a.persist.getSetting(aiSettingsKey)
	if err != nil || raw == "" {
		return defaultAISettings(), nil
	}
	var s AISettings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return defaultAISettings(), nil
	}
	return s, nil
}

func (a *App) SaveAISettings(s AISettings) error {
	if a.persist == nil {
		return nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return a.persist.saveSetting(aiSettingsKey, string(raw))
}

func defaultAISettings() AISettings {
	return AISettings{
		Enabled:  false,
		Provider: "anthropic",
		Model:    "claude-sonnet-4-6",
	}
}

// ── Main entry point ──────────────────────────────────────────────────────────

func (a *App) SendAIMessage(history []AIChatMessage) (AIResponse, error) {
	settings, err := a.GetAISettings()
	if err != nil {
		return AIResponse{}, err
	}
	if !settings.Enabled {
		return AIResponse{}, fmt.Errorf("AI assistant is disabled")
	}
	if settings.APIKey == "" {
		return AIResponse{}, fmt.Errorf("API key not configured")
	}

	sysPrompt := AISystemPrompt
	if strings.TrimSpace(settings.UserPrompt) != "" {
		sysPrompt += "\n\n## Additional context from user\n" + settings.UserPrompt
	}

	switch strings.ToLower(settings.Provider) {
	case "openai":
		return a.runOpenAILoop(settings, sysPrompt, history)
	default:
		return a.runAnthropicLoop(settings, sysPrompt, history)
	}
}

// ── Tool execution ────────────────────────────────────────────────────────────

func (a *App) execTool(name string, input map[string]any, actions *[]CanvasAction) string {
	switch name {
	case "list_tables":
		return a.toolListTables()
	case "list_canvas_nodes":
		return a.toolListCanvasNodes()
	case "run_query":
		sql, _ := input["sql"].(string)
		return a.toolRunQuery(sql)
	case "add_query_node":
		n, _ := input["name"].(string)
		s, _ := input["sql"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "query"})
		*actions = append(*actions, CanvasAction{Type: "query", NodeID: id, Name: n, SQL: s})
		return fmt.Sprintf("Added query node name=%q id=%q — use this id as source_id in add_chart_node", n, id)
	case "add_chart_node":
		n, _ := input["name"].(string)
		s, _ := input["sql"].(string)
		sid, _ := input["source_id"].(string)
		ct, _ := input["chart_type"].(string)
		xc, _ := input["x_column"].(string)
		yc, _ := input["y_column"].(string)
		cc, _ := input["color_column"].(string)
		lc, _ := input["label_column"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "chart"})
		*actions = append(*actions, CanvasAction{
			Type: "chart", NodeID: id, Name: n, SQL: s, SourceID: sid,
			ChartType: ct, XColumn: xc, YColumn: yc, ColorColumn: cc, LabelColumn: lc,
		})
		return fmt.Sprintf("Added chart node name=%q id=%q", n, id)
	case "add_markdown_node":
		n, _ := input["name"].(string)
		c, _ := input["content"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "markdown"})
		*actions = append(*actions, CanvasAction{Type: "markdown", NodeID: id, Name: n, Content: c})
		return fmt.Sprintf("Added markdown node name=%q id=%q", n, id)
	case "import_file":
		return a.toolImportFile(input, actions)
	case "import_csv_data":
		return a.toolImportCSVData(input, actions)
	case "import_url":
		return a.toolImportURL(input, actions)
	case "clear_canvas":
		if a.persist != nil {
			a.persist.saveCanvasState("[]") //nolint:errcheck
		}
		// Wipe the in-memory registry so list_canvas_nodes reflects the cleared state.
		a.mcpNodeRegistry.Range(func(k, _ any) bool { a.mcpNodeRegistry.Delete(k); return true })
		*actions = append(*actions, CanvasAction{Type: "clear", Name: ""})
		return "Canvas cleared."
	case "resize_node":
		id, _ := input["node_id"].(string)
		w, _ := input["width"].(float64)
		h, _ := input["height"].(float64)
		if id == "" {
			return "error: node_id is required"
		}
		*actions = append(*actions, CanvasAction{Type: "resize_node", NodeID: id, Width: w, Height: h})
		return fmt.Sprintf("Resized node %q to %gx%g", id, w, h)
	case "move_node":
		id, _ := input["node_id"].(string)
		x, _ := input["x"].(float64)
		y, _ := input["y"].(float64)
		if id == "" {
			return "error: node_id is required"
		}
		*actions = append(*actions, CanvasAction{Type: "move_node", NodeID: id, X: x, Y: y})
		return fmt.Sprintf("Moved node %q to (%g, %g)", id, x, y)
	case "focus_node":
		id, _ := input["node_id"].(string)
		if id == "" {
			return "error: node_id is required"
		}
		*actions = append(*actions, CanvasAction{Type: "focus_node", NodeID: id})
		return fmt.Sprintf("Focused node %q", id)
	case "update_query_node":
		id, _ := input["node_id"].(string)
		sql, _ := input["sql"].(string)
		name, _ := input["name"].(string)
		if id == "" || sql == "" {
			return "error: node_id and sql are required"
		}
		*actions = append(*actions, CanvasAction{Type: "update_query", NodeID: id, SQL: sql, Name: name})
		if name != "" {
			return fmt.Sprintf("Updated query node %q: new SQL and renamed to %q", id, name)
		}
		return fmt.Sprintf("Updated query node %q with new SQL", id)
	case "fit_view":
		mode, _ := input["mode"].(string)
		if mode == "" {
			mode = "fit"
		}
		*actions = append(*actions, CanvasAction{Type: "fit_view", Name: mode})
		if mode == "reset" {
			return "Zoom reset to 100%."
		}
		return "Zoomed to fit all canvas content."
	default:
		return "unknown tool: " + name
	}
}

func (a *App) toolImportFile(input map[string]any, actions *[]CanvasAction) string {
	path, _ := input["path"].(string)
	tableName, _ := input["table_name"].(string)
	if path == "" || tableName == "" {
		return "error: path and table_name are required"
	}
	safe := escapeDoubleQuote(tableName)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck
	if err := a.ImportFromPath(path, tableName); err != nil {
		return "error importing file: " + err.Error()
	}
	return a.toolFinishImport(tableName, "import_file", actions)
}

// validateCSVColumns checks that every data row has the same number of fields
// as the header. Returns a non-empty error string if a mismatch is found.
func validateCSVColumns(csvText string) string {
	r := csv.NewReader(strings.NewReader(csvText))
	r.FieldsPerRecord = -1 // don't enforce uniformity; we check manually
	r.LazyQuotes = true

	var headerCols int
	row := 0
	for {
		record, err := r.Read()
		if err != nil {
			break // EOF or unrecoverable parse error — let DuckDB surface the details
		}
		if row == 0 {
			headerCols = len(record)
			row++
			continue
		}
		if len(record) != headerCols {
			return fmt.Sprintf(
				"error: CSV column count mismatch — header has %d columns but row %d has %d columns. "+
					"Verify that every data row includes a value for each header field (missing or extra commas are the usual cause).",
				headerCols, row+1, len(record),
			)
		}
		row++
	}
	return ""
}

func (a *App) toolImportCSVData(input map[string]any, actions *[]CanvasAction) string {
	csvText, _ := input["csv_text"].(string)
	tableName, _ := input["table_name"].(string)
	if csvText == "" || tableName == "" {
		return "error: csv_text and table_name are required"
	}

	// Normalize line endings — Claude sometimes generates \r\n or bare \r.
	csvText = strings.ReplaceAll(csvText, "\r\n", "\n")
	csvText = strings.ReplaceAll(csvText, "\r", "\n")

	// Validate column count consistency before handing to DuckDB.
	// A mismatched header/data count causes a silent failure — catch it here
	// and return an actionable error so the caller can fix the CSV.
	if msg := validateCSVColumns(csvText); msg != "" {
		return msg
	}

	tmp, err := os.CreateTemp("", "sqgarden_mcp_*.csv")
	if err != nil {
		return "error creating temp file: " + err.Error()
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(csvText); err != nil {
		tmp.Close()
		return "error writing CSV data: " + err.Error()
	}
	tmp.Close()

	safe := escapeDoubleQuote(tableName)
	safePath := escapeSingleQuote(tmpPath)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck

	// Explicitly set delimiter and header to avoid auto-detect failures on
	// small or uniform-looking generated CSV (where DuckDB may misidentify the
	// delimiter or treat the header row as data).
	_, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(
		`CREATE TABLE "%s" AS SELECT * FROM read_csv_auto('%s', header=true, delim=',')`,
		safe, safePath,
	))
	if err != nil {
		// Fallback: disable type inference (same pattern as ImportFromPath).
		_, err = a.duck.ExecContext(a.ctx, fmt.Sprintf(
			`CREATE TABLE "%s" AS SELECT * FROM read_csv_auto('%s', all_varchar=true)`,
			safe, safePath,
		))
		if err != nil {
			return "error importing CSV: " + err.Error()
		}
	}
	return a.toolFinishImport(tableName, "import_csv_data", actions)
}

func (a *App) toolImportURL(input map[string]any, actions *[]CanvasAction) string {
	rawURL, _ := input["url"].(string)
	tableName, _ := input["table_name"].(string)
	if rawURL == "" || tableName == "" {
		return "error: url and table_name are required"
	}

	safe := escapeDoubleQuote(tableName)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck
	if err := a.ImportFromUrl(rawURL, tableName); err != nil {
		return "error importing URL: " + err.Error()
	}
	return a.toolFinishImport(tableName, "import_url", actions)
}

// toolFinishImport handles the shared post-import steps: persist, read schema,
// count rows, and emit a canvas table action.
func (a *App) toolFinishImport(tableName, toolName string, actions *[]CanvasAction) string {
	if err := a.SaveTableData(tableName); err != nil {
		fmt.Printf("mcp %s: SaveTableData failed: %v\n", toolName, err)
	}

	safe := escapeDoubleQuote(tableName)
	cols, err := a.GetTableInfo(tableName)
	if err != nil {
		return fmt.Sprintf("imported %q but couldn't read schema: %v", tableName, err)
	}

	var rowCount int64
	a.duck.QueryRowContext(a.ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, safe)).Scan(&rowCount) //nolint:errcheck

	canvasCols := make([]CanvasColumn, len(cols))
	for i, c := range cols {
		canvasCols[i] = CanvasColumn{Name: c.Name, Type: c.Type}
	}

	id := "mcp_" + nodeSlug(tableName)
	*actions = append(*actions, CanvasAction{
		Type:     "table",
		NodeID:   id,
		Name:     tableName,
		Columns:  canvasCols,
		RowCount: rowCount,
	})
	return fmt.Sprintf("Imported %q: %d rows, %d columns, id=%q", tableName, rowCount, len(cols), id)
}

// nodeSlug turns an arbitrary name into a safe node id segment.
func nodeSlug(name string) string {
	var b []byte
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b = append(b, byte(r))
		} else {
			b = append(b, '_')
		}
	}
	return strings.Trim(string(b), "_")
}

func (a *App) toolListCanvasNodes() string {
	// Build a merged map: SQLite (persisted) overlaid by the session registry
	// (nodes added this session that may not yet be auto-saved by the frontend).
	merged := make(map[string]string) // id → name, query nodes only

	// 1. Persisted canvas state (authoritative for nodes added before this session)
	if a.persist != nil {
		if raw, err := a.persist.loadCanvasState(); err == nil && raw != "" {
			var nodes []struct {
				Kind string `json:"kind"`
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if json.Unmarshal([]byte(raw), &nodes) == nil {
				for _, n := range nodes {
					if n.Kind == "query" {
						merged[n.ID] = n.Name
					}
				}
			}
		}
	}

	// 2. In-memory registry — captures nodes added this session before the
	//    frontend auto-save (1 s debounce) has written them back to SQLite.
	a.mcpNodeRegistry.Range(func(k, v any) bool {
		e := v.(mcpNodeEntry)
		if e.Kind == "query" {
			merged[e.ID] = e.Name
		}
		return true
	})

	if len(merged) == 0 {
		return "No query nodes on canvas yet."
	}
	var sb strings.Builder
	for id, name := range merged {
		fmt.Fprintf(&sb, "query id=%q name=%q\n", id, name)
	}
	return sb.String()
}

func (a *App) toolListTables() string {
	rows, err := a.duck.QueryContext(a.ctx, `
		SELECT
			database_name,
			schema_name,
			table_name,
			string_agg(column_name || ' ' || data_type, ', ' ORDER BY column_index) AS columns
		FROM duckdb_columns()
		WHERE internal = false
		GROUP BY database_name, schema_name, table_name
		ORDER BY database_name, schema_name, table_name
	`)
	if err != nil {
		return "error: " + err.Error()
	}
	defer rows.Close()

	type tbl struct {
		DB, Schema, Table, Columns string
	}
	var tables []tbl
	for rows.Next() {
		var t tbl
		rows.Scan(&t.DB, &t.Schema, &t.Table, &t.Columns)
		tables = append(tables, t)
	}
	if len(tables) == 0 {
		return "No tables loaded."
	}

	var sb strings.Builder
	for _, t := range tables {
		ref := t.Table
		if t.Schema != "main" && t.Schema != "" {
			ref = t.Schema + "." + t.Table
		}
		if t.DB != "memory" && t.DB != "" {
			ref = t.DB + "." + ref
		}
		fmt.Fprintf(&sb, "%s (%s)\n", ref, t.Columns)
	}
	return sb.String()
}

func (a *App) toolRunQuery(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return "error: empty query"
	}
	result, err := a.Query(sql)
	if err != nil {
		return "error: " + err.Error()
	}

	const maxRows = 50
	truncated := false
	rows := result.Rows
	if len(rows) > maxRows {
		rows = rows[:maxRows]
		truncated = true
	}

	out := map[string]any{
		"columns":  result.Columns,
		"rows":     rows,
		"rowCount": result.RowCount,
	}
	if truncated {
		out["note"] = fmt.Sprintf("showing first %d of %d rows", maxRows, result.RowCount)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

var anthropicTools = []map[string]any{
	{
		"name":        "list_tables",
		"description": "List every table and its columns currently loaded in DuckDB (including attached databases).",
		"input_schema": map[string]any{
			"type": "object", "properties": map[string]any{},
		},
	},
	{
		"name":        "list_canvas_nodes",
		"description": "List all query nodes currently on the canvas with their id and name. Use source_id from these results when calling add_chart_node to avoid duplicating the data fetch.",
		"input_schema": map[string]any{
			"type": "object", "properties": map[string]any{},
		},
	},
	{
		"name":        "run_query",
		"description": "Execute a DuckDB SQL query and return up to 50 rows of results.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string", "description": "SQL query to execute"},
			},
			"required": []string{"sql"},
		},
	},
	{
		"name":        "add_query_node",
		"description": "Pin a named SQL query to the user's canvas as an interactive scrollable table. Returns a node id you can pass as source_id to add_chart_node.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Short descriptive label for the node"},
				"sql":  map[string]any{"type": "string", "description": "SQL query to display in the node"},
			},
			"required": []string{"name", "sql"},
		},
	},
	{
		"name":        "add_chart_node",
		"description": "Pin a chart visualization to the canvas. Provide either source_id (id returned by add_query_node) OR sql — not both. Always confirm column names via run_query first.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string"},
				"source_id":    map[string]any{"type": "string", "description": "ID of an existing query node whose data this chart visualises (preferred over sql when the node is already on canvas)"},
				"sql":          map[string]any{"type": "string", "description": "SQL that produces the chart data — omit when source_id is provided"},
				"chart_type":   map[string]any{"type": "string", "enum": []string{"barY", "barX", "lineY", "areaY", "dot", "cell", "pie", "donut", "number", "boolean", "conditional"}, "description": "barY/barX=bar, lineY=line, areaY=area, dot=scatter, pie/donut=pie, cell=heatmap, number=big single value, boolean=true/false badge, conditional=N-state badge"},
				"x_column":     map[string]any{"type": "string", "description": "Column for the x-axis / category"},
				"y_column":     map[string]any{"type": "string", "description": "Column for the y-axis / value"},
				"color_column": map[string]any{"type": "string", "description": "Optional column for grouping/color"},
				"label_column": map[string]any{"type": "string", "description": "Optional column for slice labels (pie/donut)"},
			},
			"required": []string{"name", "chart_type", "x_column", "y_column"},
		},
	},
	{
		"name":        "add_markdown_node",
		"description": "Pin a markdown note, summary, or documentation block to the canvas.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":    map[string]any{"type": "string", "description": "Short title for the node"},
				"content": map[string]any{"type": "string", "description": "Markdown content"},
			},
			"required": []string{"name", "content"},
		},
	},
	{
		"name":        "import_file",
		"description": "Import a local CSV, Parquet, or JSON file into DuckDB and add it as a table node on the canvas.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "Absolute path to the file (CSV, Parquet, JSON, JSONL)"},
				"table_name": map[string]any{"type": "string", "description": "Name to register the table as in DuckDB"},
			},
			"required": []string{"path", "table_name"},
		},
	},
	{
		"name":        "import_csv_data",
		"description": "Load raw CSV text directly into DuckDB as a table and add it as a canvas node. Use this when you have generated or transformed data as a string — no file on disk needed.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"csv_text":   map[string]any{"type": "string", "description": "Full CSV content including header row"},
				"table_name": map[string]any{"type": "string", "description": "Name to register the table as in DuckDB (snake_case recommended)"},
			},
			"required": []string{"csv_text", "table_name"},
		},
	},
	{
		"name":        "import_url",
		"description": "Fetch a remote CSV, Parquet, or JSON file by URL, load it into DuckDB, and add it as a canvas node. The download happens server-side so CORS is not a concern.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":        map[string]any{"type": "string", "description": "Public URL to the data file (CSV, Parquet, JSON, JSONL)"},
				"table_name": map[string]any{"type": "string", "description": "Name to register the table as in DuckDB (snake_case recommended)"},
			},
			"required": []string{"url", "table_name"},
		},
	},
	{
		"name":        "focus_node",
		"description": "Pan and zoom the canvas viewport to centre on a specific node. Call after adding nodes to direct the user's attention to the most important result.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to focus (from add_query_node, add_chart_node, or list_canvas_nodes)"},
			},
			"required": []string{"node_id"},
		},
	},
	{
		"name":        "update_query_node",
		"description": "Overwrite the SQL of an existing query node and optionally rename it. Use this instead of deleting and recreating when only the query needs to change.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the query node to update"},
				"sql":     map[string]any{"type": "string", "description": "New SQL query"},
				"name":    map[string]any{"type": "string", "description": "Optional new display name for the node"},
			},
			"required": []string{"node_id", "sql"},
		},
	},
	{
		"name":        "clear_canvas",
		"description": "Remove all nodes from the canvas. Use before a full rebuild to avoid duplicates.",
		"input_schema": map[string]any{
			"type": "object", "properties": map[string]any{},
		},
	},
	{
		"name":        "fit_view",
		"description": "Adjust the canvas viewport. Use after adding nodes to bring everything into view.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"enum":        []string{"fit", "reset"},
					"description": "fit = zoom to fit all content (default); reset = set zoom to 100%",
				},
			},
		},
	},
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicResp struct {
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *App) runAnthropicLoop(s AISettings, sysPrompt string, history []AIChatMessage) (AIResponse, error) {
	msgs := make([]anthropicMsg, len(history))
	for i, m := range history {
		msgs[i] = anthropicMsg{Role: m.Role, Content: m.Content}
	}

	var canvasActions []CanvasAction
	var finalText string

	for range 10 { // guard against runaway loops
		resp, err := a.callAnthropic(s, sysPrompt, msgs)
		if err != nil {
			return AIResponse{}, err
		}

		// Collect text and tool calls from this response
		var toolUseBlocks []anthropicBlock
		for _, blk := range resp.Content {
			if blk.Type == "text" {
				finalText = blk.Text
			} else if blk.Type == "tool_use" {
				toolUseBlocks = append(toolUseBlocks, blk)
			}
		}

		if resp.StopReason == "end_turn" || len(toolUseBlocks) == 0 {
			break
		}

		// Append the assistant's response to the conversation
		msgs = append(msgs, anthropicMsg{Role: "assistant", Content: resp.Content})

		// Execute tools and build tool_result blocks
		var results []anthropicBlock
		for _, blk := range toolUseBlocks {
			var input map[string]any
			json.Unmarshal(blk.Input, &input)
			result := a.execTool(blk.Name, input, &canvasActions)
			results = append(results, anthropicBlock{
				Type:      "tool_result",
				ToolUseID: blk.ID,
				Content:   result,
			})
		}
		msgs = append(msgs, anthropicMsg{Role: "user", Content: results})
	}

	return AIResponse{Content: finalText, CanvasActions: canvasActions}, nil
}

func (a *App) callAnthropic(s AISettings, sysPrompt string, msgs []anthropicMsg) (*anthropicResp, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      s.Model,
		"max_tokens": 4096,
		"system":     sysPrompt,
		"messages":   msgs,
		"tools":      anthropicTools,
	})

	req, err := http.NewRequestWithContext(a.ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", s.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	var ar anthropicResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, fmt.Errorf("anthropic decode error: %w", err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("anthropic error: %s", ar.Error.Message)
	}
	return &ar, nil
}

// ── OpenAI ────────────────────────────────────────────────────────────────────

var openAITools = func() []map[string]any {
	var tools []map[string]any
	for _, t := range anthropicTools {
		params := t["input_schema"]
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t["name"],
				"description": t["description"],
				"parameters":  params,
			},
		})
	}
	return tools
}()

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIMsg struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIResp struct {
	Choices []struct {
		Message      openAIMsg `json:"message"`
		FinishReason string    `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *App) runOpenAILoop(s AISettings, sysPrompt string, history []AIChatMessage) (AIResponse, error) {
	msgs := []openAIMsg{{Role: "system", Content: sysPrompt}}
	for _, m := range history {
		msgs = append(msgs, openAIMsg{Role: m.Role, Content: m.Content})
	}

	var canvasActions []CanvasAction
	var finalText string

	for range 10 {
		resp, err := a.callOpenAI(s, msgs)
		if err != nil {
			return AIResponse{}, err
		}
		if len(resp.Choices) == 0 {
			break
		}

		choice := resp.Choices[0]
		msgs = append(msgs, choice.Message)

		if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
			if s, ok := choice.Message.Content.(string); ok {
				finalText = s
			}
			break
		}

		for _, tc := range choice.Message.ToolCalls {
			var input map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &input)
			result := a.execTool(tc.Function.Name, input, &canvasActions)
			msgs = append(msgs, openAIMsg{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	return AIResponse{Content: finalText, CanvasActions: canvasActions}, nil
}

func (a *App) callOpenAI(s AISettings, msgs []openAIMsg) (*openAIResp, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    s.Model,
		"messages": msgs,
		"tools":    openAITools,
	})

	req, err := http.NewRequestWithContext(a.ctx, http.MethodPost,
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+s.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	var or openAIResp
	if err := json.NewDecoder(resp.Body).Decode(&or); err != nil {
		return nil, fmt.Errorf("openai decode error: %w", err)
	}
	if or.Error != nil {
		return nil, fmt.Errorf("openai error: %s", or.Error.Message)
	}
	return &or, nil
}
