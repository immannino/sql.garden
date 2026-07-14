# MCP Overview

sql.garden runs a local [Model Context Protocol](https://modelcontextprotocol.io) server on `http://127.0.0.1:37421/mcp`. Any MCP-compatible AI tool can connect to it and control the canvas programmatically.

## What the agent can do

- Query tables and inspect schemas
- Add query nodes, chart nodes, markdown notes, and sections to the canvas
- Import files from disk, URLs, or S3
- Move, resize, and color nodes
- Materialize query results as persistent tables
- Clear the canvas or fit the view

## Supported clients

| Client | Method |
|--------|--------|
| Claude Desktop | `claude_desktop_config.json` |
| Claude Code | `claude mcp add` CLI command |
| Cursor | `.cursor/mcp.json` |
| Windsurf | MCP settings panel |
| Any MCP client | HTTP streamable transport at `:37421/mcp` |

## How it works

The MCP server starts automatically when sql.garden launches. It binds to `127.0.0.1:37421` — localhost only, never exposed to the network.

Each tool call is executed synchronously against the live DuckDB instance and canvas state. Results are returned as plain text so any model can consume them.
