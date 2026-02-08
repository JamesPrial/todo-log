# Todo-Log Plugin

A Claude Code plugin that automatically captures all TodoWrite tool activity and saves it as timestamped history.

## Overview

This plugin hooks into Claude Code's TodoWrite tool usage and maintains a comprehensive log of all todo updates in `.claude/todos.json` at your project root. Perfect for tracking task progress over time, analyzing workflow patterns, or maintaining an audit trail of your work sessions.

## Features

- **Automatic Capture**: Logs every TodoWrite tool invocation without manual intervention
- **Timestamped History**: Each entry includes ISO 8601 timestamp for precise tracking
- **Session Tracking**: Records session ID and working directory for context
- **Non-Intrusive**: Runs silently in the background via PostToolUse hook
- **Persistent Storage**: Maintains complete history across sessions
- **Dual Backends**: JSON (default) or SQLite for querying large datasets

## Installation

### From Local Marketplace

1. Add your local marketplace:
   ```bash
   /plugin marketplace add ./path/to/marketplace
   ```

2. Install the plugin:
   ```bash
   /plugin install todo-log@marketplace-name
   ```

### Manual Installation

Copy the `todo-log` directory to your Claude Code plugins location and enable it via settings.

## Usage

Once installed, the plugin works automatically. Every time Claude uses the TodoWrite tool, the plugin will:

1. Capture the todo list state
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

When using the SQLite backend, you can query your todos directly:

```bash
# Get all entries from a specific session
sqlite3 .claude/todos.db "SELECT e.timestamp, t.content, t.status FROM log_entries e JOIN todos t ON e.id = t.entry_id WHERE e.session_id = 'session-abc123'"

# Get all pending todos across all sessions
sqlite3 .claude/todos.db "SELECT content, active_form FROM todos WHERE status = 'pending'"
```

## Output Format

The plugin saves data to `.claude/todos.json` in the following format:

```json
[
  {
    "timestamp": "2025-11-14T10:30:45.123Z",
    "session_id": "abc123def456",
    "cwd": "/home/user/my-project",
    "todos": [
      {
        "content": "Implement user authentication",
        "status": "completed",
        "activeForm": "Implementing user authentication"
      },
      {
        "content": "Write unit tests",
        "status": "in_progress",
        "activeForm": "Writing unit tests"
      },
      {
        "content": "Update documentation",
        "status": "pending",
        "activeForm": "Updating documentation"
      }
    ]
  }
]
```

## File Location

- **Default location**: `.claude/todos.json` (in your project root)
- **Custom location**: Set via `TODO_LOG_PATH` environment variable (see Configuration section)
- Parent directories are automatically created if they don't exist

## Viewing Your Logs

You can view your todo history at any time:

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

### Plugin Structure

```
todo-log/
├── .claude-plugin/
│   └── plugin.json          # Plugin manifest
├── hooks/
│   └── hooks.json           # Hook configuration
├── cmd/
│   └── save-todos/
│       └── main.go          # CLI entry point
├── internal/
│   ├── hook/                # Hook input processing
│   │   ├── input.go         # Stdin parsing, todo validation
│   │   └── entry.go         # LogEntry construction
│   ├── pathutil/
│   │   └── safepath.go      # Path traversal protection
│   └── storage/
│       ├── types.go         # Types and interfaces
│       ├── factory.go       # Backend factory
│       ├── json_backend.go  # JSON file backend
│       └── sqlite_backend.go # SQLite database backend
├── bin/
│   └── save-todos           # Compiled binary (gitignored)
├── go.mod
├── Makefile
├── LICENSE
└── README.md
```

### Testing Locally

1. Build: `make build`
2. Install the plugin in test mode
3. Use TodoWrite in Claude Code
4. Verify `.claude/todos.json` is created and updated
