package hook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
)

func Test_ReadHookInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantNil    bool
		wantErr    bool
		wantSessID string
		wantCwd    string
	}{
		{
			name:       "valid TodoWrite event returns non-nil HookInput",
			input:      `{"tool_name":"TodoWrite","tool_input":{"todos":[]},"session_id":"s1","cwd":"/tmp"}`,
			wantNil:    false,
			wantErr:    false,
			wantSessID: "s1",
			wantCwd:    "/tmp",
		},
		{
			name:    "non-TodoWrite tool returns nil nil",
			input:   `{"tool_name":"Read","tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "missing tool_name treated as non-TodoWrite",
			input:   `{"tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "empty tool_name treated as non-TodoWrite",
			input:   `{"tool_name":"","tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "invalid JSON returns error",
			input:   `{not json}`,
			wantNil: true,
			wantErr: true,
		},
		{
			name:    "empty input returns error",
			input:   ``,
			wantNil: true,
			wantErr: true,
		},
		{
			name:       "TodoWrite preserves session_id and cwd",
			input:      `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"x","status":"y","activeForm":"z"}]},"session_id":"sess123","cwd":"/home"}`,
			wantNil:    false,
			wantErr:    false,
			wantSessID: "sess123",
			wantCwd:    "/home",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := strings.NewReader(tt.input)
			got, err := ReadHookInput(r)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("ReadHookInput() expected error, got nil")
				}
				if got != nil {
					t.Errorf("ReadHookInput() expected nil result when error, got %+v", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadHookInput() unexpected error: %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ReadHookInput() expected nil, got %+v", got)
				}
				return
			}

			// Non-nil case: verify fields
			if got == nil {
				t.Fatal("ReadHookInput() returned nil, expected non-nil *HookInput")
			}

			if got.ToolName != "TodoWrite" {
				t.Errorf("ReadHookInput() ToolName = %q, want %q", got.ToolName, "TodoWrite")
			}

			if got.SessionID != tt.wantSessID {
				t.Errorf("ReadHookInput() SessionID = %q, want %q", got.SessionID, tt.wantSessID)
			}

			if got.Cwd != tt.wantCwd {
				t.Errorf("ReadHookInput() Cwd = %q, want %q", got.Cwd, tt.wantCwd)
			}
		})
	}
}

func Test_ReadHookInput_ToolInput_IsRawJSON(t *testing.T) {
	t.Parallel()

	input := `{"tool_name":"TodoWrite","tool_input":{"todos":[{"content":"x","status":"y","activeForm":"z"}]},"session_id":"s1","cwd":"/tmp"}`
	r := strings.NewReader(input)
	got, err := ReadHookInput(r)
	if err != nil {
		t.Fatalf("ReadHookInput() unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("ReadHookInput() returned nil, expected non-nil *HookInput")
	}

	// ToolInput should be non-nil raw JSON
	if got.ToolInput == nil {
		t.Fatal("ReadHookInput() ToolInput is nil, expected raw JSON")
	}

	// Verify it can be unmarshaled as valid JSON
	var parsed map[string]any
	if err := json.Unmarshal(got.ToolInput, &parsed); err != nil {
		t.Errorf("ReadHookInput() ToolInput is not valid JSON: %v", err)
	}

	if _, ok := parsed["todos"]; !ok {
		t.Error("ReadHookInput() ToolInput missing 'todos' key")
	}
}

func Test_ValidateTodo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item map[string]any
		want bool
	}{
		{
			name: "all required keys present",
			item: map[string]any{
				"content":    "task",
				"status":     "pending",
				"activeForm": "Doing task",
			},
			want: true,
		},
		{
			name: "missing content",
			item: map[string]any{
				"status":     "pending",
				"activeForm": "Doing task",
			},
			want: false,
		},
		{
			name: "missing status",
			item: map[string]any{
				"content":    "task",
				"activeForm": "Doing task",
			},
			want: false,
		},
		{
			name: "missing activeForm",
			item: map[string]any{
				"content": "task",
				"status":  "pending",
			},
			want: false,
		},
		{
			name: "extra keys allowed",
			item: map[string]any{
				"content":    "task",
				"status":     "pending",
				"activeForm": "Doing task",
				"priority":   "high",
				"tags":       []string{"important"},
			},
			want: true,
		},
		{
			name: "empty string values are valid",
			item: map[string]any{
				"content":    "",
				"status":     "",
				"activeForm": "",
			},
			want: true,
		},
		{
			name: "non-string values are valid if keys exist",
			item: map[string]any{
				"content":    123,
				"status":     true,
				"activeForm": nil,
			},
			want: true,
		},
		{
			name: "empty map",
			item: map[string]any{},
			want: false,
		},
		{
			name: "nil map",
			item: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ValidateTodo(tt.item)
			if got != tt.want {
				t.Errorf("ValidateTodo(%v) = %v, want %v", tt.item, got, tt.want)
			}
		})
	}
}

func Test_ValidateTodos(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawInput  string
		useNil    bool
		wantCount int
		wantTodos []storage.TodoItem
	}{
		{
			name:      "valid single todo",
			rawInput:  `{"todos":[{"content":"task","status":"pending","activeForm":"Doing"}]}`,
			wantCount: 1,
			wantTodos: []storage.TodoItem{
				{Content: "task", Status: "pending", ActiveForm: "Doing"},
			},
		},
		{
			name:      "valid multiple todos",
			rawInput:  `{"todos":[{"content":"task1","status":"pending","activeForm":"Doing1"},{"content":"task2","status":"done","activeForm":"Doing2"}]}`,
			wantCount: 2,
			wantTodos: []storage.TodoItem{
				{Content: "task1", Status: "pending", ActiveForm: "Doing1"},
				{Content: "task2", Status: "done", ActiveForm: "Doing2"},
			},
		},
		{
			name:      "mixed valid and invalid filters out invalid",
			rawInput:  `{"todos":[{"content":"task","status":"pending","activeForm":"Doing"},{"status":"pending","activeForm":"Doing"}]}`,
			wantCount: 1,
			wantTodos: []storage.TodoItem{
				{Content: "task", Status: "pending", ActiveForm: "Doing"},
			},
		},
		{
			name:      "empty todos array",
			rawInput:  `{"todos":[]}`,
			wantCount: 0,
		},
		{
			name:      "nil input",
			useNil:    true,
			wantCount: 0,
		},
		{
			name:      "empty JSON object",
			rawInput:  `{}`,
			wantCount: 0,
		},
		{
			name:      "invalid JSON",
			rawInput:  `{bad`,
			wantCount: 0,
		},
		{
			name:      "todos field is not an array",
			rawInput:  `{"todos":"string"}`,
			wantCount: 0,
		},
		{
			name:      "non-string values converted to strings",
			rawInput:  `{"todos":[{"content":123,"status":true,"activeForm":"z"}]}`,
			wantCount: 1,
			wantTodos: []storage.TodoItem{
				{Content: "123", Status: "true", ActiveForm: "z"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var raw json.RawMessage
			if tt.useNil {
				raw = nil
			} else {
				raw = json.RawMessage(tt.rawInput)
			}

			got := ValidateTodos(raw)

			if got == nil {
				t.Fatal("ValidateTodos() returned nil, expected non-nil slice")
			}

			if len(got) != tt.wantCount {
				t.Fatalf("ValidateTodos() returned %d items, want %d", len(got), tt.wantCount)
			}

			if tt.wantTodos != nil {
				for i, want := range tt.wantTodos {
					if i >= len(got) {
						break
					}
					if got[i].Content != want.Content {
						t.Errorf("todo[%d].Content = %q, want %q", i, got[i].Content, want.Content)
					}
					if got[i].Status != want.Status {
						t.Errorf("todo[%d].Status = %q, want %q", i, got[i].Status, want.Status)
					}
					if got[i].ActiveForm != want.ActiveForm {
						t.Errorf("todo[%d].ActiveForm = %q, want %q", i, got[i].ActiveForm, want.ActiveForm)
					}
				}
			}
		})
	}
}

func Test_ValidateTodos_AllInvalid(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{"todos":[{"content":"a"},{"status":"b"},{"activeForm":"c"}]}`)
	got := ValidateTodos(raw)

	if got == nil {
		t.Fatal("ValidateTodos() returned nil, expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("ValidateTodos() returned %d items, want 0 (all invalid)", len(got))
	}
}

func Test_ValidateTodos_NonNilEmptySlice(t *testing.T) {
	t.Parallel()

	inputs := []json.RawMessage{
		nil,
		json.RawMessage(`{}`),
		json.RawMessage(`{"todos":[]}`),
		json.RawMessage(`{bad`),
	}

	for _, raw := range inputs {
		got := ValidateTodos(raw)
		if got == nil {
			t.Errorf("ValidateTodos(%s) returned nil, expected non-nil empty slice", string(raw))
		}
	}
}
