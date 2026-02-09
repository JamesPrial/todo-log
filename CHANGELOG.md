# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.1.0] - 2026-02-08

### Added
- **MCP Server**: Standalone `bin/mcp-server` binary providing Claude-native database tools via Model Context Protocol
- **9 MCP Tools**: start_postgres, stop_postgres, postgres_status, execute_query, list_tables, describe_table, insert_rows, update_rows, delete_rows
- **Ephemeral PostgreSQL**: Automatic container lifecycle management using testcontainers-go for isolated database testing
- **SQL Helpers**: Parameterized CRUD operations (INSERT/UPDATE/DELETE) with SQL injection prevention via identifier validation
- **Schema Introspection**: Tools to list tables and describe table schemas for dynamic SQL generation
- **MCP Server Registration**: `.mcp.json` plugin metadata for MCP server configuration

### Changed
- **Build System**: New Makefile targets `build-mcp` and `build-all` for flexible compilation (save-todos, mcp-server, or both)
- **Dependencies**: Added `github.com/mark3labs/mcp-go` v0.43.2 for MCP protocol implementation

## [3.0.0] - 2026-02-08

### Breaking Changes
- **Tool rename**: Hook matcher changed from `TodoWrite` to `TaskCreate|TaskUpdate` to match Claude Code's renamed tools
- **Data model**: `TodoItem` replaced with `TaskItem` — new fields: `id`, `subject`, `description`, `owner`, `blocks`, `blockedBy`, `metadata`; removed: `content`
- **Log entry format**: `todos` array replaced with singular `task` object + `tool_name` field
- **SQLite schema**: Single `log_entries` table with inline task fields (replaces normalized `log_entries` + `todos` tables)
- **Query API**: `GetTodosByStatus()` renamed to `GetTasksByStatus()`

### Added
- Support for `TaskCreate` and `TaskUpdate` PostToolUse events
- `ParseTaskInput()` for mapping TaskCreate/TaskUpdate payloads to `TaskItem`
- Task dependency tracking (`blocks`, `blockedBy` fields)
- Task ownership tracking (`owner` field)
- Arbitrary metadata support (`metadata` field)

### Changed
- Rewrote Go implementation from `TodoItem`/`TodoWrite` model to `TaskItem`/`TaskCreate`+`TaskUpdate` model
- SQLite backend uses denormalized single-table design (no foreign keys, no transactions needed)
- All 175+ tests rewritten for new data model

### Removed
- `TodoItem` struct and all `TodoWrite`-specific validation
- `ValidateTodo()` and `ValidateTodos()` functions
- Normalized SQLite `todos` table and foreign key relationships

## [1.0.1] - 2026-01-16

### Fixed
- Removed duplicate hooks field from plugin manifest

## [1.0.0] - 2025-12-28

### Added
- 60+ unit tests for comprehensive coverage

### Changed
- Atomic file writes using temp file + `os.replace()` for crash safety
- Strict type safety with TypedDict and runtime validation

### Fixed
- Edge case validation in `resolve_safe_path()` for whitespace and null bytes
- Graceful recovery from corrupted JSON (starts fresh with empty array)

### Security
- Path traversal protection via `resolve_safe_path()` function
- Symlink validation prevents escaping target directory
- Null byte handling in path resolution
- Input validation for all external data

## [0.2.0] - 2025-12-27

### Added
- PostToolUse hook that logs TodoWrite activity
- Default log location: `.claude/todos.json`
- Configurable log path via `TODO_LOG_PATH` environment variable
- ISO 8601 timestamps for each log entry
- Session ID and working directory tracking
- Zero external dependencies (Python stdlib only)
