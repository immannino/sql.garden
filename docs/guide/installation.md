# Installation

## macOS

Download the latest `.dmg` from the [releases page](https://github.com/immannino/sql.garden/releases/latest), open it, and drag sql.garden to your Applications folder.

**Requirements:** macOS 13.0 (Ventura) or later, Apple Silicon or Intel.

The app is notarized by Apple. If macOS warns you on first launch, open **System Settings → Privacy & Security** and click **Open Anyway**.

## Windows

Download the latest `.exe` installer from the [releases page](https://github.com/immannino/sql.garden/releases/latest) and run it.

**Requirements:** Windows 10 or later, 64-bit.

## Browser sandbox

No installation needed — try the app at [sql.garden/sandbox](https://sql.garden/sandbox). The sandbox runs DuckDB Wasm in your browser. Some features (MCP server, S3 credentials, local file import) are desktop-only.

## Building from source

```bash
# Prerequisites: Go 1.22+, Node 20+, Wails v2
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/immannino/sql.garden
cd sql.garden

# Run in dev mode
wails dev

# Build a production binary
wails build
```
