# 🌱 sql.garden

[![GitHub Stars](https://img.shields.io/github/stars/immannino/sql.garden?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden)
[![Latest Release](https://img.shields.io/github/v/release/immannino/sql.garden?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden/releases/latest)
[![License](https://img.shields.io/badge/license-GPL--3.0-18b569?style=flat-square&labelColor=0c0e14)](https://github.com/immannino/sql.garden/blob/main/LICENSE)
[![Build](https://img.shields.io/github/actions/workflow/status/immannino/sql.garden/deploy.yml?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden/actions)

An infinite canvas for your data. Query, visualize, and explore with DuckDB running in-process. Wire up AI agents via MCP. Your data never leaves your machine.

**[sql.garden](https://sql.garden)** · **[Docs](https://sql.garden/docs)** · **[Sandbox](https://sql.garden/sandbox)**

---

## Features

- **Infinite canvas** — place SQL query nodes, charts, tables, and markdown notes anywhere; pan and zoom freely
- **Local DuckDB** — in-process, no servers; query Parquet, CSV, JSON, live databases, and S3-compatible stores at native speed
- **MCP server** — exposes a local Model Context Protocol server so Claude, Cursor, and Windsurf can control the canvas autonomously
- **S3 / object storage** — import directly from AWS S3, Cloudflare R2, or MinIO using `s3://` URIs
- **Database connections** — connect to Postgres, MySQL, and SQLite alongside your local files
- **Browser sandbox** — a WebAssembly build runs at [sql.garden/sandbox](https://sql.garden/sandbox) with no install required

## Download

| Platform | Link |
|----------|------|
| macOS (Apple Silicon / Intel) | [Latest release](https://github.com/immannino/sql.garden/releases/latest) |
| Windows 10+ | [Latest release](https://github.com/immannino/sql.garden/releases/latest) |
| Browser | [sql.garden/sandbox](https://sql.garden/sandbox) |

## MCP quick start

With sql.garden running, add this to your AI tool's config:

**Claude Desktop / Claude Code** (`~/.claude/claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "sql-garden": {
      "url": "http://127.0.0.1:37421/mcp"
    }
  }
}
```

Or via CLI:
```bash
claude mcp add --transport http sql-garden http://127.0.0.1:37421/mcp
```

Then ask your agent:
> "Import the orders table from S3, run a revenue-by-month query, and add a bar chart."

See the [MCP tool reference](https://sql.garden/docs/mcp/tools) for all 20 available tools.

## Building from source

**Prerequisites:** Go 1.22+, Node 20+, [Wails v2](https://wails.io)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/immannino/sql.garden
cd sql.garden

# Dev mode (hot reload)
wails dev

# Production build
wails build
```

### Web sandbox only

```bash
cd frontend
npm ci
npm run build:web        # outputs to frontend/dist-web/
npm run dev:web          # local dev server
```

### Docs

```bash
cd docs
npm install
npm run dev              # local VitePress dev server at :5173
```

## Project structure

```
sql.garden/
├── app.go               # Wails app — DuckDB, S3, canvas state
├── ai.go                # AI/MCP tool implementations
├── mcp.go               # MCP server (port 37421)
├── persistence.go       # Canvas JSON save/load
├── frontend/            # Vue 3 frontend (desktop + web builds)
├── website/             # Astro landing page → sql.garden/
├── docs/                # VitePress documentation → sql.garden/docs/
├── cmd/mcp-test/        # Integration test suite for MCP tools
└── build/               # Wails build assets (icons, entitlements)
```

## License

sql.garden is open source under the [GNU General Public License v3.0](LICENSE).

Copyright © 2024–2026 Tony Mannino
