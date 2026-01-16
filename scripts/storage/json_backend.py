"""JSON file-based storage backend for todo-log plugin.

This module provides a storage backend that persists todo entries to a JSON file.
It uses atomic writes (temp file + os.replace) to ensure data consistency.
"""

from __future__ import annotations

import json
import os
import tempfile
from pathlib import Path

from storage.protocol import LogEntry, StorageBackend


class JSONStorageBackend:
    """JSON file-based storage backend for todo entries.

    Stores all log entries in a single JSON file. Uses atomic writes to prevent
    data corruption in case of unexpected failure during write operations.

    Attributes:
        log_file: The Path to the JSON log file.

    Example:
        backend = JSONStorageBackend(Path("/home/user/project/.claude/todos.json"))
        history = backend.load_history()
        backend.append_entry(new_entry)
    """

    def __init__(self, log_file: Path) -> None:
        """Initialize the JSON storage backend.

        Args:
            log_file: The path to the JSON log file.
        """
        self.log_file = log_file

    def load_history(self) -> list[LogEntry]:
        """Load all todo log entries from the JSON file.

        Returns an empty list if the file doesn't exist. If the file is corrupted
        (invalid JSON), also returns an empty list and the backend can start fresh.

        Returns:
            A list of LogEntry objects, or empty list if no entries or file missing.
        """
        if not self.log_file.exists():
            return []

        try:
            with open(self.log_file, "r", encoding="utf-8") as f:
                history = json.load(f)
                if not isinstance(history, list):
                    return []
                return history
        except (json.JSONDecodeError, OSError):
            # If file is corrupted or unreadable, start fresh
            return []

    def append_entry(self, entry: LogEntry) -> None:
        """Atomically append a new entry to the JSON log file.

        Creates the parent directory if needed. Reads existing entries, appends
        the new entry, and writes to a temporary file before atomically moving
        it to the final location. This ensures data consistency even if the
        process is interrupted.

        Args:
            entry: The LogEntry to append.

        Raises:
            OSError: If there's an error creating directories or writing files.
        """
        # Ensure parent directory exists
        self.log_file.parent.mkdir(parents=True, exist_ok=True)

        # Load existing history
        history = self.load_history()

        # Append new entry
        history.append(entry)

        # Write to temp file first, then atomically rename
        temp_fd, temp_path = tempfile.mkstemp(
            dir=self.log_file.parent, suffix=".tmp"
        )
        try:
            with os.fdopen(temp_fd, "w", encoding="utf-8") as f:
                json.dump(history, f, indent=2)
            os.replace(temp_path, self.log_file)  # Atomic on POSIX
        except (OSError, TypeError, ValueError):
            # Clean up temp file on failure
            try:
                os.unlink(temp_path)
            except OSError:
                pass
            raise
