# MCP Tool Reference

All tools are available via the MCP server at `http://127.0.0.1:37421/mcp`.

## Data tools

### `list_tables`
List all tables and views currently loaded in DuckDB.

_No parameters._

---

### `run_query`
Execute a SQL query and return the results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | ✓ | SQL to execute |

---

### `materialize_query`
Run a SQL query and save the results as a persistent DuckDB table, then add it to the canvas as a data node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | ✓ | SQL to execute |
| `table_name` | string | ✓ | Name for the new table |

---

## Canvas inspection

### `list_canvas_nodes`
Return all nodes currently on the canvas with their IDs, types, positions, and sizes.

_No parameters._

---

## Node creation

### `add_query_node`
Add a SQL query node to the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Display label for the node |
| `query` | string | ✓ | SQL to populate the node |

---

### `add_chart_node`
Add a chart node that visualizes query results.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Display label |
| `source_node` | string | ✓ | Name of the query node to visualize |
| `chart_type` | string | ✓ | `bar`, `line`, `scatter`, or `pie` |
| `x_column` | string | ✓ | Column for the X axis (or labels) |
| `y_column` | string | ✓ | Column for the Y axis (or values) |

---

### `add_markdown_node`
Add a freeform markdown text node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Title shown in the node header |
| `content` | string | ✓ | Markdown body |

---

### `add_section`
Add a section (grouped container) to the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | ✓ | Section label |
| `x` | number | | X position (canvas units) |
| `y` | number | | Y position (canvas units) |

---

## Node manipulation

### `update_query_node`
Edit an existing query node's SQL or display name.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `query` | string | | New SQL (leave blank to keep current) |
| `name` | string | | New display name |

---

### `move_node`
Move a node to a new position on the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `x` | number | ✓ | New X position |
| `y` | number | ✓ | New Y position |

---

### `resize_node`
Resize a node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `width` | number | ✓ | New width in pixels |
| `height` | number | ✓ | New height in pixels |

---

### `set_node_color`
Set the accent color for a node's header.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |
| `color` | string | ✓ | Hex color, e.g. `#18b569` |

---

### `focus_node`
Pan and zoom the canvas to center on a specific node.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `node_id` | string | ✓ | ID from `list_canvas_nodes` |

---

## Import tools

### `import_file`
Import a local file into DuckDB and add it to the canvas.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file_path` | string | ✓ | Absolute path to the file |
| `table_name` | string | ✓ | Table name to register |

---

### `import_url`
Fetch a remote data file and load it into DuckDB.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | ✓ | Public URL to a CSV, Parquet, or JSON file |
| `table_name` | string | ✓ | Table name to register |

---

### `import_s3`
Load a file from S3, Cloudflare R2, or MinIO into DuckDB. Requires S3 credentials configured in Settings → S3 Storage.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `s3_url` | string | ✓ | `s3://bucket/path/to/file.parquet` |
| `table_name` | string | ✓ | Table name to register |

---

### `import_csv_data`
Import raw CSV text (e.g. pasted from the clipboard) as a table.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `csv_data` | string | ✓ | Raw CSV content |
| `table_name` | string | ✓ | Table name to register |

---

## Canvas utilities

### `fit_view`
Zoom and pan the canvas to fit all nodes in view.

_No parameters._

---

### `clear_canvas`
Remove all nodes from the canvas. Does not drop DuckDB tables.

_No parameters._
