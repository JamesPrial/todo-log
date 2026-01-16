#!/usr/bin/env python3
"""
TodoWrite Logger - PostToolUse hook for capturing TodoWrite events.

This hook script receives JSON via stdin when the TodoWrite tool is used,
extracts the todo list, and appends a timestamped entry to a configured storage backend.

Environment Variables:
    CLAUDE_PROJECT_DIR (required): Project root directory for file operations.
    TODO_STORAGE_BACKEND (optional): Storage backend to use ("json" or "sqlite").
                                     Default: "json"
    TODO_LOG_PATH (optional): Custom log file path for JSON backend
                              (relative to project or absolute).
                              Default: .claude/todos.json
    TODO_SQLITE_PATH (optional): Custom SQLite database path (when using sqlite backend).
                                Default: .claude/todos.db
    DEBUG (optional): If set, enables debug logging to stderr.

Exit Codes:
    0: Success (or non-TodoWrite event, which is ignored)
    1: Error (missing CLAUDE_PROJECT_DIR, file I/O failure, etc.)

Input Format (stdin):
    {
        "tool_name": "TodoWrite",
        "tool_input": {"todos": [...]},
        "session_id": "abc123",
        "cwd": "/path/to/working/dir"
    }

Output Format (log file):
    Array of LogEntry objects, each containing timestamp, session_id, cwd, and todos.
"""
from __future__ import annotations

import json
import os
import sys
import traceback
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, TypedDict

from storage import get_storage_backend
from storage.protocol import LogEntry, TodoItem

# Version check
if sys.version_info < (3, 10):
    print("Error: Python 3.10+ required", file=sys.stderr)
    sys.exit(1)

# Constants
UNKNOWN_VALUE: str = "unknown"


class ToolInput(TypedDict):
    """Structure for tool_input field in hook input."""

    todos: list[TodoItem]


class HookInput(TypedDict, total=False):
    """Structure for the complete hook input from stdin."""

    tool_name: str
    tool_input: ToolInput
    session_id: str
    cwd: str


def utc_iso_timestamp() -> str:
    """Return current UTC time as ISO 8601 string with Z suffix."""
    dt = datetime.now(timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def validate_todo(item: Any) -> bool:
    """Validate a single todo item has required structure."""
    if not isinstance(item, dict):
        return False
    required_keys = {"content", "status", "activeForm"}
    return required_keys.issubset(item.keys())


def validate_todos(todos: Any) -> list[TodoItem]:
    """Validate and filter todos list, returning only valid items."""
    if not isinstance(todos, list):
        return []
    return [item for item in todos if validate_todo(item)]


def resolve_safe_path(base_dir: Path, user_path: str) -> Path | None:
    """Resolve a path, ensuring it stays within base_dir.
    
    Returns None if:
    - user_path is empty or whitespace-only
    - user_path contains null bytes
    - resolved path escapes base_dir
    """
    if not user_path or not user_path.strip():
        return None

    if '\x00' in user_path:
        return None

    candidate = Path(user_path)
    if not candidate.is_absolute():
        candidate = base_dir / candidate

    # Resolve to absolute, following symlinks
    resolved = candidate.resolve()
    base_resolved = base_dir.resolve()

    # Ensure path is within project directory
    try:
        resolved.relative_to(base_resolved)
        return resolved
    except ValueError:
        return None  # Path escapes project directory


def read_hook_input() -> HookInput | None:
    """Read and validate hook input from stdin.

    Returns None if this is not a TodoWrite event (hook should exit silently).
    """
    hook_input: HookInput = json.load(sys.stdin)

    tool_name = hook_input.get("tool_name")
    if tool_name != "TodoWrite":
        # This hook should only be registered for TodoWrite
        # If we receive other events, the hook configuration may be wrong
        if os.environ.get("DEBUG"):
            print(f"Ignoring non-TodoWrite event: {tool_name}", file=sys.stderr)
        return None

    return hook_input


def build_log_entry(hook_input: HookInput) -> LogEntry:
    """Construct a log entry from hook input."""
    tool_input = hook_input.get("tool_input", {})
    raw_todos = tool_input.get("todos", [])
    todos = validate_todos(raw_todos)

    return {
        "timestamp": utc_iso_timestamp(),
        "session_id": hook_input.get("session_id") or UNKNOWN_VALUE,
        "cwd": hook_input.get("cwd") or UNKNOWN_VALUE,
        "todos": todos,
    }


def main() -> None:
    """Main entry point for the TodoWrite logger hook."""
    try:
        # Read and validate hook input
        hook_input = read_hook_input()
        if hook_input is None:
            sys.exit(0)

        # Get project directory from environment
        project_dir_str = os.environ.get("CLAUDE_PROJECT_DIR")
        if not project_dir_str:
            print("Warning: CLAUDE_PROJECT_DIR not set", file=sys.stderr)
            sys.exit(1)

        project_dir = Path(project_dir_str)

        # Build the log entry
        entry = build_log_entry(hook_input)

        # Get storage backend (reads TODO_STORAGE_BACKEND env var)
        backend = get_storage_backend(project_dir)

        # Append entry using backend
        backend.append_entry(entry)

        # Output success message (shown in transcript mode with Ctrl-R)
        backend_type = os.environ.get("TODO_STORAGE_BACKEND", "json").strip().lower()
        print(f"Saved {len(entry['todos'])} todos ({backend_type} backend)")

        sys.exit(0)

    except (json.JSONDecodeError, OSError, KeyError, TypeError, ValueError) as e:
        print(f"Error saving todos: {e!r}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        # Unexpected errors - preserve stack trace for debugging
        print(f"Unexpected error saving todos: {e!r}", file=sys.stderr)
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
