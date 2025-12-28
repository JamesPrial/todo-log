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

### Custom Log Location

By default, the plugin saves todos to `.claude/todos.json` in your project root. You can customize this location using the `TODO_LOG_PATH` environment variable.

**Examples:**

```bash
# Use absolute path
export TODO_LOG_PATH=/home/user/logs/my-todos.json

# Use relative path (relative to project root)
export TODO_LOG_PATH=logs/todos.json

# Use custom .claude subdirectory
export TODO_LOG_PATH=.claude/history/todos.json
```

**How it works:**
- If `TODO_LOG_PATH` is set, the plugin uses that location
- Relative paths are resolved against `CLAUDE_PROJECT_DIR`
- Parent directories are created automatically if they don't exist
- If not set, defaults to `.claude/todos.json`

**Setting the environment variable:**

You can set `TODO_LOG_PATH` in your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) or project-specific configuration:

```bash
# In ~/.bashrc or ~/.zshrc
export TODO_LOG_PATH="logs/claude-todos.json"
```

Then restart Claude Code for the changes to take effect.

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

### Permission Errors

Ensure the script is executable:

```bash
chmod +x todo-log/scripts/save-todos.py
```

### Log File Not Created

- Verify `CLAUDE_PROJECT_DIR` environment variable is set
- Check Claude Code has write permissions in your project directory

## Development

### Plugin Structure

```
todo-log/
├── .claude-plugin/
│   └── plugin.json          # Plugin manifest
├── hooks/
│   └── hooks.json           # Hook configuration
├── scripts/
│   └── save-todos.py        # Todo logging handler
├── LICENSE
└── README.md
```

### Testing Locally

1. Create a local marketplace with `marketplace.json`
2. Install the plugin in test mode
3. Use TodoWrite in Claude Code
4. Verify `.claude/todos.json` is created and updated

