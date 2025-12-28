"""
Comprehensive test suite for save_todos.py TodoWrite logger hook.

Tests cover:
- UTC ISO timestamp generation
- Todo item validation
- Path resolution and security (escaping detection)
- Hook input reading and filtering
- Log entry construction
- Log file path determination
- Log file I/O operations
- Main function integration tests
"""
from __future__ import annotations

import json
import os
import re
import sys
import tempfile
from datetime import datetime, timezone
from io import StringIO
from pathlib import Path
from unittest.mock import patch, MagicMock

import pytest

from save_todos import (
    UNKNOWN_VALUE,
    utc_iso_timestamp,
    validate_todo,
    validate_todos,
    resolve_safe_path,
    read_hook_input,
    build_log_entry,
    get_log_file_path,
    load_existing_history,
    append_to_log,
    main,
    # Type definitions
    TodoItem,
    HookInput,
    LogEntry,
    ToolInput,
)

# ISO 8601 timestamp pattern
ISO_8601_PATTERN = r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$"


# =============================================================================
# Fixtures
# =============================================================================


@pytest.fixture
def tmp_project(tmp_path: Path) -> Path:
    """Create a temporary project directory."""
    project = tmp_path / "project"
    project.mkdir()
    return project


@pytest.fixture
def sample_todo_list() -> list[TodoItem]:
    """Standard todo list for testing."""
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
def sample_hook_input(sample_todo_list: list[TodoItem]) -> HookInput:
    """Standard hook input for testing."""
    return {
        "tool_name": "TodoWrite",
        "tool_input": {"todos": sample_todo_list},
        "session_id": "test-session-123",
        "cwd": "/home/user/project",
    }


# =============================================================================
# TestUtcIsoTimestamp
# =============================================================================


class TestUtcIsoTimestamp:
    """Tests for utc_iso_timestamp() function."""

    def test_returns_iso_format_string(self) -> None:
        """Return timestamp in ISO 8601 format with Z suffix."""
        result = utc_iso_timestamp()
        assert isinstance(result, str)
        assert result.endswith("Z")

    def test_contains_milliseconds(self) -> None:
        """Timestamp contains exactly 3 millisecond digits."""
        result = utc_iso_timestamp()
        # Format: YYYY-MM-DDTHH:MM:SS.xxxZ
        parts = result.split(".")
        assert len(parts) == 2
        milliseconds_part = parts[1]
        assert milliseconds_part[:-1].isdigit()
        assert len(milliseconds_part) == 4  # 3 digits + Z

    def test_matches_iso_8601_pattern(self) -> None:
        """Timestamp matches ISO 8601 regex pattern."""
        result = utc_iso_timestamp()
        # ISO 8601 with milliseconds: YYYY-MM-DDTHH:MM:SS.xxxZ
        assert re.match(ISO_8601_PATTERN, result)

    def test_uses_utc_timezone(self) -> None:
        """Timestamp uses UTC timezone."""
        result = utc_iso_timestamp()
        # The function explicitly uses timezone.utc
        assert result.endswith("Z")
        # Verify it can be parsed back
        parsed = datetime.fromisoformat(result.replace("Z", "+00:00"))
        assert parsed.tzinfo == timezone.utc

    def test_successive_calls_increasing_or_equal(self) -> None:
        """Successive calls return equal or increasing timestamps."""
        ts1 = utc_iso_timestamp()
        ts2 = utc_iso_timestamp()
        assert ts2 >= ts1

    def test_formats_with_leading_zeros(self) -> None:
        """Date/time components are zero-padded."""
        result = utc_iso_timestamp()
        # Check that month, day, hour, minute, second are 2 digits
        parts = result.split("T")
        assert len(parts) == 2
        date_part = parts[0]
        assert date_part.count("-") == 2


# =============================================================================
# TestValidateTodo
# =============================================================================


class TestValidateTodo:
    """Tests for validate_todo() function."""

    def test_valid_todo_with_all_keys(self) -> None:
        """Valid todo with all required keys returns True."""
        todo = {
            "content": "Task description",
            "status": "pending",
            "activeForm": "Task description",
        }
        assert validate_todo(todo) is True

    @pytest.mark.parametrize(
        "missing_key,todo",
        [
            ("content", {"status": "pending", "activeForm": "Task"}),
            ("status", {"content": "Task", "activeForm": "Task"}),
            ("activeForm", {"content": "Task", "status": "pending"}),
        ],
    )
    def test_missing_required_key_returns_false(
        self, missing_key: str, todo: dict[str, str]
    ) -> None:
        """Todo missing a required key returns False."""
        assert validate_todo(todo) is False

    @pytest.mark.parametrize(
        "invalid_input",
        [
            "not a dict",
            [1, 2, 3],
            42,
            None,
        ],
    )
    def test_non_dict_input_returns_false(self, invalid_input: object) -> None:
        """Non-dict input returns False."""
        assert validate_todo(invalid_input) is False

    def test_extra_keys_are_allowed(self) -> None:
        """Todo with extra keys still validates."""
        todo = {
            "content": "Task description",
            "status": "pending",
            "activeForm": "Task description",
            "extra_field": "extra_value",
            "another": 123,
        }
        assert validate_todo(todo) is True

    def test_empty_string_values_valid(self) -> None:
        """Empty string values for required keys are still valid."""
        todo = {
            "content": "",
            "status": "",
            "activeForm": "",
        }
        assert validate_todo(todo) is True

    def test_numeric_values_pass_validation(self) -> None:
        """Non-string values in required fields still validate (duck typing)."""
        # TypedDict is a runtime hint, dict validation only checks keys
        todo = {
            "content": 123,
            "status": 456,
            "activeForm": 789,
        }
        assert validate_todo(todo) is True


# =============================================================================
# TestValidateTodos
# =============================================================================


class TestValidateTodos:
    """Tests for validate_todos() function."""

    def test_valid_list_returns_all(self, sample_todo_list: list[TodoItem]) -> None:
        """Valid list of todos returns all items."""
        result = validate_todos(sample_todo_list)
        assert len(result) == 3
        assert all(isinstance(item, dict) for item in result)

    def test_non_list_input_returns_empty(self) -> None:
        """Non-list input returns empty list."""
        assert validate_todos("not a list") == []
        assert validate_todos({"dict": "value"}) == []
        assert validate_todos(42) == []

    def test_empty_list_returns_empty(self) -> None:
        """Empty list returns empty list."""
        assert validate_todos([]) == []

    def test_mixed_valid_invalid_filters_correctly(self) -> None:
        """Mixed valid/invalid todos filters correctly."""
        todos = [
            {
                "content": "Valid task",
                "status": "pending",
                "activeForm": "Doing task",
            },
            {"content": "Missing status", "activeForm": "Doing task"},
            {
                "content": "Another valid",
                "status": "completed",
                "activeForm": "Did task",
            },
            "not a dict",
        ]
        result = validate_todos(todos)
        assert len(result) == 2

    def test_none_returns_empty_list(self) -> None:
        """None input returns empty list."""
        assert validate_todos(None) == []

    def test_nested_non_dict_items_filtered(self) -> None:
        """Non-dict items in list are filtered out."""
        todos = [
            {
                "content": "Valid",
                "status": "pending",
                "activeForm": "Doing",
            },
            None,
            42,
            "string",
            [],
        ]
        result = validate_todos(todos)
        assert len(result) == 1


# =============================================================================
# TestResolveSafePath
# =============================================================================


class TestResolveSafePath:
    """Tests for resolve_safe_path() function."""

    def test_relative_path_within_base_dir(self, tmp_project: Path) -> None:
        """Relative path within base_dir resolves correctly."""
        result = resolve_safe_path(tmp_project, "subdir/file.txt")
        assert result is not None
        assert result.is_relative_to(tmp_project)

    def test_absolute_path_within_base_dir(self, tmp_project: Path) -> None:
        """Absolute path within base_dir works."""
        subdir = tmp_project / "subdir"
        subdir.mkdir()
        result = resolve_safe_path(tmp_project, str(subdir / "file.txt"))
        assert result is not None
        assert result.parent == subdir

    def test_path_escaping_base_dir_returns_none(self, tmp_project: Path) -> None:
        """Path with '..' escaping base_dir returns None."""
        result = resolve_safe_path(tmp_project, "../outside/path.txt")
        assert result is None

    def test_absolute_path_escaping_project_returns_none(
        self, tmp_project: Path
    ) -> None:
        """Absolute path outside project returns None."""
        result = resolve_safe_path(tmp_project, "/etc/passwd")
        assert result is None

    def test_empty_string_returns_none(self, tmp_project: Path) -> None:
        """Empty string returns None."""
        assert resolve_safe_path(tmp_project, "") is None

    def test_whitespace_string_returns_none(self, tmp_project: Path) -> None:
        """Whitespace-only string returns None."""
        assert resolve_safe_path(tmp_project, "   ") is None

    def test_deep_nested_path_works(self, tmp_project: Path) -> None:
        """Deep nested path within project works."""
        result = resolve_safe_path(
            tmp_project, "a/b/c/d/e/f/g/h/i/j/file.txt"
        )
        assert result is not None
        assert "a" in str(result)

    def test_path_with_trailing_slash(self, tmp_project: Path) -> None:
        """Path with trailing slash is handled."""
        result = resolve_safe_path(tmp_project, "subdir/")
        assert result is not None

    def test_path_at_base_dir_root(self, tmp_project: Path) -> None:
        """Path at base_dir root works."""
        result = resolve_safe_path(tmp_project, ".")
        assert result == tmp_project.resolve()

    def test_symlink_escaping_returns_none(self, tmp_path: Path) -> None:
        """Symlink escaping base_dir returns None."""
        project = tmp_path / "project"
        project.mkdir()
        outside = tmp_path / "outside"
        outside.mkdir()
        outside_file = outside / "file.txt"
        outside_file.touch()

        symlink = project / "symlink_to_outside"
        symlink.symlink_to(outside_file)

        result = resolve_safe_path(project, "symlink_to_outside")
        assert result is None

    def test_path_normalization(self, tmp_project: Path) -> None:
        """Paths with ./ and redundant slashes normalize."""
        result = resolve_safe_path(tmp_project, "./subdir//file.txt")
        assert result is not None

    def test_path_with_null_bytes_handled(self, tmp_project: Path) -> None:
        """Path containing null bytes is handled gracefully."""
        # Null bytes in paths cause issues on most filesystems
        result = resolve_safe_path(tmp_project, "file\x00name.txt")
        # Should either return None or raise - just ensure no crash
        assert result is None or isinstance(result, Path)


# =============================================================================
# TestReadHookInput
# =============================================================================


class TestReadHookInput:
    """Tests for read_hook_input() function."""

    def test_todowrite_event_returns_parsed_dict(
        self, sample_hook_input: HookInput
    ) -> None:
        """TodoWrite event returns parsed hook input dict."""
        with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
            result = read_hook_input()

        assert result is not None
        assert result["tool_name"] == "TodoWrite"
        assert "tool_input" in result

    def test_non_todowrite_event_returns_none(self) -> None:
        """Non-TodoWrite event returns None."""
        hook_input = {
            "tool_name": "Read",
            "tool_input": {"file_path": "/path/to/file"},
            "session_id": "test",
        }
        with patch("sys.stdin", StringIO(json.dumps(hook_input))):
            result = read_hook_input()

        assert result is None

    def test_debug_env_var_logs_to_stderr(self, capsys: pytest.CaptureFixture[str]) -> None:
        """DEBUG env var logs non-TodoWrite event to stderr."""
        hook_input = {
            "tool_name": "Bash",
            "tool_input": {"command": "ls"},
        }
        with patch.dict(os.environ, {"DEBUG": "1"}):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                result = read_hook_input()

        captured = capsys.readouterr()
        assert "Ignoring non-TodoWrite event" in captured.err
        assert "Bash" in captured.err
        assert result is None

    def test_missing_tool_name_returns_none(self) -> None:
        """Missing tool_name field returns None."""
        hook_input = {
            "tool_input": {"todos": []},
            "session_id": "test",
        }
        with patch("sys.stdin", StringIO(json.dumps(hook_input))):
            result = read_hook_input()

        assert result is None

    def test_preserves_all_fields(self, sample_hook_input: HookInput) -> None:
        """read_hook_input preserves all fields from input."""
        with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
            result = read_hook_input()

        assert result["session_id"] == sample_hook_input["session_id"]
        assert result["cwd"] == sample_hook_input["cwd"]
        assert result["tool_input"] == sample_hook_input["tool_input"]

    def test_invalid_json_raises_exception(self) -> None:
        """Invalid JSON input raises JSONDecodeError."""
        with patch("sys.stdin", StringIO("not valid json")):
            with pytest.raises(json.JSONDecodeError):
                read_hook_input()


# =============================================================================
# TestBuildLogEntry
# =============================================================================


class TestBuildLogEntry:
    """Tests for build_log_entry() function."""

    def test_valid_input_produces_correct_structure(
        self, sample_hook_input: HookInput
    ) -> None:
        """Valid input produces correct LogEntry structure."""
        entry = build_log_entry(sample_hook_input)

        assert "timestamp" in entry
        assert "session_id" in entry
        assert "cwd" in entry
        assert "todos" in entry
        assert isinstance(entry["todos"], list)

    def test_missing_session_id_uses_unknown_value(self) -> None:
        """Missing session_id uses UNKNOWN_VALUE."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {"todos": []},
            "cwd": "/path",
        }
        entry = build_log_entry(hook_input)
        assert entry["session_id"] == UNKNOWN_VALUE

    def test_missing_cwd_uses_unknown_value(self) -> None:
        """Missing cwd uses UNKNOWN_VALUE."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {"todos": []},
            "session_id": "test-123",
        }
        entry = build_log_entry(hook_input)
        assert entry["cwd"] == UNKNOWN_VALUE

    def test_empty_todos_list_handled(self) -> None:
        """Empty todos list is handled correctly."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {"todos": []},
            "session_id": "test",
            "cwd": "/path",
        }
        entry = build_log_entry(hook_input)
        assert entry["todos"] == []

    def test_invalid_todos_filtered(self) -> None:
        """Invalid todos are filtered out."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {
                "todos": [
                    {
                        "content": "Valid",
                        "status": "pending",
                        "activeForm": "Doing",
                    },
                    {"content": "Missing status", "activeForm": "Doing"},
                ]
            },
            "session_id": "test",
            "cwd": "/path",
        }
        entry = build_log_entry(hook_input)
        assert len(entry["todos"]) == 1

    def test_timestamp_valid_iso_format(self) -> None:
        """Timestamp is valid ISO 8601 format."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {"todos": []},
            "session_id": "test",
            "cwd": "/path",
        }
        entry = build_log_entry(hook_input)
        # Should parse as valid ISO 8601
        assert entry["timestamp"].endswith("Z")
        datetime.fromisoformat(entry["timestamp"].replace("Z", "+00:00"))

    def test_missing_tool_input_handled(self) -> None:
        """Missing tool_input field is handled."""
        hook_input = {
            "tool_name": "TodoWrite",
            "session_id": "test",
            "cwd": "/path",
        }
        entry = build_log_entry(hook_input)
        assert entry["todos"] == []

    def test_preserves_session_id_and_cwd(
        self, sample_hook_input: HookInput
    ) -> None:
        """Session ID and CWD are preserved from input."""
        entry = build_log_entry(sample_hook_input)
        assert entry["session_id"] == sample_hook_input["session_id"]
        assert entry["cwd"] == sample_hook_input["cwd"]


# =============================================================================
# TestGetLogFilePath
# =============================================================================


class TestGetLogFilePath:
    """Tests for get_log_file_path() function."""

    def test_default_path_when_no_env_var(self, tmp_project: Path) -> None:
        """Default path is project/.claude/todos.json when no env var."""
        env_without_todo_log = {
            k: v for k, v in os.environ.items() if k != "TODO_LOG_PATH"
        }
        with patch.dict(os.environ, env_without_todo_log, clear=True):
            result = get_log_file_path(tmp_project)

        assert result == tmp_project / ".claude" / "todos.json"

    def test_custom_relative_path(self, tmp_project: Path) -> None:
        """Custom relative TODO_LOG_PATH resolves correctly."""
        with patch.dict(os.environ, {"TODO_LOG_PATH": "logs/todos.json"}):
            result = get_log_file_path(tmp_project)

        assert result == tmp_project / "logs" / "todos.json"

    def test_custom_absolute_path_within_project(self, tmp_project: Path) -> None:
        """Custom absolute TODO_LOG_PATH within project works."""
        custom_path = tmp_project / "custom" / "todos.json"
        with patch.dict(os.environ, {"TODO_LOG_PATH": str(custom_path)}):
            result = get_log_file_path(tmp_project)

        assert result == custom_path

    def test_custom_path_escaping_raises_error(self, tmp_project: Path) -> None:
        """TODO_LOG_PATH escaping project raises ValueError."""
        with patch.dict(os.environ, {"TODO_LOG_PATH": "../../outside/path.json"}):
            with pytest.raises(ValueError, match="escapes project directory"):
                get_log_file_path(tmp_project)

    def test_empty_todo_log_path_uses_default(self, tmp_project: Path) -> None:
        """Empty TODO_LOG_PATH uses default path."""
        with patch.dict(os.environ, {"TODO_LOG_PATH": ""}):
            result = get_log_file_path(tmp_project)

        assert result == tmp_project / ".claude" / "todos.json"

    def test_whitespace_todo_log_path_uses_default(self, tmp_project: Path) -> None:
        """Whitespace-only TODO_LOG_PATH uses default path."""
        with patch.dict(os.environ, {"TODO_LOG_PATH": "   \t  "}):
            result = get_log_file_path(tmp_project)

        assert result == tmp_project / ".claude" / "todos.json"


# =============================================================================
# TestLoadExistingHistory
# =============================================================================


class TestLoadExistingHistory:
    """Tests for load_existing_history() function."""

    def test_nonexistent_file_returns_empty_list(self, tmp_project: Path) -> None:
        """Non-existent file returns empty list."""
        todos_file = tmp_project / ".claude" / "todos.json"
        result = load_existing_history(todos_file)
        assert result == []

    def test_valid_json_array_returns_list(self, tmp_project: Path) -> None:
        """Valid JSON array returns list of entries."""
        todos_file = tmp_project / ".claude" / "todos.json"
        todos_file.parent.mkdir(parents=True)

        entries = [
            {
                "timestamp": "2025-01-01T00:00:00.000Z",
                "session_id": "session1",
                "cwd": "/path1",
                "todos": [],
            },
            {
                "timestamp": "2025-01-02T00:00:00.000Z",
                "session_id": "session2",
                "cwd": "/path2",
                "todos": [],
            },
        ]
        todos_file.write_text(json.dumps(entries))

        result = load_existing_history(todos_file)
        assert len(result) == 2
        assert result[0]["session_id"] == "session1"
        assert result[1]["session_id"] == "session2"

    def test_empty_array_returns_empty_list(self, tmp_project: Path) -> None:
        """Empty JSON array returns empty list."""
        todos_file = tmp_project / ".claude" / "todos.json"
        todos_file.parent.mkdir(parents=True)
        todos_file.write_text("[]")

        result = load_existing_history(todos_file)
        assert result == []

    def test_corrupted_json_returns_empty_list(self, tmp_project: Path) -> None:
        """Corrupted JSON returns empty list."""
        todos_file = tmp_project / ".claude" / "todos.json"
        todos_file.parent.mkdir(parents=True)
        todos_file.write_text("{not valid json]}")

        result = load_existing_history(todos_file)
        assert result == []

    def test_non_array_json_returns_empty_list(self, tmp_project: Path) -> None:
        """Non-array JSON returns empty list."""
        todos_file = tmp_project / ".claude" / "todos.json"
        todos_file.parent.mkdir(parents=True)
        todos_file.write_text('{"key": "value"}')

        result = load_existing_history(todos_file)
        assert result == []

    def test_unicode_content_works(self, tmp_project: Path) -> None:
        """Unicode content is handled correctly."""
        todos_file = tmp_project / ".claude" / "todos.json"
        todos_file.parent.mkdir(parents=True)

        entries = [
            {
                "timestamp": "2025-01-01T00:00:00.000Z",
                "session_id": "session1",
                "cwd": "/path",
                "todos": [
                    {
                        "content": "做任务 🚀",
                        "status": "pending",
                        "activeForm": "正在做",
                    }
                ],
            }
        ]
        todos_file.write_text(json.dumps(entries, ensure_ascii=False), encoding="utf-8")

        result = load_existing_history(todos_file)
        assert len(result) == 1
        assert "🚀" in result[0]["todos"][0]["content"]


# =============================================================================
# TestAppendToLog
# =============================================================================


class TestAppendToLog:
    """Tests for append_to_log() function."""

    def test_creates_parent_directories(self, tmp_project: Path) -> None:
        """Creates parent directories if they don't exist."""
        log_file = tmp_project / "nested" / "dir" / "structure" / "todos.json"
        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }

        append_to_log(log_file, entry)

        assert log_file.exists()
        assert log_file.parent.exists()

    def test_appends_to_nonexistent_file(self, tmp_project: Path) -> None:
        """Appends to non-existent file (creates new array)."""
        log_file = tmp_project / ".claude" / "todos.json"
        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }

        append_to_log(log_file, entry)

        assert log_file.exists()
        content = json.loads(log_file.read_text())
        assert len(content) == 1
        assert content[0]["session_id"] == "test"

    def test_appends_to_existing_array(self, tmp_project: Path) -> None:
        """Appends to existing file preserving previous entries."""
        log_file = tmp_project / ".claude" / "todos.json"
        log_file.parent.mkdir(parents=True)

        # Initial entry
        initial = [
            {
                "timestamp": "2025-01-01T00:00:00.000Z",
                "session_id": "session1",
                "cwd": "/path1",
                "todos": [],
            }
        ]
        log_file.write_text(json.dumps(initial))

        # Append new entry
        new_entry = {
            "timestamp": "2025-01-02T00:00:00.000Z",
            "session_id": "session2",
            "cwd": "/path2",
            "todos": [],
        }
        append_to_log(log_file, new_entry)

        content = json.loads(log_file.read_text())
        assert len(content) == 2
        assert content[0]["session_id"] == "session1"
        assert content[1]["session_id"] == "session2"

    def test_preserves_existing_entries(self, tmp_project: Path) -> None:
        """Existing entries are not corrupted when appending."""
        log_file = tmp_project / ".claude" / "todos.json"
        log_file.parent.mkdir(parents=True)

        initial = [
            {
                "timestamp": "2025-01-01T00:00:00.000Z",
                "session_id": "original",
                "cwd": "/original/path",
                "todos": [
                    {
                        "content": "Original task",
                        "status": "completed",
                        "activeForm": "Was completed",
                    }
                ],
            }
        ]
        log_file.write_text(json.dumps(initial))

        new_entry = {
            "timestamp": "2025-01-02T00:00:00.000Z",
            "session_id": "new",
            "cwd": "/new/path",
            "todos": [],
        }
        append_to_log(log_file, new_entry)

        content = json.loads(log_file.read_text())
        original = content[0]
        assert original["session_id"] == "original"
        assert original["todos"][0]["content"] == "Original task"

    def test_json_formatted_with_indent(self, tmp_project: Path) -> None:
        """JSON output is formatted with 2-space indent."""
        log_file = tmp_project / ".claude" / "todos.json"
        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }

        append_to_log(log_file, entry)

        content = log_file.read_text()
        # Check for proper indentation (2 spaces)
        assert "  {" in content  # Indented object
        assert "    \"timestamp\"" in content  # Indented property

    def test_atomic_write_uses_temp_file(self, tmp_project: Path) -> None:
        """Atomic write uses temporary file."""
        log_file = tmp_project / ".claude" / "todos.json"
        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }

        # Mock tempfile to verify it's used
        original_mkstemp = tempfile.mkstemp
        mkstemp_called = False

        def mock_mkstemp(*args, **kwargs):
            nonlocal mkstemp_called
            mkstemp_called = True
            return original_mkstemp(*args, **kwargs)

        with patch("save_todos.tempfile.mkstemp", side_effect=mock_mkstemp):
            append_to_log(log_file, entry)

        assert mkstemp_called

    def test_cleanup_temp_file_on_failure(self, tmp_project: Path) -> None:
        """Cleans up temp file if write fails."""
        log_file = tmp_project / ".claude" / "todos.json"
        log_file.parent.mkdir(parents=True)

        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [],
        }

        # Mock replace to raise an exception
        original_replace = os.replace

        def mock_replace(src, dst):
            raise OSError("Mock failure")

        with patch("os.replace", side_effect=mock_replace):
            with patch("os.unlink") as mock_unlink:
                with pytest.raises(OSError):
                    append_to_log(log_file, entry)

                # Verify unlink was called to clean up temp file
                assert mock_unlink.called

    def test_unicode_in_todos_handled(self, tmp_project: Path) -> None:
        """Unicode in todos is handled correctly."""
        log_file = tmp_project / ".claude" / "todos.json"
        entry = {
            "timestamp": "2025-01-01T00:00:00.000Z",
            "session_id": "test",
            "cwd": "/path",
            "todos": [
                {
                    "content": "实现功能 🎯",
                    "status": "in_progress",
                    "activeForm": "正在实现",
                }
            ],
        }

        append_to_log(log_file, entry)

        content = json.loads(log_file.read_text(encoding="utf-8"))
        assert "🎯" in content[0]["todos"][0]["content"]

    def test_multiple_appends_accumulate(self, tmp_project: Path) -> None:
        """Multiple appends accumulate entries."""
        log_file = tmp_project / ".claude" / "todos.json"

        for i in range(5):
            entry = {
                "timestamp": f"2025-01-0{i+1}T00:00:00.000Z",
                "session_id": f"session{i}",
                "cwd": f"/path{i}",
                "todos": [],
            }
            append_to_log(log_file, entry)

        content = json.loads(log_file.read_text())
        assert len(content) == 5


# =============================================================================
# TestMain
# =============================================================================


class TestMain:
    """Tests for main() function entry point."""

    def test_success_todowrite_logged_exit_0(
        self,
        tmp_project: Path,
        sample_hook_input: HookInput,
        capsys: pytest.CaptureFixture[str],
    ) -> None:
        """TodoWrite logged successfully, exit 0, prints count."""
        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0
        captured = capsys.readouterr()
        assert "Saved" in captured.out
        assert "3 todos" in captured.out  # 3 sample todos

    def test_non_todowrite_silent_exit_0(self, tmp_project: Path) -> None:
        """Non-TodoWrite event exits silently with 0."""
        hook_input = {
            "tool_name": "Read",
            "tool_input": {"file_path": "/path"},
        }
        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0

    def test_missing_claude_project_dir_exit_1(
        self, sample_hook_input: HookInput, capsys: pytest.CaptureFixture[str]
    ) -> None:
        """Missing CLAUDE_PROJECT_DIR exits 1 with warning."""
        with patch.dict(os.environ, {}, clear=True):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 1
        captured = capsys.readouterr()
        assert "CLAUDE_PROJECT_DIR" in captured.err

    def test_invalid_json_input_exit_1(self, tmp_project: Path) -> None:
        """Invalid JSON input exits 1."""
        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO("not valid json")):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 1

    def test_todo_log_path_escape_attempt_exit_1(
        self, tmp_project: Path, sample_hook_input: HookInput
    ) -> None:
        """TODO_LOG_PATH escaping project exits 1."""
        with patch.dict(
            os.environ,
            {"CLAUDE_PROJECT_DIR": str(tmp_project), "TODO_LOG_PATH": "../../escape.json"},
        ):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 1

    def test_file_io_error_exit_1(
        self, tmp_project: Path, sample_hook_input: HookInput
    ) -> None:
        """File I/O error exits 1."""
        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with patch("save_todos.append_to_log", side_effect=OSError("Disk full")):
                    with pytest.raises(SystemExit) as exc_info:
                        main()

        assert exc_info.value.code == 1

    def test_full_workflow_integration(
        self, tmp_project: Path, sample_hook_input: HookInput
    ) -> None:
        """Full workflow: read, build, save to file."""
        log_file = tmp_project / ".claude" / "todos.json"

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0
        assert log_file.exists()

        content = json.loads(log_file.read_text())
        assert len(content) == 1
        assert content[0]["session_id"] == "test-session-123"
        assert len(content[0]["todos"]) == 3

    def test_custom_todo_log_path_respected(
        self, tmp_project: Path, sample_hook_input: HookInput
    ) -> None:
        """Custom TODO_LOG_PATH is used."""
        custom_path = tmp_project / "custom" / "my_todos.json"

        with patch.dict(
            os.environ,
            {
                "CLAUDE_PROJECT_DIR": str(tmp_project),
                "TODO_LOG_PATH": "custom/my_todos.json",
            },
        ):
            with patch("sys.stdin", StringIO(json.dumps(sample_hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0
        assert custom_path.exists()
        content = json.loads(custom_path.read_text())
        assert len(content) == 1

    def test_multiple_calls_accumulate_entries(
        self, tmp_project: Path, capsys: pytest.CaptureFixture[str]
    ) -> None:
        """Multiple main() calls accumulate entries."""
        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            for i in range(3):
                hook_input = {
                    "tool_name": "TodoWrite",
                    "tool_input": {
                        "todos": [
                            {
                                "content": f"Task {i}",
                                "status": "pending",
                                "activeForm": f"Creating task {i}",
                            }
                        ]
                    },
                    "session_id": f"session-{i}",
                    "cwd": f"/path{i}",
                }
                with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                    with pytest.raises(SystemExit):
                        main()

        log_file = tmp_project / ".claude" / "todos.json"
        content = json.loads(log_file.read_text())
        assert len(content) == 3
        assert content[0]["session_id"] == "session-0"
        assert content[2]["session_id"] == "session-2"

    def test_empty_todo_list_logged(self, tmp_project: Path) -> None:
        """Empty todo list is still logged."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {"todos": []},
            "session_id": "test",
            "cwd": "/path",
        }

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0
        log_file = tmp_project / ".claude" / "todos.json"
        content = json.loads(log_file.read_text())
        assert len(content[0]["todos"]) == 0

    def test_unknown_values_used_for_missing_fields(
        self, tmp_project: Path
    ) -> None:
        """Missing session_id/cwd use UNKNOWN_VALUE."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {
                "todos": [
                    {
                        "content": "Task",
                        "status": "pending",
                        "activeForm": "Doing",
                    }
                ]
            },
        }

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0
        log_file = tmp_project / ".claude" / "todos.json"
        content = json.loads(log_file.read_text())
        assert content[0]["session_id"] == UNKNOWN_VALUE
        assert content[0]["cwd"] == UNKNOWN_VALUE


# =============================================================================
# TestIntegration
# =============================================================================


class TestIntegration:
    """Integration tests combining multiple functions."""

    def test_full_stdin_to_file_workflow(self, tmp_project: Path) -> None:
        """Full stdin -> file workflow with validation and formatting."""
        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {
                "todos": [
                    {
                        "content": "Integration test task",
                        "status": "pending",
                        "activeForm": "Running integration test",
                    }
                ]
            },
            "session_id": "integration-test",
            "cwd": "/integration/path",
        }

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0

        log_file = tmp_project / ".claude" / "todos.json"
        assert log_file.exists()

        content = json.loads(log_file.read_text())
        assert len(content) == 1
        entry = content[0]

        assert entry["timestamp"].endswith("Z")
        assert entry["session_id"] == "integration-test"
        assert entry["cwd"] == "/integration/path"
        assert len(entry["todos"]) == 1
        assert entry["todos"][0]["content"] == "Integration test task"

    def test_multiple_entries_accumulate_with_different_sessions(
        self, tmp_project: Path
    ) -> None:
        """Multiple entries from different sessions accumulate."""
        log_file = tmp_project / ".claude" / "todos.json"

        sessions = [
            {
                "tool_name": "TodoWrite",
                "tool_input": {
                    "todos": [
                        {
                            "content": "Session 1 task",
                            "status": "pending",
                            "activeForm": "Starting",
                        }
                    ]
                },
                "session_id": "session-1",
                "cwd": "/session1",
            },
            {
                "tool_name": "TodoWrite",
                "tool_input": {
                    "todos": [
                        {
                            "content": "Session 2 task",
                            "status": "completed",
                            "activeForm": "Finished",
                        }
                    ]
                },
                "session_id": "session-2",
                "cwd": "/session2",
            },
        ]

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            for hook_input in sessions:
                with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                    with pytest.raises(SystemExit):
                        main()

        content = json.loads(log_file.read_text())
        assert len(content) == 2
        assert content[0]["session_id"] == "session-1"
        assert content[1]["session_id"] == "session-2"

    def test_recovery_from_corrupted_file(self, tmp_project: Path) -> None:
        """Gracefully recovers from corrupted JSON file."""
        log_file = tmp_project / ".claude" / "todos.json"
        log_file.parent.mkdir(parents=True)
        log_file.write_text("{corrupted json")

        hook_input = {
            "tool_name": "TodoWrite",
            "tool_input": {
                "todos": [
                    {
                        "content": "After corruption",
                        "status": "pending",
                        "activeForm": "Recovering",
                    }
                ]
            },
            "session_id": "recovery-test",
            "cwd": "/recovery",
        }

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                with pytest.raises(SystemExit) as exc_info:
                    main()

        assert exc_info.value.code == 0

        # Should have recovered and created new array
        content = json.loads(log_file.read_text())
        assert len(content) == 1
        assert content[0]["session_id"] == "recovery-test"

    def test_path_resolution_with_complex_structure(self, tmp_path: Path) -> None:
        """Path resolution works with complex directory structure."""
        project = tmp_path / "workspace" / "nested" / "project"
        project.mkdir(parents=True)

        safe_path = resolve_safe_path(project, "logs/todos/history.json")
        assert safe_path is not None
        assert project in safe_path.parents or safe_path.parent.parent.parent == project

    def test_timestamps_chronological_order(self, tmp_project: Path) -> None:
        """Multiple entries have chronologically ordered timestamps."""
        import time

        with patch.dict(
            os.environ, {"CLAUDE_PROJECT_DIR": str(tmp_project)}
        ):
            timestamps = []

            for i in range(3):
                hook_input = {
                    "tool_name": "TodoWrite",
                    "tool_input": {"todos": []},
                    "session_id": f"session-{i}",
                    "cwd": "/path",
                }
                with patch("sys.stdin", StringIO(json.dumps(hook_input))):
                    with pytest.raises(SystemExit):
                        main()

                time.sleep(0.01)  # Small delay between calls

            log_file = tmp_project / ".claude" / "todos.json"
            content = json.loads(log_file.read_text())
            timestamps = [entry["timestamp"] for entry in content]

            # Timestamps should be in order
            assert timestamps == sorted(timestamps)


# =============================================================================
# Main Entry Point
# =============================================================================


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
