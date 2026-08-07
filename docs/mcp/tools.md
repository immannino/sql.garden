# MCP Tool Reference

All tools are available via the MCP server at `http://127.0.0.1:37421/mcp`.

::: tip Canvas IDs
Most node-creation and import tools accept an optional `canvas_id` parameter. Omit it to target the active canvas, or pass an ID from `list_canvases` to write to a specific tab.
:::

---

## Data tools

### `list_tables`
List all tables and views currently loaded in DuckDB.

_No parameters._

---

### `run_query`
Execute a SQL query and return the results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `sql` | string | ✓ | SQL to execute |

---

### `materialize_query`
Run a SQL query and persist the results as a DuckDB table, then add it to the canvas as a data node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `sql` | string | ✓ | SQL to execute |
| `table_name` | string | ✓ | Name for the new table |
| `source_id` | string | | Node ID of a query node to link for staleness tracking |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

## Canvas inspection

### `list_canvases`
Return all canvas tabs with their IDs and names. The active tab is marked `[active]`.

_No parameters._

---

### `list_canvas_nodes`
Return all nodes on the canvas with their IDs, kinds, positions, and sizes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `canvas_id` | string | | Filter to a specific canvas tab; omit to list all canvases |

---

## Canvas tab management

### `create_canvas`
Create a new canvas tab and switch to it.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | | Display name for the new tab (default: `Canvas N`) |

Returns the new canvas ID in the response text — capture it to target subsequent node-creation calls.

---

### `rename_canvas`
Rename an existing canvas tab.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `canvas_id` | string | ✓ | ID from `list_canvases` |
| `name` | string | ✓ | New display name |

---

### `switch_canvas`
Switch the active canvas tab.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `canvas_id` | string | ✓ | ID from `list_canvases` |

---

### `remove_canvas`
Delete a canvas tab and all its nodes.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `canvas_id` | string | ✓ | ID from `list_canvases` |

---

## Node creation

### `add_query_node`
Add a SQL query node to the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Display label for the node |
| `sql` | string | ✓ | SQL to populate the node |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

Returns a node ID — pass it as `source_id` to `add_chart_node`.

---

### `add_chart_node`
Add a chart node that visualizes query results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Display label |
| `chart_type` | string | ✓ | See chart types below |
| `x_column` | string | ✓ | Column for the X axis (or labels for pie/donut) |
| `y_column` | string | ✓ | Column for the Y axis (or values for pie/donut) |
| `source_id` | string | | Node ID of a query/data node to source data from |
| `sql` | string | | Inline SQL (alternative to `source_id`) |
| `color_column` | string | | Column to use for color grouping |
| `label_column` | string | | Column to use for point labels |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

**Chart types:**

| `chart_type` | Label | Columns needed |
|--------------|-------|----------------|
| `barY` | Bar (vertical) | X: category or time · Y: numeric |
| `barX` | Bar (horizontal) | X: numeric · Y: category |
| `lineY` | Line | X: time or sequential · Y: numeric |
| `areaY` | Area | X: time or sequential · Y: numeric |
| `dot` | Scatter | X: numeric · Y: numeric |
| `cell` | Cell grid | X: category · Y: category · Color: numeric |
| `pie` | Pie | X: label column · Y: value column |
| `donut` | Donut | X: label column · Y: value column |
| `histogram` | Histogram | X: numeric column to bin |
| `boxplot` | Box plot | X: category (group) · Y: numeric |
| `sankey` | Sankey | X: source column · Y: target column · Color: value column |
| `waterfall` | Waterfall | X: category · Y: numeric (positive = gain, negative = loss) |
| `heatmap` | Heatmap | X: column · Y: row · Color: numeric value |
| `scatter-matrix` | Scatter matrix | X/Y: two or more numeric columns |
| `number` | Big number | First row · first numeric column |
| `boolean` | Status badge | First row · first boolean-like column |
| `conditional` | Conditional | Any column · rules evaluated top-to-bottom |
| `mermaid` | Mermaid diagram | No data — write diagram syntax directly in chart SQL field |
| `table` | Table | All result columns shown (configurable per-column) |

---

### `add_markdown_node`
Add a freeform markdown text node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Title shown in the node header |
| `content` | string | ✓ | Markdown body |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `add_section`
Add a named section container to the canvas. Sections appear behind other nodes and are useful for grouping.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Section label |
| `width` | number | | Width in canvas pixels (default: 400) |
| `height` | number | | Height in canvas pixels (default: 300) |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `add_ingest_node`
Add a Generator or Ingestion node — a node that produces or fetches data on a schedule.

**Generator mode** runs a DML SQL statement (INSERT, UPDATE, etc.) against DuckDB on a timer. Good for synthetic live data, simulations, or any workload that generates rows using DuckDB functions (`now()`, `random()`, `gen_random_uuid()`).

**Ingestion mode** fetches a URL on a schedule and appends or replaces a target table. Requires the desktop app (Go-side fetch bypasses CORS).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Display label for the node |
| `mode` | string | | `generator` or `ingestion` (default: `generator`) |
| `sql` | string | | Generator mode: DML SQL to run on each tick |
| `url` | string | | Ingestion mode: URL to fetch (CSV / Parquet / JSON) |
| `target_table` | string | | Ingestion mode: DuckDB table to write into |
| `conflict_mode` | string | | `append` or `replace` (default: `append`) |
| `interval` | number | | Run interval in seconds; fractional values for sub-second (e.g. `0.25` = 250 ms). `0` = manual only |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `add_exercise_node`
Create a self-contained exercise node with an embedded SQL editor and validator. Students write SQL directly inside the card and press **Run** to check their work against educator-defined checks. Chain multiple exercises together with `next_id` to build a guided lesson flow. Checks (`set_match`, `row_count`, `non_empty`, `no_nulls`, `column_value`, `column_exists`, `sql_pattern`) are configured interactively in the node's Edit mode after creation.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Name for the node (e.g. `ex1_select_basics`) — used as a stable ID |
| `sql` | string | | Starter SQL pre-filled in the student's editor (e.g. `SELECT ??? FROM products`) |
| `prompt` | string | | Markdown prompt shown to the student describing the exercise |
| `success_text` | string | | Markdown revealed when all checks pass — use for explanations, encouragement, or hints |
| `next_id` | string | | Node ID of the next exercise; shows a **Next →** button when this exercise passes |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

**Example:**
```json
{
  "name": "ex1_select_all",
  "sql": "SELECT ??? FROM products",
  "prompt": "Write a SELECT query that returns **all rows** from the `products` table.",
  "success_text": "Nice work! `SELECT *` is the simplest way to fetch every column.",
  "next_id": "mcp_ex2_where_clause"
}
```

---

### `add_test_node`
Create a TestNode that runs a SQL query on a schedule or manually and validates the result against configured checks. Use for data quality monitoring — automated assertions against pipeline outputs, row count guards, freshness checks.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Name for the node |
| `sql` | string | | SQL query to run as the test |
| `interval` | number | | Auto-run interval in seconds; `0` = manual only (default) |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

**Example:**
```json
{
  "name": "orders_not_empty",
  "sql": "SELECT COUNT(*) AS n FROM orders",
  "interval": 60
}
```

---

## Node manipulation

### `update_query_node`
Edit an existing query node's SQL or display name in place.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `sql` | string | | New SQL (omit to keep current) |
| `name` | string | | New display name (omit to keep current) |

---

### `move_node`
Move a node to a new position on the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `x` | number | ✓ | New X position (canvas units) |
| `y` | number | ✓ | New Y position (canvas units) |

---

### `resize_node`
Resize a node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `width` | number | ✓ | New width in canvas pixels |
| `height` | number | ✓ | New height in canvas pixels |

---

### `set_node_color`
Set the accent color for any node's header.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `color` | string | ✓ | Hex color, e.g. `#18b569` |

---

### `focus_node`
Pan and zoom the canvas viewport to center on a specific node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |

---

## Import tools

### `import_file`
Import a local file into DuckDB and add it to the canvas as a table node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | ✓ | Absolute path to a CSV, Parquet, or JSON file |
| `table_name` | string | ✓ | DuckDB table name to register |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `import_url`
Fetch a remote data file server-side and load it into DuckDB. Bypasses browser CORS restrictions entirely.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | ✓ | Public URL to a CSV, Parquet, or JSON file |
| `table_name` | string | ✓ | DuckDB table name to register |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `import_s3`
Load a file from S3, Cloudflare R2, or MinIO into DuckDB. Requires S3 credentials configured in **Settings → S3 Storage**.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `s3_url` | string | ✓ | `s3://bucket/path/to/file.parquet` |
| `table_name` | string | ✓ | DuckDB table name to register |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

### `import_csv_data`
Import raw CSV text directly as a DuckDB table — no file needed.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `csv_text` | string | ✓ | Raw CSV content (header row required) |
| `table_name` | string | ✓ | DuckDB table name to register |
| `canvas_id` | string | | Target canvas tab (default: active canvas) |

---

## Canvas utilities

### `fit_view`
Zoom and pan the canvas viewport.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `mode` | string | | `fit` = zoom to fit all content (default) · `reset` = set zoom to 100% |

---

### `clear_canvas`
Remove all nodes from a canvas. Does not drop DuckDB tables.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `canvas_id` | string | | Canvas to clear (default: active canvas) |
