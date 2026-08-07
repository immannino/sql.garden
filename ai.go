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
- list_canvases — list all canvas tabs with their id, name, node count, and which is active
- create_canvas — create a new canvas tab; returns canvas id for targeting
- rename_canvas — rename a canvas tab
- switch_canvas — make a canvas tab active (surfaces it to the user)
- remove_canvas — delete a canvas tab and all its nodes
- list_canvas_nodes — see query/data nodes on the canvas; filter by canvas_id to scope to one tab
- run_query — execute any DuckDB SQL and see real results
- add_query_node — pin a named SQL query to the canvas; returns a node id; accepts canvas_id
- add_chart_node — pin a named visualization, optionally sourced from an existing query node; accepts canvas_id
- add_markdown_node — pin a markdown note or summary to the canvas; accepts canvas_id
- materialize_query — snapshot a SQL query's results as a persistent DuckDB table and pin it as a Data node; accepts canvas_id
- import_file — import a local CSV/Parquet/JSON file into DuckDB and add a table node; accepts canvas_id
- import_csv_data — load raw CSV text you generate directly into DuckDB; accepts canvas_id
- import_url — fetch a remote CSV/Parquet/JSON by URL into DuckDB; accepts canvas_id
- import_s3 — load a file from S3, R2, or MinIO directly into DuckDB; accepts canvas_id
- add_section — create a named Section container to visually group related nodes; accepts canvas_id
- clear_canvas — remove all nodes from a canvas tab (canvas_id defaults to active)
- fit_view — adjust the canvas viewport: mode="fit" zooms to show all content, mode="reset" sets zoom to 100%
- focus_node — pan and zoom the viewport to centre on a specific node; use after adding nodes to direct the user's attention
- update_query_node — overwrite the SQL (and optionally rename) an existing query node by id
- set_node_color — apply a hex color to any canvas node by id; useful for highlighting KPIs or flagging anomalies

## Canvas tabs
The user's workspace has multiple named canvas tabs (like browser tabs). By default every tool adds to the **active** canvas. Use canvas_id to target a different tab. Typical pattern for multi-tab work:
1. list_canvases → discover existing tab ids
2. create_canvas → create a new tab, capture the returned id
3. Pass canvas_id to node-adding tools to populate that tab
4. switch_canvas → bring the new tab into view

## Workflow
1. Always call list_tables first so you know what's available.
2. Use run_query freely to explore, filter, aggregate, and validate hypotheses.
3. When you find something worth keeping, add it to the canvas.
4. Summarize findings concisely after each investigation.

## Chart type guide
- **barY / barX** — vertical/horizontal bar; x=category, y=value
- **lineY** — line over time or ordered x; x=time/category, y=value
- **areaY** — filled area; same as lineY
- **dot** — scatter; x=numeric, y=numeric, color_column=optional grouping
- **pie / donut** — proportional slices; x=category, y=value, label_column=optional label
- **cell** — structured grid heatmap; x=category, y=category, color driven by a third column (use color_column)
- **number** — single large KPI value; x_column and y_column both set to the value column
- **boolean** — true/false badge; x_column = the boolean-valued column
- **conditional** — N-state badge (e.g. status, severity); x_column = the category column
- **waterfall** — running-total bar chart showing incremental changes; x=category/step, y=numeric delta; bars auto-colored green (positive) / red (negative)
- **heatmap** — 2D density via binning; x=numeric, y=numeric; good for distributions
- **scatter-matrix** — pairwise grid of scatter plots across multiple numeric columns; set x_column to any one numeric column (the grid uses all numeric columns in the result automatically)

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
	Type        string         `json:"type"`             // "query"|"chart"|"markdown"|"table"|"clear"|"add_canvas"|"remove_canvas"|"rename_canvas"|"switch_canvas"
	NodeID      string         `json:"nodeId,omitempty"` // stable id returned to Claude so it can reference this node
	CanvasID    string         `json:"canvasId,omitempty"` // target canvas for node actions; subject canvas for tab lifecycle actions
	Name        string         `json:"name"`
	SQL         string         `json:"sql,omitempty"`
	Content     string         `json:"content,omitempty"`   // markdown body
	SourceID    string         `json:"sourceId,omitempty"`  // id of a query node whose data this chart uses
	ChartType   string         `json:"chartType,omitempty"` // "barY"|"barX"|"lineY"|"areaY"|"dot"|"cell"|"pie"|"donut"|"number"|"boolean"|"conditional"|"waterfall"|"heatmap"|"scatter-matrix"
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
	// IngestNode fields
	Mode         string  `json:"mode,omitempty"`         // "generator" | "ingestion"
	URL          string  `json:"url,omitempty"`          // ingestion: source URL
	TargetTable  string  `json:"targetTable,omitempty"`  // ingestion: DuckDB table name
	ConflictMode string  `json:"conflictMode,omitempty"` // "append" | "replace"
	Interval     float64 `json:"interval,omitempty"`     // seconds (fractional ok for ms)
	// ExerciseNode fields — used by learn tracks to pre-configure checks
	Checks           []map[string]interface{} `json:"checks,omitempty"`
	RevealHintsAfter int                      `json:"revealHintsAfter,omitempty"`
	SuccessText      string                   `json:"successText,omitempty"`
	NextID           string                   `json:"nextId,omitempty"`
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
	case "list_canvases":
		return a.toolListCanvases()
	case "list_canvas_nodes":
		filterCanvas, _ := input["canvas_id"].(string)
		return a.toolListCanvasNodes(filterCanvas)
	case "run_query":
		sql, _ := input["sql"].(string)
		return a.toolRunQuery(sql)
	case "create_canvas":
		n, _ := input["name"].(string)
		id := "mcp_tab_" + randID()
		if n == "" {
			n = "New Canvas"
		}
		a.mcpCanvases.Store(id, n)
		*actions = append(*actions, CanvasAction{Type: "add_canvas", CanvasID: id, Name: n})
		return fmt.Sprintf("Created canvas name=%q id=%q — pass id as canvas_id to target this tab", n, id)
	case "rename_canvas":
		id, _ := input["canvas_id"].(string)
		n, _ := input["name"].(string)
		if id == "" || n == "" {
			return "error: canvas_id and name are required"
		}
		a.mcpCanvases.Store(id, n)
		*actions = append(*actions, CanvasAction{Type: "rename_canvas", CanvasID: id, Name: n})
		return fmt.Sprintf("Renamed canvas %q to %q", id, n)
	case "switch_canvas":
		id, _ := input["canvas_id"].(string)
		if id == "" {
			return "error: canvas_id is required"
		}
		*actions = append(*actions, CanvasAction{Type: "switch_canvas", CanvasID: id})
		return fmt.Sprintf("Switched active canvas to %q", id)
	case "remove_canvas":
		id, _ := input["canvas_id"].(string)
		if id == "" {
			return "error: canvas_id is required"
		}
		a.mcpCanvases.Delete(id)
		*actions = append(*actions, CanvasAction{Type: "remove_canvas", CanvasID: id})
		return fmt.Sprintf("Removed canvas %q", id)
	case "add_query_node":
		n, _ := input["name"].(string)
		s, _ := input["sql"].(string)
		cid, _ := input["canvas_id"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "query", CanvasID: cid})
		*actions = append(*actions, CanvasAction{Type: "query", NodeID: id, Name: n, SQL: s, CanvasID: cid})
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
		cid, _ := input["canvas_id"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "chart", CanvasID: cid})
		*actions = append(*actions, CanvasAction{
			Type: "chart", NodeID: id, Name: n, SQL: s, SourceID: sid,
			ChartType: ct, XColumn: xc, YColumn: yc, ColorColumn: cc, LabelColumn: lc,
			CanvasID: cid,
		})
		return fmt.Sprintf("Added chart node name=%q id=%q", n, id)
	case "add_markdown_node":
		n, _ := input["name"].(string)
		c, _ := input["content"].(string)
		cid, _ := input["canvas_id"].(string)
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "markdown", CanvasID: cid})
		*actions = append(*actions, CanvasAction{Type: "markdown", NodeID: id, Name: n, Content: c, CanvasID: cid})
		return fmt.Sprintf("Added markdown node name=%q id=%q", n, id)
	case "materialize_query":
		return a.toolMaterializeQuery(input, actions)
	case "import_file":
		return a.toolImportFile(input, actions)
	case "import_csv_data":
		return a.toolImportCSVData(input, actions)
	case "import_url":
		return a.toolImportURL(input, actions)
	case "import_s3":
		return a.toolImportS3(input, actions)
	case "clear_canvas":
		cid, _ := input["canvas_id"].(string)
		if cid == "" {
			// Clear active canvas — wipe in-memory nodes that don't have a specific canvas assigned
			a.mcpNodeRegistry.Range(func(k, v any) bool {
				if e := v.(mcpNodeEntry); e.CanvasID == "" {
					a.mcpNodeRegistry.Delete(k)
				}
				return true
			})
		} else {
			// Clear a specific canvas — wipe only nodes belonging to it
			a.mcpNodeRegistry.Range(func(k, v any) bool {
				if e := v.(mcpNodeEntry); e.CanvasID == cid {
					a.mcpNodeRegistry.Delete(k)
				}
				return true
			})
		}
		*actions = append(*actions, CanvasAction{Type: "clear", CanvasID: cid, Name: ""})
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
	case "set_node_color":
		id, _ := input["node_id"].(string)
		color, _ := input["color"].(string)
		if id == "" || color == "" {
			return "error: node_id and color are required"
		}
		*actions = append(*actions, CanvasAction{Type: "set_color", NodeID: id, Name: color})
		return fmt.Sprintf("Set color of node %q to %q", id, color)
	case "add_ingest_node":
		n, _ := input["name"].(string)
		mode, _ := input["mode"].(string)
		sql, _ := input["sql"].(string)
		url, _ := input["url"].(string)
		targetTable, _ := input["target_table"].(string)
		conflictMode, _ := input["conflict_mode"].(string)
		interval, _ := input["interval"].(float64)
		cid, _ := input["canvas_id"].(string)
		if n == "" {
			return "error: name is required"
		}
		if mode == "" {
			mode = "generator"
		}
		if mode != "generator" && mode != "ingestion" {
			return "error: mode must be \"generator\" or \"ingestion\""
		}
		if mode == "ingestion" && url == "" {
			return "error: url is required for ingestion mode"
		}
		if conflictMode == "" {
			conflictMode = "append"
		}
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "ingest", CanvasID: cid})
		*actions = append(*actions, CanvasAction{
			Type: "ingest", NodeID: id, Name: n, CanvasID: cid,
			Mode: mode, SQL: sql, URL: url, TargetTable: targetTable,
			ConflictMode: conflictMode, Interval: interval,
		})
		return fmt.Sprintf("Added ingest node name=%q id=%q mode=%q interval=%gs", n, id, mode, interval)
	case "add_exercise_node":
		n, _ := input["name"].(string)
		sql, _ := input["sql"].(string)
		prompt, _ := input["prompt"].(string)
		successText, _ := input["success_text"].(string)
		nextID, _ := input["next_id"].(string)
		cid, _ := input["canvas_id"].(string)
		if n == "" {
			return "error: name is required"
		}
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "exercise", CanvasID: cid})
		*actions = append(*actions, CanvasAction{
			Type: "exercise", NodeID: id, Name: n, CanvasID: cid,
			SQL: sql, Content: prompt, SuccessText: successText, NextID: nextID,
		})
		return fmt.Sprintf("Added exercise node name=%q id=%q", n, id)
	case "add_test_node":
		n, _ := input["name"].(string)
		sql, _ := input["sql"].(string)
		interval, _ := input["interval"].(float64)
		cid, _ := input["canvas_id"].(string)
		if n == "" {
			return "error: name is required"
		}
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "test", CanvasID: cid})
		*actions = append(*actions, CanvasAction{
			Type: "test", NodeID: id, Name: n, CanvasID: cid,
			SQL: sql, Interval: interval,
		})
		return fmt.Sprintf("Added test node name=%q id=%q", n, id)
	case "add_section":
		n, _ := input["name"].(string)
		w, _ := input["width"].(float64)
		h, _ := input["height"].(float64)
		cid, _ := input["canvas_id"].(string)
		if n == "" {
			return "error: name is required"
		}
		if w <= 0 {
			w = 400
		}
		if h <= 0 {
			h = 300
		}
		id := "mcp_" + nodeSlug(n)
		a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: n, Kind: "section", CanvasID: cid})
		*actions = append(*actions, CanvasAction{Type: "section", NodeID: id, Name: n, Width: w, Height: h, CanvasID: cid})
		return fmt.Sprintf("Added section name=%q id=%q size=%gx%g", n, id, w, h)
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

func (a *App) toolMaterializeQuery(input map[string]any, actions *[]CanvasAction) string {
	tableName, _ := input["table_name"].(string)
	sql, _ := input["sql"].(string)
	sourceID, _ := input["source_id"].(string)
	canvasID, _ := input["canvas_id"].(string)
	if tableName == "" || sql == "" {
		return "error: table_name and sql are required"
	}
	sql = strings.TrimRight(strings.TrimSpace(sql), ";")
	safe := escapeDoubleQuote(tableName)
	if _, err := a.duck.ExecContext(a.ctx, fmt.Sprintf(`CREATE OR REPLACE TABLE "%s" AS (%s)`, safe, sql)); err != nil {
		return "error materializing query: " + err.Error()
	}
	cols, err := a.GetTableInfo(tableName)
	if err != nil {
		return fmt.Sprintf("materialized %q but couldn't read schema: %v", tableName, err)
	}
	var rowCount int64
	a.duck.QueryRowContext(a.ctx, fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, safe)).Scan(&rowCount) //nolint:errcheck
	if err := a.SaveTableData(tableName); err != nil {
		fmt.Printf("mcp materialize_query: SaveTableData failed: %v\n", err)
	}
	canvasCols := make([]CanvasColumn, len(cols))
	for i, c := range cols {
		canvasCols[i] = CanvasColumn{Name: c.Name, Type: c.Type}
	}
	id := "mcp_" + nodeSlug(tableName)
	a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: tableName, Kind: "data", CanvasID: canvasID})
	*actions = append(*actions, CanvasAction{
		Type:     "data",
		NodeID:   id,
		Name:     tableName,
		SQL:      sql,
		SourceID: sourceID,
		Columns:  canvasCols,
		RowCount: rowCount,
		CanvasID: canvasID,
	})
	return fmt.Sprintf("Materialized %q: %d rows, %d columns, id=%q — queryable as a DuckDB table", tableName, rowCount, len(cols), id)
}

func (a *App) toolImportFile(input map[string]any, actions *[]CanvasAction) string {
	path, _ := input["path"].(string)
	tableName, _ := input["table_name"].(string)
	canvasID, _ := input["canvas_id"].(string)
	if path == "" || tableName == "" {
		return "error: path and table_name are required"
	}
	safe := escapeDoubleQuote(tableName)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck
	if err := a.ImportFromPath(path, tableName); err != nil {
		return "error importing file: " + err.Error()
	}
	return a.toolFinishImport(tableName, "import_file", canvasID, actions)
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
	canvasID, _ := input["canvas_id"].(string)
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
	return a.toolFinishImport(tableName, "import_csv_data", canvasID, actions)
}

func (a *App) toolImportURL(input map[string]any, actions *[]CanvasAction) string {
	rawURL, _ := input["url"].(string)
	tableName, _ := input["table_name"].(string)
	canvasID, _ := input["canvas_id"].(string)
	if rawURL == "" || tableName == "" {
		return "error: url and table_name are required"
	}

	safe := escapeDoubleQuote(tableName)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck
	if err := a.ImportFromUrl(rawURL, tableName); err != nil {
		return "error importing URL: " + err.Error()
	}
	return a.toolFinishImport(tableName, "import_url", canvasID, actions)
}

func (a *App) toolImportS3(input map[string]any, actions *[]CanvasAction) string {
	s3URL, _ := input["s3_url"].(string)
	tableName, _ := input["table_name"].(string)
	canvasID, _ := input["canvas_id"].(string)
	if s3URL == "" || tableName == "" {
		return "error: s3_url and table_name are required"
	}
	if !strings.HasPrefix(s3URL, "s3://") {
		return "error: s3_url must start with s3://"
	}

	safe := escapeDoubleQuote(tableName)
	a.duck.ExecContext(a.ctx, fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, safe)) //nolint:errcheck
	if err := a.ImportFromUrl(s3URL, tableName); err != nil {
		return "error importing from S3: " + err.Error()
	}
	return a.toolFinishImport(tableName, "import_s3", canvasID, actions)
}

// toolFinishImport handles the shared post-import steps: persist, read schema,
// count rows, and emit a canvas table action.
func (a *App) toolFinishImport(tableName, toolName, canvasID string, actions *[]CanvasAction) string {
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
	a.mcpNodeRegistry.Store(id, mcpNodeEntry{ID: id, Name: tableName, Kind: "table", CanvasID: canvasID})
	*actions = append(*actions, CanvasAction{
		Type:     "table",
		NodeID:   id,
		Name:     tableName,
		Columns:  canvasCols,
		RowCount: rowCount,
		CanvasID: canvasID,
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

func (a *App) toolListCanvases() string {
	type canvasInfo struct {
		ID        string
		Name      string
		IsActive  bool
		NodeCount int
	}

	var canvases []canvasInfo
	var activeID string

	if a.persist != nil {
		if raw, err := a.persist.loadCanvasState(); err == nil && raw != "" {
			// Try v2 format
			var v2 struct {
				Version  int    `json:"version"`
				ActiveID string `json:"activeCanvasId"`
				Canvases []struct {
					ID    string            `json:"id"`
					Name  string            `json:"name"`
					Nodes []json.RawMessage `json:"nodes"`
				} `json:"canvases"`
			}
			if json.Unmarshal([]byte(raw), &v2) == nil && v2.Version == 2 {
				activeID = v2.ActiveID
				for _, c := range v2.Canvases {
					canvases = append(canvases, canvasInfo{
						ID:        c.ID,
						Name:      c.Name,
						IsActive:  c.ID == activeID,
						NodeCount: len(c.Nodes),
					})
				}
			}
		}
	}

	// Merge canvases created this MCP session not yet persisted
	a.mcpCanvases.Range(func(k, v any) bool {
		id := k.(string)
		for _, c := range canvases {
			if c.ID == id {
				return true
			}
		}
		canvases = append(canvases, canvasInfo{ID: id, Name: v.(string)})
		return true
	})

	if len(canvases) == 0 {
		return "No canvases found — app may not be running or no state has been saved yet."
	}

	var sb strings.Builder
	for _, c := range canvases {
		active := ""
		if c.IsActive {
			active = " [active]"
		}
		fmt.Fprintf(&sb, "id=%q name=%q nodes=%d%s\n", c.ID, c.Name, c.NodeCount, active)
	}
	return sb.String()
}

func (a *App) toolListCanvasNodes(filterCanvasID string) string {
	type entry struct {
		name, kind, canvasID string
	}
	merged := make(map[string]entry)

	// 1. Persisted canvas state (authoritative for nodes added before this session)
	if a.persist != nil {
		if raw, err := a.persist.loadCanvasState(); err == nil && raw != "" {
			// Try v2 multi-canvas format
			var v2 struct {
				Version  int    `json:"version"`
				ActiveID string `json:"activeCanvasId"`
				Canvases []struct {
					ID    string `json:"id"`
					Nodes []struct {
						Kind string `json:"kind"`
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"canvases"`
			}
			if json.Unmarshal([]byte(raw), &v2) == nil && v2.Version == 2 {
				for _, canvas := range v2.Canvases {
					if filterCanvasID != "" && canvas.ID != filterCanvasID {
						continue
					}
					for _, n := range canvas.Nodes {
						if n.Kind == "query" || n.Kind == "data" {
							merged[n.ID] = entry{name: n.Name, kind: n.Kind, canvasID: canvas.ID}
						}
					}
				}
			} else {
				// Legacy v1: flat array — no canvas info
				var nodes []struct {
					Kind string `json:"kind"`
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				if json.Unmarshal([]byte(raw), &nodes) == nil {
					for _, n := range nodes {
						if n.Kind == "query" || n.Kind == "data" {
							merged[n.ID] = entry{name: n.Name, kind: n.Kind}
						}
					}
				}
			}
		}
	}

	// 2. In-memory registry — captures nodes added this session before the
	//    frontend auto-save (1 s debounce) has written them back to SQLite.
	a.mcpNodeRegistry.Range(func(k, v any) bool {
		e := v.(mcpNodeEntry)
		if e.Kind != "query" && e.Kind != "data" {
			return true
		}
		// When filtering, skip nodes from other canvases (but include ambiguous "" nodes)
		if filterCanvasID != "" && e.CanvasID != "" && e.CanvasID != filterCanvasID {
			return true
		}
		merged[e.ID] = entry{name: e.Name, kind: e.Kind, canvasID: e.CanvasID}
		return true
	})

	if len(merged) == 0 {
		if filterCanvasID != "" {
			return fmt.Sprintf("No query or data nodes on canvas %q.", filterCanvasID)
		}
		return "No query or data nodes on any canvas."
	}

	var sb strings.Builder
	for id, e := range merged {
		if e.canvasID != "" {
			fmt.Fprintf(&sb, "%s id=%q name=%q canvas=%q\n", e.kind, id, e.name, e.canvasID)
		} else {
			fmt.Fprintf(&sb, "%s id=%q name=%q\n", e.kind, id, e.name)
		}
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
		"name":        "list_canvases",
		"description": "List all canvas tabs with their id, name, node count, and which is currently active. Call this before using canvas_id in other tools to discover available canvas ids.",
		"input_schema": map[string]any{
			"type": "object", "properties": map[string]any{},
		},
	},
	{
		"name":        "create_canvas",
		"description": "Create a new canvas tab. Returns the canvas id, which you can pass as canvas_id to other tools to add nodes to it.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Display name for the new canvas tab"},
			},
		},
	},
	{
		"name":        "rename_canvas",
		"description": "Rename an existing canvas tab.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "ID of the canvas to rename (from list_canvases)"},
				"name":      map[string]any{"type": "string", "description": "New display name"},
			},
			"required": []string{"canvas_id", "name"},
		},
	},
	{
		"name":        "switch_canvas",
		"description": "Make a canvas tab active — brings it into view for the user. Call after building a canvas to surface the work.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "ID of the canvas to switch to (from list_canvases or create_canvas)"},
			},
			"required": []string{"canvas_id"},
		},
	},
	{
		"name":        "remove_canvas",
		"description": "Delete a canvas tab and all its nodes. Cannot remove the last remaining canvas.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "ID of the canvas to remove (from list_canvases)"},
			},
			"required": []string{"canvas_id"},
		},
	},
	{
		"name":        "list_canvas_nodes",
		"description": "List query/data nodes with their id and name. Pass canvas_id to filter to a specific tab; omit to see all canvases. Use the returned ids as source_id in add_chart_node.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "Filter to nodes on this canvas (from list_canvases); omit for all canvases"},
			},
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
		"description": "Pin a named SQL query to the canvas as an interactive scrollable table. Returns a node id you can pass as source_id to add_chart_node.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Short descriptive label for the node"},
				"sql":       map[string]any{"type": "string", "description": "SQL query to display in the node"},
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to add the node to (from list_canvases or create_canvas); default is the active canvas"},
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
				"chart_type":   map[string]any{"type": "string", "enum": []string{"barY", "barX", "lineY", "areaY", "dot", "cell", "pie", "donut", "number", "boolean", "conditional", "waterfall", "heatmap", "scatter-matrix"}, "description": "barY/barX=bar chart, lineY=line, areaY=area, dot=scatter, pie/donut=pie chart, cell=grid heatmap, number=big single value, boolean=true/false badge, conditional=N-state badge, waterfall=running-total waterfall (needs x+y), heatmap=2D density heatmap (needs x+y), scatter-matrix=pairwise scatter grid (use x_column for any numeric col; set multiple via matrixColumns)"},
				"x_column":     map[string]any{"type": "string", "description": "Column for the x-axis / category"},
				"y_column":     map[string]any{"type": "string", "description": "Column for the y-axis / value"},
				"color_column": map[string]any{"type": "string", "description": "Optional column for grouping/color"},
				"label_column": map[string]any{"type": "string", "description": "Optional column for slice labels (pie/donut)"},
				"canvas_id":    map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
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
				"name":      map[string]any{"type": "string", "description": "Short title for the node"},
				"content":   map[string]any{"type": "string", "description": "Markdown content"},
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"name", "content"},
		},
	},
	{
		"name":        "materialize_query",
		"description": "Snapshot a SQL query's results into a named DuckDB table and pin it to the canvas as a persistent Data node. Unlike add_query_node (which re-runs SQL on demand), a Data node stores a snapshot that survives app restarts and can be queried directly. Use for expensive aggregations or stable reference datasets.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_name": map[string]any{"type": "string", "description": "DuckDB table name for the snapshot (snake_case recommended)"},
				"sql":        map[string]any{"type": "string", "description": "SQL query whose results are materialized — do not include a trailing semicolon"},
				"source_id":  map[string]any{"type": "string", "description": "Optional ID of a query node this was derived from, used for canvas linking"},
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"table_name", "sql"},
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
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
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
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
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
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"url", "table_name"},
		},
	},
	{
		"name":        "import_s3",
		"description": "Load a file from S3, Cloudflare R2, MinIO, or any S3-compatible store into DuckDB and add it as a canvas node. Credentials must be configured in Settings → S3 Storage. Accepts s3:// URIs.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"s3_url":     map[string]any{"type": "string", "description": "S3 URI to the data file, e.g. s3://my-bucket/data/sales.parquet"},
				"table_name": map[string]any{"type": "string", "description": "Name to register the table as in DuckDB (snake_case recommended)"},
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"s3_url", "table_name"},
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
		"name":        "set_node_color",
		"description": "Set the accent color of a canvas node. Use to highlight KPIs, group nodes visually, or flag anomalies.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to recolor"},
				"color":   map[string]any{"type": "string", "description": "CSS hex color, e.g. #ef4444"},
			},
			"required": []string{"node_id", "color"},
		},
	},
	{
		"name":        "add_ingest_node",
		"description": "Add a Generator or Ingestion node to the canvas. Generator mode runs a DML SQL statement on a schedule against DuckDB (good for synthetic live data). Ingestion mode fetches a URL on a schedule and appends or replaces a target table (desktop only — requires Go-side fetch to bypass CORS).",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "Display name for the node"},
				"mode":          map[string]any{"type": "string", "enum": []string{"generator", "ingestion"}, "description": "generator = run SQL on schedule; ingestion = fetch URL on schedule (default: generator)"},
				"sql":           map[string]any{"type": "string", "description": "Generator mode: DML SQL to run (INSERT, UPDATE, etc.)"},
				"url":           map[string]any{"type": "string", "description": "Ingestion mode: URL to fetch (CSV / Parquet / JSON)"},
				"target_table":  map[string]any{"type": "string", "description": "Ingestion mode: DuckDB table name to write results into"},
				"conflict_mode": map[string]any{"type": "string", "enum": []string{"append", "replace"}, "description": "How to handle existing rows in the target table (default: append)"},
				"interval":      map[string]any{"type": "number", "description": "Run interval in seconds; fractional values allowed for sub-second (e.g. 0.25 = 250 ms). 0 = manual only (default)"},
				"canvas_id":     map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"name"},
		},
	},
	{
		"name":        "add_exercise_node",
		"description": "Create a self-contained exercise node with an embedded SQL editor and checks. Students write SQL directly in the card and hit Run to check their work. Chain multiple exercises together with next_id for a guided lesson flow.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string", "description": "Name for the exercise node (e.g. 'ex1_select_basics')"},
				"sql":          map[string]any{"type": "string", "description": "Starter SQL shown to the student in the editor (e.g. 'SELECT ??? FROM sb_movies')"},
				"prompt":       map[string]any{"type": "string", "description": "Markdown prompt shown to the student describing the exercise"},
				"success_text": map[string]any{"type": "string", "description": "Markdown text revealed to the student when all checks pass — great for explanations, hints, or encouragement"},
				"next_id":      map[string]any{"type": "string", "description": "Node ID of the next exercise in the chain; shown as a 'Next →' button when this exercise passes"},
				"canvas_id":    map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"name"},
		},
	},
	{
		"name":        "add_test_node",
		"description": "Create a TestNode that runs a SQL query on a schedule or manually and validates the result against configured checks. Use for data quality monitoring and automated assertions against pipeline outputs.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Name for the test node"},
				"sql":       map[string]any{"type": "string", "description": "SQL query to run as the test"},
				"interval":  map[string]any{"type": "number", "description": "Auto-run interval in seconds; 0 = manual only (default 0)"},
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"name"},
		},
	},
	{
		"name":        "add_section",
		"description": "Create a named Section container on the canvas to visually group related nodes. Sections appear behind other nodes.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Label for the section"},
				"width":     map[string]any{"type": "number", "description": "Width in canvas pixels (default 400)"},
				"height":    map[string]any{"type": "number", "description": "Height in canvas pixels (default 300)"},
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to add the section to; default is the active canvas"},
			},
			"required": []string{"name"},
		},
	},
	{
		"name":        "clear_canvas",
		"description": "Remove all nodes from a canvas. If canvas_id is omitted, clears the active canvas.",
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to clear (from list_canvases); omit to clear the active canvas"},
			},
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
