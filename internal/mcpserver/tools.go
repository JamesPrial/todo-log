// Package mcpserver provides an MCP server implementation for PostgreSQL database operations.
package mcpserver

import (
	"github.com/mark3labs/mcp-go/mcp"
)

// startPostgresTool returns a tool definition for starting a PostgreSQL container.
func startPostgresTool() mcp.Tool {
	return mcp.NewTool("start_postgres",
		mcp.WithDescription("Start a PostgreSQL container using testcontainers-go. Optional parameters allow customization of the database instance."),
		mcp.WithString("password",
			mcp.Description("PostgreSQL password for the postgres user")),
		mcp.WithString("database",
			mcp.Description("Name of the default database to create")),
		mcp.WithString("image",
			mcp.Description("Docker image to use for PostgreSQL (e.g., postgres:16-alpine)")),
	)
}

// stopPostgresTool returns a tool definition for stopping the PostgreSQL container.
func stopPostgresTool() mcp.Tool {
	return mcp.NewTool("stop_postgres",
		mcp.WithDescription("Stop the running PostgreSQL container and clean up resources."),
	)
}

// postgresStatusTool returns a tool definition for checking PostgreSQL container status.
func postgresStatusTool() mcp.Tool {
	return mcp.NewTool("postgres_status",
		mcp.WithDescription("Get the current status of the PostgreSQL container, including connection details if running."),
	)
}

// executeQueryTool returns a tool definition for executing arbitrary SQL queries.
func executeQueryTool() mcp.Tool {
	return mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an arbitrary SQL query against the PostgreSQL database. Returns results for SELECT queries or row counts for modifications."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("The SQL query to execute")),
	)
}

// listTablesTool returns a tool definition for listing database tables.
func listTablesTool() mcp.Tool {
	return mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in the database. Optionally filter by schema."),
		mcp.WithString("schema",
			mcp.Description("Schema name to filter tables (defaults to 'public' if not specified)")),
	)
}

// describeTableTool returns a tool definition for describing table structure.
func describeTableTool() mcp.Tool {
	return mcp.NewTool("describe_table",
		mcp.WithDescription("Describe the structure of a table, including columns, types, and constraints."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to describe")),
		mcp.WithString("schema",
			mcp.Description("Schema name (defaults to 'public' if not specified)")),
	)
}

// insertRowsTool returns a tool definition for inserting rows into a table.
func insertRowsTool() mcp.Tool {
	return mcp.NewTool("insert_rows",
		mcp.WithDescription("Insert one or more rows into a table. Columns and values must align."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to insert into")),
		mcp.WithArray("columns",
			mcp.Required(),
			mcp.Description("Array of column names")),
		mcp.WithArray("values",
			mcp.Required(),
			mcp.Description("Array of arrays, where each inner array represents a row of values")),
	)
}

// updateRowsTool returns a tool definition for updating rows in a table.
func updateRowsTool() mcp.Tool {
	return mcp.NewTool("update_rows",
		mcp.WithDescription("Update rows in a table based on a WHERE clause."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to update")),
		mcp.WithObject("set",
			mcp.Required(),
			mcp.Description("Object mapping column names to new values")),
		mcp.WithString("where",
			mcp.Required(),
			mcp.Description("WHERE clause to identify rows to update (e.g., 'id = 1')")),
	)
}

// deleteRowsTool returns a tool definition for deleting rows from a table.
func deleteRowsTool() mcp.Tool {
	return mcp.NewTool("delete_rows",
		mcp.WithDescription("Delete rows from a table based on a WHERE clause."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the table to delete from")),
		mcp.WithString("where",
			mcp.Required(),
			mcp.Description("WHERE clause to identify rows to delete (e.g., 'id = 1')")),
	)
}
