package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// postgresSchemaDDL defines the database schema for the PostgreSQL backend.
//
// Uses a single log_entries table with embedded task fields.
// Array fields (blocks, blocked_by) and metadata are stored as JSONB.
const postgresSchemaDDL = `
CREATE TABLE IF NOT EXISTS log_entries (
    id BIGSERIAL PRIMARY KEY,
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
    blocks JSONB NOT NULL DEFAULT '[]'::jsonb,
    blocked_by JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_entries_session ON log_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_entries_status ON log_entries(status);
`

// PostgresBackend implements both StorageBackend and QueryableStorageBackend using PostgreSQL.
//
// This backend provides a denormalized storage model with query capabilities.
// Each log entry contains the full task data inline. Uses JSONB for efficient
// storage and querying of array and map fields.
type PostgresBackend struct {
	// ConnString is the PostgreSQL connection string (e.g., "postgres://user:pass@host:5432/dbname").
	ConnString string
}

// NewPostgresBackend creates a new PostgresBackend and initializes the database schema.
//
// The connString parameter should be a valid PostgreSQL connection string.
// The database schema is initialized immediately; returns an error if schema
// creation fails.
func NewPostgresBackend(connString string) (*PostgresBackend, error) {
	backend := &PostgresBackend{
		ConnString: connString,
	}

	if err := backend.ensureSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return backend, nil
}

// connect opens a new database connection using pgx.
//
// Returns an error if connection fails.
func (b *PostgresBackend) connect(ctx context.Context) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, b.ConnString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}

// ensureSchema creates the database schema if it doesn't exist.
//
// Uses CREATE TABLE IF NOT EXISTS and CREATE INDEX IF NOT EXISTS to safely
// initialize the schema on first use without affecting existing databases.
//
// Returns an error if connection or schema creation fails.
func (b *PostgresBackend) ensureSchema() error {
	ctx := context.Background()
	conn, err := b.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()

	if _, err := conn.Exec(ctx, postgresSchemaDDL); err != nil {
		return fmt.Errorf("failed to execute schema DDL: %w", err)
	}

	return nil
}

// LoadHistory loads all log entries from the database.
//
// Returns entries in chronological order (by log entry ID).
//
// Returns an error if database connection or query execution fails.
func (b *PostgresBackend) LoadHistory() ([]LogEntry, error) {
	ctx := context.Background()
	conn, err := b.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(ctx) }()

	query := `
		SELECT timestamp, session_id, cwd, tool_name,
		       task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		ORDER BY id
	`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer rows.Close()

	result := make([]LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		var blocksJSON, blockedByJSON, metadataJSON []byte

		if err := rows.Scan(
			&entry.Timestamp, &entry.SessionID, &entry.Cwd, &entry.ToolName,
			&entry.Task.ID, &entry.Task.Subject, &entry.Task.Description,
			&entry.Task.Status, &entry.Task.ActiveForm, &entry.Task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse JSONB array fields
		if len(blocksJSON) > 0 && string(blocksJSON) != "[]" {
			_ = json.Unmarshal(blocksJSON, &entry.Task.Blocks)
		}
		if len(blockedByJSON) > 0 && string(blockedByJSON) != "[]" {
			_ = json.Unmarshal(blockedByJSON, &entry.Task.BlockedBy)
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "{}" {
			_ = json.Unmarshal(metadataJSON, &entry.Task.Metadata)
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
// Serializes array and map fields as JSONB for storage.
//
// Returns an error if connection or insert fails.
func (b *PostgresBackend) AppendEntry(entry LogEntry) error {
	ctx := context.Background()
	conn, err := b.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()

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

	_, err = conn.Exec(ctx,
		`INSERT INTO log_entries (timestamp, session_id, cwd, tool_name,
		    task_id, subject, description, status, active_form, owner,
		    blocks, blocked_by, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb, $12::jsonb, $13::jsonb)`,
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
func (b *PostgresBackend) GetEntriesBySession(sessionID string) ([]LogEntry, error) {
	ctx := context.Background()
	conn, err := b.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(ctx) }()

	query := `
		SELECT timestamp, session_id, cwd, tool_name,
		       task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		WHERE session_id = $1
		ORDER BY id
	`

	rows, err := conn.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries by session: %w", err)
	}
	defer rows.Close()

	result := make([]LogEntry, 0)
	for rows.Next() {
		var entry LogEntry
		var blocksJSON, blockedByJSON, metadataJSON []byte

		if err := rows.Scan(
			&entry.Timestamp, &entry.SessionID, &entry.Cwd, &entry.ToolName,
			&entry.Task.ID, &entry.Task.Subject, &entry.Task.Description,
			&entry.Task.Status, &entry.Task.ActiveForm, &entry.Task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if len(blocksJSON) > 0 && string(blocksJSON) != "[]" {
			_ = json.Unmarshal(blocksJSON, &entry.Task.Blocks)
		}
		if len(blockedByJSON) > 0 && string(blockedByJSON) != "[]" {
			_ = json.Unmarshal(blockedByJSON, &entry.Task.BlockedBy)
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "{}" {
			_ = json.Unmarshal(metadataJSON, &entry.Task.Metadata)
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
func (b *PostgresBackend) GetTasksByStatus(status string) ([]TaskItem, error) {
	ctx := context.Background()
	conn, err := b.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close(ctx) }()

	query := `
		SELECT task_id, subject, description, status, active_form, owner,
		       blocks, blocked_by, metadata
		FROM log_entries
		WHERE status = $1
		ORDER BY id
	`

	rows, err := conn.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks by status: %w", err)
	}
	defer rows.Close()

	result := make([]TaskItem, 0)
	for rows.Next() {
		var task TaskItem
		var blocksJSON, blockedByJSON, metadataJSON []byte

		if err := rows.Scan(
			&task.ID, &task.Subject, &task.Description,
			&task.Status, &task.ActiveForm, &task.Owner,
			&blocksJSON, &blockedByJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		if len(blocksJSON) > 0 && string(blocksJSON) != "[]" {
			_ = json.Unmarshal(blocksJSON, &task.Blocks)
		}
		if len(blockedByJSON) > 0 && string(blockedByJSON) != "[]" {
			_ = json.Unmarshal(blockedByJSON, &task.BlockedBy)
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "{}" {
			_ = json.Unmarshal(metadataJSON, &task.Metadata)
		}

		result = append(result, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}
