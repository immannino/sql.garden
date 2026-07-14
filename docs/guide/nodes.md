# Nodes

Everything on the canvas is a node. Nodes can be moved, resized, colored, and connected.

## Query node

The core building block. Contains a SQL editor and displays results in a table below the query.

- **Run:** `⌘ Enter` / `Ctrl Enter`
- **Results** are stored in DuckDB as a view named after the node
- Results can be referenced by other query nodes: `SELECT * FROM "my_node_name"`

## Data node

Created automatically when you import a file or connect to a database table. Shows the schema (column names + types) and row count. You can't edit its SQL directly — it represents a raw source.

## Chart node

Visualizes query results. Supported chart types:

| Type | Best for |
|------|----------|
| Bar | Comparisons across categories |
| Line | Trends over time |
| Scatter | Correlations between two measures |
| Pie | Part-to-whole relationships |

Charts update live when the upstream query changes.

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
