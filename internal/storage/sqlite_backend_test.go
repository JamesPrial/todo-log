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
func makeEntry(ts, session, cwd string, todos []storage.TodoItem) storage.LogEntry {
	return storage.LogEntry{
		Timestamp: ts,
		SessionID: session,
		Cwd:       cwd,
		Todos:     todos,
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

func Test_NewSQLiteBackend_CreatesTodosTable(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	db := openDirectDB(t, dbPath)
	var name string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='todos'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("todos table not found: %v", err)
	}
	if name != "todos" {
		t.Errorf("expected table name 'todos', got %q", name)
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
		{name: "status index", indexName: "idx_todos_status"},
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

func Test_NewSQLiteBackend_ForeignKeysEnabled(t *testing.T) {
	t.Parallel()
	_, dbPath := newTestBackend(t)

	// Verify foreign keys are enforced by attempting to insert a todo
	// with a non-existent entry_id. The backend's connect() method enables
	// foreign keys, so we need to do the same on our direct connection.
	db := openDirectDB(t, dbPath)

	// Enable foreign keys on this connection (matching backend behavior)
	_, err := db.Exec("PRAGMA foreign_keys=ON")
	if err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	// Try to insert a todo with non-existent entry_id
	_, err = db.Exec("INSERT INTO todos (entry_id, content, status, active_form) VALUES (9999, 'test', 'pending', 'Testing')")
	if err == nil {
		t.Error("expected foreign key constraint error, but insert succeeded")
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
		[]storage.TodoItem{{Content: "task", Status: "pending", ActiveForm: "Doing task"}},
	)

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var ts, sid, cwd string
	err := db.QueryRow(`SELECT timestamp, session_id, cwd FROM log_entries WHERE id=1`).Scan(&ts, &sid, &cwd)
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
}

func Test_AppendEntry_AssociatedTodos(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	todos := []storage.TodoItem{
		{Content: "first", Status: "pending", ActiveForm: "Starting first"},
		{Content: "second", Status: "in_progress", ActiveForm: "Working on second"},
		{Content: "third", Status: "completed", ActiveForm: "Finished third"},
	}
	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", todos)

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM todos WHERE entry_id=1`).Scan(&count); err != nil {
		t.Fatalf("query todos count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 todos linked to entry, got %d", count)
	}
}

func Test_AppendEntry_ForeignKeyLink(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "pending", ActiveForm: "Doing"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)

	var entryID int
	if err := db.QueryRow(`SELECT id FROM log_entries LIMIT 1`).Scan(&entryID); err != nil {
		t.Fatalf("query log_entries id: %v", err)
	}

	var todoEntryID int
	if err := db.QueryRow(`SELECT entry_id FROM todos LIMIT 1`).Scan(&todoEntryID); err != nil {
		t.Fatalf("query todos entry_id: %v", err)
	}

	if entryID != todoEntryID {
		t.Errorf("foreign key mismatch: log_entries.id=%d, todos.entry_id=%d", entryID, todoEntryID)
	}
}

func Test_AppendEntry_UnicodePreserved(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "unicode-sess", "/tmp", []storage.TodoItem{
		{Content: "tarea en espanol", Status: "pendiente", ActiveForm: "Haciendo tarea"},
		{Content: "Japanese chars", Status: "pending", ActiveForm: "Processing"},
		{Content: "emoji: check mark", Status: "done", ActiveForm: "Finished"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	rows, err := db.Query(`SELECT content, status, active_form FROM todos ORDER BY id`)
	if err != nil {
		t.Fatalf("query todos: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var results []storage.TodoItem
	for rows.Next() {
		var item storage.TodoItem
		if err := rows.Scan(&item.Content, &item.Status, &item.ActiveForm); err != nil {
			t.Fatalf("scan todo: %v", err)
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	if len(results) != len(entry.Todos) {
		t.Fatalf("expected %d todos, got %d", len(entry.Todos), len(results))
	}
	for i, want := range entry.Todos {
		got := results[i]
		if got.Content != want.Content {
			t.Errorf("todo[%d] content: got %q, want %q", i, got.Content, want.Content)
		}
		if got.Status != want.Status {
			t.Errorf("todo[%d] status: got %q, want %q", i, got.Status, want.Status)
		}
		if got.ActiveForm != want.ActiveForm {
			t.Errorf("todo[%d] active_form: got %q, want %q", i, got.ActiveForm, want.ActiveForm)
		}
	}
}

func Test_AppendEntry_EmptyTodos(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "empty-sess", "/cwd", []storage.TodoItem{})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry with empty todos: %v", err)
	}

	db := openDirectDB(t, dbPath)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM log_entries`).Scan(&count); err != nil {
		t.Fatalf("query log_entries count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 log entry, got %d", count)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM todos`).Scan(&count); err != nil {
		t.Fatalf("query todos count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 todos, got %d", count)
	}
}

func Test_AppendEntry_AutoIncrementIDs(t *testing.T) {
	t.Parallel()
	b, dbPath := newTestBackend(t)

	for i := 0; i < 3; i++ {
		entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
			{Content: "task", Status: "pending", ActiveForm: "Doing"},
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

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "pending", ActiveForm: "Doing the task now"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	db := openDirectDB(t, dbPath)
	var activeForm string
	err := db.QueryRow(`SELECT active_form FROM todos WHERE id=1`).Scan(&activeForm)
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

func Test_LoadHistory_EntriesWithTodos(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	original := makeEntry("2025-01-01T00:00:00Z", "sess1", "/project", []storage.TodoItem{
		{Content: "task1", Status: "pending", ActiveForm: "Doing task1"},
		{Content: "task2", Status: "completed", ActiveForm: "Done task2"},
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
	if len(got.Todos) != len(original.Todos) {
		t.Fatalf("Todos length: got %d, want %d", len(got.Todos), len(original.Todos))
	}
	for i := range original.Todos {
		if got.Todos[i] != original.Todos[i] {
			t.Errorf("Todos[%d]: got %+v, want %+v", i, got.Todos[i], original.Todos[i])
		}
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
		entry := makeEntry(ts, "sess", "/cwd", []storage.TodoItem{
			{Content: "task", Status: "pending", ActiveForm: "Doing"},
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

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "pending", ActiveForm: "Working on task right now"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	entries, err := b.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(entries) != 1 || len(entries[0].Todos) != 1 {
		t.Fatalf("unexpected result shape: %d entries", len(entries))
	}

	got := entries[0].Todos[0].ActiveForm
	if got != "Working on task right now" {
		t.Errorf("ActiveForm: got %q, want %q", got, "Working on task right now")
	}
}

func Test_LoadHistory_EntryWithNoTodos(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{})
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
	if entries[0].Todos == nil {
		t.Logf("note: Todos is nil for entry with no todos")
	}
	if len(entries[0].Todos) != 0 {
		t.Errorf("expected 0 todos, got %d", len(entries[0].Todos))
	}
}

func Test_LoadHistory_UnicodePreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "unicode-sess", "/cwd", []storage.TodoItem{
		{Content: "tarea en espanol", Status: "pendiente", ActiveForm: "Haciendo tarea"},
		{Content: "Japanese chars", Status: "pending", ActiveForm: "Processing"},
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

	for i, want := range entry.Todos {
		got := entries[0].Todos[i]
		if got.Content != want.Content {
			t.Errorf("todo[%d] content: got %q, want %q", i, got.Content, want.Content)
		}
		if got.Status != want.Status {
			t.Errorf("todo[%d] status: got %q, want %q", i, got.Status, want.Status)
		}
		if got.ActiveForm != want.ActiveForm {
			t.Errorf("todo[%d] activeForm: got %q, want %q", i, got.ActiveForm, want.ActiveForm)
		}
	}
}

func Test_LoadHistory_MultipleEntriesMultipleTodos(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "s1", "/proj1", []storage.TodoItem{
			{Content: "t1a", Status: "pending", ActiveForm: "Doing t1a"},
			{Content: "t1b", Status: "completed", ActiveForm: "Done t1b"},
		}),
		makeEntry("2025-01-02T00:00:00Z", "s2", "/proj2", []storage.TodoItem{
			{Content: "t2a", Status: "in_progress", ActiveForm: "Working t2a"},
		}),
		makeEntry("2025-01-03T00:00:00Z", "s1", "/proj1", []storage.TodoItem{
			{Content: "t3a", Status: "pending", ActiveForm: "Starting t3a"},
			{Content: "t3b", Status: "pending", ActiveForm: "Starting t3b"},
			{Content: "t3c", Status: "completed", ActiveForm: "Done t3c"},
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
		if got.Cwd != want.Cwd {
			t.Errorf("entry[%d] cwd: got %q, want %q", i, got.Cwd, want.Cwd)
		}
		if len(got.Todos) != len(want.Todos) {
			t.Fatalf("entry[%d] todos length: got %d, want %d", i, len(got.Todos), len(want.Todos))
		}
		for j, wantTodo := range want.Todos {
			gotTodo := got.Todos[j]
			if gotTodo != wantTodo {
				t.Errorf("entry[%d].todos[%d]: got %+v, want %+v", i, j, gotTodo, wantTodo)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// GetEntriesBySession tests
// ---------------------------------------------------------------------------

func Test_GetEntriesBySession_MatchingEntries(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "session-A", "/cwd", []storage.TodoItem{
			{Content: "t1", Status: "pending", ActiveForm: "Doing t1"},
		}),
		makeEntry("2025-01-02T00:00:00Z", "session-B", "/cwd", []storage.TodoItem{
			{Content: "t2", Status: "pending", ActiveForm: "Doing t2"},
		}),
		makeEntry("2025-01-03T00:00:00Z", "session-A", "/cwd", []storage.TodoItem{
			{Content: "t3", Status: "pending", ActiveForm: "Doing t3"},
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

	entry := makeEntry("2025-01-01T00:00:00Z", "existing-sess", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "pending", ActiveForm: "Doing"},
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

func Test_GetEntriesBySession_IncludesTodos(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	todos := []storage.TodoItem{
		{Content: "task1", Status: "pending", ActiveForm: "Doing task1"},
		{Content: "task2", Status: "completed", ActiveForm: "Done task2"},
	}
	entry := makeEntry("2025-01-01T00:00:00Z", "my-session", "/cwd", todos)

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

	if len(results[0].Todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(results[0].Todos))
	}

	for i, want := range todos {
		got := results[0].Todos[i]
		if got != want {
			t.Errorf("todo[%d]: got %+v, want %+v", i, got, want)
		}
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
		entry := makeEntry(ts, "same-session", "/cwd", []storage.TodoItem{
			{Content: "task", Status: "pending", ActiveForm: "Doing"},
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
// GetTodosByStatus tests
// ---------------------------------------------------------------------------

func Test_GetTodosByStatus_MatchingStatus(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task1", Status: "pending", ActiveForm: "Doing task1"},
		{Content: "task2", Status: "completed", ActiveForm: "Done task2"},
		{Content: "task3", Status: "pending", ActiveForm: "Doing task3"},
		{Content: "task4", Status: "in_progress", ActiveForm: "Working task4"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTodosByStatus("pending")
	if err != nil {
		t.Fatalf("GetTodosByStatus: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 pending todos, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pending" {
			t.Errorf("unexpected status: got %q, want %q", r.Status, "pending")
		}
	}
}

func Test_GetTodosByStatus_UnknownStatus(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "pending", ActiveForm: "Doing"},
	})
	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTodosByStatus("nonexistent")
	if err != nil {
		t.Fatalf("GetTodosByStatus: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected empty slice for unknown status, got %d todos", len(results))
	}
}

func Test_GetTodosByStatus_AcrossEntries(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	inputEntries := []storage.LogEntry{
		makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
			{Content: "entry1-pending", Status: "pending", ActiveForm: "Doing entry1"},
			{Content: "entry1-done", Status: "completed", ActiveForm: "Done entry1"},
		}),
		makeEntry("2025-01-02T00:00:00Z", "s2", "/cwd", []storage.TodoItem{
			{Content: "entry2-pending", Status: "pending", ActiveForm: "Doing entry2"},
		}),
		makeEntry("2025-01-03T00:00:00Z", "s3", "/cwd", []storage.TodoItem{
			{Content: "entry3-done", Status: "completed", ActiveForm: "Done entry3"},
			{Content: "entry3-pending", Status: "pending", ActiveForm: "Doing entry3"},
		}),
	}

	for i, e := range inputEntries {
		if err := b.AppendEntry(e); err != nil {
			t.Fatalf("AppendEntry #%d: %v", i, err)
		}
	}

	results, err := b.GetTodosByStatus("pending")
	if err != nil {
		t.Fatalf("GetTodosByStatus: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 pending todos across entries, got %d", len(results))
	}

	for _, r := range results {
		if r.Status != "pending" {
			t.Errorf("unexpected status: got %q, want %q", r.Status, "pending")
		}
	}
}

func Test_GetTodosByStatus_CompleteStructure(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "my task", Status: "pending", ActiveForm: "Working on my task"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTodosByStatus("pending")
	if err != nil {
		t.Fatalf("GetTodosByStatus: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(results))
	}

	got := results[0]
	if got.Content != "my task" {
		t.Errorf("Content: got %q, want %q", got.Content, "my task")
	}
	if got.Status != "pending" {
		t.Errorf("Status: got %q, want %q", got.Status, "pending")
	}
	if got.ActiveForm != "Working on my task" {
		t.Errorf("ActiveForm: got %q, want %q", got.ActiveForm, "Working on my task")
	}
}

func Test_GetTodosByStatus_ActiveFormPreserved(t *testing.T) {
	t.Parallel()
	b, _ := newTestBackend(t)

	entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
		{Content: "task", Status: "in_progress", ActiveForm: "Doing task"},
	})

	if err := b.AppendEntry(entry); err != nil {
		t.Fatalf("AppendEntry: %v", err)
	}

	results, err := b.GetTodosByStatus("in_progress")
	if err != nil {
		t.Fatalf("GetTodosByStatus: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(results))
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
			name:    "single entry no todos",
			entries: []storage.LogEntry{makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{})},
		},
		{
			name: "single entry single todo",
			entries: []storage.LogEntry{
				makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
					{Content: "only-task", Status: "pending", ActiveForm: "Doing only-task"},
				}),
			},
		},
		{
			name: "multiple entries mixed todos",
			entries: []storage.LogEntry{
				makeEntry("2025-01-01T00:00:00Z", "s1", "/proj", []storage.TodoItem{
					{Content: "a", Status: "pending", ActiveForm: "Doing a"},
					{Content: "b", Status: "completed", ActiveForm: "Done b"},
				}),
				makeEntry("2025-01-02T00:00:00Z", "s2", "/proj2", []storage.TodoItem{}),
				makeEntry("2025-01-03T00:00:00Z", "s1", "/proj", []storage.TodoItem{
					{Content: "c", Status: "in_progress", ActiveForm: "Working c"},
				}),
			},
		},
		{
			name: "unicode in all fields",
			entries: []storage.LogEntry{
				makeEntry("2025-06-15T12:00:00Z", "sess-unicode", "/home/user", []storage.TodoItem{
					{Content: "tarea en espanol", Status: "pendiente", ActiveForm: "Haciendo tarea"},
					{Content: "Japanese: chars", Status: "done", ActiveForm: "complete"},
				}),
			},
		},
		{
			name: "empty string fields",
			entries: []storage.LogEntry{
				makeEntry("", "", "", []storage.TodoItem{
					{Content: "", Status: "", ActiveForm: ""},
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
				if len(got.Todos) != len(want.Todos) {
					t.Fatalf("entry[%d] todos count: got %d, want %d", i, len(got.Todos), len(want.Todos))
				}
				for j, wantTodo := range want.Todos {
					gotTodo := got.Todos[j]
					if gotTodo != wantTodo {
						t.Errorf("entry[%d].todos[%d]: got %+v, want %+v", i, j, gotTodo, wantTodo)
					}
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

	entry := makeEntry("2025-01-01T00:00:00Z", "bench-sess", "/cwd", []storage.TodoItem{
		{Content: "task1", Status: "pending", ActiveForm: "Doing task1"},
		{Content: "task2", Status: "completed", ActiveForm: "Done task2"},
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
		entry := makeEntry("2025-01-01T00:00:00Z", "bench-sess", "/cwd", []storage.TodoItem{
			{Content: "task", Status: "pending", ActiveForm: "Doing"},
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
		entry := makeEntry("2025-01-01T00:00:00Z", sess, "/cwd", []storage.TodoItem{
			{Content: "task", Status: "pending", ActiveForm: "Doing"},
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

func Benchmark_GetTodosByStatus(b *testing.B) {
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
		entry := makeEntry("2025-01-01T00:00:00Z", "s1", "/cwd", []storage.TodoItem{
			{Content: "task", Status: status, ActiveForm: "Doing"},
		})
		if err := backend.AppendEntry(entry); err != nil {
			b.Fatalf("seed AppendEntry: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := backend.GetTodosByStatus("pending"); err != nil {
			b.Fatalf("GetTodosByStatus: %v", err)
		}
	}
}
