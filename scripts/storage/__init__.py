"""Storage backend factory and exports for todo-log plugin.

This module provides a factory function to get the appropriate storage backend
based on the TODO_STORAGE_BACKEND environment variable.

Supported backends:
    - "json" (default): JSON file-based storage
    - "sqlite": SQLite database storage

Environment Variables:
    TODO_STORAGE_BACKEND: "json" (default) or "sqlite"
    TODO_LOG_PATH: Custom path for JSON backend (relative or absolute)
    TODO_SQLITE_PATH: Custom path for SQLite backend (relative or absolute)

Example:
    from storage import get_storage_backend
    from pathlib import Path

    backend = get_storage_backend(Path("/home/user/project"))
    history = backend.load_history()
    backend.append_entry(new_entry)
"""

from __future__ import annotations

import os
from pathlib import Path

from storage.json_backend import JSONStorageBackend
from storage.protocol import StorageBackend
from storage.sqlite_backend import SQLiteStorageBackend

__all__ = [
    "StorageBackend",
    "JSONStorageBackend",
    "SQLiteStorageBackend",
    "get_storage_backend",
    "_resolve_safe_path",
]


def _resolve_safe_path(base_dir: Path, user_path: str) -> Path | None:
    """Resolve a path, ensuring it stays within base_dir.

    Args:
        base_dir: The base directory paths must stay within.
        user_path: User-provided path (relative or absolute).

    Returns:
        Resolved absolute path, or None if path escapes base_dir.
    """
    if not user_path or not user_path.strip():
        return None

    if "\x00" in user_path:
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


def _get_json_path(project_dir: Path) -> Path:
    """Get the JSON log file path from environment or default.

    Args:
        project_dir: The project root directory.

    Returns:
        Path to the JSON log file.

    Raises:
        ValueError: If TODO_LOG_PATH escapes project directory.
    """
    custom_path = os.environ.get("TODO_LOG_PATH", "").strip()

    if custom_path:
        safe_path = _resolve_safe_path(project_dir, custom_path)
        if safe_path is None:
            raise ValueError(
                f"TODO_LOG_PATH '{custom_path}' escapes project directory"
            )
        return safe_path

    return project_dir / ".claude" / "todos.json"


def _get_sqlite_path(project_dir: Path) -> Path:
    """Get the SQLite database path from environment or default.

    Args:
        project_dir: The project root directory.

    Returns:
        Path to the SQLite database file.

    Raises:
        ValueError: If TODO_SQLITE_PATH escapes project directory.
    """
    custom_path = os.environ.get("TODO_SQLITE_PATH", "").strip()

    if custom_path:
        safe_path = _resolve_safe_path(project_dir, custom_path)
        if safe_path is None:
            raise ValueError(
                f"TODO_SQLITE_PATH '{custom_path}' escapes project directory"
            )
        return safe_path

    return project_dir / ".claude" / "todos.db"


def get_storage_backend(project_dir: Path) -> StorageBackend:
    """Get the configured storage backend for todo storage.

    Reads the TODO_STORAGE_BACKEND environment variable to determine which
    backend to use. Defaults to JSON if not set or unrecognized.

    Path configuration:
        - JSON backend: Uses TODO_LOG_PATH or defaults to .claude/todos.json
        - SQLite backend: Uses TODO_SQLITE_PATH or defaults to .claude/todos.db

    Args:
        project_dir: The project root directory used for resolving paths.

    Returns:
        An instance of the configured StorageBackend.

    Raises:
        ValueError: If the storage backend or path configuration is invalid.
    """
    backend_type = os.environ.get("TODO_STORAGE_BACKEND", "json").strip().lower()

    if backend_type == "json":
        log_file = _get_json_path(project_dir)
        return JSONStorageBackend(log_file)
    elif backend_type == "sqlite":
        db_path = _get_sqlite_path(project_dir)
        return SQLiteStorageBackend(db_path)
    else:
        raise ValueError(
            f"Unknown storage backend: {backend_type!r}. "
            f"Expected 'json' or 'sqlite'."
        )
