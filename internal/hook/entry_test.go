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
		wantToolName  string
		wantSubject   string
		wantStatus    string
	}{
		{
			name: "TaskCreate populates all fields",
			input: &HookInput{
				ToolName:  "TaskCreate",
				SessionID: "s1",
				Cwd:       "/tmp",
				ToolInput: json.RawMessage(`{"subject":"Write tests","description":"desc","activeForm":"Writing"}`),
			},
			wantSessionID: "s1",
			wantCwd:       "/tmp",
			wantToolName:  "TaskCreate",
			wantSubject:   "Write tests",
			wantStatus:    "pending",
		},
		{
			name: "TaskUpdate populates all fields",
			input: &HookInput{
				ToolName:  "TaskUpdate",
				SessionID: "s2",
				Cwd:       "/home",
				ToolInput: json.RawMessage(`{"taskId":"1","status":"completed"}`),
			},
			wantSessionID: "s2",
			wantCwd:       "/home",
			wantToolName:  "TaskUpdate",
			wantStatus:    "completed",
		},
		{
			name: "missing session_id defaults to unknown",
			input: &HookInput{
				ToolName:  "TaskCreate",
				SessionID: "",
				Cwd:       "/tmp",
				ToolInput: json.RawMessage(`{"subject":"task"}`),
			},
			wantSessionID: UnknownValue,
			wantCwd:       "/tmp",
			wantToolName:  "TaskCreate",
			wantSubject:   "task",
			wantStatus:    "pending",
		},
		{
			name: "missing cwd defaults to unknown",
			input: &HookInput{
				ToolName:  "TaskCreate",
				SessionID: "s1",
				Cwd:       "",
				ToolInput: json.RawMessage(`{"subject":"task"}`),
			},
			wantSessionID: "s1",
			wantCwd:       UnknownValue,
			wantToolName:  "TaskCreate",
			wantSubject:   "task",
			wantStatus:    "pending",
		},
		{
			name: "both session_id and cwd missing default to unknown",
			input: &HookInput{
				ToolName:  "TaskCreate",
				SessionID: "",
				Cwd:       "",
				ToolInput: json.RawMessage(`{"subject":"task"}`),
			},
			wantSessionID: UnknownValue,
			wantCwd:       UnknownValue,
			wantToolName:  "TaskCreate",
			wantSubject:   "task",
			wantStatus:    "pending",
		},
		{
			name: "nil ToolInput produces default task",
			input: &HookInput{
				ToolName:  "TaskCreate",
				SessionID: "s1",
				Cwd:       "/tmp",
				ToolInput: nil,
			},
			wantSessionID: "s1",
			wantCwd:       "/tmp",
			wantToolName:  "TaskCreate",
			wantStatus:    "pending",
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

			if got.ToolName != tt.wantToolName {
				t.Errorf("BuildLogEntry() ToolName = %q, want %q", got.ToolName, tt.wantToolName)
			}

			if got.Task.Subject != tt.wantSubject {
				t.Errorf("BuildLogEntry() Task.Subject = %q, want %q", got.Task.Subject, tt.wantSubject)
			}

			if got.Task.Status != tt.wantStatus {
				t.Errorf("BuildLogEntry() Task.Status = %q, want %q", got.Task.Status, tt.wantStatus)
			}
		})
	}
}

func Test_UnknownValue_Constant(t *testing.T) {
	t.Parallel()

	if UnknownValue != "unknown" {
		t.Errorf("UnknownValue = %q, want %q", UnknownValue, "unknown")
	}
}

func Test_BuildLogEntry_TaskCreateContent(t *testing.T) {
	t.Parallel()

	input := &HookInput{
		ToolName:  "TaskCreate",
		SessionID: "sess1",
		Cwd:       "/project",
		ToolInput: json.RawMessage(`{"subject":"write tests","description":"Unit tests for parser","activeForm":"Writing tests"}`),
	}

	entry := BuildLogEntry(input)

	if entry.Task.Subject != "write tests" {
		t.Errorf("Task.Subject = %q, want %q", entry.Task.Subject, "write tests")
	}
	if entry.Task.Description != "Unit tests for parser" {
		t.Errorf("Task.Description = %q, want %q", entry.Task.Description, "Unit tests for parser")
	}
	if entry.Task.Status != "pending" {
		t.Errorf("Task.Status = %q, want %q", entry.Task.Status, "pending")
	}
	if entry.Task.ActiveForm != "Writing tests" {
		t.Errorf("Task.ActiveForm = %q, want %q", entry.Task.ActiveForm, "Writing tests")
	}
}
