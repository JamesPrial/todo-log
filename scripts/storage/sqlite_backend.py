"""SQLite database storage backend for todo-log plugin.

This module provides a storage backend that persists todo entries to a SQLite
database. It uses transactions and WAL mode for reliability and concurrent access.
"""

from __future__ import annotations

import sqlite3
from pathlib import Path

from storage.protocol import LogEntry, QueryableStorageBackend, TodoItem

# SQL schema for the SQLite database
SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    session_id TEXT NOT NULL,
    cwd TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    active_form TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES log_entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_entries_session ON log_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
"""


class SQLiteStorageBackend:
    """SQLite database storage backend for todo entries.

    Stores log entries and todos in normalized tables with proper relationships
    and indexes. Uses transactions for atomicity and WAL mode for better
    concurrent access patterns.

    Attributes:
        db_path: The Path to the SQLite database file.

    Example:
        backend = SQLiteStorageBackend(Path("/home/user/project/.claude/todos.db"))
        history = backend.load_history()
        backend.append_entry(new_entry)
        entries = backend.get_entries_by_session("session123")
    """

    def __init__(self, db_path: Path) -> None:
        """Initialize the SQLite storage backend.

        Creates the database file and schema if they don't exist.

        Args:
            db_path: The path to the SQLite database file.

        Raises:
            sqlite3.Error: If there's an error initializing the database.
        """
        self.db_path = db_path
        self._ensure_schema()

    def _connect(self) -> sqlite3.Connection:
        """Create and configure a database connection.

        Enables WAL mode for better concurrent access, sets IMMEDIATE isolation
        level for transaction control, and enables foreign key constraints.

        Returns:
            A configured sqlite3.Connection object.

        Raises:
            sqlite3.Error: If there's an error connecting to the database.
        """
        # Ensure parent directory exists
        self.db_path.parent.mkdir(parents=True, exist_ok=True)

        conn = sqlite3.connect(
            str(self.db_path),
            isolation_level="IMMEDIATE",
            check_same_thread=False,  # Safe: each operation uses fresh connection
        )
        conn.execute("PRAGMA journal_mode=WAL")
        conn.execute("PRAGMA foreign_keys=ON")
        return conn

    def _ensure_schema(self) -> None:
        """Create database tables and indexes if they don't exist.

        Raises:
            sqlite3.Error: If there's an error executing schema creation.
        """
        conn = self._connect()
        try:
            conn.executescript(SCHEMA_SQL)
            conn.commit()
        finally:
            conn.close()

    def load_history(self) -> list[LogEntry]:
        """Load all todo log entries from the database.

        Reconstructs LogEntry objects by joining log_entries and todos tables,
        maintaining the original nested structure. Uses a single JOIN query
        instead of N+1 queries for efficiency.

        Returns:
            A list of LogEntry objects in chronological order.

        Raises:
            sqlite3.Error: If there's an error querying the database.
        """
        conn = self._connect()
        try:
            conn.row_factory = sqlite3.Row

            # Fetch all entries with todos in a single query using LEFT JOIN
            cursor = conn.execute(
                """
                SELECT e.id, e.timestamp, e.session_id, e.cwd,
                       t.content, t.status, t.active_form
                FROM log_entries e
                LEFT JOIN todos t ON e.id = t.entry_id
                ORDER BY e.id, t.id
                """
            )

            entries_map: dict[int, LogEntry] = {}

            for row in cursor:
                entry_id = row["id"]

                # Create entry if not already seen
                if entry_id not in entries_map:
                    entries_map[entry_id] = {
                        "timestamp": row["timestamp"],
                        "session_id": row["session_id"],
                        "cwd": row["cwd"],
                        "todos": [],
                    }

                # Add todo if this row has todo data (handles LEFT JOIN nulls)
                if row["content"] is not None:
                    entries_map[entry_id]["todos"].append({
                        "content": row["content"],
                        "status": row["status"],
                        "activeForm": row["active_form"],
                    })

            return list(entries_map.values())
        finally:
            conn.close()

    def append_entry(self, entry: LogEntry) -> None:
        """Atomically append a new entry to the database.

        Inserts the entry and all its todos in a single transaction to ensure
        consistency.

        Args:
            entry: The LogEntry to append.

        Raises:
            sqlite3.Error: If there's an error during the transaction.
            ValueError: If the entry structure is invalid.
        """
        conn = self._connect()
        try:
            cursor = conn.cursor()

            # Insert the main entry
            cursor.execute(
                "INSERT INTO log_entries (timestamp, session_id, cwd) VALUES (?, ?, ?)",
                (entry["timestamp"], entry["session_id"], entry["cwd"]),
            )
            entry_id = cursor.lastrowid

            # Insert todos for this entry
            for todo in entry["todos"]:
                cursor.execute(
                    "INSERT INTO todos (entry_id, content, status, active_form) VALUES (?, ?, ?, ?)",
                    (entry_id, todo["content"], todo["status"], todo["activeForm"]),
                )

            conn.commit()
        except sqlite3.Error:
            try:
                conn.rollback()
            except sqlite3.Error:
                pass  # Connection may be in bad state after commit failure
            raise
        finally:
            conn.close()

    def get_entries_by_session(self, session_id: str) -> list[LogEntry]:
        """Retrieve all entries for a specific session.

        Uses a single JOIN query instead of N+1 queries for efficiency.

        Args:
            session_id: The session identifier to query for.

        Returns:
            A list of LogEntry objects matching the session_id, in order.
            Empty list if no matches found.

        Raises:
            sqlite3.Error: If there's an error querying the database.
        """
        conn = self._connect()
        try:
            conn.row_factory = sqlite3.Row

            # Fetch entries for this session with todos in a single query using LEFT JOIN
            cursor = conn.execute(
                """
                SELECT e.id, e.timestamp, e.session_id, e.cwd,
                       t.content, t.status, t.active_form
                FROM log_entries e
                LEFT JOIN todos t ON e.id = t.entry_id
                WHERE e.session_id = ?
                ORDER BY e.id, t.id
                """,
                (session_id,),
            )

            entries_map: dict[int, LogEntry] = {}

            for row in cursor:
                entry_id = row["id"]

                # Create entry if not already seen
                if entry_id not in entries_map:
                    entries_map[entry_id] = {
                        "timestamp": row["timestamp"],
                        "session_id": row["session_id"],
                        "cwd": row["cwd"],
                        "todos": [],
                    }

                # Add todo if this row has todo data (handles LEFT JOIN nulls)
                if row["content"] is not None:
                    entries_map[entry_id]["todos"].append({
                        "content": row["content"],
                        "status": row["status"],
                        "activeForm": row["active_form"],
                    })

            return list(entries_map.values())
        finally:
            conn.close()

    def get_todos_by_status(self, status: str) -> list[TodoItem]:
        """Retrieve all todos with a specific status across all entries.

        Queries the todos table directly, returning all todos matching the
        given status regardless of which entry they belong to.

        Args:
            status: The status value to query for (e.g., "pending", "completed").

        Returns:
            A list of TodoItem objects with the matching status.
            Empty list if no matches found.

        Raises:
            sqlite3.Error: If there's an error querying the database.
        """
        conn = self._connect()
        try:
            conn.row_factory = sqlite3.Row

            cursor = conn.execute(
                "SELECT content, status, active_form FROM todos WHERE status = ? ORDER BY id",
                (status,),
            )
            rows = cursor.fetchall()

            return [
                {
                    "content": row["content"],
                    "status": row["status"],
                    "activeForm": row["active_form"],
                }
                for row in rows
            ]
        finally:
            conn.close()
