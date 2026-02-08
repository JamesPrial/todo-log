// Package hook handles parsing and processing of Claude Code hook events.
//
// This package is responsible for reading hook input from stdin, validating
// TodoWrite events, and filtering todo items according to the required schema.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// HookInput represents the JSON payload received from Claude Code on stdin.
//
// This structure matches the PostToolUse event format sent by Claude Code
// when a tool is invoked. The ToolInput field contains the raw JSON from
// the tool's parameters.
type HookInput struct {
	// ToolName is the name of the invoked tool (e.g., "TodoWrite").
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
// Returns (nil, nil) if the event is not a TodoWrite event (caller should exit 0).
// Returns (nil, err) if the JSON is malformed.
// Returns (*HookInput, nil) if valid TodoWrite event.
//
// When DEBUG env var is set, logs non-TodoWrite events to stderr.
func ReadHookInput(r io.Reader) (*HookInput, error) {
	var input HookInput

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode hook input: %w", err)
	}

	// Check if this is a TodoWrite event
	if input.ToolName != "TodoWrite" {
		// Log if DEBUG is enabled
		if os.Getenv("DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "Ignoring non-TodoWrite event: %s\n", input.ToolName)
		}
		return nil, nil
	}

	return &input, nil
}

// ValidateTodo checks whether a map has required keys: "content", "status", "activeForm".
//
// Returns true if all three keys are present (values can be any type).
// This ensures that todo items conform to the expected schema before being
// converted to TodoItem structs.
func ValidateTodo(item map[string]any) bool {
	_, hasContent := item["content"]
	_, hasStatus := item["status"]
	_, hasActiveForm := item["activeForm"]

	return hasContent && hasStatus && hasActiveForm
}

// ValidateTodos extracts and validates todos from raw tool_input JSON.
//
// Filters out items missing required keys. Returns empty slice if no valid todos.
// Non-string values are converted to strings using fmt.Sprintf.
// If rawToolInput is nil/empty or decode fails, returns an empty slice.
func ValidateTodos(rawToolInput json.RawMessage) []storage.TodoItem {
	// Handle nil or empty input
	if len(rawToolInput) == 0 {
		return make([]storage.TodoItem, 0)
	}

	// Decode the raw JSON into a structure with a todos field
	var input struct {
		Todos []map[string]any `json:"todos"`
	}

	if err := json.Unmarshal(rawToolInput, &input); err != nil {
		return make([]storage.TodoItem, 0)
	}

	// Filter and convert valid todos
	validTodos := make([]storage.TodoItem, 0)
	for _, item := range input.Todos {
		if !ValidateTodo(item) {
			continue
		}

		// Extract fields and convert to strings
		todo := storage.TodoItem{
			Content:    fmt.Sprintf("%v", item["content"]),
			Status:     fmt.Sprintf("%v", item["status"]),
			ActiveForm: fmt.Sprintf("%v", item["activeForm"]),
		}

		validTodos = append(validTodos, todo)
	}

	return validTodos
}
