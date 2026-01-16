"""Tests for protocol definitions and TypedDict structures.

This module tests the type definitions and protocol interfaces defined in
storage.protocol to ensure they maintain their contracts.
"""

from __future__ import annotations

import pytest

from storage.json_backend import JSONStorageBackend
from storage.protocol import LogEntry, StorageBackend, TodoItem
from storage.sqlite_backend import SQLiteStorageBackend


# =============================================================================
# TestTodoItemTypedDict
# =============================================================================


class TestTodoItemTypedDict:
    """Tests for TodoItem TypedDict structure."""

    def test_should_have_required_content_key(self, sample_todo: TodoItem) -> None:
        """Verify TodoItem has required 'content' key."""
        assert "content" in sample_todo
        assert isinstance(sample_todo["content"], str)

    def test_should_have_required_status_key(self, sample_todo: TodoItem) -> None:
        """Verify TodoItem has required 'status' key."""
        assert "status" in sample_todo
        assert isinstance(sample_todo["status"], str)

    def test_should_have_required_active_form_key(self, sample_todo: TodoItem) -> None:
        """Verify TodoItem has required 'activeForm' key."""
        assert "activeForm" in sample_todo
        assert isinstance(sample_todo["activeForm"], str)

    def test_should_accept_all_required_keys(self) -> None:
        """Verify TodoItem accepts dict with all required keys."""
        todo: TodoItem = {
            "content": "Test task",
            "status": "pending",
            "activeForm": "Testing",
        }
        assert todo["content"] == "Test task"
        assert todo["status"] == "pending"
        assert todo["activeForm"] == "Testing"

    def test_should_accept_empty_string_values(self) -> None:
        """Verify TodoItem accepts empty strings for all fields."""
        todo: TodoItem = {
            "content": "",
            "status": "",
            "activeForm": "",
        }
        assert todo["content"] == ""
        assert todo["status"] == ""
        assert todo["activeForm"] == ""

    def test_should_accept_unicode_content(self) -> None:
        """Verify TodoItem accepts Unicode characters in content."""
        todo: TodoItem = {
            "content": "实现功能 🚀",
            "status": "in_progress",
            "activeForm": "正在实现 ⚡",
        }
        assert "🚀" in todo["content"]
        assert "⚡" in todo["activeForm"]


# =============================================================================
# TestLogEntryTypedDict
# =============================================================================


class TestLogEntryTypedDict:
    """Tests for LogEntry TypedDict structure."""

    def test_should_have_required_timestamp_key(self, sample_entry: LogEntry) -> None:
        """Verify LogEntry has required 'timestamp' key."""
        assert "timestamp" in sample_entry
        assert isinstance(sample_entry["timestamp"], str)

    def test_should_have_required_session_id_key(
        self, sample_entry: LogEntry
    ) -> None:
        """Verify LogEntry has required 'session_id' key."""
        assert "session_id" in sample_entry
        assert isinstance(sample_entry["session_id"], str)

    def test_should_have_required_cwd_key(self, sample_entry: LogEntry) -> None:
        """Verify LogEntry has required 'cwd' key."""
        assert "cwd" in sample_entry
        assert isinstance(sample_entry["cwd"], str)

    def test_should_have_required_todos_key(self, sample_entry: LogEntry) -> None:
        """Verify LogEntry has required 'todos' key as list."""
        assert "todos" in sample_entry
        assert isinstance(sample_entry["todos"], list)

    def test_should_accept_all_required_keys(self, sample_todos: list[TodoItem]) -> None:
        """Verify LogEntry accepts dict with all required keys."""
        entry: LogEntry = {
            "timestamp": "2025-01-15T10:30:45.123Z",
            "session_id": "test-session",
            "cwd": "/home/user/project",
            "todos": sample_todos,
        }
        assert entry["timestamp"] == "2025-01-15T10:30:45.123Z"
        assert entry["session_id"] == "test-session"
        assert entry["cwd"] == "/home/user/project"
        assert len(entry["todos"]) == 3

    def test_should_accept_empty_todos_list(self) -> None:
        """Verify LogEntry accepts empty todos list."""
        entry: LogEntry = {
            "timestamp": "2025-01-15T10:30:45.123Z",
            "session_id": "test-session",
            "cwd": "/home/user/project",
            "todos": [],
        }
        assert entry["todos"] == []

    def test_should_accept_iso_8601_timestamp(self) -> None:
        """Verify LogEntry accepts ISO 8601 timestamp with milliseconds."""
        entry: LogEntry = {
            "timestamp": "2025-12-31T23:59:59.999Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }
        assert entry["timestamp"].endswith("Z")
        assert "." in entry["timestamp"]  # Has milliseconds


# =============================================================================
# TestStorageBackendProtocol
# =============================================================================


class TestStorageBackendProtocol:
    """Tests for StorageBackend protocol compliance."""

    def test_should_define_load_history_method(self) -> None:
        """Verify StorageBackend protocol defines load_history method."""
        assert hasattr(StorageBackend, "load_history")

    def test_should_define_append_entry_method(self) -> None:
        """Verify StorageBackend protocol defines append_entry method."""
        assert hasattr(StorageBackend, "append_entry")

    def test_json_backend_implements_protocol(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify JSONStorageBackend implements StorageBackend protocol."""
        assert hasattr(json_backend, "load_history")
        assert hasattr(json_backend, "append_entry")
        assert callable(json_backend.load_history)
        assert callable(json_backend.append_entry)

    def test_sqlite_backend_implements_protocol(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify SQLiteStorageBackend implements StorageBackend protocol."""
        assert hasattr(sqlite_backend, "load_history")
        assert hasattr(sqlite_backend, "append_entry")
        assert callable(sqlite_backend.load_history)
        assert callable(sqlite_backend.append_entry)

    def test_json_backend_load_history_returns_list(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify JSONStorageBackend.load_history returns list."""
        result = json_backend.load_history()
        assert isinstance(result, list)

    def test_sqlite_backend_load_history_returns_list(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify SQLiteStorageBackend.load_history returns list."""
        result = sqlite_backend.load_history()
        assert isinstance(result, list)

    def test_json_backend_append_entry_accepts_log_entry(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify JSONStorageBackend.append_entry accepts LogEntry."""
        # Should not raise
        json_backend.append_entry(sample_entry)

    def test_sqlite_backend_append_entry_accepts_log_entry(
        self, sqlite_backend: SQLiteStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify SQLiteStorageBackend.append_entry accepts LogEntry."""
        # Should not raise
        sqlite_backend.append_entry(sample_entry)


# =============================================================================
# TestQueryableStorageBackendProtocol
# =============================================================================


class TestQueryableStorageBackendProtocol:
    """Tests for QueryableStorageBackend protocol (SQLite only)."""

    def test_sqlite_backend_should_have_get_entries_by_session(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify SQLiteStorageBackend implements get_entries_by_session."""
        assert hasattr(sqlite_backend, "get_entries_by_session")
        assert callable(sqlite_backend.get_entries_by_session)

    def test_sqlite_backend_should_have_get_todos_by_status(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify SQLiteStorageBackend implements get_todos_by_status."""
        assert hasattr(sqlite_backend, "get_todos_by_status")
        assert callable(sqlite_backend.get_todos_by_status)

    def test_get_entries_by_session_should_return_list(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify get_entries_by_session returns list."""
        result = sqlite_backend.get_entries_by_session("unknown-session")
        assert isinstance(result, list)

    def test_get_todos_by_status_should_return_list(
        self, sqlite_backend: SQLiteStorageBackend
    ) -> None:
        """Verify get_todos_by_status returns list."""
        result = sqlite_backend.get_todos_by_status("pending")
        assert isinstance(result, list)

    def test_get_entries_by_session_should_accept_string_session_id(
        self, sqlite_backend: SQLiteStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify get_entries_by_session accepts string session_id."""
        sqlite_backend.append_entry(sample_entry)
        result = sqlite_backend.get_entries_by_session(sample_entry["session_id"])
        assert isinstance(result, list)
        assert len(result) >= 1

    def test_get_todos_by_status_should_accept_string_status(
        self, sqlite_backend: SQLiteStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify get_todos_by_status accepts string status."""
        sqlite_backend.append_entry(sample_entry)
        result = sqlite_backend.get_todos_by_status("pending")
        assert isinstance(result, list)


# =============================================================================
# Main Entry Point
# =============================================================================


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
