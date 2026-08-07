# Educational / Exercise Packs

sql.garden's **AssertionNode** lets you build interactive SQL exercises directly on the canvas. Students write SQL inside the card, press **Run**, and see immediate pass/fail feedback per check. You can chain exercises into a guided lesson, add success text that reveals on completion, and export everything as a single `.sql.garden.json` file to share with others.

## AssertionNode basics

An AssertionNode is a self-contained exercise card. It contains:

- **Prompt** — Markdown instructions shown to the student (e.g. *"Write a query that returns all rows from `products`"*)
- **SQL editor** — pre-filled starter SQL that the student edits (e.g. `SELECT ??? FROM products`)
- **Checks** — one or more validators that run against the query's output
- **Success text** — optional Markdown revealed only when all checks pass

Create one via right-click → **Add Assertion Node**, or via the MCP tool `add_assertion_node`.

## Edit mode vs. student mode

Toggle Edit mode with the pencil icon in the node header.

| Mode | What you see |
|------|-------------|
| **Edit** | Prompt editor, success text editor, Next lesson selector, check configuration |
| **Student** | Prompt (read-only), SQL editor, Run button, check results, success text on pass |

## Configuring checks

In Edit mode, open the **Checks** section. Each check has a kind and optional parameters:

| Kind | Validates |
|------|-----------|
| `row_count` | Exact number of rows returned |
| `non_empty` | At least one row returned |
| `no_nulls` | No NULL values in specified column |
| `column_exists` | A column with the given name is present |
| `column_value` | A specific cell value matches |
| `set_match` | Result set matches an expected set (order-independent) |
| `sql_pattern` | Student's SQL contains a required keyword or pattern |

You can add multiple checks per node — all must pass for the exercise to be marked complete.

## Chaining exercises

Connect exercises into a sequential lesson using **Next lesson**. When a student passes an exercise, a **Next →** button appears in the footer. Clicking it scrolls the canvas to the next exercise.

To link exercises:
1. Open Edit mode on the exercise you want to chain _from_
2. In the **Next lesson** dropdown, select the exercise that comes next
3. Repeat for each exercise in the sequence

The Exercises Panel (left rail) shows the full chain in order and tracks progress across the whole sequence.

## Success text

Success text is Markdown displayed in the right panel of an exercise card only when all checks pass. Use it for:

- Explaining the correct answer or concept
- Offering hints about what the student just learned
- Pointing to further reading

In Edit mode, paste your Markdown into the **Success text** textarea. It supports bold, code fences, lists, and links.

## Exercises Panel

Open the Exercises Panel with **⌘⇧E** (macOS) or **View → Toggle Exercises Panel**. It shows:

- All assertion nodes in the current canvas, in chain order
- Pass / Fail / Pending status for each exercise
- Attempt count and pass rate per exercise
- A progress bar across the full set
- A 🎉 celebration when every exercise passes

Click any exercise row to focus that node on the canvas.

## Exporting an exercise pack

Right-click any canvas tab → **Export tab**. This downloads a `.sql.garden.json` file containing every node on that tab — QueryNodes, MarkdownNodes, AssertionNodes, and their configuration — ready to share.

The export includes all fields: prompt, starter SQL, checks, success text, and next-lesson links.

## Importing a pack

Drag-and-drop a `.sql.garden.json` file onto the canvas, or use **File → Import**. The pack is added as a new set of nodes on the active tab.

This works on both the **desktop app** and the **web sandbox** at [sql.garden/sandbox](https://sql.garden/sandbox). To share an exercise pack publicly, host the `.sql.garden.json` file anywhere (GitHub Gist, S3, personal site) and tell your students to import it.

## Creating a pack with MCP

The `add_assertion_node` tool supports all fields needed to author a full lesson programmatically:

```json
{
  "name": "ex1_select_all",
  "sql": "SELECT ??? FROM products",
  "prompt": "Write a SELECT query that returns **all rows** from the `products` table.",
  "success_text": "Great work! `SELECT *` fetches every column. You can also name them explicitly.",
  "next_id": "mcp_ex2_where_clause",
  "canvas_id": "canvas_lesson_1"
}
```

A typical workflow:
1. Use `create_canvas` to make a dedicated canvas tab for the lesson
2. Add a `add_markdown_node` intro card with context and attribution
3. Call `add_assertion_node` for each exercise, setting `next_id` to chain them
4. Call `fit_view` so everything is visible
5. Right-click the tab → **Export tab** to download the finished pack

## Attribution and licensing

If your exercise pack uses content or datasets from third parties, add a MarkdownNode to the canvas that credits the original source. For example, sql.garden's built-in SQLBolt sample includes:

> Exercises adapted from [SQLBolt](https://sqlbolt.com) by Nick Ciubotariu, used for educational purposes. For more interactive SQL lessons, visit sqlbolt.com.

Always check the original license before redistributing adapted content.
