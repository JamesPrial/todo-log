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

// validTaskCreateJSON returns a well-formed TaskCreate JSON payload.
func validTaskCreateJSON() string {
	return `{"tool_name":"TaskCreate","tool_input":{"subject":"Test task","description":"A test","activeForm":"Testing"},"session_id":"test-session","cwd":"/test"}`
}

// validTaskUpdateJSON returns a well-formed TaskUpdate JSON payload.
func validTaskUpdateJSON() string {
	return `{"tool_name":"TaskUpdate","tool_input":{"taskId":"1","status":"completed"},"session_id":"update-session","cwd":"/test"}`
}

// nonTaskToolJSON returns a non-task hook event.
func nonTaskToolJSON() string {
	return `{"tool_name":"Read","tool_input":{"file_path":"/test.txt"},"session_id":"s","cwd":"/"}`
}

// taskCreateNoSessionNoCwdJSON returns a TaskCreate without session_id and cwd.
func taskCreateNoSessionNoCwdJSON() string {
	return `{"tool_name":"TaskCreate","tool_input":{"subject":"orphan task","activeForm":"Doing orphan"},"session_id":"","cwd":""}`
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
			name:          "success TaskCreate exits 0",
			stdin:         validTaskCreateJSON(),
			setProjectDir: true,
			wantExitCode:  0,
		},
		{
			name:          "success TaskUpdate exits 0",
			stdin:         validTaskUpdateJSON(),
			setProjectDir: true,
			wantExitCode:  0,
		},
		{
			name:          "non-task tool exits 0 silently",
			stdin:         nonTaskToolJSON(),
			setProjectDir: true,
			wantExitCode:  0,
		},
		{
			name:          "missing CLAUDE_PROJECT_DIR exits 1",
			stdin:         validTaskCreateJSON(),
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
			stdin:         validTaskCreateJSON(),
			setProjectDir: true,
			envLogPath:    "../../../evil.json",
			wantExitCode:  1,
		},
		{
			name:          "unknown values for missing session_id and cwd",
			stdin:         taskCreateNoSessionNoCwdJSON(),
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
			stdin:         validTaskCreateJSON(),
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

	stdin := strings.NewReader(validTaskCreateJSON())
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
	if entry.ToolName != "TaskCreate" {
		t.Errorf("entry ToolName = %q, want %q", entry.ToolName, "TaskCreate")
	}
	if entry.Task.Subject != "Test task" {
		t.Errorf("task Subject = %q, want %q", entry.Task.Subject, "Test task")
	}
	if entry.Task.Status != "pending" {
		t.Errorf("task Status = %q, want %q", entry.Task.Status, "pending")
	}
	if entry.Task.ActiveForm != "Testing" {
		t.Errorf("task ActiveForm = %q, want %q", entry.Task.ActiveForm, "Testing")
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
		stdin := strings.NewReader(validTaskCreateJSON())
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

	// Each entry should have task data since we used the same input.
	for i, entry := range entries {
		if entry.Task.Subject != "Test task" {
			t.Errorf("entry[%d] task.Subject = %q, want %q", i, entry.Task.Subject, "Test task")
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

	stdin := strings.NewReader(validTaskCreateJSON())
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

	if entries[0].Task.Subject != "Test task" {
		t.Errorf("task Subject = %q, want %q", entries[0].Task.Subject, "Test task")
	}
}

// ---------------------------------------------------------------------------
// run(): non-task tool produces no file
// ---------------------------------------------------------------------------

func Test_run_NonTaskTool_NoFileCreated(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(nonTaskToolJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected no file at %s for non-task event, but file exists", logPath)
	}
}

// ---------------------------------------------------------------------------
// run(): TaskUpdate in single call
// ---------------------------------------------------------------------------

func Test_run_TaskUpdateInSingleCall(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	stdin := strings.NewReader(validTaskUpdateJSON())
	exitCode := run(stdin)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0", exitCode)
	}

	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	entries := readJSONLogFile(t, logPath)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].ToolName != "TaskUpdate" {
		t.Errorf("ToolName = %q, want %q", entries[0].ToolName, "TaskUpdate")
	}
	if entries[0].Task.ID != "1" {
		t.Errorf("Task.ID = %q, want %q", entries[0].Task.ID, "1")
	}
	if entries[0].Task.Status != "completed" {
		t.Errorf("Task.Status = %q, want %q", entries[0].Task.Status, "completed")
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
		stdin := strings.NewReader(validTaskCreateJSON())
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

	stdin := strings.NewReader(validTaskCreateJSON())
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

	stdin := strings.NewReader(validTaskCreateJSON())
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

	stdin := strings.NewReader(validTaskCreateJSON())
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

	stdin := strings.NewReader(validTaskCreateJSON())
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

	stdin := strings.NewReader(validTaskCreateJSON())
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
// run(): TodoWrite is no longer accepted
// ---------------------------------------------------------------------------

func Test_run_TodoWriteIgnored(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	todoWriteJSON := `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"task","status":"pending","activeForm":"Doing"}]},"session_id":"s","cwd":"/"}`
	stdin := strings.NewReader(todoWriteJSON)
	exitCode := run(stdin)

	// TodoWrite should be silently ignored (return 0) since it's not in acceptedTools
	if exitCode != 0 {
		t.Errorf("run() exit code = %d, want 0 for ignored TodoWrite", exitCode)
	}

	// No file should be created
	logPath := filepath.Join(projectDir, ".claude", "todos.json")
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected no file at %s for TodoWrite event, but file exists", logPath)
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func Benchmark_run_TaskCreate(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	b.Setenv("TODO_STORAGE_BACKEND", "json")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	input := validTaskCreateJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		exitCode := run(stdin)
		if exitCode != 0 {
			b.Fatalf("run() exit code = %d, want 0", exitCode)
		}
	}
}

func Benchmark_run_NonTaskTool(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("CLAUDE_PROJECT_DIR", projectDir)
	b.Setenv("TODO_STORAGE_BACKEND", "")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	input := nonTaskToolJSON()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		exitCode := run(stdin)
		if exitCode != 0 {
			b.Fatalf("run() exit code = %d, want 0", exitCode)
		}
	}
}
