package storage_test

import (
	"context"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/JamesPrial/todo-log/internal/storage"
)

// dockerAvailable checks whether the Docker daemon is reachable.
// testcontainers-go panics (rather than returning an error) when Docker
// is not installed, so we probe for it up-front.
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestPostgresBackend spins up a PostgreSQL 16 container via testcontainers-go
// and returns a fully initialised PostgresBackend together with the raw
// connection string. If Docker is not available the test is skipped.
func newTestPostgresBackend(t *testing.T) (*storage.PostgresBackend, string) {
	t.Helper()

	if !dockerAvailable() {
		t.Skip("Docker not available, skipping PostgreSQL integration tests")
	}

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Skipf("failed to start PostgreSQL container: %v", err)
	}

	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(pgContainer); err != nil {
			t.Logf("failed to terminate container: %s", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	backend, err := storage.NewPostgresBackend(connStr)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	return backend, connStr
}

// requireEqualTask compares two TaskItem values field by field, including deep
// comparison for slices and maps.
func requireEqualTask(t *testing.T, want, got storage.TaskItem) {
	t.Helper()
	if got.ID != want.ID {
		t.Errorf("Task.ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Subject != want.Subject {
		t.Errorf("Task.Subject: got %q, want %q", got.Subject, want.Subject)
	}
	if got.Description != want.Description {
		t.Errorf("Task.Description: got %q, want %q", got.Description, want.Description)
	}
	if got.Status != want.Status {
		t.Errorf("Task.Status: got %q, want %q", got.Status, want.Status)
	}
	if got.ActiveForm != want.ActiveForm {
		t.Errorf("Task.ActiveForm: got %q, want %q", got.ActiveForm, want.ActiveForm)
	}
	if got.Owner != want.Owner {
		t.Errorf("Task.Owner: got %q, want %q", got.Owner, want.Owner)
	}
	if !reflect.DeepEqual(got.Blocks, want.Blocks) {
		t.Errorf("Task.Blocks: got %v, want %v", got.Blocks, want.Blocks)
	}
	if !reflect.DeepEqual(got.BlockedBy, want.BlockedBy) {
		t.Errorf("Task.BlockedBy: got %v, want %v", got.BlockedBy, want.BlockedBy)
	}
	if !reflect.DeepEqual(got.Metadata, want.Metadata) {
		t.Errorf("Task.Metadata: got %v, want %v", got.Metadata, want.Metadata)
	}
}

// ---------------------------------------------------------------------------
// Interface compliance (no container needed)
// ---------------------------------------------------------------------------

func TestPostgres_ImplementsStorageBackend(t *testing.T) {
	var _ storage.StorageBackend = (*storage.PostgresBackend)(nil)
}

func TestPostgres_ImplementsQueryableStorageBackend(t *testing.T) {
	var _ storage.QueryableStorageBackend = (*storage.PostgresBackend)(nil)
}

// ---------------------------------------------------------------------------
// User-story tests (require Docker)
// ---------------------------------------------------------------------------

// TestPostgres_FreshDatabase verifies that a brand-new database returns
// non-nil empty slices from all query methods, preventing nil-vs-[]
// JSON serialization bugs.
func TestPostgres_FreshDatabase(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if history == nil {
		t.Error("LoadHistory returned nil, want non-nil empty slice")
	}
	if len(history) != 0 {
		t.Errorf("LoadHistory: got %d entries, want 0", len(history))
	}

	bySession, err := b.GetEntriesBySession("any")
	if err != nil {
		t.Fatalf("GetEntriesBySession: %v", err)
	}
	if bySession == nil {
		t.Error("GetEntriesBySession returned nil, want non-nil empty slice")
	}
	if len(bySession) != 0 {
		t.Errorf("GetEntriesBySession: got %d entries, want 0", len(bySession))
	}

	byStatus, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}
	if byStatus == nil {
		t.Error("GetTasksByStatus returned nil, want non-nil empty slice")
	}
	if len(byStatus) != 0 {
		t.Errorf("GetTasksByStatus: got %d entries, want 0", len(byStatus))
	}
}

// TestPostgres_TaskLifecycleAuditTrail verifies that a task's full lifecycle
// (create → start → complete) produces 3 ordered entries in history.
func TestPostgres_TaskLifecycleAuditTrail(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	entries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
			Subject: "Implement feature", Status: "pending", ActiveForm: "Implementing feature",
		}),
		makeEntry("2025-01-01T01:00:00Z", "sess1", "/project", "TaskUpdate", storage.TaskItem{
			ID: "1", Status: "in_progress", ActiveForm: "Working on feature",
		}),
		makeEntry("2025-01-01T02:00:00Z", "sess1", "/project", "TaskUpdate", storage.TaskItem{
			ID: "1", Status: "completed",
		}),
	}

	for i, e := range entries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}

	// Verify order and content
	wantToolNames := []string{"TaskCreate", "TaskUpdate", "TaskUpdate"}
	wantStatuses := []string{"pending", "in_progress", "completed"}
	for i := range history {
		if history[i].ToolName != wantToolNames[i] {
			t.Errorf("entry[%d] ToolName: got %q, want %q", i, history[i].ToolName, wantToolNames[i])
		}
		if history[i].Task.Status != wantStatuses[i] {
			t.Errorf("entry[%d] Status: got %q, want %q", i, history[i].Task.Status, wantStatuses[i])
		}
	}
}

// TestPostgres_MultiSessionIsolation verifies that interleaved writes from
// two sessions are correctly isolated when queried by session, and that
// combined history contains all entries.
func TestPostgres_MultiSessionIsolation(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	// Interleave: A, B, A, B, A
	sessions := []string{"session-A", "session-B", "session-A", "session-B", "session-A"}
	for i, sess := range sessions {
		entry := makeEntry(
			fmt.Sprintf("2025-01-01T%02d:00:00Z", i),
			sess, "/project", "TaskCreate",
			storage.TaskItem{
				Subject: fmt.Sprintf("task-%d", i), Status: "pending", ActiveForm: "Working",
			},
		)
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	entriesA, err := b.GetEntriesBySession("session-A")
	if err != nil {
		t.Fatalf("GetEntriesBySession(A): %v", err)
	}
	if len(entriesA) != 3 {
		t.Errorf("session-A: got %d entries, want 3", len(entriesA))
	}
	for _, e := range entriesA {
		if e.SessionID != "session-A" {
			t.Errorf("session-A entry has SessionID %q", e.SessionID)
		}
	}

	entriesB, err := b.GetEntriesBySession("session-B")
	if err != nil {
		t.Fatalf("GetEntriesBySession(B): %v", err)
	}
	if len(entriesB) != 2 {
		t.Errorf("session-B: got %d entries, want 2", len(entriesB))
	}

	// Order within session-A preserved (task-0, task-2, task-4)
	wantSubjects := []string{"task-0", "task-2", "task-4"}
	for i, e := range entriesA {
		if e.Task.Subject != wantSubjects[i] {
			t.Errorf("session-A entry[%d] Subject: got %q, want %q", i, e.Task.Subject, wantSubjects[i])
		}
	}

	// Combined count
	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 5 {
		t.Errorf("LoadHistory: got %d entries, want 5", len(history))
	}
}

// TestPostgres_StatusDashboard_ExactMatchOnly verifies that GetTasksByStatus
// uses exact string matching — "pending" must not match "pending_review".
func TestPostgres_StatusDashboard_ExactMatchOnly(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	statuses := []string{"pending", "in_progress", "completed", "pending", "pending_review"}
	for i, status := range statuses {
		entry := makeEntry(
			fmt.Sprintf("2025-01-01T%02d:00:00Z", i),
			"sess1", "/project", "TaskCreate",
			storage.TaskItem{
				Subject: fmt.Sprintf("task-%d", i), Status: status, ActiveForm: "Working",
			},
		)
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	pending, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus(pending): %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(pending))
	}
	for _, task := range pending {
		if task.Status != "pending" {
			t.Errorf("got status %q in pending results", task.Status)
		}
	}

	// Nonexistent status returns non-nil empty slice
	none, err := b.GetTasksByStatus("nonexistent")
	if err != nil {
		t.Fatalf("GetTasksByStatus(nonexistent): %v", err)
	}
	if none == nil {
		t.Error("GetTasksByStatus(nonexistent) returned nil, want non-nil empty slice")
	}
	if len(none) != 0 {
		t.Errorf("expected 0 tasks for nonexistent status, got %d", len(none))
	}
}

// TestPostgres_DataFidelity_UnicodeAndSpecialChars verifies that exotic
// characters in task fields survive a roundtrip through the database.
func TestPostgres_DataFidelity_UnicodeAndSpecialChars(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		desc    string
	}{
		{
			name:    "CJK characters",
			subject: "修复数据库连接",
			desc:    "データベース接続の修正",
		},
		{
			name:    "emoji",
			subject: "Fix bug 🐛",
			desc:    "Resolved the 🔥 issue in production 🚀",
		},
		{
			name:    "RTL text",
			subject: "إصلاح الخطأ",
			desc:    "תיקון באג במערכת",
		},
		{
			name:    "special chars",
			subject: "task with \"quotes\" and 'apostrophes'",
			desc:    "line1\nline2\ttab\\backslash",
		},
		{
			name:    "long string",
			subject: "scale test",
			desc:    strings.Repeat("a", 10000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := newTestPostgresBackend(t)

			entry := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
				Subject:     tt.subject,
				Description: tt.desc,
				Status:      "pending",
				ActiveForm:  "Working",
			})

			if err := b.AppendEntry(entry); err != nil {
				t.Fatalf("AppendEntry: %v", err)
			}

			history, err := b.LoadHistory()
			if err != nil {
				t.Fatalf("LoadHistory: %v", err)
			}
			if len(history) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(history))
			}

			got := history[0]
			if got.Task.Subject != tt.subject {
				t.Errorf("Subject: got %q, want %q", got.Task.Subject, tt.subject)
			}
			if got.Task.Description != tt.desc {
				t.Errorf("Description: length got %d, want %d", len(got.Task.Description), len(tt.desc))
			}
		})
	}
}

// TestPostgres_TaskDependencies_RoundtripAndNilVsEmpty verifies that
// populated dependency arrays roundtrip correctly and that unset/empty
// arrays are returned as nil (the JSONB '[]' → skip-unmarshal → nil contract).
func TestPostgres_TaskDependencies_RoundtripAndNilVsEmpty(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	// Entry with populated arrays
	populated := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject:   "blocked task",
		Status:    "pending",
		Blocks:    []string{"10", "20", "30"},
		BlockedBy: []string{"1", "2"},
	})

	// Entry with zero-value (nil) arrays
	unset := makeEntry("2025-01-01T01:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject: "free task",
		Status:  "pending",
	})

	// Entry with explicitly empty arrays
	explicitEmpty := makeEntry("2025-01-01T02:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject:   "explicit empty",
		Status:    "pending",
		Blocks:    []string{},
		BlockedBy: []string{},
	})

	for _, e := range []storage.LogEntry{populated, unset, explicitEmpty} {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry: %v", err)
		}
	}

	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}

	// Populated arrays roundtrip with correct order
	got0 := history[0].Task
	if !reflect.DeepEqual(got0.Blocks, []string{"10", "20", "30"}) {
		t.Errorf("populated Blocks: got %v, want [10 20 30]", got0.Blocks)
	}
	if !reflect.DeepEqual(got0.BlockedBy, []string{"1", "2"}) {
		t.Errorf("populated BlockedBy: got %v, want [1 2]", got0.BlockedBy)
	}

	// Unset fields return nil
	got1 := history[1].Task
	if got1.Blocks != nil {
		t.Errorf("unset Blocks: got %v, want nil", got1.Blocks)
	}
	if got1.BlockedBy != nil {
		t.Errorf("unset BlockedBy: got %v, want nil", got1.BlockedBy)
	}

	// Explicit empty also returns nil ([] stored as JSONB '[]', skipped on unmarshal)
	got2 := history[2].Task
	if got2.Blocks != nil {
		t.Errorf("explicit empty Blocks: got %v, want nil", got2.Blocks)
	}
	if got2.BlockedBy != nil {
		t.Errorf("explicit empty BlockedBy: got %v, want nil", got2.BlockedBy)
	}
}

// TestPostgres_RichMetadata_TypePreservation verifies that metadata with
// varied JSON types (string, float64, bool, nil, nested map, array) all
// survive roundtrip through both LoadHistory and GetTasksByStatus.
func TestPostgres_RichMetadata_TypePreservation(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	meta := map[string]any{
		"string_val": "hello",
		"float_val":  float64(3.14),
		"bool_true":  true,
		"bool_false": false,
		"null_val":   nil,
		"nested_map": map[string]any{"inner": "value"},
		"array_val":  []any{"a", float64(1), true},
	}

	entry := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject:  "meta task",
		Status:   "pending",
		Metadata: meta,
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	// Verify via LoadHistory
	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	gotMeta := history[0].Task.Metadata
	if !reflect.DeepEqual(gotMeta, meta) {
		t.Errorf("LoadHistory metadata mismatch:\ngot:  %#v\nwant: %#v", gotMeta, meta)
	}

	// Verify via GetTasksByStatus
	tasks, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	gotMeta2 := tasks[0].Metadata
	if !reflect.DeepEqual(gotMeta2, meta) {
		t.Errorf("GetTasksByStatus metadata mismatch:\ngot:  %#v\nwant: %#v", gotMeta2, meta)
	}
}

// TestPostgres_PluginRestartResilience verifies that data persists across
// backend instances — simulating a plugin restart by creating a second
// backend against the same database.
func TestPostgres_PluginRestartResilience(t *testing.T) {
	b1, connStr := newTestPostgresBackend(t)

	for i := 0; i < 3; i++ {
		entry := makeEntry(
			fmt.Sprintf("2025-01-01T%02d:00:00Z", i),
			"sess1", "/project", "TaskCreate",
			storage.TaskItem{
				Subject: fmt.Sprintf("task-%d", i), Status: "pending", ActiveForm: "Working",
			},
		)
		if err := b1.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d via b1: %v", i, err)
		}
	}

	// Simulate restart: new backend instance, same database
	b2, err := storage.NewPostgresBackend(connStr)
	if err != nil {
		t.Fatalf("NewPostgresBackend (restart): %v", err)
	}

	history, err := b2.LoadHistory()
	if err != nil {
		t.Fatalf("b2.LoadHistory: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("b2.LoadHistory: got %d entries, want 3", len(history))
	}

	bySession, err := b2.GetEntriesBySession("sess1")
	if err != nil {
		t.Fatalf("b2.GetEntriesBySession: %v", err)
	}
	if len(bySession) != 3 {
		t.Errorf("b2.GetEntriesBySession: got %d entries, want 3", len(bySession))
	}

	byStatus, err := b2.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("b2.GetTasksByStatus: %v", err)
	}
	if len(byStatus) != 3 {
		t.Errorf("b2.GetTasksByStatus: got %d tasks, want 3", len(byStatus))
	}
}

// TestPostgres_AccumulationAtScale verifies correct behavior with 250
// entries across 5 sessions — all present, correct order, correct filtering.
func TestPostgres_AccumulationAtScale(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	const total = 250
	const numSessions = 5
	sessionIDs := make([]string, numSessions)
	for i := range sessionIDs {
		sessionIDs[i] = fmt.Sprintf("session-%d", i)
	}

	for i := 0; i < total; i++ {
		entry := makeEntry(
			fmt.Sprintf("2025-01-01T%02d:%02d:%02dZ", i/3600, (i%3600)/60, i%60),
			sessionIDs[i%numSessions], "/project", "TaskCreate",
			storage.TaskItem{
				Subject: fmt.Sprintf("task-%d", i), Status: "pending", ActiveForm: "Working",
			},
		)
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	// Full history
	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != total {
		t.Fatalf("LoadHistory: got %d entries, want %d", len(history), total)
	}

	// Verify insertion order via subjects
	for i, e := range history {
		want := fmt.Sprintf("task-%d", i)
		if e.Task.Subject != want {
			t.Errorf("entry[%d] Subject: got %q, want %q", i, e.Task.Subject, want)
			break // Don't spam 250 errors
		}
	}

	// Each session should have 50 entries
	for _, sess := range sessionIDs {
		entries, err := b.GetEntriesBySession(sess)
		if err != nil {
			t.Fatalf("GetEntriesBySession(%s): %v", sess, err)
		}
		if len(entries) != total/numSessions {
			t.Errorf("session %s: got %d entries, want %d", sess, len(entries), total/numSessions)
		}
	}
}

// TestPostgres_ConcurrentWrites verifies that 10 goroutines each writing
// 20 entries simultaneously produces no data loss.
func TestPostgres_ConcurrentWrites(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	const goroutines = 10
	const entriesPerGoroutine = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*entriesPerGoroutine)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			sessID := fmt.Sprintf("goroutine-%d", gID)
			for i := 0; i < entriesPerGoroutine; i++ {
				entry := makeEntry(
					fmt.Sprintf("2025-01-01T00:%02d:%02dZ", gID, i),
					sessID, "/project", "TaskCreate",
					storage.TaskItem{
						Subject: fmt.Sprintf("g%d-task-%d", gID, i), Status: "pending", ActiveForm: "Working",
					},
				)
				if err := b.AppendEntry(entry); err != nil {
					errCh <- fmt.Errorf("goroutine %d, entry %d: %w", gID, i, err)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent write error: %v", err)
	}

	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	wantTotal := goroutines * entriesPerGoroutine
	if len(history) != wantTotal {
		t.Errorf("total entries: got %d, want %d", len(history), wantTotal)
	}

	// Each session should have exactly entriesPerGoroutine entries
	for g := 0; g < goroutines; g++ {
		sessID := fmt.Sprintf("goroutine-%d", g)
		entries, err := b.GetEntriesBySession(sessID)
		if err != nil {
			t.Fatalf("GetEntriesBySession(%s): %v", sessID, err)
		}
		if len(entries) != entriesPerGoroutine {
			t.Errorf("session %s: got %d entries, want %d", sessID, len(entries), entriesPerGoroutine)
		}
	}
}

// TestPostgres_OptionalFieldDefaults verifies that a minimal task (only
// Subject+Status) has zero-value defaults for all optional fields.
func TestPostgres_OptionalFieldDefaults(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject: "minimal task",
		Status:  "pending",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	// Verify via LoadHistory
	history, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}

	got := history[0].Task
	if got.ID != "" {
		t.Errorf("ID: got %q, want empty", got.ID)
	}
	if got.Description != "" {
		t.Errorf("Description: got %q, want empty", got.Description)
	}
	if got.ActiveForm != "" {
		t.Errorf("ActiveForm: got %q, want empty", got.ActiveForm)
	}
	if got.Owner != "" {
		t.Errorf("Owner: got %q, want empty", got.Owner)
	}
	if got.Blocks != nil {
		t.Errorf("Blocks: got %v, want nil", got.Blocks)
	}
	if got.BlockedBy != nil {
		t.Errorf("BlockedBy: got %v, want nil", got.BlockedBy)
	}
	if got.Metadata != nil {
		t.Errorf("Metadata: got %v, want nil", got.Metadata)
	}

	// Verify same via GetTasksByStatus
	tasks, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	gotTask := tasks[0]
	if gotTask.ID != "" {
		t.Errorf("GetTasksByStatus ID: got %q, want empty", gotTask.ID)
	}
	if gotTask.Description != "" {
		t.Errorf("GetTasksByStatus Description: got %q, want empty", gotTask.Description)
	}
	if gotTask.ActiveForm != "" {
		t.Errorf("GetTasksByStatus ActiveForm: got %q, want empty", gotTask.ActiveForm)
	}
	if gotTask.Owner != "" {
		t.Errorf("GetTasksByStatus Owner: got %q, want empty", gotTask.Owner)
	}
	if gotTask.Blocks != nil {
		t.Errorf("GetTasksByStatus Blocks: got %v, want nil", gotTask.Blocks)
	}
	if gotTask.BlockedBy != nil {
		t.Errorf("GetTasksByStatus BlockedBy: got %v, want nil", gotTask.BlockedBy)
	}
	if gotTask.Metadata != nil {
		t.Errorf("GetTasksByStatus Metadata: got %v, want nil", gotTask.Metadata)
	}
}

// TestPostgres_IdempotentSchema verifies that two backend instances on the
// same database can both read and write without errors.
func TestPostgres_IdempotentSchema(t *testing.T) {
	b1, connStr := newTestPostgresBackend(t)

	entry1 := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject: "from-b1", Status: "pending", ActiveForm: "Working",
	})
	if err := b1.AppendEntry(entry1); err != nil {
		t.Fatalf("b1.AppendEntry: %v", err)
	}

	b2, err := storage.NewPostgresBackend(connStr)
	if err != nil {
		t.Fatalf("NewPostgresBackend (b2): %v", err)
	}

	// b2 reads b1's entry
	history, err := b2.LoadHistory()
	if err != nil {
		t.Fatalf("b2.LoadHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("b2 sees %d entries, want 1", len(history))
	}
	if history[0].Task.Subject != "from-b1" {
		t.Errorf("b2 entry Subject: got %q, want %q", history[0].Task.Subject, "from-b1")
	}

	// b2 writes another entry
	entry2 := makeEntry("2025-01-01T01:00:00Z", "sess2", "/project", "TaskCreate", storage.TaskItem{
		Subject: "from-b2", Status: "pending", ActiveForm: "Working",
	})
	if err := b2.AppendEntry(entry2); err != nil {
		t.Fatalf("b2.AppendEntry: %v", err)
	}

	// b1 reads both
	history, err = b1.LoadHistory()
	if err != nil {
		t.Fatalf("b1.LoadHistory (after b2 write): %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("b1 sees %d entries, want 2", len(history))
	}
}

// TestPostgres_GetTasksByStatus_ReturnsCompleteTaskItem verifies that the
// status query returns fully populated TaskItem with all fields intact.
func TestPostgres_GetTasksByStatus_ReturnsCompleteTaskItem(t *testing.T) {
	b, _ := newTestPostgresBackend(t)

	wantTask := storage.TaskItem{
		ID:          "42",
		Subject:     "Complete task",
		Description: "A fully populated task entry",
		Status:      "in_progress",
		ActiveForm:  "Working on complete task",
		Owner:       "agent-7",
		Blocks:      []string{"100", "101"},
		BlockedBy:   []string{"98"},
		Metadata:    map[string]any{"priority": "critical", "points": float64(13)},
	}

	entry := makeEntry("2025-03-15T09:00:00Z", "s-full", "/project", "TaskCreate", wantTask)
	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	tasks, err := b.GetTasksByStatus("in_progress")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	requireEqualTask(t, wantTask, tasks[0])
}
