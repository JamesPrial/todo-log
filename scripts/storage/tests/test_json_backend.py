"""Comprehensive tests for JSONStorageBackend.

This module tests the JSON file-based storage backend, verifying:
- File loading and handling of edge cases
- Atomic write operations
- Error recovery and data integrity
- Unicode support
"""

from __future__ import annotations

import json
import os
import tempfile
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from storage.json_backend import JSONStorageBackend
from storage.protocol import LogEntry, TodoItem


# =============================================================================
# TestLoadHistory
# =============================================================================


class TestLoadHistory:
    """Tests for JSONStorageBackend.load_history method."""

    def test_should_return_empty_list_when_file_does_not_exist(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history returns empty list for non-existent file."""
        result = json_backend.load_history()
        assert result == []
        assert isinstance(result, list)

    def test_should_load_valid_json_array_correctly(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify load_history loads valid JSON array."""
        # Write valid JSON directly to file
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        entries = [sample_entry]
        json_backend.log_file.write_text(json.dumps(entries), encoding="utf-8")

        result = json_backend.load_history()
        assert len(result) == 1
        assert result[0]["session_id"] == sample_entry["session_id"]
        assert result[0]["timestamp"] == sample_entry["timestamp"]

    def test_should_return_empty_list_for_corrupted_json(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history returns empty list for corrupted JSON."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text("{not valid json]}", encoding="utf-8")

        result = json_backend.load_history()
        assert result == []

    def test_should_return_empty_list_for_non_array_json(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history returns empty list for non-array JSON."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text('{"key": "value"}', encoding="utf-8")

        result = json_backend.load_history()
        assert result == []

    def test_should_return_empty_list_for_json_object(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history returns empty list when JSON is an object, not array."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text(
            '{"entries": [{"timestamp": "2025-01-01T00:00:00.000Z"}]}',
            encoding="utf-8",
        )

        result = json_backend.load_history()
        assert result == []

    def test_should_handle_unicode_content_correctly(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history handles Unicode content."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)

        unicode_entry: LogEntry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "unicode-test",
            "cwd": "/home/用户/项目",
            "todos": [
                {
                    "content": "实现功能 🚀",
                    "status": "pending",
                    "activeForm": "正在实现 ⚡",
                }
            ],
        }

        json_backend.log_file.write_text(
            json.dumps([unicode_entry], ensure_ascii=False), encoding="utf-8"
        )

        result = json_backend.load_history()
        assert len(result) == 1
        assert "🚀" in result[0]["todos"][0]["content"]
        assert "⚡" in result[0]["todos"][0]["activeForm"]
        assert "用户" in result[0]["cwd"]

    def test_should_load_empty_array_correctly(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history handles empty JSON array."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text("[]", encoding="utf-8")

        result = json_backend.load_history()
        assert result == []

    def test_should_load_multiple_entries_in_order(
        self, json_backend: JSONStorageBackend, multiple_entries: list[LogEntry]
    ) -> None:
        """Verify load_history preserves entry order."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text(
            json.dumps(multiple_entries), encoding="utf-8"
        )

        result = json_backend.load_history()
        assert len(result) == 3
        assert result[0]["session_id"] == "session-1"
        assert result[1]["session_id"] == "session-2"
        assert result[2]["session_id"] == "session-1"

    def test_should_handle_file_with_whitespace(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history handles files with leading/trailing whitespace."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text(
            '\n  \n  []  \n  \n',
            encoding="utf-8",
        )

        result = json_backend.load_history()
        assert result == []

    def test_should_handle_os_error_gracefully(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history handles OSError gracefully."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text("[]", encoding="utf-8")

        # Mock open to raise OSError
        with patch("builtins.open", side_effect=OSError("Disk error")):
            result = json_backend.load_history()
            assert result == []

    def test_should_handle_permission_error(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify load_history handles permission errors gracefully."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text("[]", encoding="utf-8")

        # Make file unreadable (Unix only)
        if os.name != "nt":
            original_mode = json_backend.log_file.stat().st_mode
            try:
                json_backend.log_file.chmod(0o000)
                result = json_backend.load_history()
                assert result == []
            finally:
                json_backend.log_file.chmod(original_mode)


# =============================================================================
# TestAppendEntry
# =============================================================================


class TestAppendEntry:
    """Tests for JSONStorageBackend.append_entry method."""

    def test_should_create_parent_directories_if_needed(
        self, tmp_project: Path, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry creates parent directories."""
        deep_path = tmp_project / "nested" / "dir" / "structure" / "todos.json"
        backend = JSONStorageBackend(deep_path)

        backend.append_entry(sample_entry)

        assert deep_path.exists()
        assert deep_path.parent.exists()

    def test_should_create_new_file_with_single_entry(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry creates new file with entry."""
        json_backend.append_entry(sample_entry)

        assert json_backend.log_file.exists()
        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 1
        assert content[0]["session_id"] == sample_entry["session_id"]

    def test_should_append_to_existing_entries(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry appends to existing file."""
        # Create initial entry
        first_entry: LogEntry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "first-session",
            "cwd": "/path/one",
            "todos": [],
        }
        json_backend.append_entry(first_entry)

        # Append second entry
        json_backend.append_entry(sample_entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 2
        assert content[0]["session_id"] == "first-session"
        assert content[1]["session_id"] == sample_entry["session_id"]

    def test_should_preserve_existing_entries(
        self, json_backend: JSONStorageBackend, multiple_entries: list[LogEntry]
    ) -> None:
        """Verify append_entry preserves all existing entries."""
        # Add first two entries
        json_backend.append_entry(multiple_entries[0])
        json_backend.append_entry(multiple_entries[1])

        # Add third entry
        json_backend.append_entry(multiple_entries[2])

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 3
        # Verify first two entries are unchanged
        assert content[0]["session_id"] == multiple_entries[0]["session_id"]
        assert content[1]["session_id"] == multiple_entries[1]["session_id"]
        assert len(content[1]["todos"]) == 2  # Preserved nested todos

    def test_should_use_atomic_write_with_temp_file(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry uses temp file for atomic write."""
        mkstemp_called = False
        original_mkstemp = tempfile.mkstemp

        def mock_mkstemp(*args, **kwargs):
            nonlocal mkstemp_called
            mkstemp_called = True
            return original_mkstemp(*args, **kwargs)

        with patch("tempfile.mkstemp", side_effect=mock_mkstemp):
            json_backend.append_entry(sample_entry)

        assert mkstemp_called

    def test_should_clean_up_temp_file_on_failure(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry cleans up temp file on write failure."""
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)

        # Mock replace to raise an exception
        def mock_replace(src, dst):
            raise OSError("Mock failure")

        with patch("os.replace", side_effect=mock_replace):
            with patch("os.unlink") as mock_unlink:
                with pytest.raises(OSError):
                    json_backend.append_entry(sample_entry)

                # Verify unlink was called to clean up
                assert mock_unlink.called

    def test_should_format_json_with_2_space_indent(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry formats JSON with 2-space indentation."""
        json_backend.append_entry(sample_entry)

        content = json_backend.log_file.read_text(encoding="utf-8")
        # Check for proper indentation
        assert "  {" in content  # Indented object
        assert '    "timestamp"' in content  # Indented property

    def test_should_handle_unicode_in_todos(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify append_entry handles Unicode content correctly."""
        unicode_entry: LogEntry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "unicode-session",
            "cwd": "/home/用户",
            "todos": [
                {
                    "content": "实现功能 🎯",
                    "status": "in_progress",
                    "activeForm": "正在实现 ⚡",
                }
            ],
        }

        json_backend.append_entry(unicode_entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert "🎯" in content[0]["todos"][0]["content"]
        assert "用户" in content[0]["cwd"]

    def test_should_handle_empty_todos_list(
        self, json_backend: JSONStorageBackend, empty_entry: LogEntry
    ) -> None:
        """Verify append_entry handles entries with empty todos list."""
        json_backend.append_entry(empty_entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 1
        assert content[0]["todos"] == []

    def test_should_handle_multiple_sequential_appends(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify multiple sequential appends accumulate correctly."""
        for i in range(5):
            entry: LogEntry = {
                "timestamp": f"2025-01-0{i+1}T00:00:00.000Z",
                "session_id": f"session-{i}",
                "cwd": f"/path/{i}",
                "todos": [],
            }
            json_backend.append_entry(entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 5
        assert content[0]["session_id"] == "session-0"
        assert content[4]["session_id"] == "session-4"

    def test_should_use_os_replace_for_atomicity(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry uses os.replace for atomic operation."""
        replace_called = False
        original_replace = os.replace

        def mock_replace(src, dst):
            nonlocal replace_called
            replace_called = True
            return original_replace(src, dst)

        with patch("os.replace", side_effect=mock_replace):
            json_backend.append_entry(sample_entry)

        assert replace_called

    def test_should_recover_from_corrupted_existing_file(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry recovers from corrupted existing file."""
        # Create corrupted file
        json_backend.log_file.parent.mkdir(parents=True, exist_ok=True)
        json_backend.log_file.write_text("{corrupted json", encoding="utf-8")

        # Append should start fresh
        json_backend.append_entry(sample_entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content) == 1
        assert content[0]["session_id"] == sample_entry["session_id"]

    def test_should_preserve_file_contents_on_exception(
        self, json_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify original file is preserved when append fails."""
        # Create initial file
        initial_entry: LogEntry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "initial",
            "cwd": "/path",
            "todos": [],
        }
        json_backend.append_entry(initial_entry)

        original_content = json_backend.log_file.read_text(encoding="utf-8")

        # Mock replace to fail
        with patch("os.replace", side_effect=OSError("Disk full")):
            with pytest.raises(OSError):
                json_backend.append_entry(sample_entry)

        # Original file should be unchanged
        assert json_backend.log_file.read_text(encoding="utf-8") == original_content

    def test_should_handle_very_long_content(
        self, json_backend: JSONStorageBackend
    ) -> None:
        """Verify append_entry handles very long content strings."""
        long_content = "x" * 10000  # 10KB string

        large_entry: LogEntry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "large-content-session",
            "cwd": "/path",
            "todos": [
                {
                    "content": long_content,
                    "status": "pending",
                    "activeForm": "Processing large content",
                }
            ],
        }

        json_backend.append_entry(large_entry)

        content = json.loads(json_backend.log_file.read_text(encoding="utf-8"))
        assert len(content[0]["todos"][0]["content"]) == 10000


# =============================================================================
# TestCrossBackendCompliance
# =============================================================================


class TestCrossBackendCompliance:
    """Cross-backend compliance tests using parameterized fixtures."""

    def test_should_return_empty_list_on_first_load(
        self, storage_backend: JSONStorageBackend
    ) -> None:
        """Verify both backends return empty list initially."""
        result = storage_backend.load_history()
        assert result == []

    def test_should_persist_appended_entry(
        self, storage_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify append_entry then load_history returns the entry."""
        storage_backend.append_entry(sample_entry)

        result = storage_backend.load_history()
        assert len(result) == 1
        assert result[0]["session_id"] == sample_entry["session_id"]
        assert result[0]["timestamp"] == sample_entry["timestamp"]
        assert result[0]["cwd"] == sample_entry["cwd"]

    def test_should_preserve_order_across_multiple_appends(
        self, storage_backend: JSONStorageBackend, multiple_entries: list[LogEntry]
    ) -> None:
        """Verify multiple appends preserve insertion order."""
        for entry in multiple_entries:
            storage_backend.append_entry(entry)

        result = storage_backend.load_history()
        assert len(result) == 3
        assert result[0]["session_id"] == multiple_entries[0]["session_id"]
        assert result[1]["session_id"] == multiple_entries[1]["session_id"]
        assert result[2]["session_id"] == multiple_entries[2]["session_id"]

    def test_should_preserve_nested_todos_structure(
        self, storage_backend: JSONStorageBackend, sample_entry: LogEntry
    ) -> None:
        """Verify nested todos are preserved correctly."""
        storage_backend.append_entry(sample_entry)

        result = storage_backend.load_history()
        assert len(result[0]["todos"]) == 3
        assert result[0]["todos"][0]["content"] == "Task 1"
        assert result[0]["todos"][1]["status"] == "in_progress"
        assert result[0]["todos"][2]["activeForm"] == "Completed task 3"


# =============================================================================
# Main Entry Point
# =============================================================================


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
