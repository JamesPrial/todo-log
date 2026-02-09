package mcpserver

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ===========================================================================
// Helpers
// ===========================================================================

// startedContainerManager creates a ContainerManager, starts a PostgreSQL
// container, and registers a cleanup function to stop it. The test is skipped
// if Docker is unavailable or if running in short mode.
func startedContainerManager(t *testing.T) *ContainerManager {
	t.Helper()

	if !dockerAvailable() {
		t.Skip("Docker not available, skipping container tests")
	}
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "start_postgres",
		},
	}
	result, err := cm.HandleStartPostgres(ctx, req)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}
	if result.IsError {
		t.Fatalf("start postgres returned error: %v", resultText(t, result))
	}

	t.Cleanup(func() {
		stopReq := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "stop_postgres",
			},
		}
		_, _ = cm.HandleStopPostgres(context.Background(), stopReq)
	})

	return cm
}

// execQuery executes a SQL query through HandleExecuteQuery and returns the
// result text. It calls t.Fatal if the handler returns a Go-level error.
func execQuery(t *testing.T, cm *ContainerManager, sql string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "execute_query",
			Arguments: map[string]any{
				"query": sql,
			},
		},
	}
	result, err := cm.HandleExecuteQuery(ctx, req)
	if err != nil {
		t.Fatalf("HandleExecuteQuery error: %v", err)
	}
	return resultText(t, result)
}

// execQueryResult executes a SQL query and returns the full CallToolResult so
// callers can inspect IsError in addition to the text.
func execQueryResult(t *testing.T, cm *ContainerManager, sql string) *mcp.CallToolResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "execute_query",
			Arguments: map[string]any{
				"query": sql,
			},
		},
	}
	result, err := cm.HandleExecuteQuery(ctx, req)
	if err != nil {
		t.Fatalf("HandleExecuteQuery error: %v", err)
	}
	return result
}

// makeExecuteQueryRequest creates a CallToolRequest for execute_query.
func makeExecuteQueryRequest(query string) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "execute_query",
			Arguments: map[string]any{
				"query": query,
			},
		},
	}
}

// makeListTablesRequest creates a CallToolRequest for list_tables with an
// optional schema parameter.
func makeListTablesRequest(schema string) mcp.CallToolRequest {
	args := map[string]any{}
	if schema != "" {
		args["schema"] = schema
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "list_tables",
			Arguments: args,
		},
	}
}

// makeDescribeTableRequest creates a CallToolRequest for describe_table with
// required table and optional schema parameters.
func makeDescribeTableRequest(table, schema string) mcp.CallToolRequest {
	args := map[string]any{
		"table": table,
	}
	if schema != "" {
		args["schema"] = schema
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "describe_table",
			Arguments: args,
		},
	}
}

// ===========================================================================
// HandleExecuteQuery Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleExecuteQuery: no container running
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleExecuteQuery(ctx, makeExecuteQueryRequest("SELECT 1"))
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleExecuteQuery() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleExecuteQuery() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: missing query parameter
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_MissingQueryParam(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Request with no arguments at all
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute_query",
			Arguments: map[string]any{},
		},
	}

	result, err := cm.HandleExecuteQuery(ctx, req)
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleExecuteQuery() IsError = false, want true when query param is missing")
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "required") && !strings.Contains(textLower, "missing") {
		t.Errorf("HandleExecuteQuery() text = %q, want it to contain 'required' or 'missing' (case-insensitive)", text)
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: valid SELECT
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_ValidSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "SELECT 1 AS num")

	if !strings.Contains(text, "num") {
		t.Errorf("result text = %q, want it to contain column name %q", text, "num")
	}
	if !strings.Contains(text, "1") {
		t.Errorf("result text = %q, want it to contain value %q", text, "1")
	}
	if !strings.Contains(text, "row(s)") {
		t.Errorf("result text = %q, want it to contain %q", text, "row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: table-driven DML and DDL tests
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_DMLAndDDL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup: create a table
	setupText := execQuery(t, cm, "CREATE TABLE test_dml(id SERIAL PRIMARY KEY, name TEXT, value INT)")
	if !strings.HasPrefix(setupText, "OK:") {
		t.Fatalf("CREATE TABLE did not return OK prefix: %q", setupText)
	}

	tests := []struct {
		name        string
		query       string
		wantPrefix  string
		wantContain string
	}{
		{
			name:        "INSERT returns OK prefix",
			query:       "INSERT INTO test_dml(name, value) VALUES ('alice', 10)",
			wantPrefix:  "OK:",
			wantContain: "",
		},
		{
			name:        "INSERT multiple rows",
			query:       "INSERT INTO test_dml(name, value) VALUES ('bob', 20), ('charlie', 30)",
			wantPrefix:  "OK:",
			wantContain: "",
		},
		{
			name:        "UPDATE returns OK prefix",
			query:       "UPDATE test_dml SET value = 99 WHERE name = 'alice'",
			wantPrefix:  "OK:",
			wantContain: "",
		},
		{
			name:        "DELETE returns OK prefix",
			query:       "DELETE FROM test_dml WHERE name = 'charlie'",
			wantPrefix:  "OK:",
			wantContain: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := execQuery(t, cm, tt.query)

			if tt.wantPrefix != "" && !strings.HasPrefix(text, tt.wantPrefix) {
				t.Errorf("query %q: result text = %q, want prefix %q", tt.query, text, tt.wantPrefix)
			}
			if tt.wantContain != "" && !strings.Contains(text, tt.wantContain) {
				t.Errorf("query %q: result text = %q, want it to contain %q", tt.query, text, tt.wantContain)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: CREATE TABLE DDL
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_CreateTableDDL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "CREATE TABLE test_ddl(id INT, label TEXT)")

	if !strings.HasPrefix(text, "OK:") {
		t.Errorf("CREATE TABLE result text = %q, want prefix %q", text, "OK:")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: invalid SQL
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_InvalidSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleExecuteQuery(ctx, makeExecuteQueryRequest("INVALID SQL GIBBERISH"))
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleExecuteQuery() IsError = false, want true for invalid SQL")
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "failed") && !strings.Contains(textLower, "error") {
		t.Errorf("HandleExecuteQuery() text = %q, want it to contain 'failed' or 'error'", text)
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: NULL values in results
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_NullValues(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "SELECT NULL AS val")

	if !strings.Contains(text, "NULL") {
		t.Errorf("result text = %q, want it to contain %q", text, "NULL")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: multiple rows
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_MultipleRows(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "SELECT generate_series(1,5)")

	if !strings.Contains(text, "5 row(s)") {
		t.Errorf("result text = %q, want it to contain %q", text, "5 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: WITH CTE
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_WithCTE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "WITH cte AS (SELECT 1 AS n) SELECT * FROM cte")

	if !strings.Contains(text, "row(s)") {
		t.Errorf("result text = %q, want it to contain %q", text, "row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleExecuteQuery(ctx, makeExecuteQueryRequest("SELECT 42 AS answer"))
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleExecuteQuery() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleExecuteQuery() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: empty query string
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_EmptyQueryString(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleExecuteQuery(ctx, makeExecuteQueryRequest(""))
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	// An empty query should be treated as an error
	if !result.IsError {
		t.Error("HandleExecuteQuery() with empty query: IsError = false, want true")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: SELECT with multiple columns
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_MultipleColumns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	text := execQuery(t, cm, "SELECT 1 AS a, 'hello' AS b, TRUE AS c")

	for _, col := range []string{"a", "b", "c"} {
		if !strings.Contains(text, col) {
			t.Errorf("result text = %q, want it to contain column %q", text, col)
		}
	}
	if !strings.Contains(text, "1 row(s)") {
		t.Errorf("result text = %q, want it to contain %q", text, "1 row(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleExecuteQuery: DROP TABLE DDL
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_DropTableDDL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create then drop
	execQuery(t, cm, "CREATE TABLE test_drop(id INT)")
	text := execQuery(t, cm, "DROP TABLE test_drop")

	if !strings.HasPrefix(text, "OK:") {
		t.Errorf("DROP TABLE result text = %q, want prefix %q", text, "OK:")
	}
}

// ===========================================================================
// HandleExecuteQuery: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleExecuteQuery_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup table for DML tests
	setupResult := execQueryResult(t, cm, "CREATE TABLE eq_cases(id SERIAL PRIMARY KEY, name TEXT, active BOOL DEFAULT TRUE)")
	if setupResult.IsError {
		t.Fatalf("setup CREATE TABLE failed: %s", resultText(t, setupResult))
	}

	tests := []struct {
		name         string
		query        string
		wantIsError  bool
		wantContains []string
		wantPrefix   string
	}{
		{
			name:         "simple SELECT returns rows",
			query:        "SELECT 'hello' AS greeting",
			wantIsError:  false,
			wantContains: []string{"greeting", "hello", "row(s)"},
		},
		{
			name:        "INSERT into created table",
			query:       "INSERT INTO eq_cases(name) VALUES ('test_user')",
			wantIsError: false,
			wantPrefix:  "OK:",
		},
		{
			name:        "UPDATE existing row",
			query:       "UPDATE eq_cases SET active = FALSE WHERE name = 'test_user'",
			wantIsError: false,
			wantPrefix:  "OK:",
		},
		{
			name:         "SELECT after UPDATE verifies change",
			query:        "SELECT name, active FROM eq_cases WHERE name = 'test_user'",
			wantIsError:  false,
			wantContains: []string{"test_user", "row(s)"},
		},
		{
			name:        "DELETE existing row",
			query:       "DELETE FROM eq_cases WHERE name = 'test_user'",
			wantIsError: false,
			wantPrefix:  "OK:",
		},
		{
			name:         "SELECT after DELETE returns 0 rows",
			query:        "SELECT * FROM eq_cases WHERE name = 'test_user'",
			wantIsError:  false,
			wantContains: []string{"0 row(s)"},
		},
		{
			name:         "SELECT with NULL",
			query:        "SELECT NULL AS empty_val",
			wantIsError:  false,
			wantContains: []string{"NULL"},
		},
		{
			name:         "WITH CTE query",
			query:        "WITH nums AS (SELECT generate_series(1,3) AS n) SELECT * FROM nums",
			wantIsError:  false,
			wantContains: []string{"3 row(s)"},
		},
		{
			name:        "reference nonexistent table",
			query:       "SELECT * FROM nonexistent_table_xyz",
			wantIsError: true,
		},
		{
			name:        "syntax error",
			query:       "SELECTT 1",
			wantIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleExecuteQuery(ctx, makeExecuteQueryRequest(tt.query))
			if err != nil {
				t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleExecuteQuery() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			text := resultText(t, result)

			if tt.wantPrefix != "" && !strings.HasPrefix(text, tt.wantPrefix) {
				t.Errorf("result text = %q, want prefix %q", text, tt.wantPrefix)
			}

			for _, substr := range tt.wantContains {
				if !strings.Contains(text, substr) {
					t.Errorf("result text = %q, want it to contain %q", text, substr)
				}
			}
		})
	}
}

// ===========================================================================
// HandleListTables Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleListTables: no container running
// ---------------------------------------------------------------------------

func Test_HandleListTables_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleListTables() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleListTables() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: empty database
// ---------------------------------------------------------------------------

func Test_HandleListTables_EmptyDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "no tables") && !strings.Contains(textLower, "0 table") {
		t.Errorf("HandleListTables() text = %q, want it to indicate no tables found", text)
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: after creating tables
// ---------------------------------------------------------------------------

func Test_HandleListTables_AfterCreatingTables(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create two tables
	createResult1 := execQueryResult(t, cm, "CREATE TABLE list_test_t1(id INT)")
	if createResult1.IsError {
		t.Fatalf("CREATE TABLE t1 failed: %s", resultText(t, createResult1))
	}
	createResult2 := execQueryResult(t, cm, "CREATE TABLE list_test_t2(id INT)")
	if createResult2.IsError {
		t.Fatalf("CREATE TABLE t2 failed: %s", resultText(t, createResult2))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleListTables() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)

	if !strings.Contains(text, "list_test_t1") {
		t.Errorf("result text = %q, want it to contain %q", text, "list_test_t1")
	}
	if !strings.Contains(text, "list_test_t2") {
		t.Errorf("result text = %q, want it to contain %q", text, "list_test_t2")
	}
	if !strings.Contains(text, "table(s)") {
		t.Errorf("result text = %q, want it to contain %q", text, "table(s)")
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: default schema (no schema param)
// ---------------------------------------------------------------------------

func Test_HandleListTables_DefaultSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create a table in public schema
	createResult := execQueryResult(t, cm, "CREATE TABLE default_schema_test(id INT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Call with no schema param -> should default to "public"
	result, err := cm.HandleListTables(ctx, makeListTablesRequest(""))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleListTables() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "default_schema_test") {
		t.Errorf("result text = %q, want it to contain %q (created in public schema)", text, "default_schema_test")
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: non-existent schema
// ---------------------------------------------------------------------------

func Test_HandleListTables_NonExistentSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("nonexistent_schema_xyz"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "no tables") && !strings.Contains(textLower, "0 table") {
		t.Errorf("HandleListTables() text = %q, want it to indicate no tables found", text)
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleListTables_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleListTables() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleListTables() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ===========================================================================
// HandleListTables: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleListTables_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create a table so the database is not empty for some tests
	createResult := execQueryResult(t, cm, "CREATE TABLE lt_cases_tbl(id INT)")
	if createResult.IsError {
		t.Fatalf("setup CREATE TABLE failed: %s", resultText(t, createResult))
	}

	tests := []struct {
		name         string
		schema       string
		wantIsError  bool
		wantContains []string
	}{
		{
			name:         "public schema shows created table",
			schema:       "public",
			wantIsError:  false,
			wantContains: []string{"lt_cases_tbl"},
		},
		{
			name:        "no schema defaults to public",
			schema:      "",
			wantIsError: false,
			// Should find the table we created in public
			wantContains: []string{"lt_cases_tbl"},
		},
		{
			name:        "nonexistent schema shows no tables",
			schema:      "does_not_exist",
			wantIsError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleListTables(ctx, makeListTablesRequest(tt.schema))
			if err != nil {
				t.Fatalf("HandleListTables() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleListTables() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			text := resultText(t, result)
			for _, substr := range tt.wantContains {
				if !strings.Contains(text, substr) {
					t.Errorf("result text = %q, want it to contain %q", text, substr)
				}
			}
		})
	}
}

// ===========================================================================
// HandleDescribeTable Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleDescribeTable: no container running
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_NoContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := NewContainerManager()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("some_table", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDescribeTable() IsError = false, want true when no container running")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("HandleDescribeTable() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: missing table parameter
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_MissingTableParam(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Request with no table argument
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "describe_table",
			Arguments: map[string]any{},
		},
	}

	result, err := cm.HandleDescribeTable(ctx, req)
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDescribeTable() IsError = false, want true when table param is missing")
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "required") && !strings.Contains(textLower, "missing") {
		t.Errorf("HandleDescribeTable() text = %q, want it to contain 'required' or 'missing' (case-insensitive)", text)
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: valid table with columns
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_ValidTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create a table with specific columns and types
	createResult := execQueryResult(t, cm, "CREATE TABLE describe_test_users(id SERIAL PRIMARY KEY, name TEXT NOT NULL, email VARCHAR(255), active BOOLEAN DEFAULT TRUE)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("describe_test_users", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)

	// Should contain column names
	if !strings.Contains(text, "id") {
		t.Errorf("result text = %q, want it to contain column %q", text, "id")
	}
	if !strings.Contains(text, "name") {
		t.Errorf("result text = %q, want it to contain column %q", text, "name")
	}
	if !strings.Contains(text, "email") {
		t.Errorf("result text = %q, want it to contain column %q", text, "email")
	}
	if !strings.Contains(text, "active") {
		t.Errorf("result text = %q, want it to contain column %q", text, "active")
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: table with PRIMARY KEY shows indexes
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_WithIndexes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	createResult := execQueryResult(t, cm, "CREATE TABLE describe_idx_test(id SERIAL PRIMARY KEY, value TEXT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("describe_idx_test", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)

	// Should contain an Indexes section since we have a PRIMARY KEY
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "index") {
		t.Errorf("result text = %q, want it to contain an 'Index' or 'Indexes' section for table with PRIMARY KEY", text)
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: nonexistent table
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_NonExistentTable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("nonexistent_table_xyz", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDescribeTable() IsError = false, want true for nonexistent table")
	}

	text := resultText(t, result)
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "not found") && !strings.Contains(textLower, "does not exist") && !strings.Contains(textLower, "no such") {
		t.Errorf("HandleDescribeTable() text = %q, want it to indicate table not found", text)
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: default schema
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_DefaultSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create table in default (public) schema
	createResult := execQueryResult(t, cm, "CREATE TABLE describe_default_schema(id INT, label TEXT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Describe without specifying schema -> should default to public
	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("describe_default_schema", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)
	if !strings.Contains(text, "id") {
		t.Errorf("result text = %q, want it to contain column %q", text, "id")
	}
	if !strings.Contains(text, "label") {
		t.Errorf("result text = %q, want it to contain column %q", text, "label")
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: result structure verification
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	createResult := execQueryResult(t, cm, "CREATE TABLE describe_content_test(id INT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("describe_content_test", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleDescribeTable() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleDescribeTable() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

// ===========================================================================
// HandleDescribeTable: Table-Driven Comprehensive Cases
// ===========================================================================

func Test_HandleDescribeTable_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Setup: create tables for tests
	tables := []struct {
		sql  string
		desc string
	}{
		{"CREATE TABLE dt_simple(id INT, name TEXT)", "simple table"},
		{"CREATE TABLE dt_with_pk(id SERIAL PRIMARY KEY, value TEXT NOT NULL)", "table with PK"},
		{"CREATE TABLE dt_multi_types(a INTEGER, b FLOAT, c BOOLEAN, d TIMESTAMP, e JSONB)", "multi-type table"},
	}
	for _, tbl := range tables {
		r := execQueryResult(t, cm, tbl.sql)
		if r.IsError {
			t.Fatalf("setup %s failed: %s", tbl.desc, resultText(t, r))
		}
	}

	tests := []struct {
		name         string
		table        string
		schema       string
		wantIsError  bool
		wantContains []string
	}{
		{
			name:         "simple table columns",
			table:        "dt_simple",
			schema:       "",
			wantIsError:  false,
			wantContains: []string{"id", "name"},
		},
		{
			name:         "table with primary key",
			table:        "dt_with_pk",
			schema:       "",
			wantIsError:  false,
			wantContains: []string{"id", "value"},
		},
		{
			name:         "table with multiple types",
			table:        "dt_multi_types",
			schema:       "",
			wantIsError:  false,
			wantContains: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:         "explicit public schema",
			table:        "dt_simple",
			schema:       "public",
			wantIsError:  false,
			wantContains: []string{"id", "name"},
		},
		{
			name:        "nonexistent table returns error",
			table:       "this_table_does_not_exist",
			schema:      "",
			wantIsError: true,
		},
		{
			name:        "table in nonexistent schema",
			table:       "dt_simple",
			schema:      "nonexistent_schema",
			wantIsError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest(tt.table, tt.schema))
			if err != nil {
				t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleDescribeTable() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			text := resultText(t, result)
			for _, substr := range tt.wantContains {
				if !strings.Contains(text, substr) {
					t.Errorf("result text = %q, want it to contain %q", text, substr)
				}
			}
		})
	}
}

// ===========================================================================
// Cross-Handler Integration Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// Integration: create table -> list tables -> describe table
// ---------------------------------------------------------------------------

func Test_Integration_CreateListDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Step 1: Create a table via execute_query
	createText := execQuery(t, cm, "CREATE TABLE integration_test(id SERIAL PRIMARY KEY, name TEXT NOT NULL, score FLOAT)")
	if !strings.HasPrefix(createText, "OK:") {
		t.Fatalf("CREATE TABLE did not return OK: %q", createText)
	}

	// Step 2: List tables should show it
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	listResult, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}
	if listResult.IsError {
		t.Fatalf("HandleListTables() IsError = true, text = %q", resultText(t, listResult))
	}

	listText := resultText(t, listResult)
	if !strings.Contains(listText, "integration_test") {
		t.Errorf("list tables text = %q, want it to contain %q", listText, "integration_test")
	}

	// Step 3: Describe the table should show columns
	descResult, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("integration_test", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}
	if descResult.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, descResult))
	}

	descText := resultText(t, descResult)
	for _, col := range []string{"id", "name", "score"} {
		if !strings.Contains(descText, col) {
			t.Errorf("describe table text = %q, want it to contain column %q", descText, col)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: create -> insert -> select roundtrip
// ---------------------------------------------------------------------------

func Test_Integration_InsertSelectRoundtrip(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create table
	createResult := execQueryResult(t, cm, "CREATE TABLE roundtrip(id SERIAL PRIMARY KEY, msg TEXT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	// Insert rows
	insertText := execQuery(t, cm, "INSERT INTO roundtrip(msg) VALUES ('hello'), ('world'), ('test')")
	if !strings.HasPrefix(insertText, "OK:") {
		t.Fatalf("INSERT did not return OK: %q", insertText)
	}

	// Select and verify count
	selectText := execQuery(t, cm, "SELECT * FROM roundtrip")
	if !strings.Contains(selectText, "3 row(s)") {
		t.Errorf("SELECT result = %q, want it to contain %q", selectText, "3 row(s)")
	}

	// Verify data content
	if !strings.Contains(selectText, "hello") {
		t.Errorf("SELECT result = %q, want it to contain %q", selectText, "hello")
	}
	if !strings.Contains(selectText, "world") {
		t.Errorf("SELECT result = %q, want it to contain %q", selectText, "world")
	}
}

// ---------------------------------------------------------------------------
// Integration: create -> drop -> list (table gone)
// ---------------------------------------------------------------------------

func Test_Integration_DropTableRemovesFromList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	// Create and then drop
	execQuery(t, cm, "CREATE TABLE drop_me(id INT)")
	execQuery(t, cm, "DROP TABLE drop_me")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleListTables(ctx, makeListTablesRequest("public"))
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	text := resultText(t, result)
	if strings.Contains(text, "drop_me") {
		t.Errorf("list tables text = %q, should NOT contain dropped table %q", text, "drop_me")
	}
}

// ---------------------------------------------------------------------------
// Integration: describe table with NOT NULL constraints
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_NotNullConstraint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	createResult := execQueryResult(t, cm, "CREATE TABLE not_null_test(id INT NOT NULL, name TEXT NOT NULL, optional_field TEXT)")
	if createResult.IsError {
		t.Fatalf("CREATE TABLE failed: %s", resultText(t, createResult))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleDescribeTable(ctx, makeDescribeTableRequest("not_null_test", ""))
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if result.IsError {
		t.Fatalf("HandleDescribeTable() IsError = true, text = %q", resultText(t, result))
	}

	text := resultText(t, result)

	// The describe output should contain column info including NOT NULL indicators
	if !strings.Contains(text, "id") {
		t.Errorf("result text = %q, want it to contain column %q", text, "id")
	}
	if !strings.Contains(text, "name") {
		t.Errorf("result text = %q, want it to contain column %q", text, "name")
	}
}

// ===========================================================================
// Nil Arguments Edge Cases
// ===========================================================================

// ---------------------------------------------------------------------------
// HandleExecuteQuery: nil arguments map
// ---------------------------------------------------------------------------

func Test_HandleExecuteQuery_NilArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute_query",
			Arguments: nil,
		},
	}

	result, err := cm.HandleExecuteQuery(ctx, req)
	if err != nil {
		t.Fatalf("HandleExecuteQuery() returned Go error: %v", err)
	}

	// With nil arguments, the required "query" param is missing
	if !result.IsError {
		t.Error("HandleExecuteQuery() with nil arguments: IsError = false, want true")
	}
}

// ---------------------------------------------------------------------------
// HandleDescribeTable: nil arguments map
// ---------------------------------------------------------------------------

func Test_HandleDescribeTable_NilArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "describe_table",
			Arguments: nil,
		},
	}

	result, err := cm.HandleDescribeTable(ctx, req)
	if err != nil {
		t.Fatalf("HandleDescribeTable() returned Go error: %v", err)
	}

	if !result.IsError {
		t.Error("HandleDescribeTable() with nil arguments: IsError = false, want true")
	}
}

// ---------------------------------------------------------------------------
// HandleListTables: nil arguments defaults to public
// ---------------------------------------------------------------------------

func Test_HandleListTables_NilArguments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container tests in short mode")
	}

	cm := startedContainerManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "list_tables",
			Arguments: nil,
		},
	}

	result, err := cm.HandleListTables(ctx, req)
	if err != nil {
		t.Fatalf("HandleListTables() returned Go error: %v", err)
	}

	// Should not be an error -- nil arguments means no schema param, which
	// should default to "public"
	if result.IsError {
		t.Errorf("HandleListTables() with nil arguments: IsError = true, want false; text = %q", resultText(t, result))
	}
}
