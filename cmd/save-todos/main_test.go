package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// validTodoWriteJSON returns a well-formed TodoWrite JSON payload.
func validTodoWriteJSON() string {
	return `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"Test task","status":"pending","activeForm":"Testing"}]},"session_id":"test-session","cwd":"/test"}`
}

// validTodoWriteMultipleTodosJSON returns a TodoWrite with multiple todos.
func validTodoWriteMultipleTodosJSON() string {
	return `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"Task one","status":"pending","activeForm":"Doing one"},{"content":"Task two","status":"in_progress","activeForm":"Doing two"},{"content":"Task three","status":"completed","activeForm":"Done three"}]},"session_id":"multi-session","cwd":"/multi"}`
}

// nonTodoWriteJSON returns a non-TodoWrite hook event.
func nonTodoWriteJSON() string {
	return `{"tool_name":"Read","tool_input":{"file_path":"/test.txt"},"session_id":"s","cwd":"/"}`
}

// todoWriteEmptyTodosJSON returns a TodoWrite with an empty todos array.
func todoWriteEmptyTodosJSON() string {
	return `{"tool_name":"TodoWrite","tool_input":{"todos":[]},"session_id":"empty-session","cwd":"/empty"}`
}

// todoWriteNoSessionNoCwdJSON returns a TodoWrite without session_id and cwd.
func todoWriteNoSessionNoCwdJSON() string {
	return `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"orphan task","status":"pending","activeForm":"Doing orphan"}]},"session_id":"","cwd":""}`
}

// readJSONLogFile reads the JSON log file and returns parsed entries.
func readJSONLogFile(t *testing.T, path string) []storage.LogEntry {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read JSON log file %s: %v", path, err)
	}
	var entries []storage.LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal JSON from %s: %v\nraw: %s", path, err, string(data))
	}
	return entries
}

// ---------------------------------------------------------------------------
// run(): exit code tests
// ---------------------------------------------------------------------------

func Test_run_Cases(t *testing.T) {
	tests := []struct {
		name          string
		stdin         string
		setProjectDir bool
		projectDir    string // used if setProjectDir is true; empty means use t.TempDir()
		envBackend    string
		envLogPath    string
		envSQLitePath string
		wantExitCode  int
		verifyFile    func(t *testing.T, projectDir string)
	}{
		{
			name:          "success TodoWrite exits 0",
			stdin:         validTodoWriteJSON(),
			setProjectDir: true,
			wantExitCode:  0,
		},
		{
			name:          "non-TodoWrite exits 0 silently",
			stdin:         nonTodoWriteJSON(),
			setProjectDir: true,
			wantExitCode:  0,
		},
		{
			name:          "missing CLAUDE_PROJECT_DIR exits 1",
			stdin:         validTodoWriteJSON(),
			setProjectDir: false,
			wantExitCode:  1,
		},
		{
			name:          "invalid JSON exits 1",
			stdin:         `{bad json`,
			setProjectDir: true,
			wantExitCode:  1,
		},
		{
			name:          "empty input exits 1",
			stdin:         "",
			setProjectDir: true,
			wantExitCode:  1,
		},
		{
			name:          "path escape exits 1",
			stdin:         validTodoWriteJSON(),
			setProjectDir: true,
			envLogPath:    "../../../evil.json",
			wantExitCode:  1,
		},
		{
			name:          "empty todos logged successfully",
			stdin:         todoWriteEmptyTodosJSON(),
			setProjectDir: true,
			wantExitCode:  0,
			verifyFile: func(t *testing.T, projectDir string) {
				t.Helper()
				logPath := filepath.Join(projectDir, ".claude", "todos.json")
				entries := readJSONLogFile(t, logPath)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if len(entries[0].Todos) != 0 {
					t.Errorf("expected 0 todos, got %d", len(entries[0].Todos))
				}
			},
		},
		{
			name:          "unknown values for missing session_id and cwd",
			stdin:         todoWriteNoSessionNoCwdJSON(),
			setProjectDir: true,
			wantExitCode:  0,
			verifyFile: func(t *testing.T, projectDir string) {
				t.Helper()
				logPath := filepath.Join(projectDir, ".claude", "todos.json")
				entries := readJSONLogFile(t, logPath)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if entries[0].SessionID != "unknown" {
					t.Errorf("SessionID = %q, want %q", entries[0].SessionID, "unknown")
				}
				if entries[0].Cwd != "unknown" {
					t.Errorf("Cwd = %q, want %q", entries[0].Cwd, "unknown")
				}
			},
		},
		{
			name:          "custom log path creates file at custom location",
			stdin:         validTodoWriteJSON(),
			setProjectDir: true,
			envLogPath:    ".claude/custom.json",
			wantExitCode:  0,
			verifyFile: func(t *testing.T, projectDir string) {
				t.Helper()
				customPath := filepath.Join(projectDir, ".claude", "custom.json")
				if _, err := os.Stat(customPath); os.IsNotExist(err) {
					t.Errorf("expected custom log file at %s, but it does not exist", customPath)
				}
				entries := readJSONLogFile(t, customPath)
				if len(entries) != 1 {
					t.Errorf("expected 1 entry in custom log, got %d", len(entries))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var projectDir string
			if tt.setProjectDir {
				if tt.projectDir != "" {
					projectDir = tt.projectDir
				} else {
					projectDir = t.TempDir()
				}
				t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
			} else {
				t.Setenv("CLAUDE_PROJECT_DIR", "")
			}

			if tt.envBackend != "" {
				t.Setenv("TODO_STORAGE_BACKEND", tt.envBackend)
			} else {
				t.Setenv("TODO_STORAGE_BACKEND", "")
			}

			if tt.envLogPath != "" {
				t.Setenv("TODO_LOG_PATH", tt.envLogPath)
			} else {
				t.Setenv("TODO_LOG_PATH", "")
			}

			if tt.envSQLitePath != "" {
				t.Setenv("TODO_SQLITE_PATH", tt.envSQLitePath)
			} else {
				t.Setenv("TODO_SQLITE_PATH", "")
			}

			stdin := strings.NewReader(tt.stdin)
			exitCode := run(stdin)

			if exitCode != tt.wantExitCode {
				t.Errorf("run() exit code = %d, want %d", exitCode, tt.wantExitCode)
			}

			if tt.verifyFile != nil && projectDir != "" {
				tt.verifyFile(t, projectDir)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// run(): file creation and content verification
// ---------------------------------------------------------------------------

func Test_run_FileCreatedWithCorrectContent(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")

	// File should exist.
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("expected log file at %s, but it does not exist", logPath)
	}

	// File should contain valid JSON.
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("log file is not valid JSON: %s", string(data))
	}

	// Parse and verify entries.
	entries := readJSONLogFile(t, logPath)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Timestamp == "" {
		t.Error("entry Timestamp is empty")
	}
	if entry.SessionID != "test-session" {
		t.Errorf("entry SessionID = %q, want %q", entry.SessionID, "test-session")
	}
	if len(entry.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(entry.Todos))
	}
	if entry.Todos[0].Content != "Test task" {
		t.Errorf("todo Content = %q, want %q", entry.Todos[0].Content, "Test task")
	}
	if entry.Todos[0].Status != "pending" {
		t.Errorf("todo Status = %q, want %q", entry.Todos[0].Status, "pending")
	}
	if entry.Todos[0].ActiveForm != "Testing" {
		t.Errorf("todo ActiveForm = %q, want %q", entry.Todos[0].ActiveForm, "Testing")
	}
}

// ---------------------------------------------------------------------------
// run(): multiple calls accumulate entries
// ---------------------------------------------------------------------------

func Test_run_MultipleCallsAccumulate(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	const numCalls = 3
	for i := 0; i < numCalls; i++ {
		stdin := strings.NewReader(validTodoWriteJSON())
		exitCode := run(stdin)
		if exitCode != 0 {
			t.Fatalf("run() call #%d exit code = %d, want 0", i+1, exitCode)
		}
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	entries := readJSONLogFile(t, logPath)

	if len(entries) != numCalls {
		t.Fatalf("expected %d entries after %d calls, got %d", numCalls, numCalls, len(entries))
	}

	// Each entry should have the same content since we used the same input.
	for i, entry := range entries {
		if len(entry.Todos) != 1 {
			t.Errorf("entry[%d] has %d todos, want 1", i, len(entry.Todos))
		}
	}
}

// ---------------------------------------------------------------------------
// run(): sqlite backend
// ---------------------------------------------------------------------------

func Test_run_SQLiteBackend(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	// The SQLite database file should exist.
	dbPath := filepath.Join(projectDir, ".claude", "todos.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected SQLite database at %s, but it does not exist", dbPath)
	}

	// Verify we can open and read the database via the backend.
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("failed to open SQLite backend: %v", err)
	}

	entries, err := backend.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in SQLite, got %d", len(entries))
	}

	if entries[0].Todos[0].Content != "Test task" {
		t.Errorf("todo Content = %q, want %q", entries[0].Todos[0].Content, "Test task")
	}
}

// ---------------------------------------------------------------------------
// run(): non-TodoWrite produces no file
// ---------------------------------------------------------------------------

func Test_run_NonTodoWrite_NoFileCreated(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(nonTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected no file at %s for non-TodoWrite event, but file exists", logPath)
	}
}

// ---------------------------------------------------------------------------
// run(): multiple todos in single call
// ---------------------------------------------------------------------------

func Test_run_MultipleTodosInSingleCall(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteMultipleTodosJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	entries := readJSONLogFile(t, logPath)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if len(entries[0].Todos) != 3 {
		t.Fatalf("expected 3 todos in entry, got %d", len(entries[0].Todos))
	}

	expectedContents := []string{"Task one", "Task two", "Task three"}
	for i, want := range expectedContents {
		if entries[0].Todos[i].Content != want {
			t.Errorf("todo[%d].Content = %q, want %q", i, entries[0].Todos[i].Content, want)
		}
	}
}

// ---------------------------------------------------------------------------
// run(): SQLite multiple calls accumulate
// ---------------------------------------------------------------------------

func Test_run_SQLiteMultipleCallsAccumulate(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	const numCalls = 3
	for i := 0; i < numCalls; i++ {
		stdin := strings.NewReader(validTodoWriteJSON())
		exitCode := run(stdin)
		if exitCode != 0 {
			t.Fatalf("run() call #%d exit code = %d, want 0", i+1, exitCode)
		}
	}

	dbPath := filepath.Join(projectDir, ".claude", "todos.db")
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("failed to open SQLite backend: %v", err)
	}

	entries, err := backend.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}

	if len(entries) != numCalls {
		t.Fatalf("expected %d entries after %d calls, got %d", numCalls, numCalls, len(entries))
	}
}

// ---------------------------------------------------------------------------
// run(): whitespace-only CLAUDE_PROJECT_DIR treated as unset
// ---------------------------------------------------------------------------

func Test_run_WhitespaceProjectDir_Exits1(t *testing.T) {
	t.Setenv("CLAUDE_PROJECT_DIR", "   ")
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 1 {
		t.Errorf("run() exit code = %d, want 1 for whitespace-only CLAUDE_PROJECT_DIR", exitCode)
	}
}

// ---------------------------------------------------------------------------
// run(): timestamp present in saved entry
// ---------------------------------------------------------------------------

func Test_run_EntryHasTimestamp(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	entries := readJSONLogFile(t, logPath)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	ts := entries[0].Timestamp
	if ts == "" {
		t.Fatal("entry Timestamp is empty")
	}

	// Timestamp should end with "Z" (UTC marker).
	if !strings.HasSuffix(ts, "Z") {
		t.Errorf("Timestamp %q does not end with Z", ts)
	}

	// Timestamp should contain T separator.
	if !strings.Contains(ts, "T") {
		t.Errorf("Timestamp %q does not contain T separator", ts)
	}
}

// ---------------------------------------------------------------------------
// run(): unknown backend error
// ---------------------------------------------------------------------------

func Test_run_UnknownBackend_Exits1(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "postgres")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 1 {
		t.Errorf("run() exit code = %d, want 1 for unknown backend", exitCode)
	}
}

// ---------------------------------------------------------------------------
// run(): log file is valid JSON after write
// ---------------------------------------------------------------------------

func Test_run_LogFileIsValidJSON(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !json.Valid(data) {
		t.Errorf("log file is not valid JSON:\n%s", string(data))
	}
}

// ---------------------------------------------------------------------------
// run(): .claude directory auto-created
// ---------------------------------------------------------------------------

func Test_run_CreatesClaudeDirectory(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	claudeDir := filepath.Join(projectDir, ".claude")

	// Verify .claude doesn't exist before run.
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf(".claude directory already exists before test")
	}

	stdin := strings.NewReader(validTodoWriteJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	// Verify .claude directory was created.
	info, err := os.Stat(claudeDir)
	if os.IsNotExist(err) {
		t.Fatal(".claude directory was not created")
	}
	if err != nil {
		t.Fatalf("stat .claude: %v", err)
	}
	if !info.IsDir() {
		t.Error(".claude exists but is not a directory")
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func Benchmark_run_TodoWrite(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	b.Setenv("TODO_STORAGE_BACKEND", "json")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	input := validTodoWriteJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		exitCode := run(stdin)
		if exitCode != 0 {
			b.Fatalf("run() exit code = %d, want 0", exitCode)
		}
	}
}

func Benchmark_run_NonTodoWrite(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	b.Setenv("TODO_STORAGE_BACKEND", "")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	input := nonTodoWriteJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		exitCode := run(stdin)
		if exitCode != 0 {
			b.Fatalf("run() exit code = %d, want 0", exitCode)
		}
	}
}
