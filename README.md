# Todo-Log Plugin

A Claude Code plugin that automatically captures all TaskCreate/TaskUpdate tool activity and saves it as timestamped history.

## Overview

This plugin hooks into Claude Code's TaskCreate and TaskUpdate tool usage and maintains a comprehensive log of all task operations in `.claude/todos.json` at your project root. Perfect for tracking task progress over time, analyzing workflow patterns, or maintaining an audit trail of your work sessions.

## Features

- **Automatic Capture**: Logs every TaskCreate and TaskUpdate invocation without manual intervention
- **Timestamped History**: Each entry includes ISO 8601 timestamp for precise tracking
- **Session Tracking**: Records session ID and working directory for context
- **Non-Intrusive**: Runs silently in the background via PostToolUse hook
- **Persistent Storage**: Maintains complete history across sessions
- **Dual Backends**: JSON (default) or SQLite for querying large datasets
- **Rich Task Model**: Tracks subject, description, status, owner, dependencies, and metadata

## Installation

### From Marketplace

1. Add the marketplace:
   ```bash
   /plugin marketplace add JamesPrial/prial-plugins
   ```

2. Install the plugin:
   ```bash
   /plugin install todo-log@prial-plugins
   ```

Pre-built binaries for macOS (Intel/Apple Silicon), Linux (x86_64/ARM64), and Windows (x86_64) are included automatically.

### Manual Installation

Copy the `todo-log` directory to your Claude Code plugins location and enable it via settings.

## Usage

Once installed, the plugin works automatically. Every time Claude uses the TaskCreate or TaskUpdate tool, the plugin will:

1. Capture the task data (subject, description, status, dependencies, etc.)
2. Add timestamp and session metadata
3. Append to `.claude/todos.json` in your project root (or custom location if configured)

## Configuration

### Storage Backend

The plugin supports two storage backends:

| Backend | Default Path | Best For |
|---------|--------------|----------|
| JSON (default) | `.claude/todos.json` | Simple logging, human-readable |
| SQLite | `.claude/todos.db` | Large datasets, querying by session/status |

**Switch to SQLite:**
```bash
export TODO_STORAGE_BACKEND=sqlite
```

### Custom Paths

**JSON Backend:**
```bash
# Use custom JSON log location
export TODO_LOG_PATH=logs/todos.json
```

**SQLite Backend:**
```bash
export TODO_STORAGE_BACKEND=sqlite
export TODO_SQLITE_PATH=data/todos.db
```

**Path resolution:**
- Relative paths are resolved against `CLAUDE_PROJECT_DIR`
- Absolute paths work too (but must stay within project for security)
- Parent directories are created automatically

### SQLite Query Features

When using the SQLite backend, you can query your tasks directly:

```bash
# Get all entries from a specific session
sqlite3 .claude/todos.db "SELECT timestamp, subject, status FROM log_entries WHERE session_id = 'session-abc123'"

# Get all pending tasks across all sessions
sqlite3 .claude/todos.db "SELECT subject, active_form FROM log_entries WHERE status = 'pending'"

# Get tasks with their dependencies
sqlite3 .claude/todos.db "SELECT subject, status, blocks, blocked_by FROM log_entries WHERE blocks != '[]' OR blocked_by != '[]'"
```

## Output Format

The plugin saves data to `.claude/todos.json` in the following format:

```json
[
  {
    "timestamp": "2025-11-14T10:30:45.123Z",
    "session_id": "abc123def456",
    "cwd": "/home/user/my-project",
    "tool_name": "TaskCreate",
    "task": {
      "subject": "Implement user authentication",
      "description": "Add JWT-based auth to the API endpoints",
      "status": "pending",
      "activeForm": "Implementing user authentication"
    }
  },
  {
    "timestamp": "2025-11-14T10:35:12.456Z",
    "session_id": "abc123def456",
    "cwd": "/home/user/my-project",
    "tool_name": "TaskUpdate",
    "task": {
      "id": "1",
      "status": "in_progress"
    }
  }
]
```

## File Location

- **Default location**: `.claude/todos.json` (in your project root)
- **Custom location**: Set via `TODO_LOG_PATH` environment variable (see Configuration section)
- Parent directories are automatically created if they don't exist

## Viewing Your Logs

You can view your task history at any time:

```bash
cat .claude/todos.json
```

Or use `jq` for formatted output:

```bash
jq '.' .claude/todos.json
```

## Troubleshooting

### Hook Not Executing

1. Verify plugin is installed: `/plugin list`
2. Check hook registration: `/hooks`
3. Run Claude with debug flag: `claude --debug`

### Log File Not Created

- Verify `CLAUDE_PROJECT_DIR` environment variable is set
- Check Claude Code has write permissions in your project directory

## Development

### Building from Source

Requires Go 1.22+:

```bash
make build    # Build bin/save-todos
make test     # Run tests with race detection
make cover    # Run tests with coverage
make clean    # Remove binary
```

### Releasing

Releases are automated via GitHub Actions. To create a new release:

1. Tag and push: `git tag v3.0.0 && git push origin v3.0.0`
2. CI builds cross-platform binaries and pushes them to the `releases` branch
3. Update the SHA in `prial-plugins` marketplace.json with the value printed by the workflow

The `releases` branch is an orphan branch containing only the plugin metadata and pre-built binaries for all platforms. A wrapper script at `bin/save-todos` detects the platform and dispatches to the correct binary. The marketplace pins to this branch via `ref` + `sha` for integrity.

### Supported Platforms

| OS | Architecture | Binary |
|----|-------------|--------|
| macOS | x86_64 | `save-todos-darwin-amd64` |
| macOS | Apple Silicon | `save-todos-darwin-arm64` |
| Linux | x86_64 | `save-todos-linux-amd64` |
| Linux | ARM64 | `save-todos-linux-arm64` |
| Windows | x86_64 | `save-todos-windows-amd64.exe` |

### Testing Locally

1. Build: `make build`
2. Install the plugin in test mode
3. Use TaskCreate/TaskUpdate in Claude Code
4. Verify `.claude/todos.json` is created and updated
