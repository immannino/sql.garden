// mcp-test runs Red/Green integration scenarios against the sql.garden MCP server.
//
// Usage:
//
//	go run ./cmd/mcp-test          # run all scenarios
//	go run ./cmd/mcp-test <filter> # run scenarios whose names contain <filter>
//
// The sql.garden app must be running before invoking this tool.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ── Config ────────────────────────────────────────────────────────────────────

const (
	mcpURL    = "http://127.0.0.1:37421/mcp"
	streamURL = "http://127.0.0.1:37421/canvas-stream"
	healthURL = "http://127.0.0.1:37421/health"

	// sseWait is how long to wait for canvas-stream events after a tool call.
	sseWait = 350 * time.Millisecond
)

// testdataDir resolves to the repo's testdata/ directory regardless of where
// the binary is invoked from.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// ── ANSI ──────────────────────────────────────────────────────────────────────

const (
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

// ── Canvas-action SSE buffer ──────────────────────────────────────────────────

type canvasAction struct {
	Type         string  `json:"type"`
	NodeID       string  `json:"nodeId"`
	CanvasID     string  `json:"canvasId"`
	Name         string  `json:"name"`
	SQL          string  `json:"sql"`
	Content      string  `json:"content"`
	SourceID     string  `json:"sourceId"`
	ChartType    string  `json:"chartType"`
	XColumn      string  `json:"xColumn"`
	YColumn      string  `json:"yColumn"`
	RowCount     int64   `json:"rowCount"`
	Width        float64 `json:"width"`
	Height       float64 `json:"height"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Mode         string  `json:"mode"`
	URL          string  `json:"url"`
	TargetTable  string  `json:"targetTable"`
	ConflictMode string  `json:"conflictMode"`
	Interval     float64 `json:"interval"`
	SuccessText  string  `json:"successText"`
	NextID       string  `json:"nextId"`
}

type actionBuffer struct {
	mu      sync.Mutex
	entries []tsAction
}

type tsAction struct {
	at     time.Time
	action canvasAction
}

func (b *actionBuffer) add(a canvasAction) {
	b.mu.Lock()
	b.entries = append(b.entries, tsAction{at: time.Now(), action: a})
	b.mu.Unlock()
}

func (b *actionBuffer) since(t time.Time) []canvasAction {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []canvasAction
	for _, e := range b.entries {
		if !e.at.Before(t) {
			out = append(out, e.action)
		}
	}
	return out
}

// subscribeSSE opens the canvas-stream and feeds events into buf until the
// returned cancel func is called.
func subscribeSSE(buf *actionBuffer) (cancel func(), err error) {
	resp, err := http.Get(streamURL)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to canvas-stream: %w", err)
	}
	done := make(chan struct{})
	go func() {
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		var dataLine string
		for sc.Scan() {
			select {
			case <-done:
				return
			default:
			}
			line := sc.Text()
			if rest, ok := strings.CutPrefix(line, "data: "); ok {
				dataLine = rest
			} else if line == "" && dataLine != "" {
				var ca canvasAction
				if json.Unmarshal([]byte(dataLine), &ca) == nil {
					buf.add(ca)
				}
				dataLine = ""
			}
		}
		_ = sc.Err()
	}()
	return func() { close(done) }, nil
}

// ── MCP JSON-RPC helpers ──────────────────────────────────────────────────────

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

var rpcID int

func callTool(name string, args map[string]any) (string, error) {
	rpcID++
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      rpcID,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	resp, err := http.Post(mcpURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	var r rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode error: %w", err)
	}
	if r.Error != nil {
		return "", fmt.Errorf("rpc error: %s", r.Error.Message)
	}

	// Unwrap result.content[0].text
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(r.Result, &result); err != nil || len(result.Content) == 0 {
		return string(r.Result), nil
	}
	return result.Content[0].Text, nil
}

func toolsList() ([]string, error) {
	rpcID++
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      rpcID,
		"method":  "tools/list",
	})
	resp, err := http.Post(mcpURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r struct {
		Result struct {
			Tools []struct{ Name string `json:"name"` } `json:"tools"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&r) //nolint:errcheck
	var names []string
	for _, t := range r.Result.Tools {
		names = append(names, t.Name)
	}
	return names, nil
}

// ── Runner ────────────────────────────────────────────────────────────────────

type runner struct {
	buf    *actionBuffer
	filter string
	pass   int
	fail   int
	skip   int
}

// run executes one named scenario. fn must return nil for Green, non-nil for Red.
func (r *runner) run(name string, fn func() error) {
	if r.filter != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(r.filter)) {
		r.skip++
		return
	}
	err := fn()
	if err != nil {
		r.fail++
		fmt.Printf("  %s%s✗%s %s\n    %s→ %s%s\n", bold, red, reset, name, red, err, reset)
	} else {
		r.pass++
		fmt.Printf("  %s✓%s %s\n", green, reset, name)
	}
}

// call is a convenience that invokes a tool, waits for SSE events, and returns
// both the text response and any canvas actions emitted after the call started.
func (r *runner) call(tool string, args map[string]any) (resp string, actions []canvasAction, err error) {
	before := time.Now()
	resp, err = callTool(tool, args)
	if err != nil {
		return
	}
	time.Sleep(sseWait)
	actions = r.buf.since(before)
	return
}

// findAction returns the first action in the slice with the given type, or nil.
func findAction(actions []canvasAction, typ string) *canvasAction {
	for i := range actions {
		if actions[i].Type == typ {
			return &actions[i]
		}
	}
	return nil
}

// extractCanvasID pulls the canvas id from a create_canvas response string.
// The response format is: …id="<id>"…
func extractCanvasID(resp string) string {
	const marker = `id="`
	idx := strings.Index(resp, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	end := strings.Index(resp[start:], `"`)
	if end < 0 {
		return ""
	}
	return resp[start : start+end]
}

// ── Scenarios ─────────────────────────────────────────────────────────────────

func (r *runner) runAll() {
	td := testdataDir()

	// ── Infrastructure ────────────────────────────────────────────────────────

	r.run("health endpoint returns 200", func() error {
		resp, err := http.Get(healthURL)
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("want 200, got %d", resp.StatusCode)
		}
		return nil
	})

	r.run("tools/list returns all expected tools", func() error {
		names, err := toolsList()
		if err != nil {
			return err
		}
		required := []string{
			"list_tables", "list_canvases", "list_canvas_nodes", "run_query",
			"create_canvas", "rename_canvas", "switch_canvas", "remove_canvas",
			"add_query_node", "add_chart_node", "add_markdown_node",
			"materialize_query",
			"import_file", "import_csv_data", "import_url", "import_s3",
			"resize_node", "move_node",
			"focus_node", "update_query_node",
			"set_node_color", "add_section", "add_ingest_node",
			"add_exercise_node", "add_test_node",
			"clear_canvas", "fit_view",
		}
		nameSet := make(map[string]bool, len(names))
		for _, n := range names {
			nameSet[n] = true
		}
		var missing []string
		for _, req := range required {
			if !nameSet[req] {
				missing = append(missing, req)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("missing tools: %s", strings.Join(missing, ", "))
		}
		return nil
	})

	// ── Data tools ────────────────────────────────────────────────────────────

	r.run("list_tables returns a string response", func() error {
		resp, _, err := r.call("list_tables", nil)
		if err != nil {
			return err
		}
		if resp == "" {
			return fmt.Errorf("empty response")
		}
		return nil
	})

	r.run("run_query executes SQL and returns rows", func() error {
		resp, _, err := r.call("run_query", map[string]any{"sql": "SELECT 42 AS answer"})
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "42") {
			return fmt.Errorf("expected 42 in response, got: %q", resp)
		}
		return nil
	})

	r.run("run_query error surfaces cleanly", func() error {
		resp, _, err := r.call("run_query", map[string]any{"sql": "SELECT * FROM nonexistent_xyz_table"})
		if err != nil {
			return err // RPC-level error is unexpected
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error message in response, got: %q", resp)
		}
		return nil
	})

	// ── Canvas: query node ────────────────────────────────────────────────────

	r.run("add_query_node emits canvas action with stable nodeId", func() error {
		resp, actions, err := r.call("add_query_node", map[string]any{
			"name": "Test Revenue",
			"sql":  "SELECT 1 AS revenue",
		})
		if err != nil {
			return err
		}
		wantID := "mcp_test_revenue"
		if !strings.Contains(resp, wantID) {
			return fmt.Errorf("response missing nodeId %q: %q", wantID, resp)
		}
		act := findAction(actions, "query")
		if act == nil {
			return fmt.Errorf("no 'query' canvas action received (got %d actions)", len(actions))
		}
		if act.NodeID != wantID {
			return fmt.Errorf("want nodeId=%q, got %q", wantID, act.NodeID)
		}
		if act.Name != "Test Revenue" {
			return fmt.Errorf("want name=%q, got %q", "Test Revenue", act.Name)
		}
		return nil
	})

	r.run("add_query_node with same name is idempotent (dedup guard)", func() error {
		// The store dedup guard should make a second add_query_node with the
		// same name a no-op on the canvas (same id → skipped).
		_, actions1, err := r.call("add_query_node", map[string]any{
			"name": "Dedup Test",
			"sql":  "SELECT 1",
		})
		if err != nil {
			return err
		}
		_, actions2, err := r.call("add_query_node", map[string]any{
			"name": "Dedup Test",
			"sql":  "SELECT 1",
		})
		if err != nil {
			return err
		}
		// Both emit a canvas action (the dedup happens in the frontend store,
		// not here) — just verify both calls succeed.
		if findAction(actions1, "query") == nil {
			return fmt.Errorf("first call emitted no canvas action")
		}
		if findAction(actions2, "query") == nil {
			return fmt.Errorf("second call emitted no canvas action")
		}
		return nil
	})

	// ── Canvas: chart node ────────────────────────────────────────────────────

	r.run("add_chart_node with inline SQL emits canvas action", func() error {
		_, actions, err := r.call("add_chart_node", map[string]any{
			"name":       "Test Bar Chart",
			"sql":        "SELECT 'A' AS category, 10 AS value",
			"chart_type": "barY",
			"x_column":   "category",
			"y_column":   "value",
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "chart")
		if act == nil {
			return fmt.Errorf("no 'chart' canvas action received")
		}
		if act.ChartType != "barY" {
			return fmt.Errorf("want chartType=barY, got %q", act.ChartType)
		}
		return nil
	})

	r.run("add_chart_node with source_id sets sourceId on action", func() error {
		// First, create a query node to reference.
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Source Query",
			"sql":  "SELECT 'X' AS cat, 5 AS val",
		})
		if err != nil {
			return err
		}
		sourceID := "mcp_source_query"

		_, actions, err := r.call("add_chart_node", map[string]any{
			"name":       "Linked Chart",
			"source_id":  sourceID,
			"chart_type": "barY",
			"x_column":   "cat",
			"y_column":   "val",
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "chart")
		if act == nil {
			return fmt.Errorf("no 'chart' canvas action received")
		}
		if act.SourceID != sourceID {
			return fmt.Errorf("want sourceId=%q, got %q", sourceID, act.SourceID)
		}
		if act.SQL != "" {
			return fmt.Errorf("expected empty sql when source_id provided, got %q", act.SQL)
		}
		return nil
	})

	r.run("add_chart_node number type emits correct chartType", func() error {
		_, actions, err := r.call("add_chart_node", map[string]any{
			"name":       "KPI Total",
			"sql":        "SELECT SUM(1) AS total",
			"chart_type": "number",
			"x_column":   "total",
			"y_column":   "total",
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "chart")
		if act == nil {
			return fmt.Errorf("no 'chart' canvas action received")
		}
		if act.ChartType != "number" {
			return fmt.Errorf("want chartType=number, got %q", act.ChartType)
		}
		return nil
	})

	// ── Canvas: markdown node ─────────────────────────────────────────────────

	r.run("add_markdown_node emits canvas action with content", func() error {
		content := "# Analysis\n\nThis dashboard shows **revenue trends**."
		_, actions, err := r.call("add_markdown_node", map[string]any{
			"name":    "Analysis Note",
			"content": content,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "markdown")
		if act == nil {
			return fmt.Errorf("no 'markdown' canvas action received")
		}
		if act.Content != content {
			return fmt.Errorf("content mismatch:\nwant: %q\n got: %q", content, act.Content)
		}
		if act.NodeID != "mcp_analysis_note" {
			return fmt.Errorf("want nodeId=mcp_analysis_note, got %q", act.NodeID)
		}
		return nil
	})

	// ── Import ────────────────────────────────────────────────────────────────

	r.run("import_file (sales.csv) emits table action with columns + rowCount", func() error {
		path := filepath.Join(td, "sales.csv")
		resp, actions, err := r.call("import_file", map[string]any{
			"path":       path,
			"table_name": "mcp_test_sales",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action received")
		}
		if act.RowCount != 24 {
			return fmt.Errorf("want 24 rows, got %d", act.RowCount)
		}
		if len(act.Name) == 0 {
			return fmt.Errorf("table action missing name")
		}
		return nil
	})

	r.run("import_file (service_health.csv) emits table action with rowCount 10", func() error {
		path := filepath.Join(td, "service_health.csv")
		resp, actions, err := r.call("import_file", map[string]any{
			"path":       path,
			"table_name": "mcp_test_svc_health",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action received")
		}
		if act.RowCount != 10 {
			return fmt.Errorf("want 10 rows, got %d", act.RowCount)
		}
		return nil
	})

	r.run("import_file bad path returns error message", func() error {
		resp, _, err := r.call("import_file", map[string]any{
			"path":       "/nonexistent/path/data.csv",
			"table_name": "bad_import",
		})
		if err != nil {
			return err // unexpected RPC error
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error in response for bad path, got: %q", resp)
		}
		return nil
	})

	// ── Canvas discovery ──────────────────────────────────────────────────────

	r.run("list_canvas_nodes returns query nodes added this session", func() error {
		// Add a fresh node with a unique name to guarantee it appears.
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "List Test Node",
			"sql":  "SELECT 99 AS n",
		})
		if err != nil {
			return err
		}
		resp, _, err := r.call("list_canvas_nodes", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "mcp_list_test_node") {
			return fmt.Errorf("expected mcp_list_test_node in list, got: %q", resp)
		}
		return nil
	})

	// ── clear_canvas ──────────────────────────────────────────────────────────

	r.run("clear_canvas emits 'clear' action", func() error {
		_, actions, err := r.call("clear_canvas", nil)
		if err != nil {
			return err
		}
		act := findAction(actions, "clear")
		if act == nil {
			return fmt.Errorf("no 'clear' canvas action received")
		}
		return nil
	})

	// ── fit_view ──────────────────────────────────────────────────────────────

	r.run("fit_view (fit) emits fit_view action with name=fit", func() error {
		_, actions, err := r.call("fit_view", map[string]any{"mode": "fit"})
		if err != nil {
			return err
		}
		act := findAction(actions, "fit_view")
		if act == nil {
			return fmt.Errorf("no 'fit_view' canvas action received")
		}
		if act.Name != "fit" {
			return fmt.Errorf("want name=fit, got %q", act.Name)
		}
		return nil
	})

	r.run("fit_view (reset) emits fit_view action with name=reset", func() error {
		_, actions, err := r.call("fit_view", map[string]any{"mode": "reset"})
		if err != nil {
			return err
		}
		act := findAction(actions, "fit_view")
		if act == nil {
			return fmt.Errorf("no 'fit_view' canvas action received")
		}
		if act.Name != "reset" {
			return fmt.Errorf("want name=reset, got %q", act.Name)
		}
		return nil
	})

	r.run("fit_view (default) emits fit_view action with name=fit", func() error {
		_, actions, err := r.call("fit_view", nil)
		if err != nil {
			return err
		}
		act := findAction(actions, "fit_view")
		if act == nil {
			return fmt.Errorf("no 'fit_view' canvas action received")
		}
		if act.Name != "fit" {
			return fmt.Errorf("want name=fit for default mode, got %q", act.Name)
		}
		return nil
	})

	// ── import_csv_data ───────────────────────────────────────────────────────

	r.run("import_csv_data: basic 3-column CSV emits table action with rowCount=3", func() error {
		csv := "name,age,city\nAlice,30,New York\nBob,25,Los Angeles\nCharlie,35,Chicago"
		resp, actions, err := r.call("import_csv_data", map[string]any{
			"csv_text":   csv,
			"table_name": "mcp_test_csv_basic",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action emitted (response: %q)", resp)
		}
		if act.RowCount != 3 {
			return fmt.Errorf("want rowCount=3, got %d", act.RowCount)
		}
		if act.Name != "mcp_test_csv_basic" {
			return fmt.Errorf("want name=mcp_test_csv_basic, got %q", act.Name)
		}
		return nil
	})

	r.run("import_csv_data: CRLF line endings normalized correctly (2 rows)", func() error {
		csv := "product,sales\r\nWidgets,100\r\nGadgets,200\r\n"
		resp, actions, err := r.call("import_csv_data", map[string]any{
			"csv_text":   csv,
			"table_name": "mcp_test_csv_crlf",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action emitted (response: %q)", resp)
		}
		if act.RowCount != 2 {
			return fmt.Errorf("want rowCount=2, got %d", act.RowCount)
		}
		return nil
	})

	r.run("import_csv_data: quoted fields with commas parse correctly (3 rows)", func() error {
		csv := `date,description,amount` + "\n" +
			`2024-01-15,"Coffee, oat milk latte",4.75` + "\n" +
			`2024-01-16,"Lunch, sandwich + chips",12.50` + "\n" +
			`2024-01-17,Groceries,87.32`
		resp, actions, err := r.call("import_csv_data", map[string]any{
			"csv_text":   csv,
			"table_name": "mcp_test_csv_quoted",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action emitted (response: %q)", resp)
		}
		if act.RowCount != 3 {
			return fmt.Errorf("want rowCount=3, got %d", act.RowCount)
		}
		return nil
	})

	r.run("import_csv_data: missing csv_text returns error (no canvas action)", func() error {
		resp, actions, err := r.call("import_csv_data", map[string]any{
			"table_name": "mcp_test_csv_nodata",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing csv_text, got: %q", resp)
		}
		if act := findAction(actions, "table"); act != nil {
			return fmt.Errorf("expected no canvas action for failed import")
		}
		return nil
	})

	// ── import_url ────────────────────────────────────────────────────────────

	r.run("import_url: local HTTP server CSV emits table action with rowCount=4", func() error {
		csvData := "ticker,price,volume\nAAPL,182.50,52000000\nGOOGL,141.23,18000000\nMSFT,378.85,22000000\nAMZN,186.40,31000000\n"
		mux := http.NewServeMux()
		mux.HandleFunc("/data.csv", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			fmt.Fprint(w, csvData)
		})
		ln, lnErr := net.Listen("tcp", "127.0.0.1:0")
		if lnErr != nil {
			return fmt.Errorf("could not start test server: %w", lnErr)
		}
		srv := &http.Server{Handler: mux}
		go srv.Serve(ln) //nolint:errcheck
		defer srv.Close()

		testURL := fmt.Sprintf("http://127.0.0.1:%d/data.csv", ln.Addr().(*net.TCPAddr).Port)
		resp, actions, err := r.call("import_url", map[string]any{
			"url":        testURL,
			"table_name": "mcp_test_url_stocks",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action emitted (response: %q)", resp)
		}
		if act.RowCount != 4 {
			return fmt.Errorf("want rowCount=4, got %d", act.RowCount)
		}
		return nil
	})

	r.run("import_url: unreachable host returns error (no canvas action)", func() error {
		resp, actions, err := r.call("import_url", map[string]any{
			"url":        "http://127.0.0.1:19999/nonexistent.csv",
			"table_name": "mcp_test_url_bad",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for unreachable URL, got: %q", resp)
		}
		if act := findAction(actions, "table"); act != nil {
			return fmt.Errorf("expected no canvas action for failed import, got one")
		}
		return nil
	})

	r.run("import_url: missing url param returns error", func() error {
		resp, _, err := r.call("import_url", map[string]any{
			"table_name": "mcp_test_url_nourl",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing url, got: %q", resp)
		}
		return nil
	})

	// ── import_s3 ────────────────────────────────────────────────────────────

	r.run("import_s3: missing s3_url returns error", func() error {
		resp, _, err := r.call("import_s3", map[string]any{
			"table_name": "mcp_test_s3_nourl",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing s3_url, got: %q", resp)
		}
		return nil
	})

	r.run("import_s3: non-s3:// URL returns error", func() error {
		resp, actions, err := r.call("import_s3", map[string]any{
			"s3_url":     "https://example.com/file.parquet",
			"table_name": "mcp_test_s3_badscheme",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for non-s3:// URL, got: %q", resp)
		}
		if act := findAction(actions, "table"); act != nil {
			return fmt.Errorf("expected no canvas action for failed import, got one")
		}
		return nil
	})

	r.run("import_s3: missing table_name returns error", func() error {
		resp, _, err := r.call("import_s3", map[string]any{
			"s3_url": "s3://my-bucket/data.parquet",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing table_name, got: %q", resp)
		}
		return nil
	})

	// ── resize_node ───────────────────────────────────────────────────────────

	r.run("resize_node emits resize_node action with correct width/height", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Resize Target",
			"sql":  "SELECT 1 AS val",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_resize_target"

		resp, actions, err := r.call("resize_node", map[string]any{
			"node_id": nodeID,
			"width":   640.0,
			"height":  480.0,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("resize returned error: %q", resp)
		}
		act := findAction(actions, "resize_node")
		if act == nil {
			return fmt.Errorf("no 'resize_node' canvas action emitted (response: %q)", resp)
		}
		if act.NodeID != nodeID {
			return fmt.Errorf("want nodeId=%q, got %q", nodeID, act.NodeID)
		}
		if act.Width != 640 {
			return fmt.Errorf("want width=640, got %g", act.Width)
		}
		if act.Height != 480 {
			return fmt.Errorf("want height=480, got %g", act.Height)
		}
		return nil
	})

	r.run("resize_node: missing node_id returns error", func() error {
		resp, _, err := r.call("resize_node", map[string]any{
			"width":  300.0,
			"height": 200.0,
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing node_id, got: %q", resp)
		}
		return nil
	})

	// ── move_node ─────────────────────────────────────────────────────────────

	r.run("move_node emits move_node action with correct coordinates", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Move Target",
			"sql":  "SELECT 2 AS val",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_move_target"

		resp, actions, err := r.call("move_node", map[string]any{
			"node_id": nodeID,
			"x":       150.0,
			"y":       300.0,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("move returned error: %q", resp)
		}
		act := findAction(actions, "move_node")
		if act == nil {
			return fmt.Errorf("no 'move_node' canvas action emitted (response: %q)", resp)
		}
		if act.NodeID != nodeID {
			return fmt.Errorf("want nodeId=%q, got %q", nodeID, act.NodeID)
		}
		if act.X != 150 {
			return fmt.Errorf("want x=150, got %g", act.X)
		}
		if act.Y != 300 {
			return fmt.Errorf("want y=300, got %g", act.Y)
		}
		return nil
	})

	r.run("move_node: missing node_id returns error", func() error {
		resp, _, err := r.call("move_node", map[string]any{
			"x": 100.0,
			"y": 200.0,
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing node_id, got: %q", resp)
		}
		return nil
	})

	// ── import_csv_data: column-count mismatch (backlog gap) ──────────────────

	r.run("import_csv_data: header/data column count mismatch returns actionable error", func() error {
		// Header has 4 columns but every data row has only 3.
		csv := "name,age,city,country\nAlice,30,New York\nBob,25,London"
		resp, actions, err := r.call("import_csv_data", map[string]any{
			"csv_text":   csv,
			"table_name": "mcp_test_csv_mismatch",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "mismatch") {
			return fmt.Errorf("expected 'mismatch' in error, got: %q", resp)
		}
		if !strings.Contains(resp, "row 2") {
			return fmt.Errorf("expected row number in error, got: %q", resp)
		}
		if act := findAction(actions, "table"); act != nil {
			return fmt.Errorf("expected no canvas action for malformed CSV, got one")
		}
		return nil
	})

	// ── canvas-stream reconnect (backlog gap) ─────────────────────────────────

	r.run("canvas-stream: new subscriber receives events after a prior connection was closed", func() error {
		// Open a second SSE connection, close it, open a third one, then verify
		// that events still flow to the third — tests that the sync.Map lifecycle
		// correctly removes closed subscribers and adds new ones.
		buf2 := &actionBuffer{}
		cancel2, err := subscribeSSE(buf2)
		if err != nil {
			return fmt.Errorf("second subscribe: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
		cancel2() // simulate disconnect
		time.Sleep(100 * time.Millisecond)

		buf3 := &actionBuffer{}
		cancel3, err := subscribeSSE(buf3)
		if err != nil {
			return fmt.Errorf("third subscribe: %w", err)
		}
		defer cancel3()
		time.Sleep(100 * time.Millisecond)

		before := time.Now()
		if _, err := callTool("fit_view", map[string]any{"mode": "fit"}); err != nil {
			return err
		}
		time.Sleep(sseWait)
		if findAction(buf3.since(before), "fit_view") == nil {
			return fmt.Errorf("reconnected SSE client did not receive canvas action")
		}
		return nil
	})

	// ── import_url: Content-Type fallback (backlog gap) ───────────────────────

	r.run("import_url: CSV served without file extension uses Content-Type fallback", func() error {
		csvData := "country,gdp_trillion,population_m\nUSA,25.46,331\nChina,17.73,1412\nGermany,4.07,84\n"
		mux := http.NewServeMux()
		mux.HandleFunc("/api/export", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			fmt.Fprint(w, csvData)
		})
		ln, lnErr := net.Listen("tcp", "127.0.0.1:0")
		if lnErr != nil {
			return fmt.Errorf("start server: %w", lnErr)
		}
		srv := &http.Server{Handler: mux}
		go srv.Serve(ln) //nolint:errcheck
		defer srv.Close()

		testURL := fmt.Sprintf("http://127.0.0.1:%d/api/export", ln.Addr().(*net.TCPAddr).Port)
		resp, actions, err := r.call("import_url", map[string]any{
			"url":        testURL,
			"table_name": "mcp_test_url_ct_fallback",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("import reported error: %q", resp)
		}
		act := findAction(actions, "table")
		if act == nil {
			return fmt.Errorf("no 'table' canvas action emitted (response: %q)", resp)
		}
		if act.RowCount != 3 {
			return fmt.Errorf("want rowCount=3, got %d", act.RowCount)
		}
		return nil
	})

	// ── batch resize (backlog gap) ────────────────────────────────────────────

	r.run("resize_node: batch of 4 nodes all emit distinct resize_node actions", func() error {
		type batchNode struct{ name, sql, id string }
		batch := []batchNode{
			{"Batch A", "SELECT 1 AS a", "mcp_batch_a"},
			{"Batch B", "SELECT 2 AS b", "mcp_batch_b"},
			{"Batch C", "SELECT 3 AS c", "mcp_batch_c"},
			{"Batch D", "SELECT 4 AS d", "mcp_batch_d"},
		}
		for _, n := range batch {
			if _, _, err := r.call("add_query_node", map[string]any{"name": n.name, "sql": n.sql}); err != nil {
				return fmt.Errorf("adding %s: %w", n.name, err)
			}
		}
		before := time.Now()
		sizes := [][2]float64{{320, 240}, {480, 360}, {640, 480}, {320, 180}}
		for i, n := range batch {
			if _, _, err := r.call("resize_node", map[string]any{
				"node_id": n.id,
				"width":   sizes[i][0],
				"height":  sizes[i][1],
			}); err != nil {
				return fmt.Errorf("resizing %s: %w", n.name, err)
			}
		}
		var resizes []canvasAction
		for _, a := range r.buf.since(before) {
			if a.Type == "resize_node" {
				resizes = append(resizes, a)
			}
		}
		if len(resizes) < 4 {
			return fmt.Errorf("want 4 resize_node actions, got %d", len(resizes))
		}
		return nil
	})

	// ── focus_node ────────────────────────────────────────────────────────────

	r.run("focus_node emits focus_node action with correct nodeId", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Focus Target",
			"sql":  "SELECT 1 AS val",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_focus_target"
		resp, actions, err := r.call("focus_node", map[string]any{"node_id": nodeID})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("focus_node returned error: %q", resp)
		}
		act := findAction(actions, "focus_node")
		if act == nil {
			return fmt.Errorf("no 'focus_node' canvas action emitted (response: %q)", resp)
		}
		if act.NodeID != nodeID {
			return fmt.Errorf("want nodeId=%q, got %q", nodeID, act.NodeID)
		}
		return nil
	})

	r.run("focus_node: missing node_id returns error", func() error {
		resp, _, err := r.call("focus_node", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing node_id, got: %q", resp)
		}
		return nil
	})

	// ── update_query_node ─────────────────────────────────────────────────────

	r.run("update_query_node emits update_query action with new SQL", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Update Target",
			"sql":  "SELECT 1 AS original",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_update_target"
		newSQL := "SELECT 42 AS updated, 'hello' AS msg"
		resp, actions, err := r.call("update_query_node", map[string]any{
			"node_id": nodeID,
			"sql":     newSQL,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("update_query_node returned error: %q", resp)
		}
		act := findAction(actions, "update_query")
		if act == nil {
			return fmt.Errorf("no 'update_query' canvas action emitted (response: %q)", resp)
		}
		if act.NodeID != nodeID {
			return fmt.Errorf("want nodeId=%q, got %q", nodeID, act.NodeID)
		}
		if act.SQL != newSQL {
			return fmt.Errorf("want sql=%q, got %q", newSQL, act.SQL)
		}
		return nil
	})

	r.run("update_query_node with name sets name on action", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Rename Me Query",
			"sql":  "SELECT 1",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_rename_me_query"
		resp, actions, err := r.call("update_query_node", map[string]any{
			"node_id": nodeID,
			"sql":     "SELECT 2 AS renamed",
			"name":    "Renamed Query",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("update returned error: %q", resp)
		}
		act := findAction(actions, "update_query")
		if act == nil {
			return fmt.Errorf("no 'update_query' canvas action emitted (response: %q)", resp)
		}
		if act.Name != "Renamed Query" {
			return fmt.Errorf("want name=%q, got %q", "Renamed Query", act.Name)
		}
		return nil
	})

	r.run("update_query_node: missing sql returns error", func() error {
		resp, _, err := r.call("update_query_node", map[string]any{
			"node_id": "mcp_update_target",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing sql, got: %q", resp)
		}
		return nil
	})

	r.run("update_query_node: missing node_id returns error", func() error {
		resp, _, err := r.call("update_query_node", map[string]any{
			"sql": "SELECT 1",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing node_id, got: %q", resp)
		}
		return nil
	})

	// ── materialize_query ─────────────────────────────────────────────────────

	r.run("materialize_query: emits data canvas action with correct rowCount and nodeId", func() error {
		resp, actions, err := r.call("materialize_query", map[string]any{
			"table_name": "mat_test",
			"sql":        "SELECT 1 AS a UNION ALL SELECT 2 UNION ALL SELECT 3",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("materialize reported error: %q", resp)
		}
		wantID := "mcp_mat_test"
		if !strings.Contains(resp, wantID) {
			return fmt.Errorf("response missing nodeId %q: %q", wantID, resp)
		}
		act := findAction(actions, "data")
		if act == nil {
			return fmt.Errorf("no 'data' canvas action received (response: %q)", resp)
		}
		if act.RowCount != 3 {
			return fmt.Errorf("want rowCount=3, got %d", act.RowCount)
		}
		if act.NodeID != wantID {
			return fmt.Errorf("want nodeId=%q, got %q", wantID, act.NodeID)
		}
		return nil
	})

	r.run("materialize_query: trailing semicolon stripped — no error", func() error {
		resp, actions, err := r.call("materialize_query", map[string]any{
			"table_name": "mat_semi",
			"sql":        "SELECT 'hello' AS msg;",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("semicolon should be stripped, got error: %q", resp)
		}
		if findAction(actions, "data") == nil {
			return fmt.Errorf("no 'data' canvas action received (response: %q)", resp)
		}
		return nil
	})

	r.run("materialize_query: source_id propagates to canvas action", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Mat Source",
			"sql":  "SELECT 1 AS v",
		})
		if err != nil {
			return err
		}
		sourceID := "mcp_mat_source"
		_, actions, err := r.call("materialize_query", map[string]any{
			"table_name": "mat_linked",
			"sql":        "SELECT 1 AS v",
			"source_id":  sourceID,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "data")
		if act == nil {
			return fmt.Errorf("no 'data' canvas action received")
		}
		if act.SourceID != sourceID {
			return fmt.Errorf("want sourceId=%q, got %q", sourceID, act.SourceID)
		}
		return nil
	})

	r.run("materialize_query: missing table_name returns error", func() error {
		resp, _, err := r.call("materialize_query", map[string]any{
			"sql": "SELECT 1",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing table_name, got: %q", resp)
		}
		return nil
	})

	r.run("materialize_query: missing sql returns error", func() error {
		resp, _, err := r.call("materialize_query", map[string]any{
			"table_name": "mat_nosql",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing sql, got: %q", resp)
		}
		return nil
	})

	r.run("list_canvas_nodes: data node appears with 'data' kind after materialize_query", func() error {
		_, _, err := r.call("materialize_query", map[string]any{
			"table_name": "mat_list_check",
			"sql":        "SELECT 1 AS n",
		})
		if err != nil {
			return err
		}
		resp, _, err := r.call("list_canvas_nodes", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "mcp_mat_list_check") {
			return fmt.Errorf("expected mcp_mat_list_check in list, got: %q", resp)
		}
		if !strings.Contains(resp, "data") {
			return fmt.Errorf("expected 'data' kind in list output, got: %q", resp)
		}
		return nil
	})

	// ── set_node_color ────────────────────────────────────────────────────────

	r.run("set_node_color: emits set_color action with correct nodeId and color", func() error {
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Color Target",
			"sql":  "SELECT 1",
		})
		if err != nil {
			return err
		}
		nodeID := "mcp_color_target"
		wantColor := "#ef4444"
		resp, actions, err := r.call("set_node_color", map[string]any{
			"node_id": nodeID,
			"color":   wantColor,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("set_node_color returned error: %q", resp)
		}
		act := findAction(actions, "set_color")
		if act == nil {
			return fmt.Errorf("no 'set_color' canvas action received (response: %q)", resp)
		}
		if act.NodeID != nodeID {
			return fmt.Errorf("want nodeId=%q, got %q", nodeID, act.NodeID)
		}
		if act.Name != wantColor {
			return fmt.Errorf("want color=%q in Name, got %q", wantColor, act.Name)
		}
		return nil
	})

	r.run("set_node_color: missing node_id returns error", func() error {
		resp, _, err := r.call("set_node_color", map[string]any{"color": "#3b82f6"})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing node_id, got: %q", resp)
		}
		return nil
	})

	r.run("set_node_color: missing color returns error", func() error {
		resp, _, err := r.call("set_node_color", map[string]any{"node_id": "mcp_color_target"})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing color, got: %q", resp)
		}
		return nil
	})

	// ── add_section ───────────────────────────────────────────────────────────

	r.run("add_section: emits section action with name and dimensions", func() error {
		resp, actions, err := r.call("add_section", map[string]any{
			"name":   "Test Group",
			"width":  600.0,
			"height": 400.0,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("add_section returned error: %q", resp)
		}
		act := findAction(actions, "section")
		if act == nil {
			return fmt.Errorf("no 'section' canvas action received (response: %q)", resp)
		}
		if act.Name != "Test Group" {
			return fmt.Errorf("want name=%q, got %q", "Test Group", act.Name)
		}
		if act.NodeID != "mcp_test_group" {
			return fmt.Errorf("want nodeId=mcp_test_group, got %q", act.NodeID)
		}
		if act.Width != 600 {
			return fmt.Errorf("want width=600, got %g", act.Width)
		}
		if act.Height != 400 {
			return fmt.Errorf("want height=400, got %g", act.Height)
		}
		return nil
	})

	r.run("add_section: default dimensions when omitted", func() error {
		_, actions, err := r.call("add_section", map[string]any{"name": "Defaults Group"})
		if err != nil {
			return err
		}
		act := findAction(actions, "section")
		if act == nil {
			return fmt.Errorf("no 'section' canvas action received")
		}
		if act.Width != 400 {
			return fmt.Errorf("want default width=400, got %g", act.Width)
		}
		if act.Height != 300 {
			return fmt.Errorf("want default height=300, got %g", act.Height)
		}
		return nil
	})

	r.run("add_section: missing name returns error", func() error {
		resp, _, err := r.call("add_section", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing name, got: %q", resp)
		}
		return nil
	})

	// ── add_ingest_node ────────────────────────────────────────────────────────

	r.run("add_ingest_node: generator mode emits ingest action", func() error {
		resp, actions, err := r.call("add_ingest_node", map[string]any{
			"name": "tick_generator",
			"mode": "generator",
			"sql":  "INSERT INTO ticks VALUES (now(), random())",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("add_ingest_node returned error: %q", resp)
		}
		if len(actions) == 0 {
			return fmt.Errorf("expected canvas action, got none")
		}
		a := actions[0]
		if a.Type != "ingest" {
			return fmt.Errorf("expected type=ingest, got %q", a.Type)
		}
		if a.Name != "tick_generator" {
			return fmt.Errorf("expected name=tick_generator, got %q", a.Name)
		}
		if a.Mode != "generator" {
			return fmt.Errorf("expected mode=generator, got %q", a.Mode)
		}
		return nil
	})

	r.run("add_ingest_node: ingestion mode emits ingest action with url fields", func() error {
		resp, actions, err := r.call("add_ingest_node", map[string]any{
			"name":          "prices_feed",
			"mode":          "ingestion",
			"url":           "https://example.com/prices.csv",
			"target_table":  "prices",
			"conflict_mode": "replace",
			"interval":      60.0,
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("add_ingest_node returned error: %q", resp)
		}
		if len(actions) == 0 {
			return fmt.Errorf("expected canvas action, got none")
		}
		a := actions[0]
		if a.URL != "https://example.com/prices.csv" {
			return fmt.Errorf("expected url=https://example.com/prices.csv, got %q", a.URL)
		}
		if a.TargetTable != "prices" {
			return fmt.Errorf("expected target_table=prices, got %q", a.TargetTable)
		}
		if a.ConflictMode != "replace" {
			return fmt.Errorf("expected conflict_mode=replace, got %q", a.ConflictMode)
		}
		if a.Interval != 60.0 {
			return fmt.Errorf("expected interval=60, got %g", a.Interval)
		}
		return nil
	})

	r.run("add_ingest_node: ingestion mode without url returns error", func() error {
		resp, _, err := r.call("add_ingest_node", map[string]any{
			"name": "bad_ingest",
			"mode": "ingestion",
		})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for ingestion without url, got: %q", resp)
		}
		return nil
	})

	r.run("add_ingest_node: missing name returns error", func() error {
		resp, _, err := r.call("add_ingest_node", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing name, got: %q", resp)
		}
		return nil
	})

	// ── Canvas tabs ───────────────────────────────────────────────────────────

	r.run("list_canvases: returns at least one canvas with [active] marker", func() error {
		resp, _, err := r.call("list_canvases", nil)
		if err != nil {
			return err
		}
		if resp == "" || strings.Contains(resp, "No canvases") {
			return fmt.Errorf("expected canvas list, got: %q", resp)
		}
		if !strings.Contains(resp, "[active]") {
			return fmt.Errorf("expected [active] marker in list, got: %q", resp)
		}
		return nil
	})

	r.run("create_canvas: emits add_canvas action with mcp_tab_ id and correct name", func() error {
		resp, actions, err := r.call("create_canvas", map[string]any{"name": "Test Tab"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if !strings.HasPrefix(cid, "mcp_tab_") {
			return fmt.Errorf("expected id starting with mcp_tab_, got %q (response: %q)", cid, resp)
		}
		act := findAction(actions, "add_canvas")
		if act == nil {
			return fmt.Errorf("no 'add_canvas' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		if act.Name != "Test Tab" {
			return fmt.Errorf("want name=%q, got %q", "Test Tab", act.Name)
		}
		return nil
	})

	r.run("create_canvas: default name used when name omitted", func() error {
		_, actions, err := r.call("create_canvas", nil)
		if err != nil {
			return err
		}
		act := findAction(actions, "add_canvas")
		if act == nil {
			return fmt.Errorf("no 'add_canvas' canvas action received")
		}
		if act.Name == "" {
			return fmt.Errorf("expected non-empty default name, got empty")
		}
		if !strings.HasPrefix(act.CanvasID, "mcp_tab_") {
			return fmt.Errorf("expected canvasId starting with mcp_tab_, got %q", act.CanvasID)
		}
		return nil
	})

	r.run("rename_canvas: emits rename_canvas action with correct canvasId and name", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Rename Me"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		resp2, actions, err := r.call("rename_canvas", map[string]any{
			"canvas_id": cid,
			"name":      "Renamed Canvas",
		})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp2), "error") {
			return fmt.Errorf("rename_canvas returned error: %q", resp2)
		}
		act := findAction(actions, "rename_canvas")
		if act == nil {
			return fmt.Errorf("no 'rename_canvas' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		if act.Name != "Renamed Canvas" {
			return fmt.Errorf("want name=%q, got %q", "Renamed Canvas", act.Name)
		}
		return nil
	})

	r.run("rename_canvas: missing canvas_id returns error", func() error {
		resp, _, err := r.call("rename_canvas", map[string]any{"name": "X"})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing canvas_id, got: %q", resp)
		}
		return nil
	})

	r.run("rename_canvas: missing name returns error", func() error {
		resp, _, err := r.call("rename_canvas", map[string]any{"canvas_id": "some_id"})
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing name, got: %q", resp)
		}
		return nil
	})

	r.run("switch_canvas: emits switch_canvas action with correct canvasId", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Switch Target"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("switch_canvas", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		act := findAction(actions, "switch_canvas")
		if act == nil {
			return fmt.Errorf("no 'switch_canvas' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("switch_canvas: missing canvas_id returns error", func() error {
		resp, _, err := r.call("switch_canvas", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing canvas_id, got: %q", resp)
		}
		return nil
	})

	r.run("remove_canvas: emits remove_canvas action with correct canvasId", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "To Be Removed"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		resp2, actions, err := r.call("remove_canvas", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp2), "error") {
			return fmt.Errorf("remove_canvas returned error: %q", resp2)
		}
		act := findAction(actions, "remove_canvas")
		if act == nil {
			return fmt.Errorf("no 'remove_canvas' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("remove_canvas: missing canvas_id returns error", func() error {
		resp, _, err := r.call("remove_canvas", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(strings.ToLower(resp), "error") {
			return fmt.Errorf("expected error for missing canvas_id, got: %q", resp)
		}
		return nil
	})

	// ── canvas_id on node-adding tools ────────────────────────────────────────

	r.run("add_query_node: canvas_id propagates to canvas action", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Query Target Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("add_query_node", map[string]any{
			"name":      "Canvas Targeted Query",
			"sql":       "SELECT 1 AS n",
			"canvas_id": cid,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "query")
		if act == nil {
			return fmt.Errorf("no 'query' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("add_chart_node: canvas_id propagates to canvas action", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Chart Target Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("add_chart_node", map[string]any{
			"name":       "Canvas Chart",
			"sql":        "SELECT 'A' AS x, 10 AS y",
			"chart_type": "barY",
			"x_column":   "x",
			"y_column":   "y",
			"canvas_id":  cid,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "chart")
		if act == nil {
			return fmt.Errorf("no 'chart' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("add_markdown_node: canvas_id propagates to canvas action", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Markdown Target Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("add_markdown_node", map[string]any{
			"name":      "Canvas Note",
			"content":   "# Note\nThis goes on a specific canvas.",
			"canvas_id": cid,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "markdown")
		if act == nil {
			return fmt.Errorf("no 'markdown' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("add_section: canvas_id propagates to canvas action", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Section Target Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("add_section", map[string]any{
			"name":      "Canvas Section",
			"canvas_id": cid,
		})
		if err != nil {
			return err
		}
		act := findAction(actions, "section")
		if act == nil {
			return fmt.Errorf("no 'section' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, act.CanvasID)
		}
		return nil
	})

	// ── list_canvas_nodes with canvas_id filter ───────────────────────────────

	r.run("list_canvas_nodes: canvas_id filter scopes results to that canvas", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Filter Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		// Add a node with a deterministic id to this specific canvas.
		_, _, err = r.call("add_query_node", map[string]any{
			"name":      "Filter Tab Node",
			"sql":       "SELECT 777 AS n",
			"canvas_id": cid,
		})
		if err != nil {
			return err
		}
		resp2, _, err := r.call("list_canvas_nodes", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		if !strings.Contains(resp2, "mcp_filter_tab_node") {
			return fmt.Errorf("expected mcp_filter_tab_node in filtered list, got: %q", resp2)
		}
		return nil
	})

	r.run("list_canvas_nodes: no canvas_id returns nodes across all canvases", func() error {
		// Add a node on the active canvas (no canvas_id).
		_, _, err := r.call("add_query_node", map[string]any{
			"name": "Global List Node",
			"sql":  "SELECT 888 AS n",
		})
		if err != nil {
			return err
		}
		resp, _, err := r.call("list_canvas_nodes", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "mcp_global_list_node") {
			return fmt.Errorf("expected mcp_global_list_node in global list, got: %q", resp)
		}
		return nil
	})

	// ── clear_canvas with canvas_id ───────────────────────────────────────────

	r.run("clear_canvas: canvas_id scopes clear action to that canvas", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Clear Target Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		resp2, actions, err := r.call("clear_canvas", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(resp2), "error") {
			return fmt.Errorf("clear_canvas returned error: %q", resp2)
		}
		act := findAction(actions, "clear")
		if act == nil {
			return fmt.Errorf("no 'clear' canvas action received")
		}
		if act.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q on clear action, got %q", cid, act.CanvasID)
		}
		return nil
	})

	r.run("clear_canvas: no canvas_id clears active canvas (empty canvasId on action)", func() error {
		_, actions, err := r.call("clear_canvas", nil)
		if err != nil {
			return err
		}
		act := findAction(actions, "clear")
		if act == nil {
			return fmt.Errorf("no 'clear' canvas action received")
		}
		if act.CanvasID != "" {
			return fmt.Errorf("want empty canvasId for active-canvas clear, got %q", act.CanvasID)
		}
		return nil
	})

	// ── add_exercise_node ─────────────────────────────────────────────────────

	r.run("add_exercise_node: emits canvas action with name and prompt", func() error {
		resp, actions, err := r.call("add_exercise_node", map[string]any{
			"name":   "ex1_select_basics",
			"prompt": "Write a SELECT query that returns all rows from `products`.",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("add_exercise_node returned error: %q", resp)
		}
		a := findAction(actions, "exercise")
		if a == nil {
			return fmt.Errorf("no 'exercise' action in response; actions: %v", actions)
		}
		if a.Name != "ex1_select_basics" {
			return fmt.Errorf("want name=ex1_select_basics, got %q", a.Name)
		}
		if a.Content != "Write a SELECT query that returns all rows from `products`." {
			return fmt.Errorf("prompt not round-tripped: %q", a.Content)
		}
		return nil
	})

	r.run("add_exercise_node: sql propagates to action", func() error {
		starterSQL := "SELECT ??? FROM sb_movies"
		resp, actions, err := r.call("add_exercise_node", map[string]any{
			"name":   "ex2_with_sql",
			"sql":    starterSQL,
			"prompt": "Select all movies.",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("add_exercise_node returned error: %q", resp)
		}
		a := findAction(actions, "exercise")
		if a == nil {
			return fmt.Errorf("no 'exercise' action found")
		}
		if a.SQL != starterSQL {
			return fmt.Errorf("want sql=%q, got %q", starterSQL, a.SQL)
		}
		return nil
	})

	r.run("add_exercise_node: missing name returns error", func() error {
		resp, _, err := r.call("add_exercise_node", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "error") {
			return fmt.Errorf("expected error for missing name, got: %q", resp)
		}
		return nil
	})

	r.run("add_exercise_node: canvas_id propagates to action", func() error {
		resp, _, err := r.call("create_canvas", map[string]any{"name": "Assertion Test Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}
		_, actions, err := r.call("add_exercise_node", map[string]any{
			"name":      "ex3_canvas_target",
			"canvas_id": cid,
		})
		if err != nil {
			return err
		}
		a := findAction(actions, "exercise")
		if a == nil {
			return fmt.Errorf("no 'exercise' action found")
		}
		if a.CanvasID != cid {
			return fmt.Errorf("want canvasId=%q, got %q", cid, a.CanvasID)
		}
		return nil
	})

	r.run("add_exercise_node: success_text propagates to action", func() error {
		txt := "Great work! **SELECT *** retrieves every column."
		resp, actions, err := r.call("add_exercise_node", map[string]any{
			"name":         "ex_success_text",
			"success_text": txt,
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("add_exercise_node returned error: %q", resp)
		}
		a := findAction(actions, "exercise")
		if a == nil {
			return fmt.Errorf("no 'exercise' action found")
		}
		if a.SuccessText != txt {
			return fmt.Errorf("want successText=%q, got %q", txt, a.SuccessText)
		}
		return nil
	})

	r.run("add_exercise_node: next_id propagates to action", func() error {
		resp, actions, err := r.call("add_exercise_node", map[string]any{
			"name":    "ex_with_next",
			"next_id": "mcp_ex_next_lesson",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("add_exercise_node returned error: %q", resp)
		}
		a := findAction(actions, "exercise")
		if a == nil {
			return fmt.Errorf("no 'exercise' action found")
		}
		if a.NextID != "mcp_ex_next_lesson" {
			return fmt.Errorf("want nextId=%q, got %q", "mcp_ex_next_lesson", a.NextID)
		}
		return nil
	})

	// ── add_test_node ─────────────────────────────────────────────────────────

	r.run("add_test_node: emits canvas action with name and sql", func() error {
		resp, actions, err := r.call("add_test_node", map[string]any{
			"name": "test_row_count",
			"sql":  "SELECT COUNT(*) AS n FROM products",
		})
		if err != nil {
			return err
		}
		if strings.Contains(resp, "error") {
			return fmt.Errorf("add_test_node returned error: %q", resp)
		}
		a := findAction(actions, "test")
		if a == nil {
			return fmt.Errorf("no 'test' action in response; actions: %v", actions)
		}
		if a.Name != "test_row_count" {
			return fmt.Errorf("want name=test_row_count, got %q", a.Name)
		}
		if a.SQL != "SELECT COUNT(*) AS n FROM products" {
			return fmt.Errorf("sql not round-tripped: %q", a.SQL)
		}
		return nil
	})

	r.run("add_test_node: missing name returns error", func() error {
		resp, _, err := r.call("add_test_node", nil)
		if err != nil {
			return err
		}
		if !strings.Contains(resp, "error") {
			return fmt.Errorf("expected error for missing name, got: %q", resp)
		}
		return nil
	})

	// ── end-to-end multi-canvas scenario ─────────────────────────────────────

	r.run("canvas tabs end-to-end: create → populate → list filter → remove", func() error {
		// 1. Create a dedicated canvas.
		resp, _, err := r.call("create_canvas", map[string]any{"name": "E2E Canvas"})
		if err != nil {
			return err
		}
		cid := extractCanvasID(resp)
		if cid == "" {
			return fmt.Errorf("could not extract canvas id from: %q", resp)
		}

		// 2. Add two query nodes targeting this canvas.
		for _, name := range []string{"E2E Query Alpha", "E2E Query Beta"} {
			if _, _, err = r.call("add_query_node", map[string]any{
				"name":      name,
				"sql":       "SELECT 1",
				"canvas_id": cid,
			}); err != nil {
				return fmt.Errorf("add %q: %w", name, err)
			}
		}

		// 3. Filtered list shows both nodes (from in-memory registry).
		listResp, _, err := r.call("list_canvas_nodes", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		for _, wantID := range []string{"mcp_e2e_query_alpha", "mcp_e2e_query_beta"} {
			if !strings.Contains(listResp, wantID) {
				return fmt.Errorf("expected %q in filtered list, got: %q", wantID, listResp)
			}
		}

		// 4. Switch to the canvas, then remove it.
		if _, _, err = r.call("switch_canvas", map[string]any{"canvas_id": cid}); err != nil {
			return fmt.Errorf("switch: %w", err)
		}
		_, actions, err := r.call("remove_canvas", map[string]any{"canvas_id": cid})
		if err != nil {
			return err
		}
		if findAction(actions, "remove_canvas") == nil {
			return fmt.Errorf("no 'remove_canvas' action after removal")
		}
		return nil
	})
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	filter := ""
	if len(os.Args) > 1 {
		filter = os.Args[1]
	}

	// Check the app is running.
	resp, err := http.Get(healthURL)
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("\n%s%s✗ sql.garden is not running%s\n", bold, red, reset)
		fmt.Printf("  Start the app first, then re-run this tool.\n\n")
		os.Exit(1)
	}
	resp.Body.Close()

	// Connect SSE buffer.
	buf := &actionBuffer{}
	cancel, err := subscribeSSE(buf)
	if err != nil {
		fmt.Printf("\n%s%s✗ canvas-stream unavailable: %v%s\n\n", bold, red, err, reset)
		os.Exit(1)
	}
	defer cancel()

	// Give the SSE connection a moment to establish.
	time.Sleep(150 * time.Millisecond)

	r := &runner{buf: buf, filter: filter}

	fmt.Printf("\n%s%s sql.garden MCP Integration Tests%s\n", bold, cyan, reset)
	if filter != "" {
		fmt.Printf("  %sfilter: %q%s\n", yellow, filter, reset)
	}
	fmt.Println()

	r.runAll()

	// Summary
	total := r.pass + r.fail + r.skip
	fmt.Printf("\n%s─────────────────────────────────────%s\n", cyan, reset)
	fmt.Printf("  %s%s%d passed%s", bold, green, r.pass, reset)
	if r.fail > 0 {
		fmt.Printf("  %s%s%d failed%s", bold, red, r.fail, reset)
	}
	if r.skip > 0 {
		fmt.Printf("  %s%d skipped%s", yellow, r.skip, reset)
	}
	fmt.Printf("  %s(%d total)%s\n\n", yellow, total, reset)

	if r.fail > 0 {
		os.Exit(1)
	}
}
