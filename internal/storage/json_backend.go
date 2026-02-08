package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// JSONBackend implements StorageBackend using a JSON file.
//
// Stores all log entries in a single JSON file with atomic writes
// to prevent data corruption during write operations.
type JSONBackend struct {
	// LogFile is the absolute path to the JSON log file.
	LogFile string
}

// NewJSONBackend creates a new JSONBackend for the given file path.
//
// The logFile parameter should be an absolute path to the JSON log file.
// Parent directories will be created automatically when appending entries.
func NewJSONBackend(logFile string) *JSONBackend {
	return &JSONBackend{
		LogFile: logFile,
	}
}

// LoadHistory reads all log entries from the JSON file.
//
// Returns an empty slice if the file doesn't exist, is corrupted,
// contains invalid JSON, or contains non-array JSON.
// This graceful recovery behavior matches the Python implementation,
// allowing the backend to start fresh if data is corrupted.
//
// Never returns an error - any issues result in an empty slice.
func (b *JSONBackend) LoadHistory() ([]LogEntry, error) {
	// Check if file exists
	if _, err := os.Stat(b.LogFile); os.IsNotExist(err) {
		return make([]LogEntry, 0), nil
	}

	// Read file contents
	data, err := os.ReadFile(b.LogFile)
	if err != nil {
		// File exists but can't be read - start fresh
		return make([]LogEntry, 0), nil
	}

	// Parse JSON
	var entries []LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		// Invalid JSON or wrong type - start fresh
		return make([]LogEntry, 0), nil
	}

	// Handle nil result from unmarshal
	if entries == nil {
		return make([]LogEntry, 0), nil
	}

	// Normalize any entries with nil Todos to empty slice
	for i := range entries {
		if entries[i].Todos == nil {
			entries[i].Todos = make([]TodoItem, 0)
		}
	}

	return entries, nil
}

// AppendEntry atomically appends a new entry to the JSON log file.
//
// Creates parent directories if needed. Uses a temporary file and os.Rename
// for atomic replacement to ensure data consistency even if the process
// is interrupted. Writes JSON with 2-space indentation and a trailing newline.
//
// Returns an error if there's a file system error during directory creation,
// file writing, or atomic rename operation.
func (b *JSONBackend) AppendEntry(entry LogEntry) error {
	// Ensure Todos is never nil to produce [] not null in JSON
	if entry.Todos == nil {
		entry.Todos = make([]TodoItem, 0)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(b.LogFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Load existing history (gracefully handles missing/corrupt files)
	history, err := b.LoadHistory()
	if err != nil {
		// LoadHistory never returns an error in current implementation,
		// but handle it defensively for future changes
		return err
	}

	// Append new entry
	history = append(history, entry)

	// Marshal to JSON with 2-space indentation
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	// Add trailing newline
	data = append(data, '\n')

	// Create temporary file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Write data to temp file
	_, writeErr := tmpFile.Write(data)
	closeErr := tmpFile.Close()

	// Check for write or close errors
	if writeErr != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath) // Clean up temp file
		return closeErr
	}

	// Atomically rename temp file to target file
	if err := os.Rename(tmpPath, b.LogFile); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file on rename failure
		return err
	}

	return nil
}
