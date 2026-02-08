package hook

import (
	"encoding/json"
	"regexp"
	"testing"
)

func Test_UTCISOTimestamp_Format(t *testing.T) {
	t.Parallel()

	ts := UTCISOTimestamp()

	// Must match ISO 8601 with millisecond precision and Z suffix
	pattern := `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`
	matched, err := regexp.MatchString(pattern, ts)
	if err != nil {
		t.Fatalf("regexp.MatchString() error: %v", err)
	}
	if !matched {
		t.Errorf("UTCISOTimestamp() = %q, does not match pattern %s", ts, pattern)
	}
}

func Test_UTCISOTimestamp_EndsWithZ(t *testing.T) {
	t.Parallel()

	ts := UTCISOTimestamp()

	if len(ts) == 0 {
		t.Fatal("UTCISOTimestamp() returned empty string")
	}
	if ts[len(ts)-1] != 'Z' {
		t.Errorf("UTCISOTimestamp() = %q, last char is %q, want 'Z'", ts, ts[len(ts)-1])
	}
}

func Test_UTCISOTimestamp_MillisecondPrecision(t *testing.T) {
	t.Parallel()

	ts := UTCISOTimestamp()

	// Extract the fractional part: between '.' and 'Z'
	pattern := regexp.MustCompile(`\.(\d+)Z$`)
	matches := pattern.FindStringSubmatch(ts)
	if len(matches) != 2 {
		t.Fatalf("UTCISOTimestamp() = %q, could not extract fractional seconds", ts)
	}
	if len(matches[1]) != 3 {
		t.Errorf("UTCISOTimestamp() fractional seconds = %q, want exactly 3 digits", matches[1])
	}
}

func Test_UTCISOTimestamp_Monotonic(t *testing.T) {
	t.Parallel()

	ts1 := UTCISOTimestamp()
	ts2 := UTCISOTimestamp()

	// Second call should be >= first (lexicographic comparison works for ISO 8601)
	if ts2 < ts1 {
		t.Errorf("UTCISOTimestamp() not monotonic: first=%q, second=%q", ts1, ts2)
	}
}

func Test_BuildLogEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         *HookInput
		wantSessionID string
		wantCwd       string
		wantTodoCount int
	}{
		{
			name: "valid input populates all fields",
			input: &HookInput{
				SessionID: "s1",
				Cwd:       "/tmp",
				ToolInput: json.RawMessage(`{"todos":[{"content":"task","status":"pending","activeForm":"Doing"}]}`),
			},
			wantSessionID: "s1",
			wantCwd:       "/tmp",
			wantTodoCount: 1,
		},
		{
			name: "missing session_id defaults to unknown",
			input: &HookInput{
				SessionID: "",
				Cwd:       "/tmp",
				ToolInput: json.RawMessage(`{"todos":[]}`),
			},
			wantSessionID: UnknownValue,
			wantCwd:       "/tmp",
			wantTodoCount: 0,
		},
		{
			name: "missing cwd defaults to unknown",
			input: &HookInput{
				SessionID: "s1",
				Cwd:       "",
				ToolInput: json.RawMessage(`{"todos":[]}`),
			},
			wantSessionID: "s1",
			wantCwd:       UnknownValue,
			wantTodoCount: 0,
		},
		{
			name: "both session_id and cwd missing default to unknown",
			input: &HookInput{
				SessionID: "",
				Cwd:       "",
				ToolInput: json.RawMessage(`{"todos":[]}`),
			},
			wantSessionID: UnknownValue,
			wantCwd:       UnknownValue,
			wantTodoCount: 0,
		},
		{
			name: "nil ToolInput produces empty todos slice",
			input: &HookInput{
				SessionID: "s1",
				Cwd:       "/tmp",
				ToolInput: nil,
			},
			wantSessionID: "s1",
			wantCwd:       "/tmp",
			wantTodoCount: 0,
		},
		{
			name: "filters invalid todos from mixed input",
			input: &HookInput{
				SessionID: "s1",
				Cwd:       "/tmp",
				ToolInput: json.RawMessage(`{"todos":[{"content":"task","status":"pending","activeForm":"Doing"},{"content":"only content"}]}`),
			},
			wantSessionID: "s1",
			wantCwd:       "/tmp",
			wantTodoCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := BuildLogEntry(tt.input)

			// Timestamp must be non-empty and well-formed
			if got.Timestamp == "" {
				t.Error("BuildLogEntry() Timestamp is empty")
			}
			pattern := `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`
			if matched, _ := regexp.MatchString(pattern, got.Timestamp); !matched {
				t.Errorf("BuildLogEntry() Timestamp = %q, does not match ISO 8601 pattern", got.Timestamp)
			}

			if got.SessionID != tt.wantSessionID {
				t.Errorf("BuildLogEntry() SessionID = %q, want %q", got.SessionID, tt.wantSessionID)
			}

			if got.Cwd != tt.wantCwd {
				t.Errorf("BuildLogEntry() Cwd = %q, want %q", got.Cwd, tt.wantCwd)
			}

			// Todos must never be nil
			if got.Todos == nil {
				t.Fatal("BuildLogEntry() Todos is nil, expected non-nil slice")
			}

			if len(got.Todos) != tt.wantTodoCount {
				t.Errorf("BuildLogEntry() Todos count = %d, want %d", len(got.Todos), tt.wantTodoCount)
			}
		})
	}
}

func Test_BuildLogEntry_TodosAlwaysNonNil(t *testing.T) {
	t.Parallel()

	// Test multiple edge cases to ensure Todos is never nil
	inputs := []*HookInput{
		{SessionID: "s", Cwd: "/", ToolInput: nil},
		{SessionID: "s", Cwd: "/", ToolInput: json.RawMessage(`{}`)},
		{SessionID: "s", Cwd: "/", ToolInput: json.RawMessage(`{"todos":[]}`)},
		{SessionID: "s", Cwd: "/", ToolInput: json.RawMessage(`{bad`)},
	}

	for _, input := range inputs {
		entry := BuildLogEntry(input)
		if entry.Todos == nil {
			t.Errorf("BuildLogEntry() with ToolInput=%s returned nil Todos", string(input.ToolInput))
		}
	}
}

func Test_UnknownValue_Constant(t *testing.T) {
	t.Parallel()

	if UnknownValue != "unknown" {
		t.Errorf("UnknownValue = %q, want %q", UnknownValue, "unknown")
	}
}

func Test_BuildLogEntry_ValidTodosContent(t *testing.T) {
	t.Parallel()

	input := &HookInput{
		SessionID: "sess1",
		Cwd:       "/project",
		ToolInput: json.RawMessage(`{"todos":[{"content":"write tests","status":"in_progress","activeForm":"Writing tests"}]}`),
	}

	entry := BuildLogEntry(input)

	if len(entry.Todos) != 1 {
		t.Fatalf("BuildLogEntry() Todos count = %d, want 1", len(entry.Todos))
	}

	todo := entry.Todos[0]
	if todo.Content != "write tests" {
		t.Errorf("todo.Content = %q, want %q", todo.Content, "write tests")
	}
	if todo.Status != "in_progress" {
		t.Errorf("todo.Status = %q, want %q", todo.Status, "in_progress")
	}
	if todo.ActiveForm != "Writing tests" {
		t.Errorf("todo.ActiveForm = %q, want %q", todo.ActiveForm, "Writing tests")
	}
}
