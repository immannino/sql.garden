// mcp-bridge proxies Claude Desktop's stdio MCP transport to the sql.garden
// HTTP MCP server running at localhost:37421. Claude Desktop only supports
// stdio (subprocess) transport for local servers, so this shim bridges the gap.
//
// Build:  go build -o sql-garden-mcp-bridge ./cmd/mcp-bridge
// Config: add to ~/Library/Application Support/Claude/claude_desktop_config.json:
//
//	{
//	  "mcpServers": {
//	    "sql-garden": { "command": "/absolute/path/to/sql-garden-mcp-bridge" }
//	  }
//	}
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	serverURL = "http://127.0.0.1:37421/mcp"
	// Scanner buffer large enough for tool results with many rows.
	bufSize = 4 * 1024 * 1024 // 4 MB
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, bufSize), bufSize)

	client := &http.Client{Timeout: 30 * time.Second}

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		// Parse just enough to grab the id — needed to send error responses.
		var envelope struct {
			ID *json.RawMessage `json:"id"`
		}
		json.Unmarshal(line, &envelope) //nolint:errcheck

		resp, err := client.Post(serverURL, "application/json", bytes.NewReader(line))
		if err != nil {
			// sql.garden isn't running — only reply if the message has an id
			// (notifications don't expect a response).
			if envelope.ID != nil {
				writeError(os.Stdout, envelope.ID, -32603,
					"sql.garden is not running. Open the app first.")
			}
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 202 means the server treated it as a notification — no response body.
		if resp.StatusCode == http.StatusAccepted || len(bytes.TrimSpace(body)) == 0 {
			continue
		}

		fmt.Fprintf(os.Stdout, "%s\n", bytes.TrimSpace(body))
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-bridge: stdin error: %v\n", err)
		os.Exit(1)
	}
}

func writeError(w io.Writer, id *json.RawMessage, code int, msg string) {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(w, "%s\n", b)
}
