// Package storage provides interfaces and types for todo log persistence.
//
// This package defines the core data structures and storage backend contracts
// used throughout the todo-log plugin. All storage backends must implement
// the StorageBackend interface to be compatible with the plugin.
package storage

// TodoItem represents a single todo task from a TodoWrite event.
//
// The JSON tags use camelCase to match the existing Python/JSON format.
type TodoItem struct {
	// Content is the task description.
	Content string `json:"content"`

	// Status is the current status (e.g., "pending", "in_progress", "completed").
	Status string `json:"status"`

	// ActiveForm is the present continuous form of the action (e.g., "Doing task").
	ActiveForm string `json:"activeForm"`
}

// LogEntry represents a timestamped snapshot of todos from a single hook invocation.
//
// The JSON tags use snake_case for fields like session_id to match the existing
// Python/JSON format.
type LogEntry struct {
	// Timestamp is an ISO 8601 UTC timestamp with Z suffix (e.g., "2025-11-14T10:30:45.123Z").
	Timestamp string `json:"timestamp"`

	// SessionID is a unique session identifier for the Claude Code session.
	SessionID string `json:"session_id"`

	// Cwd is the current working directory where the TodoWrite was invoked.
	Cwd string `json:"cwd"`

	// Todos is the list of todo items in this entry.
	Todos []TodoItem `json:"todos"`
}

// StorageBackend defines the contract for todo log persistence.
//
// All storage backends must implement these methods to be compatible
// with the todo-log plugin. Implementations should ensure atomicity
// when writing entries to prevent data corruption.
type StorageBackend interface {
	// LoadHistory loads all todo log entries from storage.
	//
	// Returns a slice of all LogEntry objects in chronological order.
	// Returns an empty slice if no entries exist.
	//
	// Returns an error if there's a file system error reading the storage
	// or if the stored data is corrupted or invalid.
	LoadHistory() ([]LogEntry, error)

	// AppendEntry atomically appends a new entry to the log.
	//
	// Appends the given entry to the end of the log while maintaining
	// atomicity to prevent data corruption in case of failure.
	//
	// Returns an error if there's a file system error writing the storage
	// or if the entry is invalid or cannot be serialized.
	AppendEntry(entry LogEntry) error
}

// QueryableStorageBackend extends StorageBackend with query capabilities.
//
// This interface provides additional query methods for advanced use cases.
// Only the SQLite backend implements this interface. Backends that don't
// support querying can implement only the base StorageBackend interface.
type QueryableStorageBackend interface {
	StorageBackend

	// GetEntriesBySession retrieves all entries for a specific session.
	//
	// Returns a slice of LogEntry objects matching the session_id,
	// in chronological order. Returns an empty slice if no matches found.
	//
	// Returns an error if there's a storage access error.
	GetEntriesBySession(sessionID string) ([]LogEntry, error)

	// GetTodosByStatus retrieves all todos with a specific status across all entries.
	//
	// Returns a slice of TodoItem objects with the matching status.
	// Returns an empty slice if no matches found.
	//
	// Returns an error if there's a storage access error.
	GetTodosByStatus(status string) ([]TodoItem, error)
}
