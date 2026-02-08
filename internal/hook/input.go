// Package hook handles parsing and processing of Claude Code hook events.
//
// This package is responsible for reading hook input from stdin, validating
// TaskCreate/TaskUpdate events, and extracting task data from tool input.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// acceptedTools is the set of tool names this hook processes.
var acceptedTools = map[string]bool{
	"TaskCreate": true,
	"TaskUpdate": true,
}

// HookInput represents the JSON payload received from Claude Code on stdin.
//
// This structure matches the PostToolUse event format sent by Claude Code
// when a tool is invoked. The ToolInput field contains the raw JSON from
// the tool's parameters.
type HookInput struct {
	// ToolName is the name of the invoked tool (e.g., "TaskCreate", "TaskUpdate").
	ToolName string `json:"tool_name"`

	// ToolInput is the raw JSON of the tool's input parameters.
	ToolInput json.RawMessage `json:"tool_input"`

	// SessionID is the unique identifier for the Claude Code session.
	SessionID string `json:"session_id"`

	// Cwd is the current working directory when the tool was invoked.
	Cwd string `json:"cwd"`
}

// ReadHookInput reads and parses JSON from the given reader.
//
// Returns (nil, nil) if the event is not a TaskCreate or TaskUpdate event (caller should exit 0).
// Returns (nil, err) if the JSON is malformed.
// Returns (*HookInput, nil) if valid task event.
//
// When DEBUG env var is set, logs non-task events to stderr.
func ReadHookInput(r io.Reader) (*HookInput, error) {
	var input HookInput

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode hook input: %w", err)
	}

	// Check if this is a task event we care about
	if !acceptedTools[input.ToolName] {
		// Log if DEBUG is enabled
		if os.Getenv("DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "Ignoring non-task event: %s\n", input.ToolName)
		}
		return nil, nil
	}

	return &input, nil
}

// ParseTaskInput extracts a TaskItem from raw tool_input JSON based on the tool name.
//
// TaskCreate input format: {"subject":"...","description":"...","activeForm":"..."}
//   - Status defaults to "pending"
//
// TaskUpdate input format: {"taskId":"1","status":"completed","subject":"...","addBlocks":["2"],...}
//   - Maps taskId to ID, addBlocks to Blocks, addBlockedBy to BlockedBy
//
// Returns a zero-value TaskItem with status "pending" if raw input is nil or empty.
func ParseTaskInput(toolName string, raw json.RawMessage) storage.TaskItem {
	if len(raw) == 0 {
		return storage.TaskItem{Status: "pending"}
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return storage.TaskItem{Status: "pending"}
	}

	task := storage.TaskItem{}

	// Common fields
	if v, ok := fields["subject"].(string); ok {
		task.Subject = v
	}
	if v, ok := fields["description"].(string); ok {
		task.Description = v
	}
	if v, ok := fields["activeForm"].(string); ok {
		task.ActiveForm = v
	}
	if v, ok := fields["status"].(string); ok {
		task.Status = v
	}
	if v, ok := fields["owner"].(string); ok {
		task.Owner = v
	}

	// TaskUpdate-specific: taskId maps to ID
	if v, ok := fields["taskId"].(string); ok {
		task.ID = v
	}

	// Array fields
	if v, ok := fields["addBlocks"]; ok {
		task.Blocks = toStringSlice(v)
	}
	if v, ok := fields["addBlockedBy"]; ok {
		task.BlockedBy = toStringSlice(v)
	}

	// Metadata
	if v, ok := fields["metadata"].(map[string]any); ok {
		task.Metadata = v
	}

	// Default status for TaskCreate
	if toolName == "TaskCreate" && task.Status == "" {
		task.Status = "pending"
	}

	return task
}

// toStringSlice converts an interface{} (expected to be []interface{}) to []string.
// Returns nil if the input is not a valid string slice.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
