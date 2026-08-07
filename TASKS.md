# sql.garden — Task Tracker

> Version: v0.0.1-alpha
> Updated: 2026-08-05 (session 14)

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
- [x] **Auto-refresh countdown indicator** — `↻ Xs` badge in footer (both normal and fullscreen views) when a refresh interval is active; ticks every second via a separate `_countdownTimer`; resets on each interval fire; green color matching the active-refresh select state.
- [x] Empty result set shows 0 rows instead of null error
- [x] Query history per node — History tab, per-node log of past SQL runs with restore
- [x] Error display moved to bottom — full multiline error in an expanding panel below the footer (red background, scrollable, dismissable ✕); removed truncated single-line error from status bar
- [x] Default card width 360px + `flex-wrap: wrap` on footer — prevents button overflow on narrower viewports

### Chart Nodes
- [x] Chart types: barY, barX, lineY, areaY, dot, cell, pie/donut, number, boolean, conditional, mermaid, table, histogram, boxplot, sankey, waterfall, heatmap, scatter-matrix
- [x] Source: inline SQL or linked Query node
- [x] Inline SQL editor (CodeMirror) with schema autocomplete
- [x] Chart-only view mode
- [x] Observable Plot rendering with padding fix (no header overhang)
- [x] Conditional formatting rules engine
- [x] Table chart with column config (format, align, hide, rename)
- [x] Chart legend toggle — show/hide color legend without re-running
- [x] Per-chart help text in properties panel (when/columns/tip)
- [x] AI chart prompt extended to all 14 types — `## Chart type guide` section in system prompt; `chart_type` enum updated in both Anthropic tool defs and MCP tool defs

### Canvas Operations
- [x] Multi-node alignment tools — 8 ops: left/centerH/right/top/middleV/bottom/distributeH/distributeV
- [x] Right-click context menu — Node: Duplicate, Bring to Front/Back, Delete. Canvas: Add nodes, Fit View.
- [x] Undo/redo — Cmd+Z / Cmd+Shift+Z, snapshots on all mutating operations (store: `snapshot()`, `undo()`, `redo()`, `canUndo`, `canRedo`)
- [x] Auto-fit mosaic layout — toolbar button + right-click "Wrap in Section" + "Mosaic Contents" on Section nodes; uses real DOM sizes via `data-node-id`
- [x] **Canvas tabs** — Multiple named canvases as tabs (Figma-style). Store: `_canvases` ref with `{ id, name, nodes[] }` per tab, writable computed `nodes` redirects all existing mutations. Tab strip in `CanvasTabs.vue`: × close (left, hover), name (center), ⌘N hint (right, always visible). ⌘1–⌘9 switch tabs (fires before `inInput` guard). Persistence: v2 format `{ version, activeCanvasId, canvases: [{id, name, nodes}] }`. `addCanvas` accepts `string | { id?, name? }` for MCP-supplied IDs.
- [x] **Node lineage arrows** — SVG overlay inside `.canvas-layer` (inherits canvas transform). Cubic bezier curves from right-edge of source query node to left-edge of dependent chart/data nodes. Uses `sourceId` field; 40% opacity `--accent` stroke with small arrowhead marker. Only rendered when lineage links exist.
- [x] **⌘K node search palette** — `SearchPalette.vue` with fuzzy name filter, kind chips, keyboard nav (↑↓ Enter Esc), jumps to node via `focusNode`. Fixed hardcoded dark-mode colors → CSS variables for light theme support.

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
- [x] `list_canvas_nodes` extended — now returns both `query` and `data` nodes, with kind prefix in output; accepts optional `canvas_id` filter for multi-canvas setups
- [x] `set_node_color` MCP tool — set accent color of any canvas node by id; useful for highlighting KPIs or flagging anomalies
- [x] `add_section` MCP tool — create a named Section container; width/height configurable, defaults to 400×300
- [x] **Canvas tab MCP tools** — `list_canvases`, `create_canvas` (random `mcp_tab_<hex>` ID, returned in response), `rename_canvas`, `switch_canvas`, `remove_canvas`; all node-adding tools accept optional `canvas_id` to target any tab (defaults to active); `clear_canvas` scoped by `canvas_id`; `CanvasAction` struct extended with `canvasId` field; `mcpCanvases sync.Map` on `App` tracks MCP-created tabs; `addNodeToCanvas` store helper temporarily redirects `_activeId` for cross-tab writes

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

### Open Source
- [x] **GPL v3 license** — `LICENSE` file with full GPL v3 text; copyright `2024-2026 Tony Mannino <goodbarnhello@gmail.com>`
- [x] **Security policy** — `SECURITY.md` with private disclosure email and 72hr response / 14-day fix SLA
- [x] **CONTRIBUTING.md** — Bug reports, feature requests, dev setup (Wails dev / web sandbox / docs), branch conventions, Red/Green MCP test requirement, sensitive areas (persistence migrations, Wails bindings, DuckDB single-connection).
- [x] **Code of Conduct** — Contributor Covenant v2.1; enforcement contact goodbarnhello@gmail.com.

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
- [x] **Canvas tab tool tests** — `tools/list` updated to assert `list_canvases`, `create_canvas`, `rename_canvas`, `switch_canvas`, `remove_canvas` are registered; 22 new scenarios across 8 groups: list_canvases, create_canvas (×2 including ID extraction from response), rename_canvas (×3), switch_canvas (×2), remove_canvas (×2), `canvas_id` targeting on 4 node tools, `list_canvas_nodes` filter (×2), `clear_canvas` scoped (×2), end-to-end multi-canvas flow; `extractCanvasID` helper parses `id="..."` from response text

---

## 🔧 Known Issues / Needs Verification

- [x] **Clipboard shim** — Go `ClipboardGet`/`ClipboardSet` methods and JS bindings are in place; `wailsjs/go/main/App.js` regenerated as part of normal build cycle.

- [x] **Edit menu items are greyed out** — Removed the Edit submenu entirely; clipboard works via the JS shim (Cmd+X/C/V/A/Z handled in App.vue and SqlEditor.vue), so the menu added no value and all items appeared disabled.

---

## 📋 Backlog

### Polish / QoL
- [ ] **Section auto-resize** — Option to auto-expand a Section node to wrap its contained nodes.
- [ ] **Pinned/auto-run queries** — Option to run a query node automatically on canvas open.
- [x] **QueryNode fullscreen editing mode** — Expand button in card header (hover-visible). Teleports to `body` as a fixed full-screen overlay: header with node name + Esc hint + ✕, full-height SQL editor, results/history tabs, all footer actions. Esc closes. Canvas state preserved underneath. Registered Esc via `capture: true` so it intercepts before canvas handlers.
- [x] **Versioned DuckDB docs links under (?) help icon** — `GetDuckDBVersion()` Go method queries `SELECT version()` at runtime. HelpPanel fetches it on mount, shows version badge next to "DuckDB Docs" link, and constructs `duckdb.org/docs/archive/{major}.{minor}/` URL. Falls back to stable docs on web/error.

### Query / Data
- [ ] **CORS proxy for URL imports** — URL imports fail for servers without permissive CORS headers. Plan: Cloudflare Worker / Vercel Edge function that fetches server-side and streams bytes back.
- [ ] **Generator / Ingestion node** — New node type that writes to a DataNode on a schedule. Two modes: (1) **Generator** — pure SQL INSERT using DuckDB functions (`now()`, `random()`, `gen_random_uuid()`) runs on a timer, no network, works offline; good for synthetic live data in the tutorial. (2) **Ingestion** — fetches a URL/API on a schedule, parses response, appends/upserts to a DataNode; dedup UI ("append all / skip duplicates / replace duplicates") compiles to `ON CONFLICT DO NOTHING` or a staging merge. Scheduling: simple intervals first (reuses `refreshInterval` pattern on Go side), full cron expressions (`robfig/cron`) as follow-on. Desktop-first — Go scheduler runs independent of webview so backgrounding isn't an issue. Card shows: last run time, rows added this run, total rows in target, run-now button. Replaces the tabled "data loaders" idea with a safer structured approach. MCP tool (`add_ingest_node`, `set_ingest_schedule`) as follow-on.

### S3 / Object Storage
- [x] **S3 bucket connection** — S3/R2/MinIO as a connection type in the Sidebar. Per-connection credentials (bucket, key, secret, region, endpoint) stored as JSON in the DSN field. On connect: httpfs secret created with bucket SCOPE so multiple S3 connections coexist. File browser lists all .parquet/.csv/.json/.jsonl/.ndjson files; clicking opens a QueryNode with `read_parquet/read_csv_auto/read_json_auto`. Refresh button re-lists. DisconnectSaved drops the secret.

### Charts
- [x] **More chart types** — Waterfall, heatmap, scatter-matrix added (all 14 types done: barY/barX/lineY/areaY/dot/cell/pie/donut/histogram/boxplot/sankey/waterfall/heatmap/scatter-matrix + number/boolean/conditional/mermaid/table display types).

### MCP Expansions
- [x] **`import_s3` MCP tool** — Dedicated S3 import tool so agents can load files from `s3://` URIs directly (credentials from stored S3 settings).

### Canvas Structure
- [x] **Canvas tabs** — See Canvas Operations above. Done.

### Samples & Learning
- [x] **Sample picker UX — multi-section layout** — Three-tab modal: Built-in / Learn SQL / Community (coming soon). Level badges, chapter/row counts, tags.
- [x] **In-app SQL learning track** — "SQL Fundamentals: E-Commerce" — 8 chapters (SELECT → WHERE → ORDER BY → GROUP BY → JOIN → Window Functions → CTEs), 4 tables (100 customers, 40 products, ~499 orders, ~1100 items) generated in DuckDB SQL, no bundled file. Wired into Learn tab in picker.
- [ ] **Personal finance / bank statement template** — In-app canvas with synthetic transaction data (generated via DuckDB SQL, no external file). Shows the full pipeline: raw import schema → normalization queries → spending by category, income vs expenses, merchant trends. Markdown node explains how to adapt column names to a real export. Lineage arrows make the raw→clean→chart flow visible.
- [ ] **Data stories (remote, post-launch)** — Remote catalog of interesting public-data canvases (GDP + recession markers, CO₂ trends, etc.). Depends on dataset directory / proxy API infrastructure. Not in-app.

### Template Distribution
- [ ] **CDN template catalog** — `templates.json` index hosted on the marketing site CDN listing available community/educator packs (name, description, author, tags, URL to `.sql.garden.json`). Sample picker fetches it lazily and renders a "Community" section. One-click import calls existing `import_url` path. Educators host their own pack files anywhere; sql.garden only hosts the index for curated/verified packs.
- [x] **`sqlgarden://` custom URL scheme** — `protocols` registered in `wails.json` (auto-generates `CFBundleURLTypes` in Info.plist). macOS: `Mac.OnUrlOpen` callback in `main.go` calls `app.handleDeepLink`. Windows: `os.Args[1]` checked on startup. Both parse `sqlgarden://import?pack=<url>` and emit `deep-link:import` Wails event. Web sandbox: reads `?import=<url>` query param on page load. `FetchPackJSON` Go method fetches pack server-side (CORS bypass). Confirmation dialog shown before any import.
- [x] **Import confirmation dialog** — Inline modal in App.vue (Teleport to body). Shows source URL + "SQL will run" warning, Cancel / Import buttons. On confirm: `FetchPackJSON` → parse → `importNodeBundle`. `ingest` nodes now handled in `importNodeBundle`.

### Discovery & Content
- [x] **In-app changelog/news feed** — "What's New" tab in Settings modal; fetches GitHub releases API (`/repos/immannino/sql.garden/releases`); displays tag, date, and release body for last 10 releases; lazy-loads on tab open.
- [ ] **Dataset directory + proxy API** — See full design below. Post-launch; data stories depend on this.
- [ ] **Community / Explore page** — Longer-term hub surfacing dataset directory, user-shared canvases, blog posts, and curated data stories. Builds on the dataset directory and news feed foundations.

### Reporting
- [ ] **Report / PDF export** — Export selected nodes as a formatted report. Approach: "report mode" re-renders selected nodes into a normal scrollable document layout (outside canvas coordinates), then `window.print()` or a Go-invoked Puppeteer/wkhtmltopdf subprocess. Chart SVGs export cleanly; tables need pagination. Scope as "export selected nodes as report" not "screenshot the canvas."

### Connections
- [ ] **SSH tunnel support** — Config for connecting to remote DBs via SSH port-forward.

### Desktop-specific
- [x] **Code signing & notarization** — Apple Developer account activated, secrets added to GitHub Actions, first notarization succeeded end-to-end. Signed + stapled builds ship on every `v*` tag push.
- [ ] **Auto-update download + relaunch** — Banner exists, links to GitHub releases. Needs: native download-and-relaunch flow. Also needs: GitHub Releases/Tags cleanup (stale pre-release tags cluttering the releases page).
- [ ] **Windows smoke test** — CI builds the Windows binary but no manual QA done.

### Web (WASM) parity
- [ ] **Canvas persistence on web** — State resets on page reload; could use IndexedDB or localStorage.

### Docs
- [ ] **MCP docs update** — `docs/` VitePress site MCP tools reference is out of date. New tools to document: `resize_node`, `move_node`, `focus_node`, `update_query_node`, `import_csv_data`, `import_url`, `import_s3`, `materialize_query`, `set_node_color`, `add_section`, `list_canvases`, `create_canvas`, `rename_canvas`, `switch_canvas`, `remove_canvas`. All node-adding tools now accept optional `canvas_id`. `list_canvas_nodes` accepts optional `canvas_id` filter. `clear_canvas` is now scoped by `canvas_id`. `add_chart_node` `chart_type` enum extended to 14 types.

### Website & Distribution
- [x] **VitePress docs site** — Scaffolded at `docs/`, serves from `/` (root). Custom theme: brand green `#18b569`, MockCanvas hero, light/dark theme support. Content: Introduction, Installation, Quick Start, Nodes (all 15 chart types), Connections, Import, MCP overview/config/tools reference. GitHub badges in hero and footer. "OPEN SOURCE · LOCAL-FIRST" pill above footer.
- [x] **Deprecated `website/` Astro site** — VitePress is now the homepage. `website/` directory deleted. `deploy.yml` simplified: docs → `/`, sandbox → `/sandbox/`.
- [x] **Sandbox CI deployment** — `frontend/vite.web.config.ts` base set to `/sandbox/`. CI assembles docs + sandbox into single Pages artifact. DuckDB Wasm fallback to single-threaded bundle (no COOP/COEP on Pages).
- [x] **MockCanvas hero component** — Dot-grid canvas preview, three nodes (Query/Data/Chart), SVG connectors. Light/dark theme reactive. Desktop: right of hero text. Mobile (≤959px): stacked column below CTAs via `home-hero-after` slot.
- [x] **Custom domain DNS** — DNSSimple apex A records → GitHub Pages IPs for `sql.garden`. GitHub Pages custom domain configured.
- [ ] **Web / mobile polish** — Sandbox canvas unusable on mobile (no touch pan/zoom, tap-to-select). Minimum: pinch-to-zoom, two-finger pan, tap to select. Short-term: "best on desktop" banner.

---

## 🗗 Dataset Directory — Design

### Goal
A curated library of public datasets importable in one click from inside sql.garden (desktop + sandbox). Strong differentiator: you open the app, see interesting data, and start querying immediately.

### Architecture

```
sql.garden app
  └─ "Explore" sidebar tab
       └─ fetches index.json from api.sql.garden/datasets
            └─ Go API (Fly.io / Railway)
                 ├─ GET /datasets          → paginated + filterable index
                 ├─ GET /datasets/:slug    → metadata + download URL
                 └─ GET /proxy/:slug       → streams file from R2/S3 (CORS bypass)

Cloudflare R2 bucket (or AWS S3)
  └─ /datasets/:slug/:file.parquet   (pre-processed snapshots)
  └─ index.json                       (auto-regenerated by updater job)

Updater job (Go binary, cron via Fly Machines or GitHub Actions schedule)
  └─ fetches source URLs (FRED, Census, OurWorldInData, etc.)
  └─ converts to Parquet via DuckDB
  └─ uploads to R2
  └─ rewrites index.json
```

### Index schema (`index.json`)
```json
{
  "updated_at": "2026-07-14T00:00:00Z",
  "datasets": [
    {
      "slug": "fred-us-gdp",
      "name": "US GDP (FRED)",
      "description": "Quarterly US real GDP from the Federal Reserve Economic Data.",
      "tags": ["economics", "united states", "time series"],
      "source": "https://fred.stlouisfed.org/series/GDPC1",
      "license": "Public Domain",
      "updated_at": "2026-07-01",
      "rows": 312,
      "size_bytes": 18400,
      "file": "fred-us-gdp/data.parquet"
    }
  ]
}
```

### In-app "Explore" panel (sidebar tab)
- Search box filters by name/tag/description client-side
- Tag chips for quick filtering (Economics, Health, Climate, Sports, etc.)
- Each row: name, row count, license badge, source link, **Import** button
- Import fires `import_url` using the proxy URL → creates TableNode + auto-runs a QueryNode
- Sandbox: uses `/proxy/:slug` route (CORS bypass). Desktop: direct R2 URL or proxy.

### Initial dataset wishlist
| Slug | Source | Update cadence |
|------|--------|----------------|
| fred-us-gdp | FRED GDPC1 | Quarterly |
| fred-cpi | FRED CPIAUCSL | Monthly |
| ourworldindata-co2 | OWID CO₂ | Annual |
| census-us-pop | US Census Bureau | Annual |
| yahoo-sp500-daily | Yahoo Finance | Daily |
| github-top-repos | GH Archive | Weekly |
| openaq-air-quality | OpenAQ API | Daily |
| imdb-top-movies | IMDb datasets | Weekly |

### Go API tasks
- [ ] **Repo** — New `sql.garden-api` repo (or `api/` subdirectory). Go + `net/http` + R2 SDK.
- [ ] **`GET /datasets`** — Reads `index.json` from R2, returns filtered/paginated JSON. Query params: `q`, `tag`, `page`, `limit`.
- [ ] **`GET /proxy/:slug`** — Streams R2 object to client with `Content-Type: application/octet-stream` + CORS headers. Replaces Cloudflare Worker plan.
- [ ] **Updater job** — Fetches sources, converts to Parquet via `go-duckdb`, uploads to R2, regenerates `index.json`. Cron via Fly Machines scheduler or GitHub Actions `schedule:`.
- [ ] **Deploy** — Fly.io (free tier, single small machine). `fly.toml` with health check on `/health`.

### In-app tasks
- [ ] **Explore sidebar tab** — New tab in `Sidebar.vue`, fetch + render dataset index, search/filter UI.
- [ ] **One-click import** — Calls `ImportFromUrl` with the proxy URL, creates TableNode, auto-generates a QueryNode pointed at it.
- [ ] **MCP tool: `import_dataset`** — Agents can import by slug: `{"slug": "fred-us-gdp"}`. Proxy URL resolved server-side.

---

## 📚 In-App SQL Learning Track — Design

### Goal
A curated canvas shipped with the binary that gets a user from "I've never written SQL" to "I can answer real business questions" in a single sitting. Not a textbook — every concept answers a concrete question against real (synthetic) data. The canvas format does pedagogical work that a tutorial page can't: query, result, and next step are all visible at once.

### Theme: E-commerce / Sales
Best fit because:
- Universally relatable domain — orders, customers, products
- Schema complexity scales naturally (start with one table, add JOINs later)
- Every concept maps to a meaningful business question, not abstract syntax
- Charts emerge naturally at every level (bar for category, line for trends)
- What most people will use SQL for in their actual jobs

Synthetic data generated entirely via DuckDB SQL (`generate_series`, random functions, `strftime`) — no bundled file, always works, teaches a useful technique.

### Canvas layout
Linear left-to-right progression with a Markdown "chapter header" node before each concept cluster. Lineage arrows connect source tables → queries → charts throughout.

### Chapters

| # | Concept | Business question answered |
|---|---------|---------------------------|
| 0 | Orientation | Data model overview — what tables exist and how they relate |
| 1 | SELECT basics | What are my most recent 10 orders? |
| 2 | Filtering (WHERE) | Which orders are over $500? Which customers are in New York? |
| 3 | Sorting & limiting | Who placed the largest single order? |
| 4 | Aggregation (GROUP BY) | How much revenue does each product category generate? |
| 5 | JOIN | Which customers have placed the most orders? (orders JOIN customers) |
| 6 | Date functions | How has monthly revenue trended over the last year? |
| 7 | Subqueries / CTEs | What's the average order value per customer, and who's above average? |
| 8 | Window functions | Running total revenue; rank customers by lifetime value |

Each chapter: 1 Markdown node (question + brief hint), 1–2 QueryNodes with SQL pre-written and results pre-run, 1 ChartNode where a chart is the natural output.

### Philosophy
- Every query answers a **question**, not "here is GROUP BY"
- SQL is pre-written and runnable — users see working code immediately, tweak from there
- Markdown nodes are brief: question, one-line hint, maybe a "try changing X" nudge
- No hand-holding beyond that — the canvas is a starting point, not a classroom
- If it ends up used by educators, great — but design for the curious practitioner

### Implementation tasks
- [ ] **Synthetic data generator** — DuckDB SQL that creates `lrn_customers`, `lrn_products`, `lrn_orders`, `lrn_order_items` with realistic names, dates, amounts (~500 orders, ~200 customers, ~50 products). Seeded random so data is consistent across loads. Data should have real shape: a clear top category, monthly seasonality, a handful of power customers — so charts produce meaningful output.
- [ ] **Chapter canvas layout** — Design the node positions, section groupings, and markdown content for all 8 chapters.
- [ ] **Wire into sample picker** — "Learn" section in the picker UI; loads the canvas fresh (clears current state with confirm).

### Educator distribution
The learning track ships in-app, but the same canvas exported as `.sql.garden.json` can be hosted anywhere and distributed via a `sqlgarden://import?pack=<url>` deep link. An educator can build their own variant, host it on their school LMS or GitHub, and share a single link that opens sql.garden and imports their pack automatically. sql.garden provides the mechanism; educators own the content and hosting.

---

## 🗑️ Tabled
- **Data loaders via local scripts** — Observable Framework-style shell-out loaders piping stdout into DuckDB tables. Superseded by the Generator/Ingestion node, which covers the same use cases without the shell security surface.
- **SSH tunnels** — Post-v1.
- **Windows code signing** — No cert yet.
- **Run queries against connections** — Ad-hoc SQL editor per connection; moot because `SELECT * FROM alias.schema.table` already works via DuckDB ATTACH.
