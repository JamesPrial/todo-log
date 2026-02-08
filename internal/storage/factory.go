package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JamesPrial/todo-log/internal/pathutil"
)

// GetStorageBackend returns the configured storage backend based on environment variables.
//
// Environment variables:
//   - TODO_STORAGE_BACKEND: "json" (default) or "sqlite"
//   - TODO_LOG_PATH: custom JSON log path (default: <projectDir>/.claude/todos.json)
//   - TODO_SQLITE_PATH: custom SQLite path (default: <projectDir>/.claude/todos.db)
//
// Returns error if backend type is unknown or custom path escapes projectDir.
func GetStorageBackend(projectDir string) (StorageBackend, error) {
	// Read backend type from environment, default to "json"
	backendType := strings.ToLower(strings.TrimSpace(os.Getenv("TODO_STORAGE_BACKEND")))
	if backendType == "" {
		backendType = "json"
	}

	switch backendType {
	case "json":
		path, err := getJSONPath(projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to determine JSON log path: %w", err)
		}
		return NewJSONBackend(path), nil

	case "sqlite":
		path, err := getSQLitePath(projectDir)
		if err != nil {
			return nil, fmt.Errorf("failed to determine SQLite database path: %w", err)
		}
		return NewSQLiteBackend(path)

	default:
		return nil, fmt.Errorf("unknown storage backend: %q. Expected 'json' or 'sqlite'", backendType)
	}
}

// getJSONPath returns the JSON log file path.
//
// Reads TODO_LOG_PATH environment variable. If set, validates the path using
// pathutil.ResolveSafePath to ensure it stays within projectDir. If not set,
// returns the default path: <projectDir>/.claude/todos.json.
//
// Returns an error if the custom path escapes projectDir.
func getJSONPath(projectDir string) (string, error) {
	customPath := strings.TrimSpace(os.Getenv("TODO_LOG_PATH"))
	if customPath != "" {
		// Validate custom path to prevent directory traversal
		safePath, err := pathutil.ResolveSafePath(projectDir, customPath)
		if err != nil {
			return "", fmt.Errorf("invalid TODO_LOG_PATH: %w", err)
		}
		return safePath, nil
	}

	// Use default path
	return filepath.Join(projectDir, ".claude", "todos.json"), nil
}

// getSQLitePath returns the SQLite database file path.
//
// Reads TODO_SQLITE_PATH environment variable. If set, validates the path using
// pathutil.ResolveSafePath to ensure it stays within projectDir. If not set,
// returns the default path: <projectDir>/.claude/todos.db.
//
// Returns an error if the custom path escapes projectDir.
func getSQLitePath(projectDir string) (string, error) {
	customPath := strings.TrimSpace(os.Getenv("TODO_SQLITE_PATH"))
	if customPath != "" {
		// Validate custom path to prevent directory traversal
		safePath, err := pathutil.ResolveSafePath(projectDir, customPath)
		if err != nil {
			return "", fmt.Errorf("invalid TODO_SQLITE_PATH: %w", err)
		}
		return safePath, nil
	}

	// Use default path
	return filepath.Join(projectDir, ".claude", "todos.db"), nil
}
