package hook

import (
	"time"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// UnknownValue is the fallback value used when session_id or cwd is missing.
const UnknownValue = "unknown"

// UTCISOTimestamp returns the current UTC time as ISO 8601 with millisecond precision.
//
// Format: "2006-01-02T15:04:05.000Z"
//
// This matches the timestamp format used in the Python version of the plugin
// and ensures consistency across log entries.
func UTCISOTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"
}

// BuildLogEntry constructs a LogEntry from the parsed hook input.
//
// Uses UnknownValue for empty session_id or cwd to ensure the log entry
// always has valid values.
//
// The returned LogEntry is ready to be passed to a storage backend's
// AppendEntry method.
func BuildLogEntry(input *HookInput) storage.LogEntry {
	sessionID := input.SessionID
	if sessionID == "" {
		sessionID = UnknownValue
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd = UnknownValue
	}

	// Extract task from the raw tool input
	task := ParseTaskInput(input.ToolName, input.ToolInput)

	return storage.LogEntry{
		Timestamp: UTCISOTimestamp(),
		SessionID: sessionID,
		Cwd:       cwd,
		ToolName:  input.ToolName,
		Task:      task,
	}
}
