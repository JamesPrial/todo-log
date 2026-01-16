# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is the **todo-log** plugin (v1.1.0) for Claude Code. It automatically logs TodoWrite tool usage using a PostToolUse hook. Supports both JSON file and SQLite database backends.

## Development Commands

```bash
# Run all tests (main + storage package)
cd scripts && PYTHONPATH=. python -m pytest test_save_todos.py storage/tests/ -v

# Run only main tests
python -m pytest scripts/test_save_todos.py -v

# Run only storage backend tests
cd scripts && PYTHONPATH=. python -m pytest storage/tests/ -v

# Run a single test
python -m pytest scripts/test_save_todos.py::TestValidateTodo::test_valid_todo_with_all_keys -v
```

## Testing Plugin Changes

1. Make changes to `scripts/save_todos.py`
2. Uninstall: `/plugin uninstall todo-log`
3. Reinstall: `/plugin install todo-log@<marketplace-name>`
4. Trigger TodoWrite in Claude Code
5. Check `.claude/todos.json` or use `claude --debug` for hook output

## Architecture

### Hook Flow

```
TodoWrite tool invoked
    ↓
PostToolUse event fires
    ↓
hooks.json matches "TodoWrite" → runs save_todos.py
    ↓
save_todos.py: reads stdin JSON → validates todos → appends to log file
```

### Key Files

- `hooks/hooks.json` - Hook configuration (PostToolUse on TodoWrite)
- `scripts/save_todos.py` - Main hook script (Python 3.10+, stdlib only)
- `scripts/storage/` - Storage backend package
  - `protocol.py` - TypedDicts and Protocol definitions
  - `json_backend.py` - JSON file storage backend
  - `sqlite_backend.py` - SQLite database backend with query support
  - `__init__.py` - Backend factory function
- `scripts/test_save_todos.py` - Main test suite (80+ tests)
- `scripts/storage/tests/` - Storage backend tests (100+ tests)

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
- Query methods: `get_entries_by_session()`, `get_todos_by_status()`
- WAL mode for concurrent access
- Better for querying and large datasets

### Exit Codes

- `0`: Success (or non-TodoWrite event, ignored)
- `1`: Error (missing env var, file I/O failure, path escape attempt)

### Security

- Path traversal protection: `resolve_safe_path()` prevents escaping project directory
- Symlink resolution: Symlinks pointing outside project are rejected
- Null byte handling: Paths with null bytes are rejected
- Atomic writes: Uses temp file + `os.replace()` for crash safety

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

### Zero Dependencies
Uses only Python stdlib - no pip install required.

### Fail-Safe Design
PostToolUse hooks never block tool execution. Errors exit with code 1 but don't interrupt Claude.

### Graceful Recovery
Corrupted JSON files are handled by starting fresh (empty array).
