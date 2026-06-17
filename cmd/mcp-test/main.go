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
	Type     string `json:"type"`
	NodeID   string `json:"nodeId"`
	Name     string `json:"name"`
	SQL      string `json:"sql"`
	Content  string `json:"content"`
	SourceID string `json:"sourceId"`
	ChartType string `json:"chartType"`
	XColumn  string `json:"xColumn"`
	YColumn  string `json:"yColumn"`
	RowCount int64  `json:"rowCount"`
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
			"list_tables", "list_canvas_nodes", "run_query",
			"add_query_node", "add_chart_node", "add_markdown_node",
			"import_file", "clear_canvas",
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
