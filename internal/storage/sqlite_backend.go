package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // register sqlite driver
)

// schemaDDL defines the database schema for the SQLite backend.
//
// Uses a single log_entries table with embedded task fields.
// Array fields (blocks, blocked_by) and metadata are stored as JSON text.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    session_id TEXT NOT NULL,
    cwd TEXT NOT NULL,
    tool_name TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    active_form TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    blocks TEXT NOT NULL DEFAULT '[]',
    blocked_by TEXT NOT NULL DEFAULT '[]',
    metadata TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_entries_session ON log_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON log_entries(status);
`

// SQLiteBackend implements both StorageBackend and QueryableStorageBackend using SQLite.
//
// This backend provides a denormalized storage model with query capabilities.
// Each log entry contains the full task data inline. Uses WAL mode for
// better concurrent access.
type SQLiteBackend struct {
	// DBPath is the absolute path to the SQLite database file.
	DBPath string
}

// NewSQLiteBackend creates a new SQLiteBackend and initializes the database schema.
//
// The dbPath parameter should be an absolute path to the SQLite database file.
// Parent directories will be created automatically if they don't exist.
// The database schema is initialized immediately; returns an error if schema
// creation fails.
func NewSQLiteBackend(dbPath string) (*SQLiteBackend, error) {
	backend := &SQLiteBackend{
		DBPath: dbPath,
	}

	if err := backend.ensureSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return backend, nil
}

// connect opens a new database connection with WAL mode and foreign keys enabled.
//
// Creates parent directories if needed. Configures the connection for:
// - WAL (Write-Ahead Logging) journal mode for better concurrent access
//
// Returns an error if directory creation, database opening, or pragma execution fails.
func (b *SQLiteBackend) connect() (*sql.DB, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(b.DBPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", b.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	return db, nil
}

// ensureSchema creates the database schema if it doesn't exist.
//
// Uses CREATE TABLE IF NOT EXISTS and CREATE INDEX IF NOT EXISTS to safely
// initialize the schema on first use without affecting existing databases.
//
// Returns an error if connection or schema creation fails.
func (b *SQLiteBackend) ensureSchema() error {
	db, err := b.connect()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("failed to execute schema DDL: %w", err)
	}

	return nil
}

// LoadHistory loads all log entries from the database.
//
// Returns entries in chronological order (by log entry ID).
//
// Returns an error if database connection or query execution fails.
func (b *SQLiteBackend) LoadHistory() ([]LogEntry, error) {
	db, err := b.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := `
		SELECT timestamp, session_id, cwd, tool_name,
		       task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		ORDER BY id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		var blocksJSON, blockedByJSON, metadataJSON string

		if err := rows.Scan(
			&entry.Timestamp, &entry.SessionID, &entry.Cwd, &entry.ToolName,
			&entry.Task.ID, &entry.Task.Subject, &entry.Task.Description,
			&entry.Task.Status, &entry.Task.ActiveForm, &entry.Task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse JSON array fields
		if blocksJSON != "[]" && blocksJSON != "" {
			_ = json.Unmarshal([]byte(blocksJSON), &entry.Task.Blocks)
		}
		if blockedByJSON != "[]" && blockedByJSON != "" {
			_ = json.Unmarshal([]byte(blockedByJSON), &entry.Task.BlockedBy)
		}
		if metadataJSON != "{}" && metadataJSON != "" {
			_ = json.Unmarshal([]byte(metadataJSON), &entry.Task.Metadata)
		}

		result = append(result, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// AppendEntry atomically appends a new entry to the database.
//
// Serializes array and map fields as JSON text for storage.
//
// Returns an error if connection or insert fails.
func (b *SQLiteBackend) AppendEntry(entry LogEntry) error {
	db, err := b.connect()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Serialize array/map fields to JSON
	blocksJSON := "[]"
	if len(entry.Task.Blocks) > 0 {
		if data, err := json.Marshal(entry.Task.Blocks); err == nil {
			blocksJSON = string(data)
		}
	}

	blockedByJSON := "[]"
	if len(entry.Task.BlockedBy) > 0 {
		if data, err := json.Marshal(entry.Task.BlockedBy); err == nil {
			blockedByJSON = string(data)
		}
	}

	metadataJSON := "{}"
	if len(entry.Task.Metadata) > 0 {
		if data, err := json.Marshal(entry.Task.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}

	_, err = db.Exec(
		`INSERT INTO log_entries (timestamp, session_id, cwd, tool_name,
		    task_id, subject, description, status, active_form, owner,
		    blocks, blocked_by, metadata)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.Timestamp, entry.SessionID, entry.Cwd, entry.ToolName,
		entry.Task.ID, entry.Task.Subject, entry.Task.Description,
		entry.Task.Status, entry.Task.ActiveForm, entry.Task.Owner,
		blocksJSON, blockedByJSON, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %w", err)
	}

	return nil
}

// GetEntriesBySession retrieves all log entries for a specific session.
//
// Returns entries in chronological order (by log entry ID) that match the
// given session_id.
//
// Returns an empty slice if no entries match the session_id.
// Returns an error if database connection or query execution fails.
func (b *SQLiteBackend) GetEntriesBySession(sessionID string) ([]LogEntry, error) {
	db, err := b.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := `
		SELECT timestamp, session_id, cwd, tool_name,
		       task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		WHERE session_id = ?
		ORDER BY id
	`

	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries by session: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		var blocksJSON, blockedByJSON, metadataJSON string

		if err := rows.Scan(
			&entry.Timestamp, &entry.SessionID, &entry.Cwd, &entry.ToolName,
			&entry.Task.ID, &entry.Task.Subject, &entry.Task.Description,
			&entry.Task.Status, &entry.Task.ActiveForm, &entry.Task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if blocksJSON != "[]" && blocksJSON != "" {
			_ = json.Unmarshal([]byte(blocksJSON), &entry.Task.Blocks)
		}
		if blockedByJSON != "[]" && blockedByJSON != "" {
			_ = json.Unmarshal([]byte(blockedByJSON), &entry.Task.BlockedBy)
		}
		if metadataJSON != "{}" && metadataJSON != "" {
			_ = json.Unmarshal([]byte(metadataJSON), &entry.Task.Metadata)
		}

		result = append(result, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// GetTasksByStatus retrieves all tasks with a specific status across all entries.
//
// Returns tasks in order by their entry ID that match the given status.
// Returns an empty slice if no tasks match the status.
//
// Returns an error if database connection or query execution fails.
func (b *SQLiteBackend) GetTasksByStatus(status string) ([]TaskItem, error) {
	db, err := b.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := `
		SELECT task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		WHERE status = ?
		ORDER BY id
	`

	rows, err := db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]TaskItem, 0)
	for rows.Next() {
		var task TaskItem
		var blocksJSON, blockedByJSON, metadataJSON string

		if err := rows.Scan(
			&task.ID, &task.Subject, &task.Description,
			&task.Status, &task.ActiveForm, &task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if blocksJSON != "[]" && blocksJSON != "" {
			_ = json.Unmarshal([]byte(blocksJSON), &task.Blocks)
		}
		if blockedByJSON != "[]" && blockedByJSON != "" {
			_ = json.Unmarshal([]byte(blockedByJSON), &task.BlockedBy)
		}
		if metadataJSON != "{}" && metadataJSON != "" {
			_ = json.Unmarshal([]byte(metadataJSON), &task.Metadata)
		}

		result = append(result, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}
