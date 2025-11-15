#!/usr/bin/env python3
"""
TodoWrite Logger - Captures todo updates and saves to timestamped history
"""
import json
import os
import sys
from datetime import datetime
from pathlib import Path


def main():
    try:
        # Read hook input from stdin
        hook_input = json.load(sys.stdin)

        # Verify this is a TodoWrite event
        if hook_input.get("tool_name") != "TodoWrite":
            sys.exit(0)

        # Extract relevant information
        tool_input = hook_input.get("tool_input", {})
        todos = tool_input.get("todos", [])

        # Get project directory from environment
        project_dir = os.environ.get("CLAUDE_PROJECT_DIR")
        if not project_dir:
            print("Warning: CLAUDE_PROJECT_DIR not set", file=sys.stderr)
            sys.exit(1)

        # Prepare the log entry
        log_entry = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "session_id": hook_input.get("session_id", "unknown"),
            "cwd": hook_input.get("cwd", "unknown"),
            "todos": todos
        }

        # Determine log file path from environment or use default
        custom_log_path = os.environ.get("TODO_LOG_PATH", "").strip()

        if custom_log_path:
            # Use custom path (support both absolute and relative paths)
            todos_file = Path(custom_log_path)
            if not todos_file.is_absolute():
                # Resolve relative path against project directory
                todos_file = Path(project_dir) / todos_file
        else:
            # Use default location: .claude/todos.json
            todos_file = Path(project_dir) / ".claude" / "todos.json"

        # Ensure parent directory exists
        todos_file.parent.mkdir(parents=True, exist_ok=True)

        # Load existing history or create new
        history = []
        if todos_file.exists():
            try:
                with open(todos_file, "r") as f:
                    history = json.load(f)
                    if not isinstance(history, list):
                        history = []
            except json.JSONDecodeError:
                # If file is corrupted, start fresh
                history = []

        # Append new entry
        history.append(log_entry)

        # Save updated history
        with open(todos_file, "w") as f:
            json.dump(history, f, indent=2)

        # Output success message (shown in transcript mode with Ctrl-R)
        print(f"Saved {len(todos)} todos to {todos_file}")

        sys.exit(0)

    except Exception as e:
        print(f"Error saving todos: {str(e)}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
