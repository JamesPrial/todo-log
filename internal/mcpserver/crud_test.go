package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ===========================================================================
// isValidIdentifier unit tests
// ===========================================================================

func Test_isValidIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple name", "users", true},
		{"underscore prefix", "_private", true},
		{"alphanumeric", "col_123", true},
		{"single letter", "A", true},
		{"mixed case", "MyTable", true},
		{"all underscore", "___", true},
		{"empty string", "", false},
		{"starts with digit", "123abc", false},
		{"contains space", "table name", false},
		{"contains semicolon", "users;DROP", false},
		{"contains hyphen", "col-name", false},
		{"contains dot", "schema.table", false},
		{"contains paren", "fn()", false},
		{"single quote", "user's", false},
		{"double quote", `"users"`, false},
		{"backtick", "`users`", false},
		{"star", "*", false},
		{"equals", "a=b", false},
		{"newline", "col\nname", false},
		{"sql injection attempt", "users; DROP TABLE users", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isValidIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("isValidIdentifier(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Helpers
// ===========================================================================

// makeInsertRequest creates a CallToolRequest for insert_rows with the
// given table, columns, and values. Columns are converted from []string to
// []any and each row in values is used as-is ([]any).
func makeInsertRequest(table string, columns []string, values [][]any) mcp.CallToolRequest {
	colsAny := make([]any, len(columns))
	for i, c := range columns {
		colsAny[i] = c
	}
	valsAny := make([]any, len(values))
	for i, row := range values {
		rowAny := make([]any, len(row))
		copy(rowAny, row)
		valsAny[i] = rowAny
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "insert_rows",
			Arguments: map[string]any{
				"table":   table,
				"columns": colsAny,
				"values":  valsAny,
			},
		},
	}
}

// makeUpdateRequest creates a CallToolRequest for update_rows.
func makeUpdateRequest(table string, set map[string]any, where string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "update_rows",
			Arguments: map[string]any{
				"table": table,
				"set":   set,
				"where": where,
			},
		},
	}
}

// makeDeleteRequest creates a CallToolRequest for delete_rows.
func makeDeleteRequest(table string, where string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "delete_rows",
			Arguments: map[string]any{
				"table": table,
				"where": where,
			},
		},
	}
}

// setupCRUDTable creates a test table and returns its name. The table includes
// id (SERIAL PK), name (TEXT), age (INT), and active (BOOLEAN DEFAULT TRUE).
func setupCRUDTable(t *testing.T, cm *ContainerManager, tableName string) {
	t.Helper()
	sql := "CREATE TABLE " + tableName + "(id SERIAL PRIMARY KEY, name TEXT, age INT, active BOOLEAN DEFAULT TRUE)"
	result := execQueryResult(t, cm, sql)
	if result.IsError {
		t.Fatalf("setup: CREATE TABLE %s failed: %s", tableName, resultText(t, result))
	}
}

// ===========================================================================
// HandleInsertRows Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleInsertRows: no container running
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest("users", []string{"name"}, [][]any{{"Alice"}}))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleInsertRows() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleInsertRows() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: missing required parameters
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_MissingParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing table param",
			args: map[string]any{
				"columns": []any{"name"},
				"values":  []any{[]any{"Alice"}},
			},
		},
		{
			name: "missing columns param",
			args: map[string]any{
				"table":  "some_table",
				"values": []any{[]any{"Alice"}},
			},
		},
		{
			name: "missing values param",
			args: map[string]any{
				"table":   "some_table",
				"columns": []any{"name"},
			},
		},
		{
			name: "nil arguments",
			args: nil,
		},
		{
			name: "empty arguments",
			args: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "insert_rows",
					Arguments: tt.args,
				},
			}

			result, err := cm.HandleInsertRows(ctx, req)
			if err != nil {
				t.Fatalf("HandleInsertRows() returned Go error: %v", err)
			}

			if !result.IsError {
				t.Errorf("HandleInsertRows() IsError = false, want true for %s", tt.name)
			}

			text := resultText(t, result)
			textLower := strings.ToLower(text)
			if !strings.Contains(textLower, "required") && !strings.Contains(textLower, "missing") {
				t.Errorf("HandleInsertRows() text = %q, want it to contain 'required' or 'missing' (case-insensitive)", text)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: insert single row
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_SingleRow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_single")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_single",
		[]string{"name", "age"},
		[][]any{{"Alice", 30}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "inserted") && !strings.Contains(textLower, "1") {
		t.Errorf("HandleInsertRows() text = %q, want it to contain 'inserted' or '1'", text)
	}

	// Verify the row was actually inserted
	selectText := execQuery(t, cm, "SELECT name, age FROM insert_single")
	if !strings.Contains(selectText, "Alice") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q", selectText, "Alice")
	}
	if !strings.Contains(selectText, "30") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q", selectText, "30")
	}
	if !strings.Contains(selectText, "1 row(s)") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q", selectText, "1 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: insert multiple rows
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_MultipleRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_multi")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_multi",
		[]string{"name", "age"},
		[][]any{
			{"Alice", 30},
			{"Bob", 25},
			{"Charlie", 35},
		},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "inserted") && !strings.Contains(textLower, "3") {
		t.Errorf("HandleInsertRows() text = %q, want it to indicate 3 rows inserted", text)
	}

	// Verify all rows
	selectText := execQuery(t, cm, "SELECT name FROM insert_multi ORDER BY name")
	if !strings.Contains(selectText, "Alice") {
		t.Errorf("SELECT after insert: missing Alice in %q", selectText)
	}
	if !strings.Contains(selectText, "Bob") {
		t.Errorf("SELECT after insert: missing Bob in %q", selectText)
	}
	if !strings.Contains(selectText, "Charlie") {
		t.Errorf("SELECT after insert: missing Charlie in %q", selectText)
	}
	if !strings.Contains(selectText, "3 row(s)") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q", selectText, "3 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: empty columns array
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_EmptyColumns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_empty_cols")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_empty_cols",
		[]string{},
		[][]any{{"Alice"}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleInsertRows() IsError = false, want true for empty columns array")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: empty values array
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_EmptyValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_empty_vals")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_empty_vals",
		[]string{"name"},
		[][]any{},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleInsertRows() IsError = false, want true for empty values array")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: column count mismatch
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_ColumnCountMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_mismatch")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// More values than columns
	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_mismatch",
		[]string{"name"},
		[][]any{{"Alice", 30, true}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleInsertRows() IsError = false, want true for column count mismatch")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: nonexistent table
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_NonexistentTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"nonexistent_table_xyz",
		[]string{"name"},
		[][]any{{"Alice"}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleInsertRows() IsError = false, want true for nonexistent table")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: NULL values
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_NullValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_nulls")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_nulls",
		[]string{"name", "age"},
		[][]any{{"Alice", nil}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, result))
	}

	// Verify NULL was stored correctly
	selectText := execQuery(t, cm, "SELECT name, age FROM insert_nulls")
	if !strings.Contains(selectText, "Alice") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q", selectText, "Alice")
	}
	if !strings.Contains(selectText, "NULL") {
		t.Errorf("SELECT after insert: text = %q, want it to contain %q for NULL age", selectText, "NULL")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: various types (int, string, float, bool)
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_VariousTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create a table with various column types
	createResult := execQueryResult(t, cm,
		"CREATE TABLE insert_types(id SERIAL PRIMARY KEY, name TEXT, score FLOAT, active BOOLEAN, count INT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_types",
		[]string{"name", "score", "active", "count"},
		[][]any{{"Alice", 95.5, true, 42}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, result))
	}

	// Verify data was stored correctly
	selectText := execQuery(t, cm, "SELECT name, score, active, count FROM insert_types")
	if !strings.Contains(selectText, "Alice") {
		t.Errorf("SELECT result missing %q in %q", "Alice", selectText)
	}
	if !strings.Contains(selectText, "1 row(s)") {
		t.Errorf("SELECT result = %q, want it to contain %q", selectText, "1 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleInsertRows: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleInsertRows_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_content_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_content_test",
		[]string{"name"},
		[][]any{{"Alice"}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleInsertRows() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleInsertRows() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ===========================================================================
// HandleInsertRows: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleInsertRows_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup tables for all subtests
	setupCRUDTable(t, cm, "ic_single")
	setupCRUDTable(t, cm, "ic_multi")
	setupCRUDTable(t, cm, "ic_null")

	tests := []struct {
		name         string
		table        string
		columns      []string
		values       [][]any
		wantIsError  bool
		wantContains []string
		verifySQL    string
		verifySub    string
	}{
		{
			name:         "single row insert",
			table:        "ic_single",
			columns:      []string{"name", "age"},
			values:       [][]any{{"Dave", 40}},
			wantIsError:  false,
			wantContains: []string{"insert"},
			verifySQL:    "SELECT name FROM ic_single WHERE name = 'Dave'",
			verifySub:    "Dave",
		},
		{
			name:         "multiple row insert",
			table:        "ic_multi",
			columns:      []string{"name", "age"},
			values:       [][]any{{"Eve", 28}, {"Frank", 33}},
			wantIsError:  false,
			wantContains: []string{"insert"},
			verifySQL:    "SELECT count(*) AS cnt FROM ic_multi",
			verifySub:    "2",
		},
		{
			name:        "insert with NULL value",
			table:       "ic_null",
			columns:     []string{"name", "age"},
			values:      [][]any{{"Grace", nil}},
			wantIsError: false,
			verifySQL:   "SELECT age FROM ic_null WHERE name = 'Grace'",
			verifySub:   "NULL",
		},
		{
			name:        "empty columns",
			table:       "ic_single",
			columns:     []string{},
			values:      [][]any{{"x"}},
			wantIsError: true,
		},
		{
			name:        "empty values",
			table:       "ic_single",
			columns:     []string{"name"},
			values:      [][]any{},
			wantIsError: true,
		},
		{
			name:        "nonexistent table",
			table:       "no_such_table",
			columns:     []string{"name"},
			values:      [][]any{{"test"}},
			wantIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleInsertRows(ctx, makeInsertRequest(tt.table, tt.columns, tt.values))
			if err != nil {
				t.Fatalf("HandleInsertRows() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleInsertRows() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			text := resultText(t, result)
			textLower := strings.ToLower(text)
			for _, substr := range tt.wantContains {
				if !strings.Contains(textLower, strings.ToLower(substr)) {
					t.Errorf("result text = %q, want it to contain %q (case-insensitive)", text, substr)
				}
			}

			// Verify data if SQL provided
			if tt.verifySQL != "" && !tt.wantIsError {
				verifyText := execQuery(t, cm, tt.verifySQL)
				if !strings.Contains(verifyText, tt.verifySub) {
					t.Errorf("verify query result = %q, want it to contain %q", verifyText, tt.verifySub)
				}
			}
		})
	}
}

// ===========================================================================
// HandleUpdateRows Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleUpdateRows: no container running
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"users",
		map[string]any{"name": "Bob"},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleUpdateRows() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleUpdateRows() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: missing required parameters
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_MissingParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing table param",
			args: map[string]any{
				"set":   map[string]any{"name": "Bob"},
				"where": "name = 'Alice'",
			},
		},
		{
			name: "missing set param",
			args: map[string]any{
				"table": "some_table",
				"where": "name = 'Alice'",
			},
		},
		{
			name: "missing where param",
			args: map[string]any{
				"table": "some_table",
				"set":   map[string]any{"name": "Bob"},
			},
		},
		{
			name: "nil arguments",
			args: nil,
		},
		{
			name: "empty arguments",
			args: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "update_rows",
					Arguments: tt.args,
				},
			}

			result, err := cm.HandleUpdateRows(ctx, req)
			if err != nil {
				t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
			}

			if !result.IsError {
				t.Errorf("HandleUpdateRows() IsError = false, want true for %s", tt.name)
			}

			text := resultText(t, result)
			textLower := strings.ToLower(text)
			if !strings.Contains(textLower, "required") && !strings.Contains(textLower, "missing") {
				t.Errorf("HandleUpdateRows() text = %q, want it to contain 'required' or 'missing' (case-insensitive)", text)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: update matching rows
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_MatchingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "update_match")

	// Insert test data
	execQuery(t, cm, "INSERT INTO update_match(name, age) VALUES ('Alice', 30), ('Bob', 25)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"update_match",
		map[string]any{"age": 31},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleUpdateRows() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "updated") && !strings.Contains(textLower, "1") {
		t.Errorf("HandleUpdateRows() text = %q, want it to indicate rows updated", text)
	}

	// Verify the update
	selectText := execQuery(t, cm, "SELECT age FROM update_match WHERE name = 'Alice'")
	if !strings.Contains(selectText, "31") {
		t.Errorf("SELECT after update: text = %q, want it to contain %q", selectText, "31")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: update no matching rows
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_NoMatchingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "update_nomatch")

	// Insert test data
	execQuery(t, cm, "INSERT INTO update_nomatch(name, age) VALUES ('Alice', 30)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"update_nomatch",
		map[string]any{"age": 99},
		"name = 'ZZZ_NONEXISTENT'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	// Should not be a hard error -- just 0 rows affected
	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "0") && !strings.Contains(textLower, "no rows") {
		t.Errorf("HandleUpdateRows() text = %q, want it to indicate 0 rows affected", text)
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: empty set
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_EmptySet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "update_empty_set")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"update_empty_set",
		map[string]any{},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleUpdateRows() IsError = false, want true for empty set")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: nonexistent table
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_NonexistentTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"nonexistent_table_xyz",
		map[string]any{"name": "Bob"},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleUpdateRows() IsError = false, want true for nonexistent table")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: update multiple columns
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_MultipleColumns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "update_multicol")

	// Insert test data
	execQuery(t, cm, "INSERT INTO update_multicol(name, age, active) VALUES ('Alice', 30, true)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"update_multicol",
		map[string]any{"name": "Alicia", "age": 31, "active": false},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleUpdateRows() IsError = true, text = %q", resultText(t, result))
	}

	// Verify
	selectText := execQuery(t, cm, "SELECT name, age, active FROM update_multicol")
	if !strings.Contains(selectText, "Alicia") {
		t.Errorf("SELECT after update: text = %q, want it to contain %q", selectText, "Alicia")
	}
	if !strings.Contains(selectText, "31") {
		t.Errorf("SELECT after update: text = %q, want it to contain %q", selectText, "31")
	}
}

// ---------------------------------------------------------------------------
// HandleUpdateRows: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleUpdateRows_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "update_content_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"update_content_test",
		map[string]any{"name": "Bob"},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleUpdateRows() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleUpdateRows() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ===========================================================================
// HandleUpdateRows: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleUpdateRows_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup table with test data
	setupCRUDTable(t, cm, "uc_cases")
	execQuery(t, cm, "INSERT INTO uc_cases(name, age, active) VALUES ('Alice', 30, true), ('Bob', 25, true), ('Charlie', 35, false)")

	tests := []struct {
		name        string
		table       string
		set         map[string]any
		where       string
		wantIsError bool
		verifySQL   string
		verifySub   string
	}{
		{
			name:        "update single matching row",
			table:       "uc_cases",
			set:         map[string]any{"age": 99},
			where:       "name = 'Alice'",
			wantIsError: false,
			verifySQL:   "SELECT age FROM uc_cases WHERE name = 'Alice'",
			verifySub:   "99",
		},
		{
			name:        "update multiple matching rows",
			table:       "uc_cases",
			set:         map[string]any{"active": false},
			where:       "active = true",
			wantIsError: false,
		},
		{
			name:        "update no matching rows",
			table:       "uc_cases",
			set:         map[string]any{"age": 0},
			where:       "name = 'ZZZZZ'",
			wantIsError: false,
		},
		{
			name:        "empty set object",
			table:       "uc_cases",
			set:         map[string]any{},
			where:       "name = 'Alice'",
			wantIsError: true,
		},
		{
			name:        "nonexistent table",
			table:       "no_such_table_xyz",
			set:         map[string]any{"x": 1},
			where:       "1=1",
			wantIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(tt.table, tt.set, tt.where))
			if err != nil {
				t.Fatalf("HandleUpdateRows() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleUpdateRows() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			// Verify data if SQL provided
			if tt.verifySQL != "" && !tt.wantIsError {
				verifyText := execQuery(t, cm, tt.verifySQL)
				if !strings.Contains(verifyText, tt.verifySub) {
					t.Errorf("verify query result = %q, want it to contain %q", verifyText, tt.verifySub)
				}
			}
		})
	}
}

// ===========================================================================
// HandleDeleteRows Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleDeleteRows: no container running
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("users", "name = 'Alice'"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDeleteRows() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleDeleteRows() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: missing required parameters
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_MissingParams(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "missing table param",
			args: map[string]any{
				"where": "name = 'Alice'",
			},
		},
		{
			name: "missing where param",
			args: map[string]any{
				"table": "some_table",
			},
		},
		{
			name: "nil arguments",
			args: nil,
		},
		{
			name: "empty arguments",
			args: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			req := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "delete_rows",
					Arguments: tt.args,
				},
			}

			result, err := cm.HandleDeleteRows(ctx, req)
			if err != nil {
				t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
			}

			if !result.IsError {
				t.Errorf("HandleDeleteRows() IsError = false, want true for %s", tt.name)
			}

			text := resultText(t, result)
			textLower := strings.ToLower(text)
			if !strings.Contains(textLower, "required") && !strings.Contains(textLower, "missing") {
				t.Errorf("HandleDeleteRows() text = %q, want it to contain 'required' or 'missing' (case-insensitive)", text)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: delete matching rows
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_MatchingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "delete_match")

	// Insert test data
	execQuery(t, cm, "INSERT INTO delete_match(name, age) VALUES ('Alice', 30), ('Bob', 25), ('Charlie', 35)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("delete_match", "name = 'Alice'"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDeleteRows() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "deleted") && !strings.Contains(textLower, "1") {
		t.Errorf("HandleDeleteRows() text = %q, want it to indicate rows deleted", text)
	}

	// Verify the row is gone
	selectText := execQuery(t, cm, "SELECT count(*) AS cnt FROM delete_match")
	if !strings.Contains(selectText, "2") {
		t.Errorf("SELECT after delete: text = %q, want count to be 2", selectText)
	}

	// Verify Alice is gone specifically
	selectText2 := execQuery(t, cm, "SELECT name FROM delete_match ORDER BY name")
	if strings.Contains(selectText2, "Alice") {
		t.Errorf("SELECT after delete: text = %q, should NOT contain deleted row %q", selectText2, "Alice")
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: delete no matching rows
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_NoMatchingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "delete_nomatch")

	// Insert test data
	execQuery(t, cm, "INSERT INTO delete_nomatch(name, age) VALUES ('Alice', 30)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("delete_nomatch", "name = 'ZZZ_NONEXISTENT'"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "0") {
		t.Errorf("HandleDeleteRows() text = %q, want it to indicate 0 rows deleted", text)
	}

	// Verify original data is intact
	selectText := execQuery(t, cm, "SELECT count(*) AS cnt FROM delete_nomatch")
	if !strings.Contains(selectText, "1") {
		t.Errorf("SELECT after no-op delete: text = %q, want count to still be 1", selectText)
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: nonexistent table
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_NonexistentTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("nonexistent_table_xyz", "1=1"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDeleteRows() IsError = false, want true for nonexistent table")
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: delete multiple matching rows
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_MultipleMatchingRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "delete_multi")

	// Insert test data
	execQuery(t, cm, "INSERT INTO delete_multi(name, age, active) VALUES ('Alice', 30, true), ('Bob', 25, true), ('Charlie', 35, false)")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Delete all active users (should be 2)
	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("delete_multi", "active = true"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDeleteRows() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "deleted") && !strings.Contains(textLower, "2") {
		t.Errorf("HandleDeleteRows() text = %q, want it to indicate 2 rows deleted", text)
	}

	// Verify only Charlie remains
	selectText := execQuery(t, cm, "SELECT name FROM delete_multi")
	if !strings.Contains(selectText, "Charlie") {
		t.Errorf("SELECT after delete: text = %q, want it to contain %q", selectText, "Charlie")
	}
	if !strings.Contains(selectText, "1 row(s)") {
		t.Errorf("SELECT after delete: text = %q, want it to contain %q", selectText, "1 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleDeleteRows: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleDeleteRows_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "delete_content_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("delete_content_test", "1=1"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleDeleteRows() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleDeleteRows() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ===========================================================================
// HandleDeleteRows: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleDeleteRows_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup table with test data
	setupCRUDTable(t, cm, "dc_cases")
	execQuery(t, cm, "INSERT INTO dc_cases(name, age, active) VALUES ('Alice', 30, true), ('Bob', 25, true), ('Charlie', 35, false)")

	tests := []struct {
		name        string
		table       string
		where       string
		wantIsError bool
		verifySQL   string
		verifySub   string
	}{
		{
			name:        "delete single matching row",
			table:       "dc_cases",
			where:       "name = 'Charlie'",
			wantIsError: false,
			verifySQL:   "SELECT count(*) AS cnt FROM dc_cases",
			verifySub:   "2",
		},
		{
			name:        "delete no matching rows",
			table:       "dc_cases",
			where:       "name = 'ZZZZZ'",
			wantIsError: false,
		},
		{
			name:        "nonexistent table",
			table:       "no_such_table_xyz",
			where:       "1=1",
			wantIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleDeleteRows(ctx, makeDeleteRequest(tt.table, tt.where))
			if err != nil {
				t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleDeleteRows() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			// Verify data if SQL provided
			if tt.verifySQL != "" && !tt.wantIsError {
				verifyText := execQuery(t, cm, tt.verifySQL)
				if !strings.Contains(verifyText, tt.verifySub) {
					t.Errorf("verify query result = %q, want it to contain %q", verifyText, tt.verifySub)
				}
			}
		})
	}
}

// ===========================================================================
// Integration: Full CRUD Lifecycle
// ===========================================================================

// ---------------------------------------------------------------------------
// Insert -> Select -> Update -> Select -> Delete -> Select
// ---------------------------------------------------------------------------

func Test_Integration_CRUDLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "crud_lifecycle")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Step 1: Insert rows via HandleInsertRows
	insertResult, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"crud_lifecycle",
		[]string{"name", "age", "active"},
		[][]any{
			{"Alice", 30, true},
			{"Bob", 25, true},
			{"Charlie", 35, false},
		},
	))
	if err != nil {
		t.Fatalf("Step 1 (Insert): returned Go error: %v", err)
	}
	if insertResult.IsError {
		t.Fatalf("Step 1 (Insert): IsError = true, text = %q", resultText(t, insertResult))
	}

	// Step 2: Verify via SELECT
	selectText1 := execQuery(t, cm, "SELECT name, age, active FROM crud_lifecycle ORDER BY name")
	if !strings.Contains(selectText1, "3 row(s)") {
		t.Errorf("Step 2 (Select after insert): text = %q, want it to contain %q", selectText1, "3 row(s)")
	}
	if !strings.Contains(selectText1, "Alice") {
		t.Errorf("Step 2: missing Alice in %q", selectText1)
	}
	if !strings.Contains(selectText1, "Bob") {
		t.Errorf("Step 2: missing Bob in %q", selectText1)
	}
	if !strings.Contains(selectText1, "Charlie") {
		t.Errorf("Step 2: missing Charlie in %q", selectText1)
	}

	// Step 3: Update Alice's age via HandleUpdateRows
	updateResult, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"crud_lifecycle",
		map[string]any{"age": 31, "active": false},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("Step 3 (Update): returned Go error: %v", err)
	}
	if updateResult.IsError {
		t.Fatalf("Step 3 (Update): IsError = true, text = %q", resultText(t, updateResult))
	}

	// Step 4: Verify the update via SELECT
	selectText2 := execQuery(t, cm, "SELECT age, active FROM crud_lifecycle WHERE name = 'Alice'")
	if !strings.Contains(selectText2, "31") {
		t.Errorf("Step 4 (Select after update): text = %q, want it to contain %q", selectText2, "31")
	}

	// Step 5: Delete Bob via HandleDeleteRows
	deleteResult, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("crud_lifecycle", "name = 'Bob'"))
	if err != nil {
		t.Fatalf("Step 5 (Delete): returned Go error: %v", err)
	}
	if deleteResult.IsError {
		t.Fatalf("Step 5 (Delete): IsError = true, text = %q", resultText(t, deleteResult))
	}

	// Step 6: Verify the delete via SELECT
	selectText3 := execQuery(t, cm, "SELECT name FROM crud_lifecycle ORDER BY name")
	if !strings.Contains(selectText3, "2 row(s)") {
		t.Errorf("Step 6 (Select after delete): text = %q, want it to contain %q", selectText3, "2 row(s)")
	}
	if strings.Contains(selectText3, "Bob") {
		t.Errorf("Step 6: deleted row Bob should NOT appear in %q", selectText3)
	}
	if !strings.Contains(selectText3, "Alice") {
		t.Errorf("Step 6: remaining row Alice should appear in %q", selectText3)
	}
	if !strings.Contains(selectText3, "Charlie") {
		t.Errorf("Step 6: remaining row Charlie should appear in %q", selectText3)
	}
}

// ===========================================================================
// Integration: Insert via handler, then describe table
// ===========================================================================

func Test_Integration_InsertThenDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "insert_describe")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert a row via handler
	insertResult, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"insert_describe",
		[]string{"name", "age"},
		[][]any{{"Alice", 30}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}
	if insertResult.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, insertResult))
	}

	// Describe the table (should still show structure)
	descResult, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("insert_describe", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}
	if descResult.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, descResult))
	}

	descText := resultText(t, descResult)
	for _, col := range []string{"id", "name", "age", "active"} {
		if !strings.Contains(descText, col) {
			t.Errorf("describe table text = %q, want it to contain column %q", descText, col)
		}
	}
}

// ===========================================================================
// Integration: Insert via handler, list tables, verify table appears
// ===========================================================================

func Test_Integration_InsertThenListTables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "crud_list_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert a row via handler
	insertResult, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"crud_list_test",
		[]string{"name"},
		[][]any{{"Test"}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}
	if insertResult.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, insertResult))
	}

	// List tables should show our table
	listResult, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("HandleListTables() IsError = true, text = %q", resultText(t, listResult))
	}

	listText := resultText(t, listResult)
	if !strings.Contains(listText, "crud_list_test") {
		t.Errorf("list tables text = %q, want it to contain %q", listText, "crud_list_test")
	}
}

// ===========================================================================
// Integration: Delete all rows, then verify table is empty
// ===========================================================================

func Test_Integration_DeleteAllRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "delete_all")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert rows
	insertResult, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"delete_all",
		[]string{"name", "age"},
		[][]any{{"Alice", 30}, {"Bob", 25}},
	))
	if err != nil {
		t.Fatalf("HandleInsertRows() returned Go error: %v", err)
	}
	if insertResult.IsError {
		t.Fatalf("HandleInsertRows() IsError = true, text = %q", resultText(t, insertResult))
	}

	// Delete all rows
	deleteResult, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("delete_all", "1=1"))
	if err != nil {
		t.Fatalf("HandleDeleteRows() returned Go error: %v", err)
	}
	if deleteResult.IsError {
		t.Fatalf("HandleDeleteRows() IsError = true, text = %q", resultText(t, deleteResult))
	}

	// Verify table is empty
	selectText := execQuery(t, cm, "SELECT count(*) AS cnt FROM delete_all")
	if !strings.Contains(selectText, "0") {
		t.Errorf("SELECT count after delete all: text = %q, want count of 0", selectText)
	}
}

// ===========================================================================
// Integration: Update then delete sequence
// ===========================================================================

func Test_Integration_UpdateThenDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)
	setupCRUDTable(t, cm, "upd_del")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Insert
	insertResult, err := cm.HandleInsertRows(ctx, makeInsertRequest(
		"upd_del",
		[]string{"name", "age", "active"},
		[][]any{{"Alice", 30, true}, {"Bob", 25, true}},
	))
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}
	if insertResult.IsError {
		t.Fatalf("insert IsError: %s", resultText(t, insertResult))
	}

	// Update Alice to inactive
	updateResult, err := cm.HandleUpdateRows(ctx, makeUpdateRequest(
		"upd_del",
		map[string]any{"active": false},
		"name = 'Alice'",
	))
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if updateResult.IsError {
		t.Fatalf("update IsError: %s", resultText(t, updateResult))
	}

	// Delete inactive users
	deleteResult, err := cm.HandleDeleteRows(ctx, makeDeleteRequest("upd_del", "active = false"))
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if deleteResult.IsError {
		t.Fatalf("delete IsError: %s", resultText(t, deleteResult))
	}

	// Verify only Bob remains
	selectText := execQuery(t, cm, "SELECT name FROM upd_del")
	if !strings.Contains(selectText, "Bob") {
		t.Errorf("SELECT after update+delete: text = %q, want it to contain %q", selectText, "Bob")
	}
	if strings.Contains(selectText, "Alice") {
		t.Errorf("SELECT after update+delete: text = %q, should NOT contain %q", selectText, "Alice")
	}
	if !strings.Contains(selectText, "1 row(s)") {
		t.Errorf("SELECT after update+delete: text = %q, want it to contain %q", selectText, "1 row(s)")
	}
}
