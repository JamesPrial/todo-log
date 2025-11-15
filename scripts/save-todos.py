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

        # Ensure .claude directory exists
        claude_dir = Path(project_dir) / ".claude"
        claude_dir.mkdir(exist_ok=True)

        # Path to the todos log file
        todos_file = claude_dir / "todos.json"

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
