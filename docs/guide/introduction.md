[![GitHub Stars](https://img.shields.io/github/stars/immannino/sql.garden?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden)
[![Latest Release](https://img.shields.io/github/v/release/immannino/sql.garden?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden/releases/latest)
[![License](https://img.shields.io/badge/license-GPL--3.0-18b569?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden/blob/main/LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/immannino/sql.garden/deploy.yml?style=flat-square&labelColor=0c0e14&color=18b569)](https://github.com/immannino/sql.garden/actions)

# Introduction

sql.garden is a local-first desktop app for querying and exploring data on an infinite canvas. It runs [DuckDB](https://duckdb.org) in-process — no servers, no cloud, no latency.

## What it is

- **An infinite canvas** where you place SQL query nodes, charts, data tables, and markdown notes
- **A local DuckDB environment** that can query Parquet, CSV, JSON, databases, and S3-compatible stores
- **An MCP server** that exposes the canvas to AI assistants like Claude and Cursor

## What it isn't

sql.garden is not a replacement for a production database GUI. It's a scratchpad and exploration tool — built for analysts, engineers, and anyone who thinks visually about data.

## Architecture

```
┌─────────────────────────────────┐
│          Desktop App            │
│  ┌──────────┐  ┌─────────────┐  │
│  │  Vue 3   │  │   Go/Wails  │  │
│  │ Frontend │◄─►│   Backend   │  │
│  └──────────┘  └──────┬──────┘  │
│                        │        │
│                ┌───────▼──────┐ │
│                │   DuckDB     │ │
│                │ (in-process) │ │
│                └──────┬───────┘ │
└───────────────────────┼─────────┘
                        │
          ┌─────────────▼──────────────┐
          │     MCP Server :37421      │
          │  Claude / Cursor / etc.    │
          └────────────────────────────┘
```

- The **frontend** is a Vue 3 app embedded via Wails. In the browser sandbox it runs as a standalone SPA.
- The **backend** (Go) owns DuckDB and exposes it via Wails bindings and the MCP server.
- The **MCP server** runs on `http://127.0.0.1:37421/mcp` and is reachable by any local MCP client.

## Open source

sql.garden is released under the [GPL v3 license](https://github.com/immannino/sql.garden/blob/main/LICENSE). The source is on [GitHub](https://github.com/immannino/sql.garden).
