package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
)

// HandleExecuteQuery executes a SQL query against the managed PostgreSQL container.
// The query parameter is required and contains the SQL statement to execute.
// Returns a formatted result for SELECT queries or execution status for DDL/DML.
func (m *ContainerManager) HandleExecuteQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract required query parameter
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: query"), nil
	}
	if strings.TrimSpace(query) == "" {
		return mcp.NewToolResultError("Query cannot be empty"), nil
	}

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// Execute query
	result, err := executeSQL(ctx, conn, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query failed: %v", err)), nil
	}

	return mcp.NewToolResultText(result), nil
}

// HandleListTables lists all tables in the specified schema.
// The schema parameter is optional and defaults to "public".
// Returns a formatted list of tables with approximate row counts.
func (m *ContainerManager) HandleListTables(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract optional schema parameter
	schema := request.GetString("schema", "public")

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// List tables
	result, err := listTablesSQL(ctx, conn, schema)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list tables: %v", err)), nil
	}

	return mcp.NewToolResultText(result), nil
}

// HandleDescribeTable describes the structure of a table including columns and indexes.
// The table parameter is required. The schema parameter is optional and defaults to "public".
// Returns a formatted description of the table structure.
func (m *ContainerManager) HandleDescribeTable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract required table parameter
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError("Missing required parameter: table"), nil
	}

	// Extract optional schema parameter
	schema := request.GetString("schema", "public")

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// Describe table
	result, err := describeTableSQL(ctx, conn, schema, table)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to describe table: %v", err)), nil
	}

	return mcp.NewToolResultText(result), nil
}

// executeSQL routes the query to the appropriate handler based on query type.
// SELECT-like queries are handled by executeQueryRows, others by executeExec.
func executeSQL(ctx context.Context, conn *pgx.Conn, query string) (string, error) {
	// Normalize query for type detection
	trimmed := strings.TrimSpace(query)
	upper := strings.ToUpper(trimmed)

	// Detect SELECT-like queries
	if strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "TABLE") ||
		strings.HasPrefix(upper, "VALUES") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "EXPLAIN") {
		return executeQueryRows(ctx, conn, query)
	}

	// All other queries (DDL, DML)
	return executeExec(ctx, conn, query)
}

// executeQueryRows executes a query that returns rows and formats the result as a table.
// Formats column names, separator line, data rows, and row count.
func executeQueryRows(ctx context.Context, conn *pgx.Conn, query string) (string, error) {
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	// Get field descriptions for column names
	fields := rows.FieldDescriptions()
	columnNames := make([]string, len(fields))
	for i, field := range fields {
		columnNames[i] = string(field.Name)
	}

	// Build header row
	var result strings.Builder
	result.WriteString(strings.Join(columnNames, " | "))
	result.WriteString("\n")

	// Add separator line
	separators := make([]string, len(columnNames))
	for i := range separators {
		separators[i] = strings.Repeat("-", len(columnNames[i]))
	}
	result.WriteString(strings.Join(separators, "-|-"))
	result.WriteString("\n")

	// Iterate rows
	rowCount := 0
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return "", err
		}

		// Format each value
		formattedValues := make([]string, len(values))
		for i, val := range values {
			if val == nil {
				formattedValues[i] = "NULL"
			} else {
				formattedValues[i] = fmt.Sprintf("%v", val)
			}
		}

		result.WriteString(strings.Join(formattedValues, " | "))
		result.WriteString("\n")
		rowCount++
	}

	// Check for iteration errors
	if err := rows.Err(); err != nil {
		return "", err
	}

	// Add row count
	result.WriteString(fmt.Sprintf("\n(%d row(s))", rowCount))

	return result.String(), nil
}

// executeExec executes a non-query SQL statement (DDL, DML) and returns the command tag.
// Returns a success message with the number of affected rows for DML statements.
func executeExec(ctx context.Context, conn *pgx.Conn, query string) (string, error) {
	tag, err := conn.Exec(ctx, query)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("OK: %s", tag.String()), nil
}

// listTablesSQL queries information_schema for tables in the specified schema.
// Returns a formatted list of table names with approximate row counts.
func listTablesSQL(ctx context.Context, conn *pgx.Conn, schema string) (string, error) {
	query := `
		SELECT t.table_name, COALESCE(c.reltuples, 0)::bigint AS row_count
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.table_schema
		WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
		ORDER BY t.table_name
	`

	rows, err := conn.Query(ctx, query, schema)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var result strings.Builder
	result.WriteString("Table Name | Rows (approx)\n")
	result.WriteString("-----------|---------------\n")

	tableCount := 0
	for rows.Next() {
		var tableName string
		var rowCount int64
		if err := rows.Scan(&tableName, &rowCount); err != nil {
			return "", err
		}

		result.WriteString(fmt.Sprintf("%s | %d\n", tableName, rowCount))
		tableCount++
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if tableCount == 0 {
		result.WriteString("(no tables found)\n")
	}

	result.WriteString(fmt.Sprintf("\n%d table(s) total", tableCount))

	return result.String(), nil
}

// describeTableSQL queries information_schema for column definitions and indexes.
// Returns a formatted description of the table structure including column types,
// nullability, defaults, and indexes.
func describeTableSQL(ctx context.Context, conn *pgx.Conn, schema, table string) (string, error) {
	// Query for column definitions
	columnQuery := `
		SELECT
			column_name,
			CASE
				WHEN data_type = 'character varying' THEN 'varchar(' || character_maximum_length || ')'
				WHEN data_type = 'character' THEN 'char(' || character_maximum_length || ')'
				ELSE data_type
			END AS data_type,
			is_nullable,
			column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`

	rows, err := conn.Query(ctx, columnQuery, schema, table)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var result strings.Builder
	result.WriteString("Column | Type | Nullable | Default\n")
	result.WriteString("-------|------|----------|--------\n")

	columnCount := 0
	for rows.Next() {
		var columnName, dataType, isNullable string
		var columnDefault *string

		if err := rows.Scan(&columnName, &dataType, &isNullable, &columnDefault); err != nil {
			return "", err
		}

		defaultVal := "NULL"
		if columnDefault != nil {
			defaultVal = *columnDefault
		}

		result.WriteString(fmt.Sprintf("%s | %s | %s | %s\n",
			columnName, dataType, isNullable, defaultVal))
		columnCount++
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if columnCount == 0 {
		return "", fmt.Errorf("table '%s.%s' not found", schema, table)
	}

	// Query for indexes
	indexQuery := `
		SELECT
			indexname,
			indexdef
		FROM pg_indexes
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY indexname
	`

	indexRows, err := conn.Query(ctx, indexQuery, schema, table)
	if err != nil {
		return "", err
	}
	defer indexRows.Close()

	result.WriteString("\nIndexes:\n")
	indexCount := 0
	for indexRows.Next() {
		var indexName, indexDef string
		if err := indexRows.Scan(&indexName, &indexDef); err != nil {
			return "", err
		}

		result.WriteString(fmt.Sprintf("  %s: %s\n", indexName, indexDef))
		indexCount++
	}

	if err := indexRows.Err(); err != nil {
		return "", err
	}

	if indexCount == 0 {
		result.WriteString("  (no indexes)\n")
	}

	return result.String(), nil
}
