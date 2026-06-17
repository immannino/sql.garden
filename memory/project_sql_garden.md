---
name: project-sql-garden
description: sql.garden desktop app — architecture, shipped features, and roadmap
metadata:
  type: project
---

## What it is
Wails v2 desktop app (Go backend + Vue 3 frontend). Infinite canvas for DuckDB — drag, connect, and visualize data with TableNodes, QueryNodes, ChartNodes, MarkdownNodes, and SectionNodes.

## Architecture
- **Go backend**: DuckDB via go-duckdb, SQLite for canvas state, MCP HTTP server at 127.0.0.1:37421
- **Frontend**: Vue 3 + Pinia, Observable Plot for charts, Vitest for tests
- **MCP tools**: list_tables, list_canvas_nodes, run_query, add_query_node, add_chart_node, add_markdown_node, import_file, clear_canvas, fit_view

## Shipped features
- Canvas: pan (Space+drag / middle-click), zoom (scroll), marquee select, multi-drag, resize, Delete/Backspace to remove selected nodes
- Node types: Table, Query (with SQL editor + results + CSV/TSV/JSON/MD export), Chart (11 types, sourced or inline SQL, CSV export), Markdown, Section (labeled grouping box, moves children on drag)
- Refresh intervals on QueryNodes: Off / 5s / 30s / 1m / 5m / 30m
- Sample dataset library: F1, Markets, Economy, E-commerce, Weather — pre-built canvas layouts, shown on first launch
- Settings modal: Appearance (Light/Dark/System theme), MCP setup snippets, Updates tab
- Canvas Markdown export: queries + markdown nodes + table schemas → canvas-export.md
- Vitest suite: 11 schema store tests
- macOS: TitleBarHiddenInset, left-rail nav, 🌱 emoji app icon

## Roadmap / parked ideas

### Enhanced canvas export (parked — come back to this)
Current export is Markdown only (queries + markdown + table schemas; charts/sections skipped).
Plan: make it a proper export modal with format options:
- SVG export of each ChartNode (Observable Plot already renders SVG — just serialize the DOM node)
- Full HTML page: embed chart SVGs inline, include query results as `<table>`, styled
- PDF via browser print API
- Markdown enhanced: inline chart SVGs as `<img src="data:...">` tags
Implementation note: ChartCard renders into a `<div ref="plotContainer">` — grab its innerHTML for the SVG.

### Still on active roadmap
- Website Playground: polish the existing web-only branch as "try before you download"
- Getting started / onboarding: first-run tooltip tour or guided walkthrough
- App update story: check GitHub releases API for new version, show badge in Settings → Updates tab
- Windows build + signing
