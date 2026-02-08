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
		wantTool   string
		wantSessID string
		wantCwd    string
	}{
		{
			name:       "valid TaskCreate event returns non-nil HookInput",
			input:      `{"tool_name":"TaskCreate","tool_input":{"subject":"Test","description":"desc","activeForm":"Testing"},"session_id":"s1","cwd":"/tmp"}`,
			wantNil:    false,
			wantErr:    false,
			wantTool:   "TaskCreate",
			wantSessID: "s1",
			wantCwd:    "/tmp",
		},
		{
			name:       "valid TaskUpdate event returns non-nil HookInput",
			input:      `{"tool_name":"TaskUpdate","tool_input":{"taskId":"1","status":"completed"},"session_id":"s2","cwd":"/home"}`,
			wantNil:    false,
			wantErr:    false,
			wantTool:   "TaskUpdate",
			wantSessID: "s2",
			wantCwd:    "/home",
		},
		{
			name:    "non-task tool returns nil nil",
			input:   `{"tool_name":"Read","tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "TodoWrite is no longer accepted",
			input:   `{"tool_name":"TodoWrite","tool_input":{"todos":[]},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "TaskGet is not accepted",
			input:   `{"tool_name":"TaskGet","tool_input":{"taskId":"1"},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "TaskList is not accepted",
			input:   `{"tool_name":"TaskList","tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "missing tool_name treated as non-task",
			input:   `{"tool_input":{},"session_id":"s","cwd":"/"}`,
			wantNil: true,
			wantErr: false,
		},
		{
			name:    "empty tool_name treated as non-task",
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
			name:       "TaskCreate preserves session_id and cwd",
			input:      `{"tool_name":"TaskCreate","tool_input":{"subject":"x"},"session_id":"sess123","cwd":"/home"}`,
			wantNil:    false,
			wantErr:    false,
			wantTool:   "TaskCreate",
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

			if got.ToolName != tt.wantTool {
				t.Errorf("ReadHookInput() ToolName = %q, want %q", got.ToolName, tt.wantTool)
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

	input := `{"tool_name":"TaskCreate","tool_input":{"subject":"test task","description":"desc","activeForm":"Testing"},"session_id":"s1","cwd":"/tmp"}`
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

	if _, ok := parsed["subject"]; !ok {
		t.Error("ReadHookInput() ToolInput missing 'subject' key")
	}
}

func Test_ParseTaskInput_TaskCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawInput string
		useNil   bool
		want     storage.TaskItem
	}{
		{
			name:     "full TaskCreate input",
			rawInput: `{"subject":"Write tests","description":"Write unit tests for the parser","activeForm":"Writing tests"}`,
			want: storage.TaskItem{
				Subject:     "Write tests",
				Description: "Write unit tests for the parser",
				Status:      "pending",
				ActiveForm:  "Writing tests",
			},
		},
		{
			name:     "minimal TaskCreate input",
			rawInput: `{"subject":"Quick task"}`,
			want: storage.TaskItem{
				Subject: "Quick task",
				Status:  "pending",
			},
		},
		{
			name:   "nil input defaults to pending",
			useNil: true,
			want:   storage.TaskItem{Status: "pending"},
		},
		{
			name:     "empty JSON object",
			rawInput: `{}`,
			want:     storage.TaskItem{Status: "pending"},
		},
		{
			name:     "invalid JSON",
			rawInput: `{bad`,
			want:     storage.TaskItem{Status: "pending"},
		},
		{
			name:     "with metadata",
			rawInput: `{"subject":"Task with meta","activeForm":"Working","metadata":{"priority":"high"}}`,
			want: storage.TaskItem{
				Subject:    "Task with meta",
				Status:     "pending",
				ActiveForm: "Working",
				Metadata:   map[string]any{"priority": "high"},
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

			got := ParseTaskInput("TaskCreate", raw)

			if got.Subject != tt.want.Subject {
				t.Errorf("Subject = %q, want %q", got.Subject, tt.want.Subject)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.ActiveForm != tt.want.ActiveForm {
				t.Errorf("ActiveForm = %q, want %q", got.ActiveForm, tt.want.ActiveForm)
			}
		})
	}
}

func Test_ParseTaskInput_TaskUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawInput string
		want     storage.TaskItem
	}{
		{
			name:     "status update only",
			rawInput: `{"taskId":"1","status":"completed"}`,
			want: storage.TaskItem{
				ID:     "1",
				Status: "completed",
			},
		},
		{
			name:     "full update",
			rawInput: `{"taskId":"2","status":"in_progress","subject":"Updated subject","description":"New desc","activeForm":"Working","owner":"agent-1"}`,
			want: storage.TaskItem{
				ID:          "2",
				Subject:     "Updated subject",
				Description: "New desc",
				Status:      "in_progress",
				ActiveForm:  "Working",
				Owner:       "agent-1",
			},
		},
		{
			name:     "update with blocks",
			rawInput: `{"taskId":"3","status":"pending","addBlocks":["4","5"],"addBlockedBy":["1"]}`,
			want: storage.TaskItem{
				ID:        "3",
				Status:    "pending",
				Blocks:    []string{"4", "5"},
				BlockedBy: []string{"1"},
			},
		},
		{
			name:     "TaskUpdate with no status keeps empty",
			rawInput: `{"taskId":"4","subject":"Renamed"}`,
			want: storage.TaskItem{
				ID:      "4",
				Subject: "Renamed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			raw := json.RawMessage(tt.rawInput)
			got := ParseTaskInput("TaskUpdate", raw)

			if got.ID != tt.want.ID {
				t.Errorf("ID = %q, want %q", got.ID, tt.want.ID)
			}
			if got.Subject != tt.want.Subject {
				t.Errorf("Subject = %q, want %q", got.Subject, tt.want.Subject)
			}
			if got.Status != tt.want.Status {
				t.Errorf("Status = %q, want %q", got.Status, tt.want.Status)
			}
			if got.ActiveForm != tt.want.ActiveForm {
				t.Errorf("ActiveForm = %q, want %q", got.ActiveForm, tt.want.ActiveForm)
			}
			if got.Owner != tt.want.Owner {
				t.Errorf("Owner = %q, want %q", got.Owner, tt.want.Owner)
			}

			// Check slice fields
			if len(got.Blocks) != len(tt.want.Blocks) {
				t.Errorf("Blocks length = %d, want %d", len(got.Blocks), len(tt.want.Blocks))
			} else {
				for i := range tt.want.Blocks {
					if got.Blocks[i] != tt.want.Blocks[i] {
						t.Errorf("Blocks[%d] = %q, want %q", i, got.Blocks[i], tt.want.Blocks[i])
					}
				}
			}
			if len(got.BlockedBy) != len(tt.want.BlockedBy) {
				t.Errorf("BlockedBy length = %d, want %d", len(got.BlockedBy), len(tt.want.BlockedBy))
			} else {
				for i := range tt.want.BlockedBy {
					if got.BlockedBy[i] != tt.want.BlockedBy[i] {
						t.Errorf("BlockedBy[%d] = %q, want %q", i, got.BlockedBy[i], tt.want.BlockedBy[i])
					}
				}
			}
		})
	}
}

func Test_toStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{
			name:  "valid string slice",
			input: []any{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty slice",
			input: []any{},
			want:  nil,
		},
		{
			name:  "non-string elements filtered",
			input: []any{"a", 123, "b"},
			want:  []string{"a", "b"},
		},
		{
			name:  "all non-string elements",
			input: []any{1, 2, 3},
			want:  nil,
		},
		{
			name:  "not a slice",
			input: "string",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := toStringSlice(tt.input)

			if tt.want == nil {
				if got != nil {
					t.Errorf("toStringSlice() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("toStringSlice() length = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("toStringSlice()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
