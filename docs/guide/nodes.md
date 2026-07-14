# Nodes

Everything on the canvas is a node. Nodes can be moved, resized, colored, and connected.

## Query node

The core building block. Contains a SQL editor and displays results in a table below the query.

- **Run:** `⌘ Enter` / `Ctrl Enter`
- Results are stored in DuckDB as a view named after the node
- Results can be referenced by other query nodes: `SELECT * FROM "my_node_name"`

## Data node

Created automatically when you import a file or connect to a database table. Shows the schema (column names + types) and row count. You can't edit its SQL directly — it represents a raw source.

## Chart node

Visualizes query results. Pick a chart type, then map your columns to the available axes.

### Plot charts

| Type | Value | Best for |
|------|-------|----------|
| Bar Y | `barY` | Compare values across categories or time — vertical bars |
| Bar X | `barX` | Horizontal bars, great for long category names |
| Line | `lineY` | Trends over continuous X (time, index); add a Color column for multiple series |
| Area | `areaY` | Like Line but filled — good for cumulative or volume data |
| Scatter | `dot` | Correlations between two numeric columns; Color groups points |
| Cell | `cell` | Color-encoded grid across two categorical axes (heatmap) |
| Pie | `pie` | Part-to-whole for a small number of categories (best under 8 slices) |
| Donut | `donut` | Like Pie with a center hole showing the total |
| Histogram | `histogram` | Distribution of a single numeric column — bins are automatic |
| Box Plot | `boxplot` | Median, IQR, and outliers; compare distributions across groups |

### Flow charts

| Type | Value | Best for |
|------|-------|----------|
| Sankey | `sankey` | Flow volume between two sets of categories — funnels, networks. Each row is one source → target link with its weight |

### Display / stat charts

| Type | Value | Best for |
|------|-------|----------|
| Number | `number` | Display a single key metric as a large number (first row only) |
| Badge | `boolean` | Green/red status badge driven by a boolean or truthy value |
| Status | `conditional` | Color-coded badge driven by custom match rules evaluated top-to-bottom |

### Diagram

| Type | Value | Best for |
|------|-------|----------|
| Mermaid | `mermaid` | Flowcharts, sequence diagrams, ER diagrams — write Mermaid.js code directly |
| Table | `table` | Formatted data grid with per-column rename, formatting, and alignment |

## Markdown node

A freeform text/notes node. Supports standard Markdown including headings, lists, code blocks, and links. Useful for annotating your canvas or documenting findings.

## Section

A resizable container that groups related nodes together. Sections sit behind nodes on the canvas — drag nodes into a section to associate them visually.

## Node actions

Right-click any node for context menu options:

- **Rename** — change the display label
- **Set color** — pick an accent color for the node header
- **Duplicate** — copy the node in place
- **Export** — download results as CSV or JSON
- **Delete** — remove the node (does not drop the underlying DuckDB table)
