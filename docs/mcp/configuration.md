# MCP Configuration

## Claude Desktop

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sql-garden": {
      "url": "http://127.0.0.1:37421/mcp"
    }
  }
}
```

Restart Claude Desktop after saving.

## Claude Code

```bash
claude mcp add --transport http sql-garden http://127.0.0.1:37421/mcp
```

Or open **Settings → MCP** inside sql.garden and click **Auto-configure Claude Code** to run this for you.

## Cursor

Add to `.cursor/mcp.json` in your project root (or the global Cursor MCP config):

```json
{
  "mcpServers": {
    "sql-garden": {
      "url": "http://127.0.0.1:37421/mcp"
    }
  }
}
```

## Windsurf

Open Windsurf → Settings → MCP Servers → Add Server, set:
- **Transport:** HTTP
- **URL:** `http://127.0.0.1:37421/mcp`
- **Name:** `sql-garden`

## Verifying the connection

With sql.garden running, open a terminal and check the tools list:

```bash
curl -s http://127.0.0.1:37421/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | jq '.result.tools[].name'
```

You should see all available tool names printed.
