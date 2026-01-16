"""Shared fixtures and utilities for storage backend tests.

This module provides common test fixtures used across all storage backend tests,
including sample data, temporary directories, and parameterized backend instances.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from storage.json_backend import JSONStorageBackend
from storage.protocol import LogEntry, StorageBackend, TodoItem
from storage.sqlite_backend import SQLiteStorageBackend


@pytest.fixture
def tmp_project(tmp_path: Path) -> Path:
    """Create a temporary project directory for testing.

    Returns:
        Path to a clean temporary directory.
    """
    project = tmp_path / "project"
    project.mkdir()
    return project


@pytest.fixture
def sample_todo() -> TodoItem:
    """Create a sample todo item for testing.

    Returns:
        A valid TodoItem with all required fields.
    """
    return {
        "content": "Sample task",
        "status": "pending",
        "activeForm": "Doing sample task",
    }


@pytest.fixture
def sample_todos() -> list[TodoItem]:
    """Create a list of sample todo items for testing.

    Returns:
        A list of TodoItem objects with varied statuses.
    """
    return [
        {
            "content": "Task 1",
            "status": "pending",
            "activeForm": "Creating task 1",
        },
        {
            "content": "Task 2",
            "status": "in_progress",
            "activeForm": "Working on task 2",
        },
        {
            "content": "Task 3",
            "status": "completed",
            "activeForm": "Completed task 3",
        },
    ]


@pytest.fixture
def sample_entry(sample_todos: list[TodoItem]) -> LogEntry:
    """Create a sample log entry for testing.

    Args:
        sample_todos: Fixture providing sample todo items.

    Returns:
        A valid LogEntry with sample data.
    """
    return {
        "timestamp": "2025-01-01T00:00:00.000Z",
        "session_id": "test-session-123",
        "cwd": "/home/user/project",
        "todos": sample_todos,
    }


@pytest.fixture
def empty_entry() -> LogEntry:
    """Create a log entry with no todos.

    Returns:
        A valid LogEntry with an empty todos list.
    """
    return {
        "timestamp": "2025-01-01T12:00:00.000Z",
        "session_id": "empty-session",
        "cwd": "/tmp/test",
        "todos": [],
    }


@pytest.fixture
def multiple_entries(sample_todos: list[TodoItem]) -> list[LogEntry]:
    """Create multiple log entries for testing.

    Args:
        sample_todos: Fixture providing sample todo items.

    Returns:
        A list of LogEntry objects from different sessions.
    """
    return [
        {
            "timestamp": "2025-01-01T10:00:00.000Z",
            "session_id": "session-1",
            "cwd": "/path/one",
            "todos": [sample_todos[0]],
        },
        {
            "timestamp": "2025-01-01T11:00:00.000Z",
            "session_id": "session-2",
            "cwd": "/path/two",
            "todos": [sample_todos[1], sample_todos[2]],
        },
        {
            "timestamp": "2025-01-01T12:00:00.000Z",
            "session_id": "session-1",
            "cwd": "/path/one",
            "todos": [],
        },
    ]


@pytest.fixture(params=["json", "sqlite"])
def storage_backend(request, tmp_project: Path) -> StorageBackend:
    """Parameterized fixture providing both storage backend types.

    This fixture enables cross-backend compliance testing by running
    the same tests against both JSON and SQLite implementations.

    Args:
        request: Pytest request object with param.
        tmp_project: Temporary project directory.

    Returns:
        An instance of either JSONStorageBackend or SQLiteStorageBackend.
    """
    if request.param == "json":
        return JSONStorageBackend(tmp_project / "todos.json")
    else:
        return SQLiteStorageBackend(tmp_project / "todos.db")


@pytest.fixture
def json_backend(tmp_project: Path) -> JSONStorageBackend:
    """Create a JSON storage backend for JSON-specific tests.

    Args:
        tmp_project: Temporary project directory.

    Returns:
        An instance of JSONStorageBackend.
    """
    return JSONStorageBackend(tmp_project / "todos.json")


@pytest.fixture
def sqlite_backend(tmp_project: Path) -> SQLiteStorageBackend:
    """Create a SQLite storage backend for SQLite-specific tests.

    Args:
        tmp_project: Temporary project directory.

    Returns:
        An instance of SQLiteStorageBackend.
    """
    return SQLiteStorageBackend(tmp_project / "todos.db")
