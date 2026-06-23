# sql.garden — Task Tracker

> Version: v0.0.0-alpha.6
> Updated: 2026-06-23

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

### Chart Nodes
- [x] Chart types: barY, barX, lineY, areaY, dot, cell, pie/donut, number, boolean, conditional, mermaid, table
- [x] Source: inline SQL or linked Query node
- [x] Inline SQL editor (CodeMirror) with schema autocomplete
- [x] Chart-only view mode
- [x] Observable Plot rendering with padding fix (no header overhang)
- [x] Conditional formatting rules engine
- [x] Table chart with column config (format, align, hide, rename)

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

### Keyboard & Input
- [x] Global hotkey guard: no app shortcuts fire when focus is in a text field or code editor
- [x] Cmd+A/C/X/V shim for plain input/textarea in Wails WKWebView (via Go runtime.Clipboard*)
- [x] CodeMirror clipboard keymaps (copy/paste route through Go runtime, not DOM events)
- [x] Delete/Backspace bulk-deletes selected canvas nodes (with table data cleanup)

### CI / Release
- [x] GitHub Actions: macOS universal build + Windows build on tag push
- [x] macOS .app zipped before artifact upload (preserves bundle structure)

---

## 🔧 Known Issues / Needs Verification

- [ ] **Clipboard shim needs rebuild** — The Go `ClipboardGet`/`ClipboardSet` methods and their JS bindings are new. Requires `wails dev` or `wails build` to regenerate `wailsjs/go/main/App.js` before the shim in App.vue and SqlEditor.vue takes effect.

- [ ] **Edit menu items are greyed out** — The Edit submenu was added with `nil` callbacks; Wails doesn't wire nil-callback items to the native NSResponder selectors so they appear disabled in the menu bar. Clipboard now works via the JS shim, but the menu looks wrong. Options: remove the Edit menu entirely (clipboard shim handles everything), or replace nil callbacks with JS-dispatching callbacks that call `document.execCommand`.

---

## 📋 Backlog

### Polish / QoL
- [x] **Node color picker** — Hover header to reveal palette button; 12 preset swatches + native color input. Works on all node types including sections.
- [x] **Canvas background grid/dots** — Optional dot grid overlay, toggle in Settings > Appearance.
- [x] **Node search / jump-to** — Cmd+K palette: search nodes by name, jump viewport to them.
- [x] **Duplicate node** — Cmd+D clones selected node(s) offset by 24px, auto-selects the copies.
- [x] **Undo/redo for canvas operations** — Cmd+Z / Cmd+Shift+Z. Snapshots on add, delete, duplicate, z-order, drag start, resize start.
- [ ] **Section auto-resize** — Option to auto-expand a Section to wrap its contained nodes.
- [ ] **Multi-node alignment tools** — Align left/right/top/bottom, distribute evenly.
- [x] **Right-click context menu** — Node menu: Duplicate, Bring to Front/Back, Delete. Canvas menu: Add Query/Chart/Note/Section, Fit View.

### Query / Data
- [x] **DuckDB-specific autocomplete** — 150+ DuckDB functions added to CodeMirror, requires 2+ chars, boost -1 so schema results float above.
- [x] **Query result column type badges** — INT/FLOAT/TEXT/BOOL/DATE/TS chips in results table headers (QueryCard + QueryPanel).
- [ ] **Query history per node** — Log of previously run SQL per node, mini timeline to revert.
- [ ] **Pinned/auto-run queries** — Option to run a query node automatically on open.
- [ ] **CORS proxy for URL imports** — URL imports fail for servers without permissive CORS headers. Plan: Cloudflare Worker / Vercel Edge function that fetches server-side and streams bytes back.

### Charts
- [x] **Chart export as PNG/SVG** — Download button on chart header; native save dialog on desktop, browser download on web. Fonts and CSS vars inlined for standalone SVG.
- [ ] **Chart legend toggle** — Show/hide Observable Plot legend without re-running query.
- [ ] **More chart types** — Histogram, scatter matrix, waterfall, heatmap.

### Connections
- [x] **Create query node from connection table** — Hover a table/view in the Connections explorer and click `+` to hoist it as a canvas QueryNode with `SELECT * FROM alias.schema.table LIMIT 1000`.
- [ ] **Run queries against connections** — Ad-hoc SQL editor scoped to an attached connection (currently connections only have schema explorer).
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
