// Package storage provides interfaces and types for todo log persistence.
//
// This package defines the core data structures and storage backend contracts
// used throughout the todo-log plugin. All storage backends must implement
// the StorageBackend interface to be compatible with the plugin.
package storage

// TaskItem represents a single task from a TaskCreate or TaskUpdate event.
//
// The JSON tags match the field names used by Claude Code's task tools.
type TaskItem struct {
	// ID is the task identifier (from TaskUpdate's taskId).
	ID string `json:"id,omitempty"`

	// Subject is the task title.
	Subject string `json:"subject"`

	// Description is the detailed task description.
	Description string `json:"description,omitempty"`

	// Status is the current status (e.g., "pending", "in_progress", "completed").
	Status string `json:"status"`

	// ActiveForm is the present continuous form shown in spinner (e.g., "Running tests").
	ActiveForm string `json:"activeForm,omitempty"`

	// Owner is the agent or user assigned to the task.
	Owner string `json:"owner,omitempty"`

	// Blocks lists task IDs that this task blocks.
	Blocks []string `json:"blocks,omitempty"`

	// BlockedBy lists task IDs that block this task.
	BlockedBy []string `json:"blockedBy,omitempty"`

	// Metadata is arbitrary key-value data attached to the task.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// LogEntry represents a timestamped record of a single task event from a hook invocation.
//
// Each entry captures one TaskCreate or TaskUpdate event.
type LogEntry struct {
	// Timestamp is an ISO 8601 UTC timestamp with Z suffix (e.g., "2025-11-14T10:30:45.123Z").
	Timestamp string `json:"timestamp"`

	// SessionID is a unique session identifier for the Claude Code session.
	SessionID string `json:"session_id"`

	// Cwd is the current working directory where the task tool was invoked.
	Cwd string `json:"cwd"`

	// ToolName is the tool that triggered this entry (e.g., "TaskCreate", "TaskUpdate").
	ToolName string `json:"tool_name"`

	// Task is the task data from this event.
	Task TaskItem `json:"task"`
}

// StorageBackend defines the contract for todo log persistence.
//
// All storage backends must implement these methods to be compatible
// with the todo-log plugin. Implementations should ensure atomicity
// when writing entries to prevent data corruption.
type StorageBackend interface {
	// LoadHistory loads all log entries from storage.
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

	// GetTasksByStatus retrieves all tasks with a specific status across all entries.
	//
	// Returns a slice of TaskItem objects with the matching status.
	// Returns an empty slice if no matches found.
	//
	// Returns an error if there's a storage access error.
	GetTasksByStatus(status string) ([]TaskItem, error)
}
