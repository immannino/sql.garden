# Importing Data

## Drag and drop

Drop any supported file directly onto the canvas. sql.garden detects the format and loads it into DuckDB automatically.

**Supported formats:** CSV, TSV, Parquet, JSON, NDJSON

## Import File

Use the toolbar **Import → File** to open a file picker. The file is read once and the data is stored in DuckDB — the original file is not needed after import.

## Import URL

Paste any public URL to a data file. sql.garden fetches it and loads the result into DuckDB.

```
https://example.com/data/sales.parquet
https://raw.githubusercontent.com/.../.../data.csv
```

DuckDB supports remote Parquet and CSV files natively via its `httpfs` extension.

## Import S3

Load a file directly from S3, Cloudflare R2, or MinIO using an `s3://` URI.

**Requires:** S3 credentials configured in Settings → S3 Storage.

```
s3://my-bucket/data/orders.parquet
s3://my-bucket/logs/2024/events.json
```

## Import CSV data

Paste raw CSV text directly from the clipboard. Useful for quick one-off exploration without creating a file.

## Via MCP

If you have an AI agent connected, you can ask it to import data:

> "Import s3://my-bucket/sales.parquet as the table sales_data"

The agent calls `import_s3` or `import_url` on your behalf. See the [MCP tools reference](/mcp/tools).
