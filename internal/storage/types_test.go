package storage_test

import (
	"encoding/json"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// ---------------------------------------------------------------------------
// TaskItem JSON tests
// ---------------------------------------------------------------------------

func Test_TaskItem_JSONMarshal_CamelCaseActiveForm(t *testing.T) {
	t.Parallel()

	item := storage.TaskItem{
		Subject:    "task",
		Status:     "pending",
		ActiveForm: "Doing",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal(TaskItem) unexpected error: %v", err)
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
	for _, key := range []string{"subject", "status"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q, got keys: %v", key, keysOf(raw))
		}
	}
}

func Test_TaskItem_JSONUnmarshal(t *testing.T) {
	t.Parallel()

	input := `{"subject":"x","status":"y","activeForm":"z"}`

	var item storage.TaskItem
	if err := json.Unmarshal([]byte(input), &item); err != nil {
		t.Fatalf("json.Unmarshal(TaskItem) unexpected error: %v", err)
	}

	if item.Subject != "x" {
		t.Errorf("Subject = %q, want %q", item.Subject, "x")
	}
	if item.Status != "y" {
		t.Errorf("Status = %q, want %q", item.Status, "y")
	}
	if item.ActiveForm != "z" {
		t.Errorf("ActiveForm = %q, want %q", item.ActiveForm, "z")
	}
}

func Test_TaskItem_Roundtrip_BasicFields(t *testing.T) {
	t.Parallel()

	original := storage.TaskItem{
		Subject:    "implement feature",
		Status:     "in_progress",
		ActiveForm: "Implementing feature",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored storage.TaskItem
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if original.Subject != restored.Subject {
		t.Errorf("Subject mismatch: %q vs %q", original.Subject, restored.Subject)
	}
	if original.Status != restored.Status {
		t.Errorf("Status mismatch: %q vs %q", original.Status, restored.Status)
	}
	if original.ActiveForm != restored.ActiveForm {
		t.Errorf("ActiveForm mismatch: %q vs %q", original.ActiveForm, restored.ActiveForm)
	}
}

func Test_TaskItem_Roundtrip_AllFields(t *testing.T) {
	t.Parallel()

	original := storage.TaskItem{
		ID:          "42",
		Subject:     "Full task",
		Description: "A complete task with all fields",
		Status:      "in_progress",
		ActiveForm:  "Working on full task",
		Owner:       "agent-1",
		Blocks:      []string{"43", "44"},
		BlockedBy:   []string{"40", "41"},
		Metadata:    map[string]any{"priority": "high", "estimate": float64(3)},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var restored storage.TaskItem
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID mismatch: %q vs %q", restored.ID, original.ID)
	}
	if restored.Subject != original.Subject {
		t.Errorf("Subject mismatch: %q vs %q", restored.Subject, original.Subject)
	}
	if restored.Description != original.Description {
		t.Errorf("Description mismatch: %q vs %q", restored.Description, original.Description)
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: %q vs %q", restored.Status, original.Status)
	}
	if restored.ActiveForm != original.ActiveForm {
		t.Errorf("ActiveForm mismatch: %q vs %q", restored.ActiveForm, original.ActiveForm)
	}
	if restored.Owner != original.Owner {
		t.Errorf("Owner mismatch: %q vs %q", restored.Owner, original.Owner)
	}
	if len(restored.Blocks) != len(original.Blocks) {
		t.Fatalf("Blocks length mismatch: %d vs %d", len(restored.Blocks), len(original.Blocks))
	}
	for i := range original.Blocks {
		if restored.Blocks[i] != original.Blocks[i] {
			t.Errorf("Blocks[%d] mismatch: %q vs %q", i, restored.Blocks[i], original.Blocks[i])
		}
	}
	if len(restored.BlockedBy) != len(original.BlockedBy) {
		t.Fatalf("BlockedBy length mismatch: %d vs %d", len(restored.BlockedBy), len(original.BlockedBy))
	}
	for i := range original.BlockedBy {
		if restored.BlockedBy[i] != original.BlockedBy[i] {
			t.Errorf("BlockedBy[%d] mismatch: %q vs %q", i, restored.BlockedBy[i], original.BlockedBy[i])
		}
	}
	if restored.Metadata["priority"] != original.Metadata["priority"] {
		t.Errorf("Metadata[priority] mismatch: %v vs %v", restored.Metadata["priority"], original.Metadata["priority"])
	}
}

func Test_TaskItem_OmitEmptyFields(t *testing.T) {
	t.Parallel()

	// A minimal TaskItem should omit empty optional fields.
	item := storage.TaskItem{
		Subject: "minimal",
		Status:  "pending",
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal into map: %v", err)
	}

	// These fields should be omitted when empty due to omitempty.
	omittedFields := []string{"id", "description", "activeForm", "owner", "blocks", "blockedBy", "metadata"}
	for _, key := range omittedFields {
		if _, ok := raw[key]; ok {
			t.Errorf("expected field %q to be omitted when empty, but it was present", key)
		}
	}

	// These fields should always be present.
	requiredFields := []string{"subject", "status"}
	for _, key := range requiredFields {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected field %q to be present, but it was missing", key)
		}
	}
}

func Test_TaskItem_JSONUnmarshal_AllFields(t *testing.T) {
	t.Parallel()

	input := `{"id":"5","subject":"s","description":"d","status":"pending","activeForm":"af","owner":"o","blocks":["1","2"],"blockedBy":["3"],"metadata":{"key":"val"}}`

	var item storage.TaskItem
	if err := json.Unmarshal([]byte(input), &item); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if item.ID != "5" {
		t.Errorf("ID = %q, want %q", item.ID, "5")
	}
	if item.Subject != "s" {
		t.Errorf("Subject = %q, want %q", item.Subject, "s")
	}
	if item.Description != "d" {
		t.Errorf("Description = %q, want %q", item.Description, "d")
	}
	if item.Status != "pending" {
		t.Errorf("Status = %q, want %q", item.Status, "pending")
	}
	if item.ActiveForm != "af" {
		t.Errorf("ActiveForm = %q, want %q", item.ActiveForm, "af")
	}
	if item.Owner != "o" {
		t.Errorf("Owner = %q, want %q", item.Owner, "o")
	}
	if len(item.Blocks) != 2 || item.Blocks[0] != "1" || item.Blocks[1] != "2" {
		t.Errorf("Blocks = %v, want [1 2]", item.Blocks)
	}
	if len(item.BlockedBy) != 1 || item.BlockedBy[0] != "3" {
		t.Errorf("BlockedBy = %v, want [3]", item.BlockedBy)
	}
	if item.Metadata["key"] != "val" {
		t.Errorf("Metadata[key] = %v, want %q", item.Metadata["key"], "val")
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
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:    "task1",
			Status:     "pending",
			ActiveForm: "Doing task1",
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

	// tool_name must be present
	if _, ok := raw["tool_name"]; !ok {
		t.Errorf("expected JSON key \"tool_name\", got keys: %v", keysOf(raw))
	}

	// task must be an object
	taskRaw, ok := raw["task"]
	if !ok {
		t.Fatalf("expected JSON key \"task\", got keys: %v", keysOf(raw))
	}
	_, ok = taskRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected \"task\" to be an object, got %T", taskRaw)
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

	input := `{"timestamp":"t","session_id":"s","cwd":"c","tool_name":"TaskCreate","task":{"subject":"subj","status":"pending"}}`

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
	if entry.ToolName != "TaskCreate" {
		t.Errorf("ToolName = %q, want %q", entry.ToolName, "TaskCreate")
	}
	if entry.Task.Subject != "subj" {
		t.Errorf("Task.Subject = %q, want %q", entry.Task.Subject, "subj")
	}
	if entry.Task.Status != "pending" {
		t.Errorf("Task.Status = %q, want %q", entry.Task.Status, "pending")
	}
}

func Test_LogEntry_TaskAlwaysPresent(t *testing.T) {
	t.Parallel()

	// Even with a zero-value Task, it should marshal as an object, not null.
	entry := storage.LogEntry{
		Timestamp: "2025-01-01T00:00:00Z",
		SessionID: "sess1",
		Cwd:       "/tmp",
		ToolName:  "TaskCreate",
		Task:      storage.TaskItem{},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}

	taskJSON := string(raw["task"])
	if taskJSON == "null" {
		t.Errorf("expected task to be an object, got %q", taskJSON)
	}
}

func Test_LogEntry_Roundtrip(t *testing.T) {
	t.Parallel()

	original := storage.LogEntry{
		Timestamp: "2025-11-14T10:30:45.123Z",
		SessionID: "abc123def456",
		Cwd:       "/home/user/project",
		ToolName:  "TaskCreate",
		Task: storage.TaskItem{
			Subject:     "Task one",
			Description: "First task description",
			Status:      "pending",
			ActiveForm:  "Doing task one",
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
	if original.ToolName != restored.ToolName {
		t.Errorf("ToolName mismatch: %q vs %q", original.ToolName, restored.ToolName)
	}
	if original.Task.Subject != restored.Task.Subject {
		t.Errorf("Task.Subject mismatch: %q vs %q", original.Task.Subject, restored.Task.Subject)
	}
	if original.Task.Description != restored.Task.Description {
		t.Errorf("Task.Description mismatch: %q vs %q", original.Task.Description, restored.Task.Description)
	}
	if original.Task.Status != restored.Task.Status {
		t.Errorf("Task.Status mismatch: %q vs %q", original.Task.Status, restored.Task.Status)
	}
	if original.Task.ActiveForm != restored.Task.ActiveForm {
		t.Errorf("Task.ActiveForm mismatch: %q vs %q", original.Task.ActiveForm, restored.Task.ActiveForm)
	}
}

// ---------------------------------------------------------------------------
// Table-driven tests for comprehensive TaskItem scenarios
// ---------------------------------------------------------------------------

func Test_TaskItem_JSONMarshalUnmarshal_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item storage.TaskItem
	}{
		{
			name: "all basic fields populated",
			item: storage.TaskItem{Subject: "do thing", Status: "pending", ActiveForm: "Doing thing"},
		},
		{
			name: "empty strings",
			item: storage.TaskItem{Subject: "", Status: "", ActiveForm: ""},
		},
		{
			name: "unicode content",
			item: storage.TaskItem{Subject: "tarea en espanol", Status: "pending", ActiveForm: "Haciendo tarea"},
		},
		{
			name: "special characters",
			item: storage.TaskItem{Subject: `task with "quotes" and \backslash`, Status: "done", ActiveForm: "line1\nline2"},
		},
		{
			name: "long content",
			item: storage.TaskItem{
				Subject:    string(make([]byte, 10000)),
				Status:     "pending",
				ActiveForm: "processing",
			},
		},
		{
			name: "with all optional fields",
			item: storage.TaskItem{
				ID:          "99",
				Subject:     "full item",
				Description: "a very full item",
				Status:      "in_progress",
				ActiveForm:  "Working",
				Owner:       "me",
				Blocks:      []string{"100"},
				BlockedBy:   []string{"98"},
				Metadata:    map[string]any{"key": "value"},
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

			var got storage.TaskItem
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}

			if got.Subject != tt.item.Subject {
				t.Errorf("Subject: got %q, want %q", got.Subject, tt.item.Subject)
			}
			if got.Status != tt.item.Status {
				t.Errorf("Status: got %q, want %q", got.Status, tt.item.Status)
			}
			if got.ActiveForm != tt.item.ActiveForm {
				t.Errorf("ActiveForm: got %q, want %q", got.ActiveForm, tt.item.ActiveForm)
			}
			if got.ID != tt.item.ID {
				t.Errorf("ID: got %q, want %q", got.ID, tt.item.ID)
			}
			if got.Description != tt.item.Description {
				t.Errorf("Description: got %q, want %q", got.Description, tt.item.Description)
			}
			if got.Owner != tt.item.Owner {
				t.Errorf("Owner: got %q, want %q", got.Owner, tt.item.Owner)
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
			name: "TaskCreate entry",
			entry: storage.LogEntry{
				Timestamp: "2025-01-01T00:00:00Z",
				SessionID: "s1",
				Cwd:       "/project",
				ToolName:  "TaskCreate",
				Task:      storage.TaskItem{Subject: "one", Status: "pending", ActiveForm: "doing one"},
			},
		},
		{
			name: "TaskUpdate entry",
			entry: storage.LogEntry{
				Timestamp: "2025-06-15T12:00:00Z",
				SessionID: "multi-session",
				Cwd:       "/home/user/workspace",
				ToolName:  "TaskUpdate",
				Task: storage.TaskItem{
					ID:     "5",
					Status: "completed",
				},
			},
		},
		{
			name: "empty string fields",
			entry: storage.LogEntry{
				Timestamp: "",
				SessionID: "",
				Cwd:       "",
				ToolName:  "",
				Task:      storage.TaskItem{},
			},
		},
		{
			name: "task with all fields",
			entry: storage.LogEntry{
				Timestamp: "2025-03-15T08:00:00Z",
				SessionID: "full-session",
				Cwd:       "/full",
				ToolName:  "TaskCreate",
				Task: storage.TaskItem{
					ID:          "10",
					Subject:     "Full task",
					Description: "A task with all fields",
					Status:      "in_progress",
					ActiveForm:  "Working",
					Owner:       "agent",
					Blocks:      []string{"11"},
					BlockedBy:   []string{"9"},
					Metadata:    map[string]any{"tag": "important"},
				},
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
			if got.ToolName != tt.entry.ToolName {
				t.Errorf("ToolName: got %q, want %q", got.ToolName, tt.entry.ToolName)
			}
			if got.Task.Subject != tt.entry.Task.Subject {
				t.Errorf("Task.Subject: got %q, want %q", got.Task.Subject, tt.entry.Task.Subject)
			}
			if got.Task.Status != tt.entry.Task.Status {
				t.Errorf("Task.Status: got %q, want %q", got.Task.Status, tt.entry.Task.Status)
			}
			if got.Task.ID != tt.entry.Task.ID {
				t.Errorf("Task.ID: got %q, want %q", got.Task.ID, tt.entry.Task.ID)
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
