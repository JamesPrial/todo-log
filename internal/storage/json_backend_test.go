package storage_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// ---------------------------------------------------------------------------
// Test helpers (JSON backend specific)
// ---------------------------------------------------------------------------

// makeJSONEntry builds a LogEntry with a TaskItem for concise test setup.
func makeJSONEntry(subject, status string) storage.LogEntry {
	return storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00.000Z",
		SessionID: "test-session",
		Cwd:       "/test",
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:    subject,
			Status:     status,
			ActiveForm: "Testing",
		},
	}
}

// makeJSONEntryWithTimestamp builds a LogEntry with a custom timestamp for ordering tests.
func makeJSONEntryWithTimestamp(ts, subject string) storage.LogEntry {
	return storage.LogEntry{
		Timestamp: ts,
		SessionID: "test-session",
		Cwd:       "/test",
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:    subject,
			Status:     "pending",
			ActiveForm: "Testing",
		},
	}
}

// readFileJSON reads the file at path and unmarshals it into a []storage.LogEntry.
// Returns the raw bytes and the parsed entries.
func readFileJSON(t *testing.T, path string) ([]byte, []storage.LogEntry) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}
	var entries []storage.LogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("failed to unmarshal JSON from %s: %v\nraw content: %s", path, err, string(data))
	}
	return data, entries
}

// ---------------------------------------------------------------------------
// NewJSONBackend
// ---------------------------------------------------------------------------

func Test_NewJSONBackend_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	b := storage.NewJSONBackend("/some/path/todos.json")
	if b == nil {
		t.Fatal("NewJSONBackend returned nil")
	}
}

func Test_NewJSONBackend_ImplementsStorageBackend(t *testing.T) {
	t.Parallel()
	var _ storage.StorageBackend = storage.NewJSONBackend("/some/path")
}

// ---------------------------------------------------------------------------
// LoadHistory
// ---------------------------------------------------------------------------

func Test_JSONBackend_LoadHistory_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setup        func(t *testing.T, path string)
		wantLen      int
		wantErr      bool
		wantContents func(t *testing.T, entries []storage.LogEntry)
	}{
		{
			name:    "nonexistent file returns empty slice no error",
			setup:   nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "valid array with entries",
			setup: func(t *testing.T, path string) {
				t.Helper()
				entries := []storage.LogEntry{
					makeJSONEntry("task one", "pending"),
					makeJSONEntry("task two", "completed"),
				}
				data, err := json.Marshal(entries)
				if err != nil {
					t.Fatalf("marshal setup data: %v", err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("write setup file: %v", err)
				}
			},
			wantLen: 2,
			wantErr: false,
			wantContents: func(t *testing.T, entries []storage.LogEntry) {
				t.Helper()
				if entries[0].Task.Subject != "task one" {
					t.Errorf("first entry subject = %q, want %q", entries[0].Task.Subject, "task one")
				}
				if entries[1].Task.Subject != "task two" {
					t.Errorf("second entry subject = %q, want %q", entries[1].Task.Subject, "task two")
				}
			},
		},
		{
			name: "corrupted JSON returns empty slice no error",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{{{invalid"), 0644); err != nil {
					t.Fatalf("write corrupted file: %v", err)
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "non-array JSON returns empty slice no error",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(`{"key":"value"}`), 0644); err != nil {
					t.Fatalf("write non-array file: %v", err)
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "empty array returns empty slice no error",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
					t.Fatalf("write empty array file: %v", err)
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "empty file returns empty slice no error",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(""), 0644); err != nil {
					t.Fatalf("write empty file: %v", err)
				}
			},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "unicode content preserved",
			setup: func(t *testing.T, path string) {
				t.Helper()
				entries := []storage.LogEntry{
					{
						Timestamp: "2025-01-01T00:00:00.000Z",
						SessionID: "unicode-session",
						Cwd:       "/proyecto/espa\u00f1ol",
						ToolName:  "TaskCreate",
						Task: storage.TaskItem{
							Subject:    "\u4f60\u597d\u4e16\u754c",
							Status:     "pending",
							ActiveForm: "\u5904\u7406\u4e2d",
						},
					},
				}
				data, err := json.Marshal(entries)
				if err != nil {
					t.Fatalf("marshal unicode data: %v", err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("write unicode file: %v", err)
				}
			},
			wantLen: 1,
			wantErr: false,
			wantContents: func(t *testing.T, entries []storage.LogEntry) {
				t.Helper()
				if entries[0].Task.Subject != "\u4f60\u597d\u4e16\u754c" {
					t.Errorf("chinese content not preserved: got %q", entries[0].Task.Subject)
				}
				if entries[0].Cwd != "/proyecto/espa\u00f1ol" {
					t.Errorf("unicode cwd not preserved: got %q", entries[0].Cwd)
				}
			},
		},
		{
			name: "multiple entries returned in order",
			setup: func(t *testing.T, path string) {
				t.Helper()
				entries := []storage.LogEntry{
					makeJSONEntryWithTimestamp("2025-01-01T00:00:00Z", "first"),
					makeJSONEntryWithTimestamp("2025-01-02T00:00:00Z", "second"),
					makeJSONEntryWithTimestamp("2025-01-03T00:00:00Z", "third"),
				}
				data, err := json.Marshal(entries)
				if err != nil {
					t.Fatalf("marshal ordered data: %v", err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("write ordered file: %v", err)
				}
			},
			wantLen: 3,
			wantErr: false,
			wantContents: func(t *testing.T, entries []storage.LogEntry) {
				t.Helper()
				expected := []string{"first", "second", "third"}
				for i, want := range expected {
					got := entries[i].Task.Subject
					if got != want {
						t.Errorf("entry[%d] subject = %q, want %q", i, got, want)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "todos.json")

			if tt.setup != nil {
				tt.setup(t, path)
			}

			backend := storage.NewJSONBackend(path)
			entries, err := backend.LoadHistory()

			if tt.wantErr && err == nil {
				t.Fatal("LoadHistory() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("LoadHistory() unexpected error: %v", err)
			}

			if len(entries) != tt.wantLen {
				t.Fatalf("LoadHistory() returned %d entries, want %d", len(entries), tt.wantLen)
			}

			if tt.wantContents != nil {
				tt.wantContents(t, entries)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AppendEntry
// ---------------------------------------------------------------------------

func Test_JSONBackend_AppendEntry_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, path string)
		entry   storage.LogEntry
		wantErr bool
		verify  func(t *testing.T, path string)
	}{
		{
			name:  "new file single entry creates file with 1-entry array",
			setup: nil,
			entry: makeJSONEntry("first task", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if entries[0].Task.Subject != "first task" {
					t.Errorf("subject = %q, want %q", entries[0].Task.Subject, "first task")
				}
			},
		},
		{
			name: "append to existing file adds entry",
			setup: func(t *testing.T, path string) {
				t.Helper()
				entries := []storage.LogEntry{makeJSONEntry("existing", "pending")}
				data, _ := json.Marshal(entries)
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("write setup: %v", err)
				}
			},
			entry: makeJSONEntry("new task", "in_progress"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(entries))
				}
				if entries[0].Task.Subject != "existing" {
					t.Errorf("first entry subject = %q, want %q", entries[0].Task.Subject, "existing")
				}
				if entries[1].Task.Subject != "new task" {
					t.Errorf("second entry subject = %q, want %q", entries[1].Task.Subject, "new task")
				}
			},
		},
		{
			name:  "creates parent directories",
			setup: nil,
			entry: makeJSONEntry("nested", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
			},
		},
		{
			name:  "2-space JSON indentation",
			setup: nil,
			entry: makeJSONEntry("indented", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read file: %v", err)
				}
				content := string(data)
				if !strings.Contains(content, "\n  ") {
					t.Errorf("expected 2-space indentation in JSON output, got:\n%s", content)
				}
				var entries []storage.LogEntry
				if err := json.Unmarshal(data, &entries); err != nil {
					t.Fatalf("output is not valid JSON: %v", err)
				}
			},
		},
		{
			name:  "task fields preserved in JSON",
			setup: nil,
			entry: storage.LogEntry{
				Timestamp: "2025-01-01T00:00:00.000Z",
				SessionID: "task-fields-session",
				Cwd:       "/test",
				ToolName:  "TaskCreate",
				Task: storage.TaskItem{
					Subject:    "my task",
					Status:     "pending",
					ActiveForm: "Working",
				},
			},
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if entries[0].Task.Subject != "my task" {
					t.Errorf("Task.Subject = %q, want %q", entries[0].Task.Subject, "my task")
				}
				if entries[0].ToolName != "TaskCreate" {
					t.Errorf("ToolName = %q, want %q", entries[0].ToolName, "TaskCreate")
				}
			},
		},
		{
			name:  "unicode roundtrip through append",
			setup: nil,
			entry: storage.LogEntry{
				Timestamp: "2025-01-01T00:00:00.000Z",
				SessionID: "unicode-sess",
				Cwd:       "/proyecto/espa\u00f1ol",
				ToolName:  "TaskCreate",
				Task: storage.TaskItem{
					Subject:    "\u4f60\u597d\u4e16\u754c",
					Status:     "pending",
					ActiveForm: "\u5904\u7406\u4e2d",
				},
			},
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if entries[0].Cwd != "/proyecto/espa\u00f1ol" {
					t.Errorf("cwd unicode not preserved: %q", entries[0].Cwd)
				}
				if entries[0].Task.Subject != "\u4f60\u597d\u4e16\u754c" {
					t.Errorf("chinese subject not preserved: %q", entries[0].Task.Subject)
				}
			},
		},
		{
			name: "recovery from corruption starts fresh",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{{{invalid json garbage"), 0644); err != nil {
					t.Fatalf("write corrupted file: %v", err)
				}
			},
			entry: makeJSONEntry("fresh start", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry after recovery, got %d", len(entries))
				}
				if entries[0].Task.Subject != "fresh start" {
					t.Errorf("subject = %q, want %q", entries[0].Task.Subject, "fresh start")
				}
			},
		},
		{
			name:  "large content handled without error",
			setup: nil,
			entry: storage.LogEntry{
				Timestamp: "2025-01-01T00:00:00.000Z",
				SessionID: "large-content-session",
				Cwd:       "/test",
				ToolName:  "TaskCreate",
				Task: storage.TaskItem{
					Subject:    strings.Repeat("x", 10*1024),
					Status:     "pending",
					ActiveForm: "Processing large content",
				},
			},
			verify: func(t *testing.T, path string) {
				t.Helper()
				_, entries := readFileJSON(t, path)
				if len(entries) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(entries))
				}
				if len(entries[0].Task.Subject) != 10*1024 {
					t.Errorf("subject length = %d, want %d", len(entries[0].Task.Subject), 10*1024)
				}
			},
		},
		{
			name:  "file content is valid JSON after append",
			setup: nil,
			entry: makeJSONEntry("valid json check", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read file: %v", err)
				}
				if !json.Valid(data) {
					t.Errorf("file content is not valid JSON:\n%s", string(data))
				}
			},
		},
		{
			name:  "file ends with trailing newline",
			setup: nil,
			entry: makeJSONEntry("trailing newline", "pending"),
			verify: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read file: %v", err)
				}
				if !bytes.HasSuffix(data, []byte("\n")) {
					end := len(data)
					start := end - 20
					if start < 0 {
						start = 0
					}
					t.Errorf("file does not end with newline, last bytes: %q", string(data[start:end]))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			var path string

			if tt.name == "creates parent directories" {
				path = filepath.Join(dir, "nested", "deeply", "todos.json")
			} else {
				path = filepath.Join(dir, "todos.json")
			}

			if tt.setup != nil {
				tt.setup(t, path)
			}

			backend := storage.NewJSONBackend(path)
			err := backend.AppendEntry(tt.entry)

			if tt.wantErr && err == nil {
				t.Fatal("AppendEntry() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("AppendEntry() unexpected error: %v", err)
			}

			if tt.verify != nil {
				tt.verify(t, path)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AppendEntry: multiple sequential appends
// ---------------------------------------------------------------------------

func Test_JSONBackend_AppendEntry_MultipleSequentialAppends(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)

	const count = 5
	for i := 0; i < count; i++ {
		entry := makeJSONEntry("task "+string(rune('A'+i)), "pending")
		entry.Timestamp = "2025-01-0" + string(rune('1'+i)) + "T00:00:00.000Z"
		if err := backend.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	_, entries := readFileJSON(t, path)
	if len(entries) != count {
		t.Fatalf("expected %d entries, got %d", count, len(entries))
	}

	for i := 0; i < count; i++ {
		want := "task " + string(rune('A'+i))
		got := entries[i].Task.Subject
		if got != want {
			t.Errorf("entry[%d] subject = %q, want %q", i, got, want)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !json.Valid(data) {
		t.Errorf("file is not valid JSON after %d appends", count)
	}
}

// ---------------------------------------------------------------------------
// AppendEntry then LoadHistory roundtrip
// ---------------------------------------------------------------------------

func Test_JSONBackend_AppendEntry_LoadHistory_Roundtrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)

	original := storage.LogEntry{
		Timestamp: "2025-06-15T12:30:45.123Z",
		SessionID: "roundtrip-session-xyz",
		Cwd:       "/home/user/project",
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:     "implement feature",
			Description: "Feature implementation",
			Status:      "in_progress",
			ActiveForm:  "Implementing feature",
		},
	}

	if err := backend.AppendEntry(original); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := backend.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0]
	if got.Timestamp != original.Timestamp {
		t.Errorf("Timestamp = %q, want %q", got.Timestamp, original.Timestamp)
	}
	if got.SessionID != original.SessionID {
		t.Errorf("SessionID = %q, want %q", got.SessionID, original.SessionID)
	}
	if got.Cwd != original.Cwd {
		t.Errorf("Cwd = %q, want %q", got.Cwd, original.Cwd)
	}
	if got.ToolName != original.ToolName {
		t.Errorf("ToolName = %q, want %q", got.ToolName, original.ToolName)
	}
	if got.Task.Subject != original.Task.Subject {
		t.Errorf("Task.Subject = %q, want %q", got.Task.Subject, original.Task.Subject)
	}
	if got.Task.Status != original.Task.Status {
		t.Errorf("Task.Status = %q, want %q", got.Task.Status, original.Task.Status)
	}
	if got.Task.ActiveForm != original.Task.ActiveForm {
		t.Errorf("Task.ActiveForm = %q, want %q", got.Task.ActiveForm, original.Task.ActiveForm)
	}
}

// ---------------------------------------------------------------------------
// AppendEntry then LoadHistory: multiple append-load cycles
// ---------------------------------------------------------------------------

func Test_JSONBackend_AppendEntry_LoadHistory_IncrementalGrowth(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)

	for i := 1; i <= 3; i++ {
		entry := makeJSONEntry("task"+strings.Repeat("!", i), "pending")
		if err := backend.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}

		entries, err := backend.LoadHistory()
		if err != nil {
			t.Fatalf("LoadHistory after append #%d: %v", i, err)
		}
		if len(entries) != i {
			t.Fatalf("after %d appends: LoadHistory returned %d entries, want %d", i, len(entries), i)
		}
	}
}

// ---------------------------------------------------------------------------
// LoadHistory: fresh backend reads previously written file
// ---------------------------------------------------------------------------

func Test_JSONBackend_LoadHistory_FreshBackendReadsExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")

	b1 := storage.NewJSONBackend(path)
	if err := b1.AppendEntry(makeJSONEntry("persisted", "completed")); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	b2 := storage.NewJSONBackend(path)
	entries, err := b2.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory with fresh backend: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Task.Subject != "persisted" {
		t.Errorf("subject = %q, want %q", entries[0].Task.Subject, "persisted")
	}
}

// ---------------------------------------------------------------------------
// AppendEntry: JSON key name verification in file
// ---------------------------------------------------------------------------

func Test_JSONBackend_AppendEntry_JSONKeyNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)

	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00Z",
		SessionID: "key-check-session",
		Cwd:       "/test",
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:    "task",
			Status:     "pending",
			ActiveForm: "Doing",
		},
	}
	if err := backend.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, `"session_id"`) {
		t.Errorf("expected \"session_id\" key in JSON output")
	}
	if !strings.Contains(content, `"activeForm"`) {
		t.Errorf("expected \"activeForm\" key in JSON output")
	}
	if !strings.Contains(content, `"tool_name"`) {
		t.Errorf("expected \"tool_name\" key in JSON output")
	}
	for _, key := range []string{"timestamp", "cwd", "subject", "status", "task"} {
		if !strings.Contains(content, `"`+key+`"`) {
			t.Errorf("expected %q key in JSON output", key)
		}
	}
}

// ---------------------------------------------------------------------------
// LoadHistory with partial/malformed JSON data
// ---------------------------------------------------------------------------

func Test_JSONBackend_LoadHistory_MalformedJSON_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "truncated array", content: `[{"timestamp":"t","session_id":"s","cwd":"c","tool_name":"TaskCreate","task":{"subject":"x","status":"p"}}`},
		{name: "null literal", content: "null"},
		{name: "number literal", content: "42"},
		{name: "string literal", content: `"hello"`},
		{name: "boolean literal", content: "true"},
		{name: "nested object not array", content: `{"entries":[{"timestamp":"t"}]}`},
		{name: "whitespace only", content: "   \n\t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "todos.json")
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("write file: %v", err)
			}

			backend := storage.NewJSONBackend(path)
			entries, err := backend.LoadHistory()

			if err != nil {
				t.Fatalf("LoadHistory() returned error for %q content: %v", tt.name, err)
			}
			if len(entries) != 0 {
				t.Errorf("LoadHistory() returned %d entries for %q content, want 0", len(entries), tt.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark: JSON AppendEntry
// ---------------------------------------------------------------------------

func Benchmark_JSONBackend_AppendEntry(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)
	entry := makeJSONEntry("benchmark task", "pending")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := backend.AppendEntry(entry); err != nil {
			b.Fatalf("AppendEntry: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: JSON LoadHistory
// ---------------------------------------------------------------------------

func Benchmark_JSONBackend_LoadHistory(b *testing.B) {
	dir := b.TempDir()
	path := filepath.Join(dir, "todos.json")
	backend := storage.NewJSONBackend(path)

	for i := 0; i < 100; i++ {
		if err := backend.AppendEntry(makeJSONEntry("bench task", "pending")); err != nil {
			b.Fatalf("seed AppendEntry: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := backend.LoadHistory(); err != nil {
			b.Fatalf("LoadHistory: %v", err)
		}
	}
}
