package mcpserver

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
)

// identifierRegex validates SQL identifiers (table and column names).
// Only allows alphanumeric characters and underscores, must start with letter or underscore.
var identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// isValidIdentifier checks if a name is a valid SQL identifier.
// This prevents SQL injection for table and column names which cannot be parameterized.
func isValidIdentifier(name string) bool {
	return identifierRegex.MatchString(name)
}

// HandleInsertRows inserts one or more rows into a table.
// Parameters:
//   - table (string, required): table name
//   - columns ([]any -> []string, required): column names
//   - values ([]any -> [][]any, required): rows to insert
//
// Returns the number of rows inserted or an error.
func (m *ContainerManager) HandleInsertRows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract required parameters
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultError("Missing required parameters"), nil
	}

	tableName, ok := args["table"].(string)
	if !ok || tableName == "" {
		return mcp.NewToolResultError("Missing required parameter: table"), nil
	}
	if !isValidIdentifier(tableName) {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid table name: %s", tableName)), nil
	}

	columnsRaw, ok := args["columns"].([]any)
	if !ok || len(columnsRaw) == 0 {
		return mcp.NewToolResultError("Missing required parameter: columns"), nil
	}

	// Convert columns from []any to []string
	columns := make([]string, len(columnsRaw))
	for i, col := range columnsRaw {
		colStr, ok := col.(string)
		if !ok || colStr == "" {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid column at index %d", i)), nil
		}
		if !isValidIdentifier(colStr) {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid column name: %s", colStr)), nil
		}
		columns[i] = colStr
	}

	valuesRaw, ok := args["values"].([]any)
	if !ok || len(valuesRaw) == 0 {
		return mcp.NewToolResultError("Missing required parameter: values"), nil
	}

	// Validate and convert values
	values := make([][]any, len(valuesRaw))
	for i, rowRaw := range valuesRaw {
		rowSlice, ok := rowRaw.([]any)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid row at index %d: expected array", i)), nil
		}
		if len(rowSlice) != len(columns) {
			return mcp.NewToolResultError(fmt.Sprintf("Row %d has %d values but expected %d (column count mismatch)", i, len(rowSlice), len(columns))), nil
		}
		values[i] = rowSlice
	}

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// Build parameterized INSERT query
	// INSERT INTO tablename (col1, col2) VALUES ($1, $2), ($3, $4), ...
	var queryBuilder strings.Builder
	queryBuilder.WriteString("INSERT INTO ")
	queryBuilder.WriteString(tableName)
	queryBuilder.WriteString(" (")
	queryBuilder.WriteString(strings.Join(columns, ", "))
	queryBuilder.WriteString(") VALUES ")

	// Build placeholders for each row
	placeholders := make([]string, len(values))
	paramIndex := 1
	for i := range values {
		rowPlaceholders := make([]string, len(columns))
		for j := range columns {
			rowPlaceholders[j] = fmt.Sprintf("$%d", paramIndex)
			paramIndex++
		}
		placeholders[i] = "(" + strings.Join(rowPlaceholders, ", ") + ")"
	}
	queryBuilder.WriteString(strings.Join(placeholders, ", "))

	query := queryBuilder.String()

	// Flatten values for parameterized query
	flatValues := make([]any, 0, len(values)*len(columns))
	for _, row := range values {
		flatValues = append(flatValues, row...)
	}

	// Execute insert
	tag, err := conn.Exec(ctx, query, flatValues...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Insert failed: %v", err)), nil
	}

	rowsAffected := tag.RowsAffected()
	return mcp.NewToolResultText(fmt.Sprintf("Inserted %d row(s) into %s", rowsAffected, tableName)), nil
}

// HandleUpdateRows updates rows in a table based on a WHERE clause.
// Parameters:
//   - table (string, required): table name
//   - set (map[string]any, required): column-value pairs to update
//   - where (string, required): WHERE clause
//
// Returns the number of rows updated or an error.
func (m *ContainerManager) HandleUpdateRows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract required parameters
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultError("Missing required parameters"), nil
	}

	tableName, ok := args["table"].(string)
	if !ok || tableName == "" {
		return mcp.NewToolResultError("Missing required parameter: table"), nil
	}
	if !isValidIdentifier(tableName) {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid table name: %s", tableName)), nil
	}

	setRaw, ok := args["set"].(map[string]any)
	if !ok || len(setRaw) == 0 {
		return mcp.NewToolResultError("Missing required parameter: set"), nil
	}

	whereClause, ok := args["where"].(string)
	if !ok || strings.TrimSpace(whereClause) == "" {
		return mcp.NewToolResultError("Missing required parameter: where"), nil
	}

	// Validate column names in set map
	setClauses := make([]string, 0, len(setRaw))
	setValues := make([]any, 0, len(setRaw))
	paramIndex := 1
	for colName, colValue := range setRaw {
		if !isValidIdentifier(colName) {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid column name: %s", colName)), nil
		}
		setClauses = append(setClauses, fmt.Sprintf("%s=$%d", colName, paramIndex))
		setValues = append(setValues, colValue)
		paramIndex++
	}

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// Build UPDATE query
	// UPDATE tablename SET col1=$1, col2=$2 WHERE <where_clause>
	var queryBuilder strings.Builder
	queryBuilder.WriteString("UPDATE ")
	queryBuilder.WriteString(tableName)
	queryBuilder.WriteString(" SET ")
	queryBuilder.WriteString(strings.Join(setClauses, ", "))
	queryBuilder.WriteString(" WHERE ")
	queryBuilder.WriteString(whereClause)

	query := queryBuilder.String()

	// Execute update
	tag, err := conn.Exec(ctx, query, setValues...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Update failed: %v", err)), nil
	}

	rowsAffected := tag.RowsAffected()
	return mcp.NewToolResultText(fmt.Sprintf("Updated %d row(s) in %s", rowsAffected, tableName)), nil
}

// HandleDeleteRows deletes rows from a table based on a WHERE clause.
// Parameters:
//   - table (string, required): table name
//   - where (string, required): WHERE clause
//
// Returns the number of rows deleted or an error.
func (m *ContainerManager) HandleDeleteRows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lock to check container state
	m.mu.Lock()
	if m.container == nil || m.connStr == "" {
		m.mu.Unlock()
		return mcp.NewToolResultError("No PostgreSQL container is running. Use start_postgres first."), nil
	}
	connStr := m.connStr
	m.mu.Unlock()

	// Extract required parameters
	args := request.GetArguments()
	if args == nil {
		return mcp.NewToolResultError("Missing required parameters"), nil
	}

	tableName, ok := args["table"].(string)
	if !ok || tableName == "" {
		return mcp.NewToolResultError("Missing required parameter: table"), nil
	}
	if !isValidIdentifier(tableName) {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid table name: %s", tableName)), nil
	}

	whereClause, ok := args["where"].(string)
	if !ok || strings.TrimSpace(whereClause) == "" {
		return mcp.NewToolResultError("Missing required parameter: where"), nil
	}

	// Connect to database
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to database: %v", err)), nil
	}
	defer func() { _ = conn.Close(ctx) }()

	// Build DELETE query
	// DELETE FROM tablename WHERE <where_clause>
	query := fmt.Sprintf("DELETE FROM %s WHERE %s", tableName, whereClause)

	// Execute delete
	tag, err := conn.Exec(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Delete failed: %v", err)), nil
	}

	rowsAffected := tag.RowsAffected()
	return mcp.NewToolResultText(fmt.Sprintf("Deleted %d row(s) from %s", rowsAffected, tableName)), nil
}
