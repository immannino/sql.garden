# sql.garden — Task Tracker

> Version: v0.0.1-alpha
> Updated: 2026-07-09 (session 9)

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
- [x] Error display moved to bottom — full multiline error in an expanding panel below the footer (red background, scrollable, dismissable ✕); removed truncated single-line error from status bar
- [x] Default card width 360px + `flex-wrap: wrap` on footer — prevents button overflow on narrower viewports

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
- [x] V/T icon bug fixed — `GetSchemaTables` now uses `information_schema.tables` with `CASE table_type WHEN 'VIEW' THEN 'view' ELSE 'table' END`; works correctly for all attachment types (SQLite, Postgres, DuckDB native)

### S3 / Object Storage
- [x] S3 URL import — `ImportFromUrl` intercepts `s3://` scheme, configures DuckDB `httpfs` secret from stored credentials, reads parquet/csv/json directly; supports AWS, R2, MinIO via optional endpoint field
- [x] S3 credentials Settings tab — desktop-only tab in Settings modal; fields for key, secret, region, endpoint; saved to persistence DB; tip text explains `s3://` URL usage in import modal

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
- [x] `import_s3` MCP tool — load files from `s3://` URIs; validates scheme prefix, delegates to `importFromS3` with stored credentials; 3 validation error tests
- [x] `materialize_query` MCP tool — snapshot a SQL query as a persistent DuckDB table + DataNode; survives restarts, accepts optional `source_id` for canvas linking; trailing semicolons stripped automatically
- [x] `list_canvas_nodes` extended — now returns both `query` and `data` nodes, with kind prefix in output (`query id=... name=...` / `data id=... name=...`)
- [x] `set_node_color` MCP tool — set accent color of any canvas node by id; useful for highlighting KPIs or flagging anomalies
- [x] `add_section` MCP tool — create a named Section container; width/height configurable, defaults to 400×300

### Data Nodes
- [x] `DataNode` type (`kind: 'data'`) — materialized DuckDB table as a first-class canvas node; `name` is the DuckDB table name
- [x] Column type casting — click any type badge on DataCard to open an inline editor; pick a target type (DOUBLE, DECIMAL(18,2), BIGINT, VARCHAR, DATE, TIMESTAMP, BOOLEAN) with an optional `USING` expression; DOUBLE/DECIMAL auto-fill `regexp_replace` strip expression; casts stored in `columnCasts` on the node, persisted to canvas state, and re-applied automatically on Refresh and startup restore
- [x] `DataCard.vue` — cylinder icon, column list, row count, stale badge, Refresh button, inline rename (renames DuckDB table), two-click delete (drops table)
- [x] Stale detection — amber border + "stale" badge + amber Refresh button when source QueryNode's SQL has changed since last materialize
- [x] Refresh — re-runs `CREATE OR REPLACE TABLE` from current source SQL, updates columns/rowCount, saves on desktop
- [x] Rename — `ALTER TABLE old RENAME TO new` + `DeleteTableData`/`SaveTableData` on desktop
- [x] Delete — drops DuckDB table + removes canvas node
- [x] "→ Data" materialize button on QueryCard — inline name input (default `<name>_snapshot`), creates DataNode offset to the right; trailing semicolons stripped before `CREATE OR REPLACE TABLE`
- [x] ChartCard `runLinked()` — DataNode source queries `SELECT * FROM "<name>"` instead of re-running SQL
- [x] ChartPropertiesPanel — source picker includes DataNodes (`kind === 'data'`), labeled with `(table)` suffix; `runLinked()` handles DataNode
- [x] Desktop persistence — DataNode saves/restores via `SaveTableData`/parquet (same path as imported CSV tables)
- [x] Web persistence — DataNode skipped on page reload (materialized data doesn't survive reload, same as TableNode)

### Keyboard & Input
- [x] Global hotkey guard: no app shortcuts fire when focus is in a text field or code editor
- [x] Cmd+A/C/X/V shim for plain input/textarea in Wails WKWebView (via Go runtime.Clipboard*)
- [x] CodeMirror clipboard keymaps (copy/paste route through Go runtime, not DOM events)
- [x] Delete/Backspace bulk-deletes selected canvas nodes (with table data cleanup)
- [x] Node color picker — 12 preset swatches + native color input, all node types
- [x] Canvas background grid/dots — dot grid overlay, toggle in Settings > Appearance
- [x] Node search / jump-to — Cmd+K palette
- [x] Duplicate node — Cmd+D, offset 24px, auto-selects copies

### Sidebar
- [x] **Resizable sidebar** — drag handle on right edge, 160–560px range, accent highlight on hover/drag
- [x] **S3 file browser QoL** — search/filter box, Name/Date sort toggle, modified date display per file, file count shows filtered/total when searching; `LastModified` + `Size` added to `S3Object` backend struct
- [x] **Edit menu removed** — greyed-out Edit submenu deleted from `buildMenu()`; clipboard fully handled by JS shim
- [x] **In-app changelog** — "What's New" tab in Settings; lazy-fetches GitHub releases API, shows tag/date/body for last 10 releases

### Canvas Operations
- [x] **Node export / import (.sql.garden.json)** — right-click any node (or multi-select) → "Export node…" triggers native save dialog (desktop) or blob download (web). Data/table nodes embed full row data as `_rows`. Import via canvas right-click, file picker, or drag-drop a `.sql.garden.json` onto the window. Import centers bundle on current viewport, remaps IDs/names to avoid conflicts, processes charts last so `sourceId` refs resolve correctly. Entire import is a single undo snapshot.

### CI / Release
- [x] GitHub Actions: macOS universal build + Windows build on tag push
- [x] macOS .app zipped before artifact upload (preserves bundle structure)
- [x] **Code signing & notarization workflow** — `build/darwin/entitlements.plist` created (allow-jit, network.client/server, files.user-selected.read-write); bundle ID set to `garden.sql`; release.yml updated with keychain import → `codesign --options runtime` → `ditto` packaging → `notarytool submit --wait` → `stapler staple` → re-package. All steps guarded by secret presence so unsigned builds still work. Notarize step captures Apple rejection log on failure via `notarytool log`. First successful notarization confirmed.

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
- [x] `materialize_query` — emits data canvas action, rowCount correct, stable nodeId, trailing semicolons stripped, source_id propagation, missing field errors
- [x] `list_canvas_nodes` extended — data node appears with 'data' kind prefix after materialize_query
- [x] `tools/list` — updated to assert `materialize_query`, `set_node_color`, `add_section` are registered
- [x] `set_node_color` — emits set_color action with nodeId + color, missing field errors (3 scenarios)
- [x] `add_section` — emits section action with name/dimensions, defaults to 400×300, missing name error (3 scenarios)
- [x] `tools/list` — updated to assert `import_s3` is registered
- [x] `import_s3` — missing s3_url error, non-s3:// scheme error, missing table_name error (3 validation scenarios)

---

## 🔧 Known Issues / Needs Verification

- [ ] **Clipboard shim needs rebuild** — The Go `ClipboardGet`/`ClipboardSet` methods and their JS bindings are new. Requires `wails dev` or `wails build` to regenerate `wailsjs/go/main/App.js` before the shim in App.vue and SqlEditor.vue takes effect.

- [x] **Edit menu items are greyed out** — Removed the Edit submenu entirely; clipboard works via the JS shim (Cmd+X/C/V/A/Z handled in App.vue and SqlEditor.vue), so the menu added no value and all items appeared disabled.

---

## 📋 Backlog

### Polish / QoL
- [ ] **Section auto-resize** — Option to auto-expand a Section node to wrap its contained nodes.
- [ ] **Pinned/auto-run queries** — Option to run a query node automatically on canvas open.

### Query / Data
- [ ] **CORS proxy for URL imports** — URL imports fail for servers without permissive CORS headers. Plan: Cloudflare Worker / Vercel Edge function that fetches server-side and streams bytes back.

### S3 / Object Storage
- [x] **S3 bucket connection** — S3/R2/MinIO as a connection type in the Sidebar. Per-connection credentials (bucket, key, secret, region, endpoint) stored as JSON in the DSN field. On connect: httpfs secret created with bucket SCOPE so multiple S3 connections coexist. File browser lists all .parquet/.csv/.json/.jsonl/.ndjson files; clicking opens a QueryNode with `read_parquet/read_csv_auto/read_json_auto`. Refresh button re-lists. DisconnectSaved drops the secret.

### Charts
- [ ] **More chart types** — Scatter matrix, waterfall, heatmap (histogram/boxplot/sankey done).

### MCP Expansions
- [x] **`import_s3` MCP tool** — Dedicated S3 import tool so agents can load files from `s3://` URIs directly (credentials from stored S3 settings).

### Canvas Structure
- [ ] **Canvas tabs** — Multiple named canvases as tabs within the app (Figma documents-style). Data model: `nodes` keyed by `canvasId`; SQLite `canvas_state` gets a `canvas_id` column + migration. Tab strip in toolbar. Significant effort — needs a dedicated planning session before touching persistence layer.

### Discovery & Content
- [x] **In-app changelog/news feed** — "What's New" tab in Settings modal; fetches GitHub releases API (`/repos/immannino/sql.garden/releases`); displays tag, date, and release body for last 10 releases; lazy-loads on tab open.
- [ ] **Dataset directory** — Hosted JSON index of curated public datasets (FRED, Census, Our World in Data, stock market, healthcare, etc.) with name, source URL, description, tags, and license. In-app "Explore" panel fetches + searches the index; clicking a dataset fires the existing URL import flow. Index starts as a hand-curated JSON file in a GitHub repo — no database needed. Strong differentiator; dataset index is a separate hosted artifact.
- [ ] **Community / Explore page** — Longer-term hub surfacing dataset directory, user-shared canvases, blog posts, and curated data stories. Builds on the dataset directory and news feed foundations.

### Reporting
- [ ] **Report / PDF export** — Export selected nodes as a formatted report. Approach: "report mode" re-renders selected nodes into a normal scrollable document layout (outside canvas coordinates), then `window.print()` or a Go-invoked Puppeteer/wkhtmltopdf subprocess. Chart SVGs export cleanly; tables need pagination. Scope as "export selected nodes as report" not "screenshot the canvas."

### Connections
- [ ] **SSH tunnel support** — Config for connecting to remote DBs via SSH port-forward.

### Desktop-specific
- [x] **Code signing & notarization** — Apple Developer account activated, secrets added to GitHub Actions, first notarization succeeded end-to-end. Signed + stapled builds ship on every `v*` tag push.
- [ ] **Auto-update download + relaunch** — Banner appears when update is available but only links to GitHub releases. Wire a native download-and-relaunch flow.
- [ ] **Windows smoke test** — CI builds the Windows binary but no manual QA done.

### Web (WASM) parity
- [ ] **Canvas persistence on web** — State resets on page reload; could use IndexedDB or localStorage.

### Website & Distribution
- [ ] **`immannino/sql.garden-www`** — New repo. Astro + Starlight: marketing homepage at `/`, docs at `/docs`. Deploy to GitHub Pages with custom domain `sql.garden`. DNSSimple: apex A records → GitHub Pages IPs. Content needed: hero + demo GIF, feature highlights, download CTA, Getting Started, node types, MCP setup, S3 setup, keyboard shortcuts.
- [ ] **Sandbox deployment** — Add CI job to existing `sql.garden` repo: build Vite frontend (Wasm mode) → push to `gh-pages` branch. GitHub Pages custom domain `sandbox.sql.garden`. DNSSimple: CNAME `sandbox` → `immannino.github.io`. Gate/hide desktop-only features (MCP tab, native file dialogs).
- [ ] **Web / mobile polish** — Canvas unusable on mobile (no touch pan/zoom, no tap-to-select). Minimum bar for Sandbox launch: pinch-to-zoom, two-finger pan, tap to select. Add "best experienced on desktop" banner for mobile viewports as a short-term graceful degradation.

---

## 🗑️ Tabled
- **Data loaders via local scripts** — Observable Framework-style shell-out loaders piping stdout into DuckDB tables. Post-v1, desktop-only via Wails exec.
- **SSH tunnels** — Post-v1.
- **Windows code signing** — No cert yet.
- **Run queries against connections** — Ad-hoc SQL editor per connection; moot because `SELECT * FROM alias.schema.table` already works via DuckDB ATTACH.
