package storage_test

import (
	"encoding/json"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// ---------------------------------------------------------------------------
// TodoItem JSON tests
// ---------------------------------------------------------------------------

func Test_TodoItem_JSONMarshal_CamelCaseActiveForm(t *testing.T) {
	t.Parallel()

	item := storage.TodoItem{
		Content:    "task",
		Status:     "pending",
		ActiveForm: "Doing",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal(TodoItem) unexpected error: %v", err)
	}

	// Unmarshal into a generic map to inspect raw JSON key names.
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map: %v", err)
	}

	// Verify camelCase tag for activeForm
	if _, ok := raw["activeForm"]; !ok {
		t.Errorf("expected JSON key \"activeForm\" (camelCase), got keys: %v", keysOf(raw))
	}
	// Verify other keys
	for _, key := range []string{"content", "status"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q, got keys: %v", key, keysOf(raw))
		}
	}
}

func Test_TodoItem_JSONUnmarshal(t *testing.T) {
	t.Parallel()

	input := `{"content":"x","status":"y","activeForm":"z"}`

	var item storage.TodoItem
	if err := json.Unmarshal([]byte(input), &item); err != nil {
		t.Fatalf("json.Unmarshal(TodoItem) unexpected error: %v", err)
	}

	if item.Content != "x" {
		t.Errorf("Content = %q, want %q", item.Content, "x")
	}
	if item.Status != "y" {
		t.Errorf("Status = %q, want %q", item.Status, "y")
	}
	if item.ActiveForm != "z" {
		t.Errorf("ActiveForm = %q, want %q", item.ActiveForm, "z")
	}
}

func Test_TodoItem_Roundtrip(t *testing.T) {
	t.Parallel()

	original := storage.TodoItem{
		Content:    "implement feature",
		Status:     "in_progress",
		ActiveForm: "Implementing feature",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored storage.TodoItem
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if original != restored {
		t.Errorf("roundtrip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

// ---------------------------------------------------------------------------
// LogEntry JSON tests
// ---------------------------------------------------------------------------

func Test_LogEntry_JSONMarshal_Keys(t *testing.T) {
	t.Parallel()

	entry := storage.LogEntry{
		Timestamp: "2025-11-14T10:30:45.123Z",
		SessionID: "abc123",
		Cwd:       "/home/user/project",
		Todos: []storage.TodoItem{
			{Content: "task1", Status: "pending", ActiveForm: "Doing task1"},
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal(LogEntry) unexpected error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal into map: %v", err)
	}

	// session_id must be snake_case
	if _, ok := raw["session_id"]; !ok {
		t.Errorf("expected JSON key \"session_id\" (snake_case), got keys: %v", keysOf(raw))
	}

	// todos must be an array
	todosRaw, ok := raw["todos"]
	if !ok {
		t.Fatalf("expected JSON key \"todos\", got keys: %v", keysOf(raw))
	}
	todosArr, ok := todosRaw.([]interface{})
	if !ok {
		t.Fatalf("expected \"todos\" to be an array, got %T", todosRaw)
	}
	if len(todosArr) != 1 {
		t.Errorf("expected 1 todo, got %d", len(todosArr))
	}

	// Verify other expected keys
	for _, key := range []string{"timestamp", "cwd"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q, got keys: %v", key, keysOf(raw))
		}
	}
}

func Test_LogEntry_JSONUnmarshal(t *testing.T) {
	t.Parallel()

	input := `{"timestamp":"t","session_id":"s","cwd":"c","todos":[]}`

	var entry storage.LogEntry
	if err := json.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("json.Unmarshal(LogEntry) unexpected error: %v", err)
	}

	if entry.Timestamp != "t" {
		t.Errorf("Timestamp = %q, want %q", entry.Timestamp, "t")
	}
	if entry.SessionID != "s" {
		t.Errorf("SessionID = %q, want %q", entry.SessionID, "s")
	}
	if entry.Cwd != "c" {
		t.Errorf("Cwd = %q, want %q", entry.Cwd, "c")
	}
	if entry.Todos == nil {
		t.Error("Todos should not be nil after unmarshalling empty array")
	}
	if len(entry.Todos) != 0 {
		t.Errorf("Todos length = %d, want 0", len(entry.Todos))
	}
}

func Test_LogEntry_EmptyTodos_MarshalNotNull(t *testing.T) {
	t.Parallel()

	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00Z",
		SessionID: "sess1",
		Cwd:       "/tmp",
		Todos:     make([]storage.TodoItem, 0), // explicitly empty, non-nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	todosJSON := string(raw["todos"])
	if todosJSON != "[]" {
		t.Errorf("expected todos to be \"[]\", got %q", todosJSON)
	}
}

func Test_LogEntry_NilTodos_MarshalNull(t *testing.T) {
	t.Parallel()

	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00Z",
		SessionID: "sess1",
		Cwd:       "/tmp",
		Todos:     nil, // explicitly nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	todosJSON := string(raw["todos"])
	if todosJSON != "null" {
		t.Errorf("expected nil todos to marshal as \"null\", got %q", todosJSON)
	}
}

func Test_LogEntry_Roundtrip(t *testing.T) {
	t.Parallel()

	original := storage.LogEntry{
		Timestamp: "2025-11-14T10:30:45.123Z",
		SessionID: "abc123def456",
		Cwd:       "/home/user/project",
		Todos: []storage.TodoItem{
			{Content: "Task one", Status: "pending", ActiveForm: "Doing task one"},
			{Content: "Task two", Status: "completed", ActiveForm: "Done task two"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored storage.LogEntry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if original.Timestamp != restored.Timestamp {
		t.Errorf("Timestamp mismatch: %q vs %q", original.Timestamp, restored.Timestamp)
	}
	if original.SessionID != restored.SessionID {
		t.Errorf("SessionID mismatch: %q vs %q", original.SessionID, restored.SessionID)
	}
	if original.Cwd != restored.Cwd {
		t.Errorf("Cwd mismatch: %q vs %q", original.Cwd, restored.Cwd)
	}
	if len(original.Todos) != len(restored.Todos) {
		t.Fatalf("Todos length mismatch: %d vs %d", len(original.Todos), len(restored.Todos))
	}
	for i := range original.Todos {
		if original.Todos[i] != restored.Todos[i] {
			t.Errorf("Todos[%d] mismatch:\n  original: %+v\n  restored: %+v",
				i, original.Todos[i], restored.Todos[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests for comprehensive TodoItem scenarios
// ---------------------------------------------------------------------------

func Test_TodoItem_JSONMarshalUnmarshal_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item storage.TodoItem
	}{
		{
			name: "all fields populated",
			item: storage.TodoItem{Content: "do thing", Status: "pending", ActiveForm: "Doing thing"},
		},
		{
			name: "empty strings",
			item: storage.TodoItem{Content: "", Status: "", ActiveForm: ""},
		},
		{
			name: "unicode content",
			item: storage.TodoItem{Content: "tarea en espanol", Status: "pending", ActiveForm: "Haciendo tarea"},
		},
		{
			name: "special characters",
			item: storage.TodoItem{Content: `task with "quotes" and \backslash`, Status: "done", ActiveForm: "line1\nline2"},
		},
		{
			name: "long content",
			item: storage.TodoItem{
				Content:    string(make([]byte, 10000)),
				Status:     "pending",
				ActiveForm: "processing",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.item)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var got storage.TodoItem
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got != tt.item {
				t.Errorf("roundtrip mismatch:\n  want: %+v\n  got:  %+v", tt.item, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests for LogEntry edge cases
// ---------------------------------------------------------------------------

func Test_LogEntry_JSONMarshalUnmarshal_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		entry storage.LogEntry
	}{
		{
			name: "single todo",
			entry: storage.LogEntry{
				Timestamp: "2025-01-01T00:00:00Z",
				SessionID: "s1",
				Cwd:       "/project",
				Todos:     []storage.TodoItem{{Content: "one", Status: "pending", ActiveForm: "doing one"}},
			},
		},
		{
			name: "multiple todos",
			entry: storage.LogEntry{
				Timestamp: "2025-06-15T12:00:00Z",
				SessionID: "multi-session",
				Cwd:       "/home/user/workspace",
				Todos: []storage.TodoItem{
					{Content: "first", Status: "pending", ActiveForm: "Starting first"},
					{Content: "second", Status: "in_progress", ActiveForm: "Working on second"},
					{Content: "third", Status: "completed", ActiveForm: "Finished third"},
				},
			},
		},
		{
			name: "empty string fields",
			entry: storage.LogEntry{
				Timestamp: "",
				SessionID: "",
				Cwd:       "",
				Todos:     []storage.TodoItem{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := json.Marshal(tt.entry)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}

			var got storage.LogEntry
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got.Timestamp != tt.entry.Timestamp {
				t.Errorf("Timestamp: got %q, want %q", got.Timestamp, tt.entry.Timestamp)
			}
			if got.SessionID != tt.entry.SessionID {
				t.Errorf("SessionID: got %q, want %q", got.SessionID, tt.entry.SessionID)
			}
			if got.Cwd != tt.entry.Cwd {
				t.Errorf("Cwd: got %q, want %q", got.Cwd, tt.entry.Cwd)
			}
			if len(got.Todos) != len(tt.entry.Todos) {
				t.Fatalf("Todos length: got %d, want %d", len(got.Todos), len(tt.entry.Todos))
			}
			for i := range tt.entry.Todos {
				if got.Todos[i] != tt.entry.Todos[i] {
					t.Errorf("Todos[%d]: got %+v, want %+v", i, got.Todos[i], tt.entry.Todos[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
