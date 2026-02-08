// Package main implements the save-todos hook for Claude Code's todo-log plugin.
//
// This program reads PostToolUse hook events from stdin (JSON format), validates
// TaskCreate/TaskUpdate events, and appends them to the configured storage backend.
//
// Exit codes:
//   - 0: Success (task saved or non-task event ignored)
//   - 1: Error (invalid input, missing environment variable, storage failure)
//
// Environment variables:
//   - CLAUDE_PROJECT_DIR: Required. The project root directory for path resolution.
//   - TODO_STORAGE_BACKEND: Optional. Backend type: "json" (default) or "sqlite".
//   - TODO_LOG_PATH: Optional. Custom path for JSON log file.
//   - TODO_SQLITE_PATH: Optional. Custom path for SQLite database.
//   - DEBUG: Optional. Enable debug logging to stderr.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JamesPrial/todo-log/internal/hook"
	"github.com/JamesPrial/todo-log/internal/storage"
)

// run contains the main logic, returning an exit code.
//
// Accepts an io.Reader for stdin to enable testing without modifying global state.
//
// Process flow:
//  1. Read and parse hook input from stdin
//  2. Return 0 if not a TaskCreate/TaskUpdate event (silently ignore)
//  3. Get CLAUDE_PROJECT_DIR environment variable
//  4. Build log entry from validated hook input
//  5. Get storage backend (JSON or SQLite)
//  6. Append entry to storage
//  7. Print success message and return 0
//
// Error handling:
//   - All errors are printed to stderr with "Error saving todos: " prefix
//   - Returns exit code 1 on any error
func run(stdin io.Reader) int {
	// Step 1: Read and parse hook input from stdin
	input, err := hook.ReadHookInput(stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving todos: %v\n", err)
		return 1
	}

	// Step 2: If not a task event, silently ignore (return 0)
	if input == nil {
		return 0
	}

	// Step 3: Get CLAUDE_PROJECT_DIR environment variable
	projectDir := strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
	if projectDir == "" {
		fmt.Fprintln(os.Stderr, "Warning: CLAUDE_PROJECT_DIR not set")
		return 1
	}

	// Step 4: Build log entry from validated hook input
	entry := hook.BuildLogEntry(input)

	// Step 5: Get storage backend (JSON or SQLite)
	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error saving todos: %v\n", err)
		return 1
	}

	// Step 6: Append entry to storage
	if err := backend.AppendEntry(entry); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving todos: %v\n", err)
		return 1
	}

	// Step 7: Print success message and return 0
	backendType := strings.ToLower(strings.TrimSpace(os.Getenv("TODO_STORAGE_BACKEND")))
	if backendType == "" {
		backendType = "json"
	}

	fmt.Printf("Saved %s task (%s backend)\n", input.ToolName, backendType)
	return 0
}

func main() {
	os.Exit(run(os.Stdin))
}
