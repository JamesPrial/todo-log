package storage_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/JamesPrial/todo-log/internal/storage"
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestBackend creates a SQLiteBackend using a temporary directory managed
// by t.TempDir(). The database file is automatically cleaned up when the
// test finishes.
func newTestBackend(t *testing.T) (*storage.SQLiteBackend, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	b, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	return b, dbPath
}

// makeEntry is a convenience constructor for LogEntry values used in tests.
func makeEntry(ts, session, cwd, toolName string, task storage.TaskItem) storage.LogEntry {
	return storage.LogEntry{
		Timestamp: ts,
		SessionID: session,
		Cwd:       cwd,
		ToolName:  toolName,
		Task:      task,
	}
}

// openDirectDB opens a direct sql.DB connection for schema verification,
// bypassing the backend abstraction.
func openDirectDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db directly: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Schema tests
// ---------------------------------------------------------------------------

func Test_NewSQLiteBackend_CreatesLogEntriesTable(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)
	var name string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='log_entries'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("log_entries table not found: %v", err)
	}
	if name != "log_entries" {
		t.Errorf("expected table name 'log_entries', got %q", name)
	}
}

func Test_NewSQLiteBackend_NoSeparateTodosTable(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='todos'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	if count != 0 {
		t.Errorf("expected no 'todos' table (denormalized schema), but found one")
	}
}

func Test_NewSQLiteBackend_CreatesIndexes(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)

	tests := []struct {
		name      string
		indexName string
	}{
		{name: "session index", indexName: "idx_entries_session"},
		{name: "status index", indexName: "idx_entries_status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var found string
			err := db.QueryRow(
				`SELECT name FROM sqlite_master WHERE type='index' AND name=?`,
				tt.indexName,
			).Scan(&found)
			if err != nil {
				t.Fatalf("index %q not found: %v", tt.indexName, err)
			}
			if found != tt.indexName {
				t.Errorf("expected index %q, got %q", tt.indexName, found)
			}
		})
	}
}

func Test_NewSQLiteBackend_Idempotent(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	b1, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("first NewSQLiteBackend: %v", err)
	}
	_ = b1

	b2, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("second NewSQLiteBackend on same path: %v", err)
	}
	_ = b2
}

func Test_NewSQLiteBackend_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "a", "b", "c", "deep.db")

	_, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteBackend with nested dirs: %v", err)
	}
}

func Test_NewSQLiteBackend_WALMode(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode query failed: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", mode)
	}
}

func Test_NewSQLiteBackend_LogEntriesColumns(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)
	rows, err := db.Query(`PRAGMA table_info(log_entries)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	defer func() { _ = rows.Close() }()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scan column info: %v", err)
		}
		columns[name] = true
	}

	expected := []string{
		"id", "timestamp", "session_id", "cwd", "tool_name",
		"task_id", "subject", "description", "status", "active_form",
		"owner", "blocks", "blocked_by", "metadata",
	}
	for _, col := range expected {
		if !columns[col] {
			t.Errorf("expected column %q in log_entries, not found. Columns: %v", col, columns)
		}
	}
}

// ---------------------------------------------------------------------------
// AppendEntry tests
// ---------------------------------------------------------------------------

func Test_AppendEntry_CorrectFieldsInLogEntries(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry(
		"2025-11-14T10:30:45.123Z",
		"session-abc",
		"/home/user/project",
		"TaskCreate",
		storage.TaskItem{Subject: "task", Status: "pending", ActiveForm: "Doing task"},
	)

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var ts, sid, cwd, toolName, subject, status, activeForm string
	err := db.QueryRow(
		`SELECT timestamp, session_id, cwd, tool_name, subject, status, active_form FROM log_entries WHERE id=1`,
	).Scan(&ts, &sid, &cwd, &toolName, &subject, &status, &activeForm)
	if err != nil {
		t.Fatalf("query log_entries: %v", err)
	}

	if ts != entry.Timestamp {
		t.Errorf("timestamp: got %q, want %q", ts, entry.Timestamp)
	}
	if sid != entry.SessionID {
		t.Errorf("session_id: got %q, want %q", sid, entry.SessionID)
	}
	if cwd != entry.Cwd {
		t.Errorf("cwd: got %q, want %q", cwd, entry.Cwd)
	}
	if toolName != entry.ToolName {
		t.Errorf("tool_name: got %q, want %q", toolName, entry.ToolName)
	}
	if subject != entry.Task.Subject {
		t.Errorf("subject: got %q, want %q", subject, entry.Task.Subject)
	}
	if status != entry.Task.Status {
		t.Errorf("status: got %q, want %q", status, entry.Task.Status)
	}
	if activeForm != entry.Task.ActiveForm {
		t.Errorf("active_form: got %q, want %q", activeForm, entry.Task.ActiveForm)
	}
}

func Test_AppendEntry_TaskFieldsStored(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskUpdate", storage.TaskItem{
		ID:          "42",
		Subject:     "Updated task",
		Description: "A description",
		Status:      "in_progress",
		ActiveForm:  "Working",
		Owner:       "agent-1",
		Blocks:      []string{"43", "44"},
		BlockedBy:   []string{"41"},
		Metadata:    map[string]any{"priority": "high"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var taskID, subject, desc, status, activeForm, owner, blocksJSON, blockedByJSON, metaJSON string
	err := db.QueryRow(
		`SELECT task_id, subject, description, status, active_form, owner, blocks, blocked_by, metadata FROM log_entries WHERE id=1`,
	).Scan(&taskID, &subject, &desc, &status, &activeForm, &owner, &blocksJSON, &blockedByJSON, &metaJSON)
	if err != nil {
		t.Fatalf("query log_entries: %v", err)
	}

	if taskID != "42" {
		t.Errorf("task_id: got %q, want %q", taskID, "42")
	}
	if subject != "Updated task" {
		t.Errorf("subject: got %q, want %q", subject, "Updated task")
	}
	if desc != "A description" {
		t.Errorf("description: got %q, want %q", desc, "A description")
	}
	if status != "in_progress" {
		t.Errorf("status: got %q, want %q", status, "in_progress")
	}
	if owner != "agent-1" {
		t.Errorf("owner: got %q, want %q", owner, "agent-1")
	}
	if blocksJSON != `["43","44"]` {
		t.Errorf("blocks: got %q, want %q", blocksJSON, `["43","44"]`)
	}
	if blockedByJSON != `["41"]` {
		t.Errorf("blocked_by: got %q, want %q", blockedByJSON, `["41"]`)
	}
}

func Test_AppendEntry_UnicodePreserved(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "unicode-sess", "/tmp", "TaskCreate", storage.TaskItem{
		Subject:    "tarea en espanol",
		Status:     "pendiente",
		ActiveForm: "Haciendo tarea",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var subject, status, activeForm string
	err := db.QueryRow(`SELECT subject, status, active_form FROM log_entries WHERE id=1`).Scan(&subject, &status, &activeForm)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	if subject != "tarea en espanol" {
		t.Errorf("subject: got %q, want %q", subject, "tarea en espanol")
	}
	if status != "pendiente" {
		t.Errorf("status: got %q, want %q", status, "pendiente")
	}
	if activeForm != "Haciendo tarea" {
		t.Errorf("active_form: got %q, want %q", activeForm, "Haciendo tarea")
	}
}

func Test_AppendEntry_AutoIncrementIDs(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	for i := 0; i < 3; i++ {
		entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: "pending", ActiveForm: "Doing",
		})
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i+1, err)
		}
	}

	db := openDirectDB(t, dbPath)
	rows, err := db.Query(`SELECT id FROM log_entries ORDER BY id`)
	if err != nil {
		t.Fatalf("query log_entries ids: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	if len(ids) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ids))
	}
	for i, wantID := range []int{1, 2, 3} {
		if ids[i] != wantID {
			t.Errorf("entry %d: id=%d, want %d", i, ids[i], wantID)
		}
	}
}

func Test_AppendEntry_ActiveFormToActiveFormColumn(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task", Status: "pending", ActiveForm: "Doing the task now",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var activeForm string
	err := db.QueryRow(`SELECT active_form FROM log_entries WHERE id=1`).Scan(&activeForm)
	if err != nil {
		t.Fatalf("query active_form: %v", err)
	}
	if activeForm != "Doing the task now" {
		t.Errorf("active_form: got %q, want %q", activeForm, "Doing the task now")
	}
}

// ---------------------------------------------------------------------------
// LoadHistory tests
// ---------------------------------------------------------------------------

func Test_LoadHistory_EmptyDatabase(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory on empty db: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func Test_LoadHistory_EntryWithTask(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	original := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", "TaskCreate", storage.TaskItem{
		Subject: "task1", Status: "pending", ActiveForm: "Doing task1",
	})

	if err := b.AppendEntry(original); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0]
	if got.Timestamp != original.Timestamp {
		t.Errorf("Timestamp: got %q, want %q", got.Timestamp, original.Timestamp)
	}
	if got.SessionID != original.SessionID {
		t.Errorf("SessionID: got %q, want %q", got.SessionID, original.SessionID)
	}
	if got.Cwd != original.Cwd {
		t.Errorf("Cwd: got %q, want %q", got.Cwd, original.Cwd)
	}
	if got.ToolName != original.ToolName {
		t.Errorf("ToolName: got %q, want %q", got.ToolName, original.ToolName)
	}
	if got.Task.Subject != original.Task.Subject {
		t.Errorf("Task.Subject: got %q, want %q", got.Task.Subject, original.Task.Subject)
	}
	if got.Task.Status != original.Task.Status {
		t.Errorf("Task.Status: got %q, want %q", got.Task.Status, original.Task.Status)
	}
	if got.Task.ActiveForm != original.Task.ActiveForm {
		t.Errorf("Task.ActiveForm: got %q, want %q", got.Task.ActiveForm, original.Task.ActiveForm)
	}
}

func Test_LoadHistory_MaintainsInsertionOrder(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	timestamps := []string{
		"2025-01-01T00:00:00Z",
		"2025-01-02T00:00:00Z",
		"2025-01-03T00:00:00Z",
	}

	for i, ts := range timestamps {
		entry := makeEntry(ts, "sess", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: "pending", ActiveForm: "Doing",
		})
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	for i, want := range timestamps {
		if entries[i].Timestamp != want {
			t.Errorf("entry[%d] timestamp: got %q, want %q", i, entries[i].Timestamp, want)
		}
	}
}

func Test_LoadHistory_ActiveFormMapping(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task", Status: "pending", ActiveForm: "Working on task right now",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("unexpected result shape: %d entries", len(entries))
	}

	got := entries[0].Task.ActiveForm
	if got != "Working on task right now" {
		t.Errorf("ActiveForm: got %q, want %q", got, "Working on task right now")
	}
}

func Test_LoadHistory_UnicodePreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "unicode-sess", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "tarea en espanol", Status: "pendiente", ActiveForm: "Haciendo tarea",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0].Task
	if got.Subject != "tarea en espanol" {
		t.Errorf("Subject: got %q, want %q", got.Subject, "tarea en espanol")
	}
	if got.Status != "pendiente" {
		t.Errorf("Status: got %q, want %q", got.Status, "pendiente")
	}
	if got.ActiveForm != "Haciendo tarea" {
		t.Errorf("ActiveForm: got %q, want %q", got.ActiveForm, "Haciendo tarea")
	}
}

func Test_LoadHistory_MultipleEntries(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "s1", "/proj1", "TaskCreate", storage.TaskItem{
			Subject: "t1", Status: "pending", ActiveForm: "Doing t1",
		}),
		makeEntry("2025-01-02T00:00:00Z", "s2", "/proj2", "TaskUpdate", storage.TaskItem{
			ID: "1", Status: "in_progress", ActiveForm: "Working t1",
		}),
		makeEntry("2025-01-03T00:00:00Z", "s1", "/proj1", "TaskCreate", storage.TaskItem{
			Subject: "t2", Status: "pending", ActiveForm: "Starting t2",
		}),
	}

	for i, e := range inputEntries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	loaded, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(loaded) != len(inputEntries) {
		t.Fatalf("expected %d entries, got %d", len(inputEntries), len(loaded))
	}

	for i, want := range inputEntries {
		got := loaded[i]
		if got.Timestamp != want.Timestamp {
			t.Errorf("entry[%d] timestamp: got %q, want %q", i, got.Timestamp, want.Timestamp)
		}
		if got.SessionID != want.SessionID {
			t.Errorf("entry[%d] session_id: got %q, want %q", i, got.SessionID, want.SessionID)
		}
		if got.ToolName != want.ToolName {
			t.Errorf("entry[%d] tool_name: got %q, want %q", i, got.ToolName, want.ToolName)
		}
		if got.Task.Subject != want.Task.Subject {
			t.Errorf("entry[%d] task.subject: got %q, want %q", i, got.Task.Subject, want.Task.Subject)
		}
		if got.Task.Status != want.Task.Status {
			t.Errorf("entry[%d] task.status: got %q, want %q", i, got.Task.Status, want.Task.Status)
		}
	}
}

func Test_LoadHistory_BlocksAndBlockedByPreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskUpdate", storage.TaskItem{
		ID:        "5",
		Status:    "pending",
		Blocks:    []string{"6", "7"},
		BlockedBy: []string{"3", "4"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0].Task
	if len(got.Blocks) != 2 || got.Blocks[0] != "6" || got.Blocks[1] != "7" {
		t.Errorf("Blocks: got %v, want [6 7]", got.Blocks)
	}
	if len(got.BlockedBy) != 2 || got.BlockedBy[0] != "3" || got.BlockedBy[1] != "4" {
		t.Errorf("BlockedBy: got %v, want [3 4]", got.BlockedBy)
	}
}

func Test_LoadHistory_MetadataPreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject:  "meta task",
		Status:   "pending",
		Metadata: map[string]any{"priority": "high", "estimate": float64(5)},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	got := entries[0].Task.Metadata
	if got["priority"] != "high" {
		t.Errorf("Metadata[priority]: got %v, want %q", got["priority"], "high")
	}
}

// ---------------------------------------------------------------------------
// GetEntriesBySession tests
// ---------------------------------------------------------------------------

func Test_GetEntriesBySession_MatchingEntries(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "session-A", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "t1", Status: "pending", ActiveForm: "Doing t1",
		}),
		makeEntry("2025-01-02T00:00:00Z", "session-B", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "t2", Status: "pending", ActiveForm: "Doing t2",
		}),
		makeEntry("2025-01-03T00:00:00Z", "session-A", "/cwd", "TaskUpdate", storage.TaskItem{
			ID: "1", Status: "completed",
		}),
	}

	for i, e := range inputEntries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	results, err := b.GetEntriesBySession("session-A")
	if err != nil {
		t.Fatalf("GetEntriesBySession: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 entries for session-A, got %d", len(results))
	}

	for _, r := range results {
		if r.SessionID != "session-A" {
			t.Errorf("unexpected session_id: got %q, want %q", r.SessionID, "session-A")
		}
	}
}

func Test_GetEntriesBySession_UnknownSession(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "existing-sess", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task", Status: "pending", ActiveForm: "Doing",
	})
	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetEntriesBySession("nonexistent")
	if err != nil {
		t.Fatalf("GetEntriesBySession: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty slice for unknown session, got %d entries", len(results))
	}
}

func Test_GetEntriesBySession_IncludesTaskData(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "my-session", "/cwd", "TaskCreate", storage.TaskItem{
		Subject:     "task1",
		Description: "a detailed task",
		Status:      "pending",
		ActiveForm:  "Doing task1",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetEntriesBySession("my-session")
	if err != nil {
		t.Fatalf("GetEntriesBySession: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(results))
	}

	got := results[0].Task
	if got.Subject != "task1" {
		t.Errorf("Task.Subject: got %q, want %q", got.Subject, "task1")
	}
	if got.Description != "a detailed task" {
		t.Errorf("Task.Description: got %q, want %q", got.Description, "a detailed task")
	}
	if got.ActiveForm != "Doing task1" {
		t.Errorf("Task.ActiveForm: got %q, want %q", got.ActiveForm, "Doing task1")
	}
}

func Test_GetEntriesBySession_ChronologicalOrder(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	timestamps := []string{
		"2025-01-01T00:00:00Z",
		"2025-01-02T00:00:00Z",
		"2025-01-03T00:00:00Z",
	}

	for i, ts := range timestamps {
		entry := makeEntry(ts, "same-session", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: "pending", ActiveForm: "Doing",
		})
		if err := b.AppendEntry(entry); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	results, err := b.GetEntriesBySession("same-session")
	if err != nil {
		t.Fatalf("GetEntriesBySession: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(results))
	}

	for i, wantTS := range timestamps {
		if results[i].Timestamp != wantTS {
			t.Errorf("entry[%d] timestamp: got %q, want %q", i, results[i].Timestamp, wantTS)
		}
	}
}

// ---------------------------------------------------------------------------
// GetTasksByStatus tests
// ---------------------------------------------------------------------------

func Test_GetTasksByStatus_MatchingStatus(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task1", Status: "pending", ActiveForm: "Doing task1",
		}),
		makeEntry("2025-01-02T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task2", Status: "completed", ActiveForm: "Done task2",
		}),
		makeEntry("2025-01-03T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task3", Status: "pending", ActiveForm: "Doing task3",
		}),
		makeEntry("2025-01-04T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task4", Status: "in_progress", ActiveForm: "Working task4",
		}),
	}

	for i, e := range entries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	results, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 pending tasks, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pending" {
			t.Errorf("unexpected status: got %q, want %q", r.Status, "pending")
		}
	}
}

func Test_GetTasksByStatus_UnknownStatus(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task", Status: "pending", ActiveForm: "Doing",
	})
	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTasksByStatus("nonexistent")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty slice for unknown status, got %d tasks", len(results))
	}
}

func Test_GetTasksByStatus_AcrossEntries(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "entry1-pending", Status: "pending", ActiveForm: "Doing entry1",
		}),
		makeEntry("2025-01-02T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "entry2-done", Status: "completed", ActiveForm: "Done entry2",
		}),
		makeEntry("2025-01-03T00:00:00Z", "s2", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "entry3-pending", Status: "pending", ActiveForm: "Doing entry3",
		}),
		makeEntry("2025-01-04T00:00:00Z", "s3", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "entry4-pending", Status: "pending", ActiveForm: "Doing entry4",
		}),
	}

	for i, e := range inputEntries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	results, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 pending tasks across entries, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pending" {
			t.Errorf("unexpected status: got %q, want %q", r.Status, "pending")
		}
	}
}

func Test_GetTasksByStatus_CompleteStructure(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "my task", Status: "pending", ActiveForm: "Working on my task",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTasksByStatus("pending")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 task, got %d", len(results))
	}

	got := results[0]
	if got.Subject != "my task" {
		t.Errorf("Subject: got %q, want %q", got.Subject, "my task")
	}
	if got.Status != "pending" {
		t.Errorf("Status: got %q, want %q", got.Status, "pending")
	}
	if got.ActiveForm != "Working on my task" {
		t.Errorf("ActiveForm: got %q, want %q", got.ActiveForm, "Working on my task")
	}
}

func Test_GetTasksByStatus_ActiveFormPreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task", Status: "in_progress", ActiveForm: "Doing task",
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTasksByStatus("in_progress")
	if err != nil {
		t.Fatalf("GetTasksByStatus: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 task, got %d", len(results))
	}

	if results[0].ActiveForm != "Doing task" {
		t.Errorf("ActiveForm: got %q, want %q", results[0].ActiveForm, "Doing task")
	}
}

// ---------------------------------------------------------------------------
// Interface compliance tests
// ---------------------------------------------------------------------------

func Test_SQLiteBackend_ImplementsStorageBackend(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	var _ storage.StorageBackend = b
}

func Test_SQLiteBackend_ImplementsQueryableStorageBackend(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	var _ storage.QueryableStorageBackend = b
}

// ---------------------------------------------------------------------------
// Roundtrip integration tests (Append -> Load)
// ---------------------------------------------------------------------------

func Test_AppendThenLoad_Roundtrip_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []storage.LogEntry
	}{
		{
			name: "single entry minimal task",
			entries: []storage.LogEntry{
				makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
					Subject: "only-task", Status: "pending", ActiveForm: "Doing only-task",
				}),
			},
		},
		{
			name: "multiple entries mixed tools",
			entries: []storage.LogEntry{
				makeEntry("2025-01-01T00:00:00Z", "s1", "/proj", "TaskCreate", storage.TaskItem{
					Subject: "a", Status: "pending", ActiveForm: "Doing a",
				}),
				makeEntry("2025-01-02T00:00:00Z", "s2", "/proj2", "TaskUpdate", storage.TaskItem{
					ID: "1", Status: "completed",
				}),
				makeEntry("2025-01-03T00:00:00Z", "s1", "/proj", "TaskCreate", storage.TaskItem{
					Subject: "c", Status: "in_progress", ActiveForm: "Working c",
				}),
			},
		},
		{
			name: "unicode in all fields",
			entries: []storage.LogEntry{
				makeEntry("2025-06-15T12:00:00Z", "sess-unicode", "/home/user", "TaskCreate", storage.TaskItem{
					Subject: "tarea en espanol", Status: "pendiente", ActiveForm: "Haciendo tarea",
				}),
			},
		},
		{
			name: "empty string fields",
			entries: []storage.LogEntry{
				makeEntry("", "", "", "", storage.TaskItem{
					Subject: "", Status: "", ActiveForm: "",
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, _ := newTestBackend(t)

			for i, e := range tt.entries {
				if err := b.AppendEntry(e); err != nil {
					t.Fatalf("AppendEntry #%d: %v", i, err)
				}
			}

			loaded, err := b.LoadHistory()
			if err != nil {
				t.Fatalf("LoadHistory: %v", err)
			}

			if len(loaded) != len(tt.entries) {
				t.Fatalf("entries count: got %d, want %d", len(loaded), len(tt.entries))
			}

			for i, want := range tt.entries {
				got := loaded[i]
				if got.Timestamp != want.Timestamp {
					t.Errorf("entry[%d] Timestamp: got %q, want %q", i, got.Timestamp, want.Timestamp)
				}
				if got.SessionID != want.SessionID {
					t.Errorf("entry[%d] SessionID: got %q, want %q", i, got.SessionID, want.SessionID)
				}
				if got.Cwd != want.Cwd {
					t.Errorf("entry[%d] Cwd: got %q, want %q", i, got.Cwd, want.Cwd)
				}
				if got.ToolName != want.ToolName {
					t.Errorf("entry[%d] ToolName: got %q, want %q", i, got.ToolName, want.ToolName)
				}
				if got.Task.Subject != want.Task.Subject {
					t.Errorf("entry[%d] Task.Subject: got %q, want %q", i, got.Task.Subject, want.Task.Subject)
				}
				if got.Task.Status != want.Task.Status {
					t.Errorf("entry[%d] Task.Status: got %q, want %q", i, got.Task.Status, want.Task.Status)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmark tests
// ---------------------------------------------------------------------------

func Benchmark_AppendEntry(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		b.Fatalf("NewSQLiteBackend: %v", err)
	}

	entry := makeEntry("2025-01-01T00:00:00Z", "bench-sess", "/cwd", "TaskCreate", storage.TaskItem{
		Subject: "task1", Status: "pending", ActiveForm: "Doing task1",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := backend.AppendEntry(entry); err != nil {
			b.Fatalf("AppendEntry: %v", err)
		}
	}
}

func Benchmark_LoadHistory(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		b.Fatalf("NewSQLiteBackend: %v", err)
	}

	for i := 0; i < 100; i++ {
		entry := makeEntry("2025-01-01T00:00:00Z", "bench-sess", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: "pending", ActiveForm: "Doing",
		})
		if err := backend.AppendEntry(entry); err != nil {
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

func Benchmark_GetEntriesBySession(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		b.Fatalf("NewSQLiteBackend: %v", err)
	}

	for i := 0; i < 50; i++ {
		sess := "session-A"
		if i%3 == 0 {
			sess = "session-B"
		}
		entry := makeEntry("2025-01-01T00:00:00Z", sess, "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: "pending", ActiveForm: "Doing",
		})
		if err := backend.AppendEntry(entry); err != nil {
			b.Fatalf("seed AppendEntry: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := backend.GetEntriesBySession("session-A"); err != nil {
			b.Fatalf("GetEntriesBySession: %v", err)
		}
	}
}

func Benchmark_GetTasksByStatus(b *testing.B) {
	dbPath := filepath.Join(b.TempDir(), "bench.db")
	backend, err := storage.NewSQLiteBackend(dbPath)
	if err != nil {
		b.Fatalf("NewSQLiteBackend: %v", err)
	}

	for i := 0; i < 50; i++ {
		status := "pending"
		if i%2 == 0 {
			status = "completed"
		}
		entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", "TaskCreate", storage.TaskItem{
			Subject: "task", Status: status, ActiveForm: "Doing",
		})
		if err := backend.AppendEntry(entry); err != nil {
			b.Fatalf("seed AppendEntry: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := backend.GetTasksByStatus("pending"); err != nil {
			b.Fatalf("GetTasksByStatus: %v", err)
		}
	}
}
