# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is the **todo-log** plugin for Claude Code. Written in Go for compiled binary distribution. It consists of two components:
1. **save-todos hook** — PostToolUse hook that logs TaskCreate/TaskUpdate tool usage to JSON or SQLite
2. **mcp-server** — MCP server providing 9 tools for PostgreSQL database operations with ephemeral container management

## Development Commands

```bash
# Build the save-todos hook binary
make build

# Build the MCP server binary
make build-mcp

# Build both binaries
make build-all

# Run all tests (including MCP server tests)
make test

# Run tests with coverage
make cover

# Run specific package tests
go test -v ./internal/hook/...
go test -v ./internal/storage/...
go test -v ./internal/mcpserver/...

# Clean build artifacts
make clean
```

## Testing Plugin Changes

1. Make changes to Go source files
2. Rebuild: `make build`
3. Uninstall: `/plugin uninstall todo-log`
4. Reinstall: `/plugin install todo-log@<marketplace-name>`
5. Trigger TaskCreate/TaskUpdate in Claude Code
6. Check `.claude/todos.json` or use `claude --debug` for hook output

## Architecture

### Hook Flow

```
TaskCreate/TaskUpdate tool invoked
    ↓
PostToolUse event fires
    ↓
hooks.json matches "TaskCreate|TaskUpdate" → runs bin/save-todos
    ↓
save-todos: reads stdin JSON → parses task input → appends to storage backend
```

### Key Files

**Hook System (save-todos)**
- `hooks/hooks.json` - Hook configuration (PostToolUse on TaskCreate|TaskUpdate)
- `cmd/save-todos/main.go` - CLI entry point (`run(stdin io.Reader) int`)
- `internal/hook/` - Hook input processing
  - `input.go` - Stdin JSON parsing, TaskCreate/TaskUpdate input mapping
  - `entry.go` - LogEntry construction, timestamps
- `internal/pathutil/` - Security utilities
  - `safepath.go` - Path traversal protection
- `internal/storage/` - Storage backend package
  - `types.go` - TaskItem, LogEntry structs, StorageBackend interface
  - `factory.go` - Backend factory (`GetStorageBackend`)
  - `json_backend.go` - JSON file storage backend
  - `sqlite_backend.go` - SQLite database backend with query support

**MCP Server (mcp-server)**
- `.mcp.json` - MCP server registration and configuration
- `cmd/mcp-server/main.go` - MCP server entry point
- `internal/mcpserver/` - MCP server package
  - `server.go` - Server creation and initialization
  - `tools.go` - 9 MCP tool definitions
  - `container.go` - PostgreSQL container lifecycle (start/stop/status) via testcontainers-go
  - `query.go` - SQL execution and schema introspection helpers
  - `crud.go` - Parameterized INSERT/UPDATE/DELETE with SQL injection prevention

**Build & Configuration**
- `Makefile` - Build targets: `build`, `build-mcp`, `build-all`
- `go.mod` - Go module (dependencies: `modernc.org/sqlite`, `github.com/mark3labs/mcp-go`)

### Release & Distribution

Binary distribution uses a **releases branch strategy**:

- `main` branch: source code only (`bin/` is gitignored)
- `releases` branch: orphan branch with pre-built binaries for all platforms + a wrapper script
- Marketplace (`prial-plugins`) pins to `releases` branch via `ref` + `sha`

**Release flow:**
1. Tag on main: `git tag vX.Y.Z && git push origin vX.Y.Z`
2. GitHub Actions (`.github/workflows/release.yml`) cross-compiles 5 platform binaries
3. Composes a release tree: wrapper script (`bin/save-todos`) + platform binaries + plugin metadata
4. Force-pushes to orphan `releases` branch
5. Prints the new SHA for marketplace.json update

**Wrapper script** (`bin/save-todos` on releases branch): detects `uname -s`/`uname -m`, dispatches to the correct `save-todos-{os}-{arch}` binary via `exec`.

**Supported platforms:** darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `CLAUDE_PROJECT_DIR` | Yes | Project root, used for resolving paths |
| `TODO_STORAGE_BACKEND` | No | `json` (default) or `sqlite` |
| `TODO_LOG_PATH` | No | Custom JSON log path (default: `.claude/todos.json`) |
| `TODO_SQLITE_PATH` | No | Custom SQLite path (default: `.claude/todos.db`) |
| `DEBUG` | No | Enable debug logging to stderr |

### Storage Backends

**JSON Backend** (default):
- Stores entries in a JSON array file
- Atomic writes using temp file + rename
- Good for simple use cases

**SQLite Backend**:
- Denormalized single `log_entries` table with inline task fields
- Query methods: `GetEntriesBySession()`, `GetTasksByStatus()`
- WAL mode for concurrent access
- Better for querying and large datasets

### Exit Codes

- `0`: Success (or non-TaskCreate/TaskUpdate event, ignored)
- `1`: Error (missing env var, file I/O failure, path escape attempt)

### Security

- Path traversal protection: `ResolveSafePath()` prevents escaping project directory
- Symlink resolution: Symlinks pointing outside project are rejected
- Null byte handling: Paths with null bytes are rejected
- Atomic writes: Uses temp file + `os.Rename()` for crash safety

### Log Entry Format

```json
{
  "timestamp": "2025-11-14T10:30:45.123Z",
  "session_id": "abc123def456",
  "cwd": "/home/user/project",
  "tool_name": "TaskCreate",
  "task": {
    "subject": "Implement feature",
    "description": "Details here",
    "status": "pending",
    "activeForm": "Implementing feature"
  }
}
```

## Key Patterns

### Minimal Dependencies
Only external dependency is `modernc.org/sqlite` (pure Go, no CGO) for the SQLite backend.

### Fail-Safe Design
PostToolUse hooks never block tool execution. Errors exit with code 1 but don't interrupt Claude.

### Graceful Recovery
Corrupted JSON files are handled by starting fresh (empty array).

### Idiomatic Go
- Proper error handling with context wrapping
- Interface-based storage abstraction
- Table-driven tests for comprehensive coverage
- Nil-safe operations throughout
