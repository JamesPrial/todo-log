package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // register sqlite driver
)

// schemaDDL defines the database schema for the SQLite backend.
//
// This schema matches the Python implementation exactly, using:
// - log_entries table for session metadata (timestamp, session_id, cwd)
// - todos table for individual todo items linked via entry_id foreign key
// - Indexes on session_id and status for efficient querying
const schemaDDL = `
CREATE TABLE IF NOT EXISTS log_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL,
    session_id TEXT NOT NULL,
    cwd TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS todos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    active_form TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES log_entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_entries_session ON log_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
`

// SQLiteBackend implements both StorageBackend and QueryableStorageBackend using SQLite.
//
// This backend provides a normalized relational storage model with query capabilities.
// Each method opens and closes its own database connection for simplicity, matching
// the Python implementation. Uses WAL mode for better concurrent access and foreign
// keys for referential integrity.
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
// - Foreign key constraints enabled for referential integrity
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

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
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
// Returns entries in chronological order (by log entry ID). Each entry's Todos
// slice is initialized as an empty slice (never nil), even for entries with no
// associated todos (e.g., from LEFT JOIN results).
//
// Returns an error if database connection or query execution fails.
func (b *SQLiteBackend) LoadHistory() ([]LogEntry, error) {
	db, err := b.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := `
		SELECT e.id, e.timestamp, e.session_id, e.cwd, t.content, t.status, t.active_form
		FROM log_entries e
		LEFT JOIN todos t ON e.id = t.entry_id
		ORDER BY e.id, t.id
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Use ordered map pattern to group todos by entry
	entryMap := make(map[int64]*LogEntry)
	entryOrder := make([]int64, 0)

	for rows.Next() {
		var (
			entryID    int64
			timestamp  string
			sessionID  string
			cwd        string
			content    sql.NullString
			status     sql.NullString
			activeForm sql.NullString
		)

		if err := rows.Scan(&entryID, &timestamp, &sessionID, &cwd, &content, &status, &activeForm); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create entry if first time seeing this entryID
		entry, exists := entryMap[entryID]
		if !exists {
			entry = &LogEntry{
				Timestamp: timestamp,
				SessionID: sessionID,
				Cwd:       cwd,
				Todos:     make([]TodoItem, 0),
			}
			entryMap[entryID] = entry
			entryOrder = append(entryOrder, entryID)
		}

		// Add todo if present (LEFT JOIN can produce NULL todo fields)
		if content.Valid {
			todo := TodoItem{
				Content:    content.String,
				Status:     status.String,
				ActiveForm: activeForm.String,
			}
			entry.Todos = append(entry.Todos, todo)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Build result in original order
	result := make([]LogEntry, 0, len(entryOrder))
	for _, id := range entryOrder {
		result = append(result, *entryMap[id])
	}

	return result, nil
}

// AppendEntry atomically appends a new entry to the database.
//
// Inserts the log entry and all associated todos in a single transaction.
// Ensures entry.Todos is never nil (uses empty slice if nil).
//
// Returns an error if connection, transaction, insert, or commit fails.
// Automatically rolls back the transaction on any error.
func (b *SQLiteBackend) AppendEntry(entry LogEntry) error {
	// Ensure Todos is never nil
	if entry.Todos == nil {
		entry.Todos = make([]TodoItem, 0)
	}

	db, err := b.connect()
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert log entry
	result, err := tx.Exec(
		"INSERT INTO log_entries (timestamp, session_id, cwd) VALUES (?, ?, ?)",
		entry.Timestamp, entry.SessionID, entry.Cwd,
	)
	if err != nil {
		return fmt.Errorf("failed to insert log entry: %w", err)
	}

	// Get the entry ID
	entryID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	// Insert todos
	for _, todo := range entry.Todos {
		_, err := tx.Exec(
			"INSERT INTO todos (entry_id, content, status, active_form) VALUES (?, ?, ?, ?)",
			entryID, todo.Content, todo.Status, todo.ActiveForm,
		)
		if err != nil {
			return fmt.Errorf("failed to insert todo: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEntriesBySession retrieves all log entries for a specific session.
//
// Returns entries in chronological order (by log entry ID) that match the
// given session_id. Each entry's Todos slice is initialized as an empty
// slice (never nil), even for entries with no associated todos.
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
		SELECT e.id, e.timestamp, e.session_id, e.cwd, t.content, t.status, t.active_form
		FROM log_entries e
		LEFT JOIN todos t ON e.id = t.entry_id
		WHERE e.session_id = ?
		ORDER BY e.id, t.id
	`

	rows, err := db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries by session: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Use ordered map pattern to group todos by entry
	entryMap := make(map[int64]*LogEntry)
	entryOrder := make([]int64, 0)

	for rows.Next() {
		var (
			entryID         int64
			timestamp       string
			sessionIDResult string
			cwd             string
			content         sql.NullString
			status          sql.NullString
			activeForm      sql.NullString
		)

		if err := rows.Scan(&entryID, &timestamp, &sessionIDResult, &cwd, &content, &status, &activeForm); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Create entry if first time seeing this entryID
		entry, exists := entryMap[entryID]
		if !exists {
			entry = &LogEntry{
				Timestamp: timestamp,
				SessionID: sessionIDResult,
				Cwd:       cwd,
				Todos:     make([]TodoItem, 0),
			}
			entryMap[entryID] = entry
			entryOrder = append(entryOrder, entryID)
		}

		// Add todo if present (LEFT JOIN can produce NULL todo fields)
		if content.Valid {
			todo := TodoItem{
				Content:    content.String,
				Status:     status.String,
				ActiveForm: activeForm.String,
			}
			entry.Todos = append(entry.Todos, todo)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Build result in original order
	result := make([]LogEntry, 0, len(entryOrder))
	for _, id := range entryOrder {
		result = append(result, *entryMap[id])
	}

	return result, nil
}

// GetTodosByStatus retrieves all todos with a specific status across all entries.
//
// Returns todos in order by their ID that match the given status.
// Returns an empty slice if no todos match the status.
//
// Returns an error if database connection or query execution fails.
func (b *SQLiteBackend) GetTodosByStatus(status string) ([]TodoItem, error) {
	db, err := b.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	query := `
		SELECT content, status, active_form
		FROM todos
		WHERE status = ?
		ORDER BY id
	`

	rows, err := db.Query(query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query todos by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]TodoItem, 0)
	for rows.Next() {
		var todo TodoItem
		if err := rows.Scan(&todo.Content, &todo.Status, &todo.ActiveForm); err != nil {
			return nil, fmt.Errorf("failed to scan todo: %w", err)
		}
		result = append(result, todo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}
