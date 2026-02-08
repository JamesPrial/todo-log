# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is the **todo-log** plugin (v2.0.0) for Claude Code. Rewritten in Go for compiled binary distribution. It automatically logs TodoWrite tool usage using a PostToolUse hook. Supports both JSON file and SQLite database backends.

## Development Commands

```bash
# Build the binary
make build

# Run all tests
make test

# Run tests with coverage
make cover

# Run specific package tests
go test -v ./internal/hook/...
go test -v ./internal/storage/...

# Clean build artifacts
make clean
```

## Testing Plugin Changes

1. Make changes to Go source files
2. Rebuild: `make build`
3. Uninstall: `/plugin uninstall todo-log`
4. Reinstall: `/plugin install todo-log@<marketplace-name>`
5. Trigger TodoWrite in Claude Code
6. Check `.claude/todos.json` or use `claude --debug` for hook output

## Architecture

### Hook Flow

```
TodoWrite tool invoked
    ↓
PostToolUse event fires
    ↓
hooks.json matches "TodoWrite" → runs bin/save-todos
    ↓
save-todos: reads stdin JSON → validates todos → appends to storage backend
```

### Key Files

- `hooks/hooks.json` - Hook configuration (PostToolUse on TodoWrite)
- `cmd/save-todos/main.go` - CLI entry point (`run(stdin io.Reader) int`)
- `internal/hook/` - Hook input processing
  - `input.go` - Stdin JSON parsing, todo validation
  - `entry.go` - LogEntry construction, timestamps
- `internal/pathutil/` - Security utilities
  - `safepath.go` - Path traversal protection
- `internal/storage/` - Storage backend package
  - `types.go` - TodoItem, LogEntry structs, StorageBackend interface
  - `factory.go` - Backend factory (`GetStorageBackend`)
  - `json_backend.go` - JSON file storage backend
  - `sqlite_backend.go` - SQLite database backend with query support
- `Makefile` - Build and test commands
- `go.mod` - Go module (`modernc.org/sqlite` for pure-Go SQLite)

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
- Normalized tables: `log_entries` and `todos`
- Query methods: `GetEntriesBySession()`, `GetTodosByStatus()`
- WAL mode for concurrent access
- Better for querying and large datasets

### Exit Codes

- `0`: Success (or non-TodoWrite event, ignored)
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
  "todos": [
    {"content": "Task", "status": "pending", "activeForm": "Doing task"}
  ]
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
