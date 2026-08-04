# Contributing to sql.garden

Thanks for your interest in contributing. sql.garden is a Wails v2 desktop app (Go + Vue 3 + DuckDB) with a web sandbox and VitePress docs site.

## Ways to contribute

- **Bug reports** — Open an issue with steps to reproduce, OS/version, and any console output.
- **Feature requests** — Open an issue describing the use case and why it fits the local-first, canvas-based model.
- **Code** — Fork, branch, implement, open a PR. See the workflow below.
- **Docs** — The `docs/` VitePress site lives in this repo. Edits to `.md` files are very welcome.
- **Dataset suggestions** — We're building a curated dataset library. Open an issue tagged `datasets` with the source URL and license.

## Development setup

### Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.21+ | `brew install go` |
| Node.js | 20+ | `brew install node` |
| Wails CLI | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| npm | bundled with Node | — |

On macOS, Xcode Command Line Tools are also required (`xcode-select --install`).

### Run the desktop app

```bash
# Install frontend deps
cd frontend && npm install && cd ..

# Start in dev mode (hot-reload for both Go and Vue)
wails dev
```

### Run the web sandbox

```bash
cd frontend
npm install
npm run dev:web        # serves at http://localhost:5174
```

### Run the docs site

```bash
cd docs
npm install
npm run dev            # serves at http://localhost:5173
```

## Branch & PR conventions

- Branch from `main`.
- Branch naming: `feat/<short-description>`, `fix/<short-description>`, `docs/<short-description>`.
- Keep PRs focused — one feature or fix per PR.
- Reference the relevant issue number in the PR description.
- The `wails` branch is used for CI builds of the desktop app binary. Don't target it for feature work.

## Testing

### MCP integration tests (required for MCP changes)

Any change to an MCP tool must ship with a corresponding Red/Green integration test in `cmd/mcp-test/main.go`. Tests must pass against the running app before the PR is merged.

```bash
# Start the app first (wails dev or built binary), then:
go run ./cmd/mcp-test

# Run a specific scenario by name filter:
go run ./cmd/mcp-test import_csv
```

A "Red/Green" test means:
- **Red** — assert the tool returns an error for invalid input (missing fields, wrong types, bad paths).
- **Green** — assert the tool succeeds and emits the correct canvas action for valid input.

Both halves are required. A test that only checks the happy path will not be merged.

### Frontend

```bash
cd frontend
npm run type-check     # TypeScript
npm run lint           # ESLint
```

There are no frontend unit tests yet — this is an area where contributions are welcome.

## Code style

- **Go** — standard `gofmt` formatting. Run `go vet ./...` before committing.
- **Vue/TypeScript** — ESLint config in `frontend/.eslintrc`. No `any` types without a comment explaining why.
- **Comments** — only when the *why* is non-obvious. Don't describe what the code does; well-named identifiers do that.

## Commit messages

Use the imperative mood, present tense: `add import_dataset MCP tool`, not `added` or `adds`.

One-line subject (≤72 chars) is enough for small changes. For larger changes, add a blank line then a short paragraph on *why*.

## Sensitive areas

A few parts of the codebase need extra care:

- **`persistence.go`** — SQLite schema migrations. Any column addition needs a `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` migration guard.
- **`mcp.go`** — MCP tool registration. Adding a tool without a corresponding `cmd/mcp-test` test will be rejected.
- **`app.go`** — All exported methods become Wails bindings and are callable from the frontend. Changing a method signature requires updating `frontend/wailsjs/go/main/App.d.ts` and `App.js` manually (Wails codegen doesn't run in CI).
- **`go-duckdb`** — DuckDB requires `SetMaxOpenConns(1)`. Do not open multiple connections. See known quirks in `TASKS.md`.

## Reporting security issues

Please do **not** open a public issue for security vulnerabilities. See [SECURITY.md](SECURITY.md) for the private disclosure process.
