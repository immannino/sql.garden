package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
)

const MCPPort = 37421

// ── JSON-RPC 2.0 ──────────────────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ── Server ────────────────────────────────────────────────────────────────────

type mcpServer struct {
	app      *App
	sessions sync.Map // sessionID → chan rpcResponse (legacy SSE transport)
	subs     sync.Map // subID → chan CanvasAction (canvas stream)
	srv      *http.Server
}

func (a *App) startMCPServer() {
	ms := &mcpServer{app: a}
	a.mcpSrv = ms

	mux := http.NewServeMux()

	// Streamable HTTP transport (Claude Desktop / Claude Code ≥ 2025)
	// Both GET (SSE notification stream) and POST (JSON-RPC) share one endpoint.
	mux.HandleFunc("/mcp", ms.handleStreamableHTTP)

	// Legacy SSE transport — kept for older Claude Code versions
	// GET /sse opens the SSE channel; POST /message sends JSON-RPC.
	mux.HandleFunc("/sse", ms.handleSSE)
	mux.HandleFunc("/message", ms.handleMessage)

	// Canvas-action push stream for the sql.garden frontend
	mux.HandleFunc("/canvas-stream", ms.handleCanvasStream)

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })

	ms.srv = &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", MCPPort), Handler: mux}

	ln, err := net.Listen("tcp", ms.srv.Addr)
	if err != nil {
		log.Printf("sql.garden MCP: could not bind port %d: %v", MCPPort, err)
		return
	}
	log.Printf("sql.garden MCP server: http://127.0.0.1:%d", MCPPort)
	ms.srv.Serve(ln) //nolint:errcheck
}

// ── Streamable HTTP transport (POST /mcp) ─────────────────────────────────────
//
// Claude Desktop sends initialize + tool calls as regular HTTP POSTs and reads
// the JSON response body. No session management required.

func (ms *mcpServer) handleStreamableHTTP(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)

	switch r.Method {
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
		w.WriteHeader(204)
		return

	case http.MethodGet:
		// Server-initiated notification stream — hold open, we have no server pushes.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		fmt.Fprintf(w, ": connected\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		return

	case http.MethodPost:
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad JSON", http.StatusBadRequest)
			return
		}
		resp := ms.handle(req)
		if resp == nil {
			// Notification — no body.
			w.WriteHeader(202)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ── Legacy SSE transport (GET /sse + POST /message) ──────────────────────────

func (ms *mcpServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	id := randID()
	ch := make(chan rpcResponse, 32)
	ms.sessions.Store(id, ch)
	defer ms.sessions.Delete(id)

	// Absolute URL so all MCP clients can resolve it regardless of base.
	fmt.Fprintf(w, "event: endpoint\ndata: http://localhost:%d/message?sessionId=%s\n\n", MCPPort, id)
	w.(http.Flusher).Flush()

	for {
		select {
		case msg := <-ch:
			b, _ := json.Marshal(msg)
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (ms *mcpServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(204)
		return
	}

	v, ok := ms.sessions.Load(r.URL.Query().Get("sessionId"))
	if !ok {
		http.Error(w, "unknown session", http.StatusBadRequest)
		return
	}
	ch := v.(chan rpcResponse)

	var req rpcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad JSON", http.StatusBadRequest)
		return
	}
	w.WriteHeader(202)
	go func() {
		if resp := ms.handle(req); resp != nil {
			select {
			case ch <- *resp:
			default:
			}
		}
	}()
}

// ── Shared JSON-RPC dispatcher ────────────────────────────────────────────────

// handle returns nil for notifications (no response expected).
func (ms *mcpServer) handle(req rpcRequest) *rpcResponse {
	var result any
	var rpcErr *rpcError

	switch req.Method {
	case "initialize":
		result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "sql-garden", "version": "1.0.0"},
		}

	case "notifications/initialized", "notifications/cancelled":
		return nil // notifications — no response

	case "ping":
		result = map[string]any{}

	case "tools/list":
		result = map[string]any{"tools": mcpToolDefs}

	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			rpcErr = &rpcError{Code: -32602, Message: "invalid params"}
			break
		}
		text := ms.callTool(p.Name, p.Arguments)
		result = map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}

	default:
		rpcErr = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}

	resp := &rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	return resp
}

func (ms *mcpServer) callTool(name string, args map[string]any) string {
	var actions []CanvasAction
	result := ms.app.execTool(name, args, &actions)
	for _, ca := range actions {
		ms.pushCanvas(ca)
	}
	return result
}

// ── Canvas-action stream (frontend subscription) ──────────────────────────────

func (ms *mcpServer) handleCanvasStream(w http.ResponseWriter, r *http.Request) {
	corsHeaders(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	id := randID()
	ch := make(chan CanvasAction, 32)
	ms.subs.Store(id, ch)
	defer ms.subs.Delete(id)

	fmt.Fprintf(w, ": connected\n\n")
	w.(http.Flusher).Flush()

	for {
		select {
		case ca := <-ch:
			b, _ := json.Marshal(ca)
			fmt.Fprintf(w, "event: canvas-action\ndata: %s\n\n", b)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (ms *mcpServer) pushCanvas(ca CanvasAction) {
	ms.subs.Range(func(_, v any) bool {
		ch := v.(chan CanvasAction)
		select {
		case ch <- ca:
		default:
		}
		return true
	})
}

// ── MCP tool definitions ──────────────────────────────────────────────────────

var mcpToolDefs = []map[string]any{
	{
		"name":        "list_tables",
		"description": "List every table and its columns in sql.garden's DuckDB instance (including attached databases).",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "list_canvases",
		"description": "List all canvas tabs with their id, name, node count, and which is currently active. Call this before using canvas_id in other tools.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "create_canvas",
		"description": "Create a new canvas tab. Returns the canvas id to use as canvas_id in node-adding tools.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string", "description": "Display name for the new canvas tab"},
			},
		},
	},
	{
		"name":        "rename_canvas",
		"description": "Rename an existing canvas tab.",
		"inputSchema": map[string]any{
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
		"description": "Make a canvas tab active — brings it into view for the user.",
		"inputSchema": map[string]any{
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
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "ID of the canvas to remove (from list_canvases)"},
			},
			"required": []string{"canvas_id"},
		},
	},
	{
		"name":        "list_canvas_nodes",
		"description": "List query/data nodes with their id and name. Pass canvas_id to filter to a specific tab; omit to see all canvases. Use returned ids as source_id in add_chart_node.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "Filter to a specific canvas (from list_canvases); omit for all canvases"},
			},
		},
	},
	{
		"name":        "run_query",
		"description": "Execute a DuckDB SQL query and return up to 50 rows of results.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string", "description": "SQL query to execute"},
			},
			"required": []string{"sql"},
		},
	},
	{
		"name":        "add_query_node",
		"description": "Pin a named SQL query to a canvas as an interactive scrollable table. Returns a node id you can pass as source_id to add_chart_node.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":      map[string]any{"type": "string", "description": "Short label for the node"},
				"sql":       map[string]any{"type": "string", "description": "SQL to display in the node"},
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"name", "sql"},
		},
	},
	{
		"name":        "add_chart_node",
		"description": "Pin a chart visualization to a canvas. Provide either source_id (from add_query_node) OR sql — not both. Always confirm column names via run_query first.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":         map[string]any{"type": "string"},
				"source_id":    map[string]any{"type": "string", "description": "ID of an existing query node whose data this chart visualises"},
				"sql":          map[string]any{"type": "string", "description": "SQL that produces the chart data — omit when source_id is provided"},
				"chart_type":   map[string]any{"type": "string", "enum": []string{"barY", "barX", "lineY", "areaY", "dot", "cell", "pie", "donut", "number", "boolean", "conditional", "waterfall", "heatmap", "scatter-matrix"}, "description": "barY/barX=bar chart, lineY=line, areaY=area, dot=scatter, pie/donut=pie chart, cell=grid heatmap, number=big single value, boolean=true/false badge, conditional=N-state badge, waterfall=running-total waterfall (needs x+y), heatmap=2D density heatmap (needs x+y), scatter-matrix=pairwise scatter grid"},
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
		"description": "Pin a markdown note, summary, or documentation block to a canvas.",
		"inputSchema": map[string]any{
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
		"description": "Snapshot a SQL query's results into a named DuckDB table and pin it to a canvas as a persistent Data node. Survives app restarts and can be queried directly.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_name": map[string]any{"type": "string", "description": "DuckDB table name for the snapshot (snake_case recommended)"},
				"sql":        map[string]any{"type": "string", "description": "SQL query whose results are materialized — do not include a trailing semicolon"},
				"source_id":  map[string]any{"type": "string", "description": "Optional ID of a query node this was derived from"},
				"canvas_id":  map[string]any{"type": "string", "description": "Canvas tab to add the node to; default is the active canvas"},
			},
			"required": []string{"table_name", "sql"},
		},
	},
	{
		"name":        "import_file",
		"description": "Import a local CSV, Parquet, or JSON file into DuckDB and add it as a table node on a canvas.",
		"inputSchema": map[string]any{
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
		"description": "Load raw CSV text directly into DuckDB as a table and add it as a canvas node. Use when you have generated data as a string — no file on disk needed.",
		"inputSchema": map[string]any{
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
		"description": "Fetch a remote CSV, Parquet, or JSON file by URL, load it into DuckDB, and add it as a canvas node. Download is server-side so CORS is not a concern.",
		"inputSchema": map[string]any{
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
		"inputSchema": map[string]any{
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
		"name":        "resize_node",
		"description": "Set the width and height of an existing canvas node.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to resize"},
				"width":   map[string]any{"type": "number", "description": "New width in canvas pixels"},
				"height":  map[string]any{"type": "number", "description": "New height in canvas pixels"},
			},
			"required": []string{"node_id", "width", "height"},
		},
	},
	{
		"name":        "move_node",
		"description": "Set the canvas position (x, y) of an existing node. Coordinates are in canvas pixels; (0, 0) is the default origin.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to move"},
				"x":       map[string]any{"type": "number", "description": "Canvas x position"},
				"y":       map[string]any{"type": "number", "description": "Canvas y position"},
			},
			"required": []string{"node_id", "x", "y"},
		},
	},
	{
		"name":        "focus_node",
		"description": "Pan and zoom the canvas viewport to centre on a specific node. Call after adding nodes to direct the user's attention.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to focus"},
			},
			"required": []string{"node_id"},
		},
	},
	{
		"name":        "update_query_node",
		"description": "Overwrite the SQL of an existing query node and optionally rename it. Use instead of delete-and-recreate when only the query needs to change.",
		"inputSchema": map[string]any{
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
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"node_id": map[string]any{"type": "string", "description": "ID of the node to recolor"},
				"color":   map[string]any{"type": "string", "description": "CSS hex color, e.g. #ef4444"},
			},
			"required": []string{"node_id", "color"},
		},
	},
	{
		"name":        "add_section",
		"description": "Create a named Section container on the canvas to visually group related nodes. Sections appear behind other nodes.",
		"inputSchema": map[string]any{
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
		"description": "Remove all nodes from a canvas tab. Omit canvas_id to clear the active canvas.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"canvas_id": map[string]any{"type": "string", "description": "Canvas tab to clear; omit for the active canvas"},
			},
		},
	},
	{
		"name":        "fit_view",
		"description": "Adjust the canvas viewport. Use after adding nodes to bring everything into view.",
		"inputSchema": map[string]any{
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

// ── Helpers ───────────────────────────────────────────────────────────────────

func corsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func randID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}
