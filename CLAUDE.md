# sql.garden — Claude Code Instructions

## Working agreements

### Every new node type requires all three:
1. **MCP tool** — `add_<kind>_node` in `ai.go` (`mcpToolDefs` + `execTool` switch case). Fields must be added to `CanvasAction` struct first. Return a stable node ID in the response text.
2. **Integration test** — Red/Green test scenarios in `cmd/mcp-test/main.go`: happy path, missing-required-field errors, and `tools/list` assertion updated to include the new tool name.
3. **Docs** — `docs/mcp/tools.md` updated with parameter table and description before the task is considered done.

Skipping any of the three means the feature is incomplete. Check TASKS.md for the working agreement and test status.

### Parameter naming conventions (MCP ↔ Go ↔ Frontend)
- MCP tool params use `snake_case` (e.g. `source_id`, `canvas_id`, `target_table`)
- `CanvasAction` struct uses Go exported camelCase with `json:"camelCase"` tags (e.g. `SourceID`, `TargetTable`)
- Frontend reads JSON keys directly — they must match the struct tags

### canvas_id is always optional
Every node-creation and import tool must accept an optional `canvas_id` parameter that targets a specific canvas tab. Omitting it defaults to the active canvas. Use the `withTarget` helper in `App.vue`'s `handleCanvasAction`.

### Sensitive areas
- **Persistence migrations** — adding fields to persisted node types requires handling in both `usePersistence.ts` (desktop) and `usePersistence.web.ts` (web). Serialize and deserialize explicitly — no spread operators over unknown shapes.
- **Wails bindings** — `wailsjs/go/` is auto-generated. Don't edit it manually; run `wails generate` (or a full build) to regenerate after changing Go method signatures.
- **DuckDB connection** — single connection, not goroutine-safe. All DuckDB calls from Go must be serialized (the existing `a.db` mutex pattern). Don't add concurrent DuckDB calls.
