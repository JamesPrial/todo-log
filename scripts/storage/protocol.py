"""Protocols and type definitions for storage backends.

This module defines the interfaces and data structures used by storage backends.
All backends must implement the StorageBackend protocol.
"""

from __future__ import annotations

from typing import Protocol, TypedDict


class TodoItem(TypedDict):
    """Structure for a single todo item.

    Attributes:
        content: The task description.
        status: The current status (e.g., "pending", "in_progress", "completed").
        activeForm: The present continuous form of the action (e.g., "Doing task").
    """

    content: str
    status: str
    activeForm: str


class LogEntry(TypedDict):
    """Structure for a todo log entry.

    Attributes:
        timestamp: ISO 8601 UTC timestamp with Z suffix (e.g., "2025-11-14T10:30:45.123Z").
        session_id: Unique session identifier for the Claude Code session.
        cwd: Current working directory where the TodoWrite was invoked.
        todos: List of todo items in this entry.
    """

    timestamp: str
    session_id: str
    cwd: str
    todos: list[TodoItem]


class StorageBackend(Protocol):
    """Protocol for todo storage backends.

    All storage backends must implement these methods to be compatible
    with the todo-log plugin.
    """

    def load_history(self) -> list[LogEntry]:
        """Load all todo log entries from storage.

        Returns:
            A list of all LogEntry objects in chronological order.
            Returns an empty list if no entries exist.

        Raises:
            OSError: If there's a file system error reading the storage.
            ValueError: If the stored data is corrupted or invalid.
        """
        ...

    def append_entry(self, entry: LogEntry) -> None:
        """Atomically append a new entry to the log.

        Appends the given entry to the end of the log while maintaining
        atomicity to prevent data corruption in case of failure.

        Args:
            entry: The LogEntry to append.

        Raises:
            OSError: If there's a file system error writing the storage.
            ValueError: If the entry is invalid or cannot be serialized.
        """
        ...


class QueryableStorageBackend(StorageBackend, Protocol):
    """Extended protocol with query capabilities for advanced use cases.

    This protocol extends StorageBackend with additional query methods.
    Not all backends may implement this protocol.
    """

    def get_entries_by_session(self, session_id: str) -> list[LogEntry]:
        """Retrieve all entries for a specific session.

        Args:
            session_id: The session identifier to query for.

        Returns:
            A list of LogEntry objects matching the session_id,
            in chronological order. Empty list if no matches.

        Raises:
            OSError: If there's a storage access error.
        """
        ...

    def get_todos_by_status(self, status: str) -> list[TodoItem]:
        """Retrieve all todos with a specific status across all entries.

        Args:
            status: The status value to query for (e.g., "pending", "completed").

        Returns:
            A list of TodoItem objects with the matching status.
            Empty list if no matches.

        Raises:
            OSError: If there's a storage access error.
        """
        ...
