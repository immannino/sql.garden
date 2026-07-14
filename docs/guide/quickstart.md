# Quick Start

## 1. Open the app

Launch sql.garden. You'll land on an empty infinite canvas.

## 2. Import some data

**Drag and drop** a CSV or Parquet file onto the canvas — it loads instantly into DuckDB and appears as a data node.

Or use the toolbar:
- **Import File** — opens a file picker (CSV, Parquet, JSON, NDJSON)
- **Import URL** — paste a public URL to a data file
- **Import S3** — load from S3, R2, or MinIO (configure credentials in Settings → S3)

## 3. Write a query

Click **+ Query** to add a query node. Type SQL and press `⌘ Enter` (macOS) or `Ctrl Enter` (Windows) to run it.

```sql
SELECT *
FROM your_table
LIMIT 100
```

Results appear inline in the node.

## 4. Add a chart

With a query node selected, click **+ Chart**. Pick a chart type and map your columns to axes. The chart updates live as you edit the query.

## 5. Connect an AI agent

Open **Settings → MCP**, copy the config snippet, and paste it into your AI tool's config. Then ask it:

> "Show me the top 10 customers by revenue from the orders table and add a bar chart."

See the [MCP overview](/mcp/overview) for full setup instructions.
