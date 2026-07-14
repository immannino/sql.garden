---
layout: home

hero:
  name: "🌱 sql.garden"
  text: "An infinite canvas for your data"
  tagline: Query, visualize, and explore with DuckDB running in-process. Wire up AI agents via MCP. Your data never leaves your machine.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/introduction
    - theme: alt
      text: MCP Tools Reference
      link: /mcp/tools
    - theme: alt
      text: Try in Browser
      link: https://sql.garden/sandbox

features:
  - icon:
      src: /duckdb-logo.svg
      width: 32
      height: 32
    title: Local DuckDB
    details: DuckDB runs in-process — no servers, no cloud, no latency. Query Parquet, CSV, JSON, and live databases at native speed.
  - icon: 🌱
    title: Infinite Canvas
    details: Place query nodes, charts, tables, and notes anywhere. Pan, zoom, and group into sections. Build dashboards that match how you think.
  - icon: 🤖
    title: MCP / AI Integration
    details: sql.garden exposes a local MCP server. Connect Claude, Cursor, or Windsurf and let agents query data, build charts, and add nodes autonomously.
  - icon: ☁️
    title: S3 & Connections
    details: Connect to Postgres, MySQL, SQLite, and DuckDB files. Import directly from S3, R2, or MinIO. Parquet, CSV, JSON — whatever DuckDB speaks.
---
