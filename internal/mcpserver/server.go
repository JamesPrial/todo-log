package mcpserver

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures a new MCP server with all PostgreSQL tools registered.
// Each server instance has its own ContainerManager for independent container lifecycle management.
func NewServer() (*server.MCPServer, error) {
	cm := NewContainerManager()

	s := server.NewMCPServer(
		"todo-log-postgres",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Container lifecycle tools
	s.AddTool(startPostgresTool(), cm.HandleStartPostgres)
	s.AddTool(stopPostgresTool(), cm.HandleStopPostgres)
	s.AddTool(postgresStatusTool(), cm.HandlePostgresStatus)

	// Query execution tools
	s.AddTool(executeQueryTool(), cm.HandleExecuteQuery)
	s.AddTool(listTablesTool(), cm.HandleListTables)
	s.AddTool(describeTableTool(), cm.HandleDescribeTable)

	// CRUD helper tools
	s.AddTool(insertRowsTool(), cm.HandleInsertRows)
	s.AddTool(updateRowsTool(), cm.HandleUpdateRows)
	s.AddTool(deleteRowsTool(), cm.HandleDeleteRows)

	return s, nil
}
