# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-12-28

### Added
- 60+ unit tests for comprehensive coverage

### Changed
- Atomic file writes using temp file + `os.replace()` for crash safety
- Strict type safety with TypedDict and runtime validation

### Fixed
- Edge case validation in `resolve_safe_path()` for whitespace and null bytes
- Graceful recovery from corrupted JSON (starts fresh with empty array)

### Security
- Path traversal protection via `resolve_safe_path()` function
- Symlink validation prevents escaping target directory
- Null byte handling in path resolution
- Input validation for all external data

## [0.2.0] - 2025-12-27

### Added
- PostToolUse hook that logs TodoWrite activity
- Default log location: `.claude/todos.json`
- Configurable log path via `TODO_LOG_PATH` environment variable
- ISO 8601 timestamps for each log entry
- Session ID and working directory tracking
- Zero external dependencies (Python stdlib only)
