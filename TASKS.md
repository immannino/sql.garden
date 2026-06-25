# sql.garden — Task Tracker

> Version: v0.0.0-alpha.6
> Updated: 2026-06-24 (session 2)

---

## ✅ Done

### Core App
- [x] Wails v2 desktop app (Go + Vue 3 + DuckDB in-process)
- [x] Infinite canvas with pan/zoom, fit-view, marquee selection
- [x] Node types: Table, Query, Chart, Markdown, Section
- [x] Node resize (e/s/se handles), drag, multi-select
- [x] Persist canvas state + table data (SQLite state.db + parquet)
- [x] Auto-save on change, restore on launch
- [x] Sample datasets picker (Weather, Stock Markets, etc.)
- [x] Import: CSV/Parquet/JSON from file, URL, paste, SQLite DB
- [x] Export: CSV, JSON, Parquet from query results
- [x] Fit view (F / Cmd+Shift+F), zoom in/out (Cmd+=/-)
- [x] Window drag on macOS toolbar (Wails CSSDragProperty)
- [x] Toolbar dblclick to maximize (WindowToggleMaximise)
- [x] Startup update check banner
- [x] Import history — Recent tab in Import modal, re-import from file path or URL

### Layers Panel
- [x] Flat z-order list (top = front, Figma-style)
- [x] Drag handles to reorder z-index
- [x] Bring to Front / Send to Back hover buttons
- [x] Drop indicator line while dragging

### Query Nodes
- [x] SQL editor with CodeMirror 6 (syntax highlighting, line numbers, undo/redo)
- [x] Cmd+Enter to run, debounced 400ms auto-save to store
- [x] Results table with 500-row display cap + export
- [x] Publish as DuckDB VIEW toggle
- [x] Auto-refresh interval support
- [x] Schema-aware SQL autocomplete (table + column names from canvas)
- [x] Empty result set shows 0 rows instead of null error
- [x] Query history per node — History tab, per-node log of past SQL runs with restore

### Chart Nodes
- [x] Chart types: barY, barX, lineY, areaY, dot, cell, pie/donut, number, boolean, conditional, mermaid, table, histogram, boxplot, sankey
- [x] Source: inline SQL or linked Query node
- [x] Inline SQL editor (CodeMirror) with schema autocomplete
- [x] Chart-only view mode
- [x] Observable Plot rendering with padding fix (no header overhang)
- [x] Conditional formatting rules engine
- [x] Table chart with column config (format, align, hide, rename)
- [x] Chart legend toggle — show/hide color legend without re-running
- [x] Per-chart help text in properties panel (when/columns/tip)

### Canvas Operations
- [x] Multi-node alignment tools — 8 ops: left/centerH/right/top/middleV/bottom/distributeH/distributeV
- [x] Right-click context menu — Node: Duplicate, Bring to Front/Back, Delete. Canvas: Add nodes, Fit View.
- [x] Undo/redo — Cmd+Z / Cmd+Shift+Z, snapshots on all mutating operations
- [x] Auto-fit mosaic layout — toolbar button + right-click "Wrap in Section" + "Mosaic Contents" on Section nodes; uses real DOM sizes via `data-node-id`

### Sidebar
- [x] Layers tab (z-order, drag reorder, front/back)
- [x] Connections tab (PostgreSQL, SQLite, DuckDB, MySQL)
- [x] Connection schema explorer (expand schemas → tables → columns)
- [x] Auto-reconnect saved connections on startup

### MCP / AI
- [x] MCP server over SSE (port 37421) — canvas tools exposed
- [x] Settings modal MCP tab: copy snippets for Claude Code / Claude Desktop / Cursor / Windsurf
- [x] Auto-configure Claude Desktop (claude_desktop_config.json write)
- [x] AI panel with chat + tool-call streaming
- [x] HelpPanel "Open MCP Setup" deep-link into Settings
- [x] `resize_node` and `move_node` MCP tools — agents can programmatically size and reposition canvas nodes
- [x] `focus_node` MCP tool — pan and zoom viewport to a specific node by id; useful for directing attention after dashboard build
- [x] `update_query_node` MCP tool — overwrite SQL (and optionally rename) an existing query node in-place; no delete-and-recreate needed
- [x] `import_csv_data` MCP tool — agents pipe raw CSV text directly into DuckDB; pre-import column-count validation returns actionable errors on malformed CSV
- [x] `import_url` MCP tool — server-side fetch of remote CSV/Parquet/JSON; bypasses CORS entirely

### Keyboard & Input
- [x] Global hotkey guard: no app shortcuts fire when focus is in a text field or code editor
- [x] Cmd+A/C/X/V shim for plain input/textarea in Wails WKWebView (via Go runtime.Clipboard*)
- [x] CodeMirror clipboard keymaps (copy/paste route through Go runtime, not DOM events)
- [x] Delete/Backspace bulk-deletes selected canvas nodes (with table data cleanup)
- [x] Node color picker — 12 preset swatches + native color input, all node types
- [x] Canvas background grid/dots — dot grid overlay, toggle in Settings > Appearance
- [x] Node search / jump-to — Cmd+K palette
- [x] Duplicate node — Cmd+D, offset 24px, auto-selects copies

### CI / Release
- [x] GitHub Actions: macOS universal build + Windows build on tag push
- [x] macOS .app zipped before artifact upload (preserves bundle structure)

---

## 🧪 Testing

### Working Agreement
Every new feature or tool must ship with a corresponding Red/Green integration test in `cmd/mcp-test/main.go` before it is considered done. Tests must pass against the running app before moving to the next task.

### MCP Integration Tests (`cmd/mcp-test`)
Run with: `go run ./cmd/mcp-test` (app must be running first)

- [x] Health endpoint
- [x] `tools/list` — asserts all expected tool names are registered
- [x] `list_tables`, `run_query` (success + error surface)
- [x] `add_query_node` — stable nodeId, idempotent dedup
- [x] `add_chart_node` — inline SQL, source_id linking, number chart type
- [x] `add_markdown_node` — content round-trip, nodeId
- [x] `import_file` — sales.csv (24 rows), service_health.csv (10 rows), bad path error
- [x] `list_canvas_nodes` — node added this session appears in list
- [x] `clear_canvas` — emits clear action
- [x] `fit_view` — fit, reset, default modes
- [x] `import_csv_data` — 3-col basic (rowCount=3), CRLF normalization (rowCount=2), quoted fields with embedded commas (rowCount=3), missing csv_text error
- [x] `import_csv_data` column-count mismatch — header/data field count mismatch returns error naming the row number; no canvas action emitted
- [x] `import_url` — local HTTP test server (rowCount=4), unreachable host error, missing url error
- [x] `import_url` Content-Type fallback — CSV served at extensionless URL with `Content-Type: text/csv` header imports correctly
- [x] `resize_node` — correct width/height on action, missing node_id error
- [x] `resize_node` batch — 4 nodes resized in sequence all emit distinct resize_node actions
- [x] `move_node` — correct x/y on action, missing node_id error
- [x] Canvas stream reconnect — new subscriber after prior connection closed still receives events
- [x] `focus_node` — emits focus_node action with correct nodeId, missing node_id error
- [x] `update_query_node` — emits update_query action with new SQL, optional name propagation, missing field errors

---

## 🔧 Known Issues / Needs Verification

- [ ] **Clipboard shim needs rebuild** — The Go `ClipboardGet`/`ClipboardSet` methods and their JS bindings are new. Requires `wails dev` or `wails build` to regenerate `wailsjs/go/main/App.js` before the shim in App.vue and SqlEditor.vue takes effect.

- [ ] **Edit menu items are greyed out** — The Edit submenu was added with `nil` callbacks; Wails doesn't wire nil-callback items to the native NSResponder selectors so they appear disabled in the menu bar. Clipboard now works via the JS shim, but the menu looks wrong. Options: remove the Edit menu entirely (clipboard shim handles everything), or replace nil callbacks with JS-dispatching callbacks that call `document.execCommand`.

---

## 📋 Backlog

### Polish / QoL
- [ ] **Section auto-resize** — Option to auto-expand a Section node to wrap its contained nodes.
- [ ] **Pinned/auto-run queries** — Option to run a query node automatically on canvas open.

### Query / Data
- [ ] **CORS proxy for URL imports** — URL imports fail for servers without permissive CORS headers. Plan: Cloudflare Worker / Vercel Edge function that fetches server-side and streams bytes back.

### Charts
- [ ] **More chart types** — Scatter matrix, waterfall, heatmap (histogram/boxplot/sankey done).

### MCP Expansions
- [ ] **`set_node_color` MCP tool** — Let agents apply color coding to canvas nodes (e.g. highlight a KPI node in red when a threshold is breached).
- [ ] **`add_section` MCP tool** — Agents can create Section containers to visually group related nodes without the user needing to right-click.

### Canvas Structure
- [ ] **Canvas tabs** — Multiple named canvases as tabs within the app (Figma documents-style). Data model: `nodes` keyed by `canvasId`; SQLite `canvas_state` gets a `canvas_id` column + migration. Tab strip in toolbar. Significant effort — needs a dedicated planning session before touching persistence layer.

### Discovery & Content
- [ ] **In-app changelog/news feed** — Small panel (or Settings tab) that fetches a hosted JSON/RSS feed you control. Surfaces release notes, tips, and announcements. Weekend-scale effort.
- [ ] **Dataset directory** — Hosted JSON index of curated public datasets (FRED, Census, Our World in Data, stock market, healthcare, etc.) with name, source URL, description, tags, and license. In-app "Explore" panel fetches + searches the index; clicking a dataset fires the existing URL import flow. Index starts as a hand-curated JSON file in a GitHub repo — no database needed. Strong differentiator; dataset index is a separate hosted artifact.
- [ ] **Community / Explore page** — Longer-term hub surfacing dataset directory, user-shared canvases, blog posts, and curated data stories. Builds on the dataset directory and news feed foundations.

### Reporting
- [ ] **Report / PDF export** — Export selected nodes as a formatted report. Approach: "report mode" re-renders selected nodes into a normal scrollable document layout (outside canvas coordinates), then `window.print()` or a Go-invoked Puppeteer/wkhtmltopdf subprocess. Chart SVGs export cleanly; tables need pagination. Scope as "export selected nodes as report" not "screenshot the canvas."

### Connections
- [ ] **SSH tunnel support** — Config for connecting to remote DBs via SSH port-forward.

### Desktop-specific
- [ ] **Code signing & notarization** — CI secrets scaffolded but not tested end-to-end. Without notarization, macOS requires "Allow from anywhere" Gatekeeper override.
- [ ] **Auto-update download + relaunch** — Banner appears when update is available but only links to GitHub releases. Wire a native download-and-relaunch flow.
- [ ] **Windows smoke test** — CI builds the Windows binary but no manual QA done.

### Web (WASM) parity
- [ ] **Canvas persistence on web** — State resets on page reload; could use IndexedDB or localStorage.

---

## 🗑️ Tabled
- **Data loaders via local scripts** — Observable Framework-style shell-out loaders piping stdout into DuckDB tables. Post-v1, desktop-only via Wails exec.
- **SSH tunnels** — Post-v1.
- **Windows code signing** — No cert yet.
- **Run queries against connections** — Ad-hoc SQL editor per connection; moot because `SELECT * FROM alias.schema.table` already works via DuckDB ATTACH.
