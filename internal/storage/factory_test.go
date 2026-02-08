package storage_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// windowsOrUnixPath returns winPath on Windows, unixPath otherwise.
// Used in path-escape tests where Unix absolute paths like "/etc/passwd"
// are not actually absolute on Windows (no drive letter).
func windowsOrUnixPath(unixPath, winPath string) string {
	if runtime.GOOS == "windows" {
		return winPath
	}
	return unixPath
}

// ---------------------------------------------------------------------------
// GetStorageBackend: backend type selection
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_Cases(t *testing.T) {

	tests := []struct {
		name          string
		envBackend    string // TODO_STORAGE_BACKEND value ("" means unset)
		envLogPath    string // TODO_LOG_PATH value ("" means unset)
		envSQLitePath string // TODO_SQLITE_PATH value ("" means unset)
		wantJSON      bool   // expect *JSONBackend
		wantSQLite    bool   // expect *SQLiteBackend
		wantErr       bool
		errContains   string // substring expected in error message
		setBackend    bool   // whether to set TODO_STORAGE_BACKEND at all
		setLogPath    bool   // whether to set TODO_LOG_PATH at all
		setSQLitePath bool   // whether to set TODO_SQLITE_PATH at all
	}{
		{
			name:       "default returns JSON backend when no env set",
			wantJSON:   true,
			setBackend: false,
		},
		{
			name:       "explicit json returns JSON backend",
			envBackend: "json",
			wantJSON:   true,
			setBackend: true,
		},
		{
			name:       "explicit sqlite returns SQLite backend",
			envBackend: "sqlite",
			wantSQLite: true,
			setBackend: true,
		},
		{
			name:       "case insensitive JSON uppercase",
			envBackend: "JSON",
			wantJSON:   true,
			setBackend: true,
		},
		{
			name:       "case insensitive SQLite mixed case",
			envBackend: "SQLite",
			wantSQLite: true,
			setBackend: true,
		},
		{
			name:        "unknown backend returns error",
			envBackend:  "postgres",
			wantErr:     true,
			errContains: "unknown",
			setBackend:  true,
		},
		{
			name:       "whitespace trimmed around json",
			envBackend: "  json  ",
			wantJSON:   true,
			setBackend: true,
		},
		{
			name:       "whitespace trimmed around sqlite",
			envBackend: "  sqlite  ",
			wantSQLite: true,
			setBackend: true,
		},
		{
			name:       "custom JSON path accepted",
			envLogPath: ".claude/custom.json",
			wantJSON:   true,
			setBackend: false,
			setLogPath: true,
		},
		{
			name:          "custom SQLite path accepted",
			envBackend:    "sqlite",
			envSQLitePath: ".claude/custom.db",
			wantSQLite:    true,
			setBackend:    true,
			setSQLitePath: true,
		},
		{
			name:       "JSON path escape rejected",
			envLogPath: "../../../etc/evil.json",
			wantErr:    true,
			setBackend: false,
			setLogPath: true,
		},
		{
			name:          "SQLite absolute path escape rejected",
			envBackend:    "sqlite",
			envSQLitePath: windowsOrUnixPath("/etc/evil.db", `C:\Windows\evil.db`),
			wantErr:       true,
			setBackend:    true,
			setSQLitePath: true,
		},
		{
			name:          "SQLite relative path escape rejected",
			envBackend:    "sqlite",
			envSQLitePath: "../../../etc/evil.db",
			wantErr:       true,
			setBackend:    true,
			setSQLitePath: true,
		},
		{
			name:       "empty string backend defaults to JSON",
			envBackend: "",
			wantJSON:   true,
			setBackend: true,
		},
		{
			name:       "whitespace-only backend defaults to JSON",
			envBackend: "   ",
			wantJSON:   true,
			setBackend: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Environment variable manipulation requires sequential execution.
			// t.Setenv cannot be used with t.Parallel().
			projectDir := t.TempDir()

			if tt.setBackend {
				t.Setenv("TODO_STORAGE_BACKEND", tt.envBackend)
			} else {
				t.Setenv("TODO_STORAGE_BACKEND", "")
			}

			if tt.setLogPath {
				t.Setenv("TODO_LOG_PATH", tt.envLogPath)
			} else {
				t.Setenv("TODO_LOG_PATH", "")
			}

			if tt.setSQLitePath {
				t.Setenv("TODO_SQLITE_PATH", tt.envSQLitePath)
			} else {
				t.Setenv("TODO_SQLITE_PATH", "")
			}

			backend, err := storage.GetStorageBackend(projectDir)

			if tt.wantErr {
				if err == nil {
					t.Fatal("GetStorageBackend() expected error, got nil")
				}
				if tt.errContains != "" {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errContains)) {
						t.Errorf("GetStorageBackend() error = %q, want it to contain %q", err.Error(), tt.errContains)
					}
				}
				if backend != nil {
					t.Errorf("GetStorageBackend() returned non-nil backend on error: %v", backend)
				}
				return
			}

			if err != nil {
				t.Fatalf("GetStorageBackend() unexpected error: %v", err)
			}

			if backend == nil {
				t.Fatal("GetStorageBackend() returned nil backend without error")
			}

			if tt.wantJSON {
				jsonBackend, ok := backend.(*storage.JSONBackend)
				if !ok {
					t.Fatalf("GetStorageBackend() returned %T, want *storage.JSONBackend", backend)
				}
				if jsonBackend == nil {
					t.Fatal("GetStorageBackend() returned nil *JSONBackend")
				}
			}

			if tt.wantSQLite {
				sqliteBackend, ok := backend.(*storage.SQLiteBackend)
				if !ok {
					t.Fatalf("GetStorageBackend() returned %T, want *storage.SQLiteBackend", backend)
				}
				if sqliteBackend == nil {
					t.Fatal("GetStorageBackend() returned nil *SQLiteBackend")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetStorageBackend: default path verification
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_DefaultJSONPath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	jsonBackend, ok := backend.(*storage.JSONBackend)
	if !ok {
		t.Fatalf("expected *JSONBackend, got %T", backend)
	}

	// The default JSON path should be <projectDir>/.claude/todos.json.
	// On macOS, t.TempDir() may resolve to /private/var/... via symlinks,
	// so we compare against the resolved projectDir.
	wantSuffix := filepath.Join(".claude", "todos.json")
	if !strings.HasSuffix(jsonBackend.LogFile, wantSuffix) {
		t.Errorf("JSONBackend.LogFile = %q, want it to end with %q", jsonBackend.LogFile, wantSuffix)
	}
}

func Test_GetStorageBackend_DefaultSQLitePath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	sqliteBackend, ok := backend.(*storage.SQLiteBackend)
	if !ok {
		t.Fatalf("expected *SQLiteBackend, got %T", backend)
	}

	wantSuffix := filepath.Join(".claude", "todos.db")
	if !strings.HasSuffix(sqliteBackend.DBPath, wantSuffix) {
		t.Errorf("SQLiteBackend.DBPath = %q, want it to end with %q", sqliteBackend.DBPath, wantSuffix)
	}
}

// ---------------------------------------------------------------------------
// GetStorageBackend: custom path verification
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_CustomJSONPath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "json")
	t.Setenv("TODO_LOG_PATH", ".claude/custom.json")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	jsonBackend, ok := backend.(*storage.JSONBackend)
	if !ok {
		t.Fatalf("expected *JSONBackend, got %T", backend)
	}

	if !strings.HasSuffix(jsonBackend.LogFile, "custom.json") {
		t.Errorf("JSONBackend.LogFile = %q, want it to end with %q", jsonBackend.LogFile, "custom.json")
	}
}

func Test_GetStorageBackend_CustomSQLitePath(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_SQLITE_PATH", ".claude/custom.db")
	t.Setenv("TODO_LOG_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	sqliteBackend, ok := backend.(*storage.SQLiteBackend)
	if !ok {
		t.Fatalf("expected *SQLiteBackend, got %T", backend)
	}

	if !strings.HasSuffix(sqliteBackend.DBPath, "custom.db") {
		t.Errorf("SQLiteBackend.DBPath = %q, want it to end with %q", sqliteBackend.DBPath, "custom.db")
	}
}

// ---------------------------------------------------------------------------
// GetStorageBackend: interface compliance
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_JSONImplementsStorageBackend(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "json")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	// The returned value should satisfy the StorageBackend interface.
	_ = backend
}

func Test_GetStorageBackend_SQLiteImplementsStorageBackend(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	_ = backend
}

func Test_GetStorageBackend_SQLiteImplementsQueryableStorageBackend(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	_, ok := backend.(storage.QueryableStorageBackend)
	if !ok {
		t.Errorf("SQLite backend (%T) does not implement QueryableStorageBackend", backend)
	}
}

// ---------------------------------------------------------------------------
// GetStorageBackend: returned backend is functional
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_JSONBackend_Functional(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "json")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00.000Z",
		SessionID: "factory-test-session",
		Cwd:       "/test",
		Todos: []storage.TodoItem{
			{Content: "factory task", Status: "pending", ActiveForm: "Testing factory"},
		},
	}

	if err := backend.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry() unexpected error: %v", err)
	}

	entries, err := backend.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Todos[0].Content != "factory task" {
		t.Errorf("entry content = %q, want %q", entries[0].Todos[0].Content, "factory task")
	}
}

func Test_GetStorageBackend_SQLiteBackend_Functional(t *testing.T) {
	projectDir := t.TempDir()
	t.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	t.Setenv("TODO_LOG_PATH", "")
	t.Setenv("TODO_SQLITE_PATH", "")

	backend, err := storage.GetStorageBackend(projectDir)
	if err != nil {
		t.Fatalf("GetStorageBackend() unexpected error: %v", err)
	}

	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00.000Z",
		SessionID: "factory-test-session",
		Cwd:       "/test",
		Todos: []storage.TodoItem{
			{Content: "sqlite factory task", Status: "pending", ActiveForm: "Testing sqlite factory"},
		},
	}

	if err := backend.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry() unexpected error: %v", err)
	}

	entries, err := backend.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() unexpected error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Todos[0].Content != "sqlite factory task" {
		t.Errorf("entry content = %q, want %q", entries[0].Todos[0].Content, "sqlite factory task")
	}
}

// ---------------------------------------------------------------------------
// GetStorageBackend: path traversal edge cases
// ---------------------------------------------------------------------------

func Test_GetStorageBackend_PathEscape_Cases(t *testing.T) {
	tests := []struct {
		name          string
		envBackend    string
		envLogPath    string
		envSQLitePath string
	}{
		{
			name:       "JSON parent traversal",
			envBackend: "json",
			envLogPath: "../../escape.json",
		},
		{
			name:       "JSON absolute path outside project",
			envBackend: "json",
			envLogPath: windowsOrUnixPath("/tmp/outside.json", `C:\Windows\outside.json`),
		},
		{
			name:          "SQLite parent traversal",
			envBackend:    "sqlite",
			envSQLitePath: "../../escape.db",
		},
		{
			name:          "SQLite absolute path outside project",
			envBackend:    "sqlite",
			envSQLitePath: windowsOrUnixPath("/tmp/outside.db", `C:\Windows\outside.db`),
		},
		{
			name:       "JSON complex traversal",
			envBackend: "json",
			envLogPath: "sub/../.././../escape.json",
		},
		{
			name:          "SQLite complex traversal",
			envBackend:    "sqlite",
			envSQLitePath: "sub/../.././../escape.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			t.Setenv("TODO_STORAGE_BACKEND", tt.envBackend)
			t.Setenv("TODO_LOG_PATH", tt.envLogPath)
			t.Setenv("TODO_SQLITE_PATH", tt.envSQLitePath)

			backend, err := storage.GetStorageBackend(projectDir)
			if err == nil {
				t.Fatalf("GetStorageBackend() expected error for path escape, got backend %T", backend)
			}
			if backend != nil {
				t.Errorf("GetStorageBackend() returned non-nil backend on error: %v", backend)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func Benchmark_GetStorageBackend_JSON(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("TODO_STORAGE_BACKEND", "json")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := storage.GetStorageBackend(projectDir)
		if err != nil {
			b.Fatalf("GetStorageBackend() error: %v", err)
		}
	}
}

func Benchmark_GetStorageBackend_SQLite(b *testing.B) {
	projectDir := b.TempDir()
	b.Setenv("TODO_STORAGE_BACKEND", "sqlite")
	b.Setenv("TODO_LOG_PATH", "")
	b.Setenv("TODO_SQLITE_PATH", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := storage.GetStorageBackend(projectDir)
		if err != nil {
			b.Fatalf("GetStorageBackend() error: %v", err)
		}
	}
}
