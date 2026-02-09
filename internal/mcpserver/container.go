package mcpserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// ContainerManager manages the lifecycle of a PostgreSQL container.
// It provides thread-safe access to container state and connection details.
type ContainerManager struct {
	mu        sync.Mutex
	container *postgres.PostgresContainer
	connStr   string
	startedAt time.Time
}

// NewContainerManager creates a new ContainerManager instance.
// The returned manager has no running container initially.
func NewContainerManager() *ContainerManager {
	return &ContainerManager{}
}

// ConnStr returns the current PostgreSQL connection string.
// Returns an empty string if no container is running.
// This method is thread-safe.
func (m *ContainerManager) ConnStr() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connStr
}

const (
	defaultPassword = "todolog"
	defaultDatabase = "todolog"
	defaultImage    = "postgres:16-alpine"
	defaultUsername = "postgres"
	containerLabel  = "todo-log-postgres"
)

// HandleStartPostgres starts a PostgreSQL container with optional configuration.
// Parameters:
//   - password: PostgreSQL password (default: "todolog")
//   - database: Database name (default: "todolog")
//   - image: Docker image (default: "postgres:16-alpine")
//
// Returns an error result if a container is already running.
func (m *ContainerManager) HandleStartPostgres(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if container already running
	if m.container != nil {
		return mcp.NewToolResultText(fmt.Sprintf("PostgreSQL container already running.\nConnection string: %s", m.connStr)), nil
	}

	// Extract optional parameters
	password := request.GetString("password", defaultPassword)
	database := request.GetString("database", defaultDatabase)
	image := request.GetString("image", defaultImage)

	// Start container
	pgContainer, err := postgres.Run(ctx,
		image,
		postgres.WithDatabase(database),
		postgres.WithUsername(defaultUsername),
		postgres.WithPassword(password),
		postgres.BasicWaitStrategies(),
		testcontainers.WithLabels(map[string]string{
			"managed-by": containerLabel,
		}),
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start PostgreSQL container: %v", err)), nil
	}

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		// Terminate container on failure
		_ = testcontainers.TerminateContainer(pgContainer)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get connection string: %v", err)), nil
	}

	// Store container state
	m.container = pgContainer
	m.connStr = connStr
	m.startedAt = time.Now()

	return mcp.NewToolResultText(fmt.Sprintf("PostgreSQL container started successfully.\nConnection string: %s", connStr)), nil
}

// HandleStopPostgres stops and removes the managed PostgreSQL container.
// Returns an error result if no container is currently running.
func (m *ContainerManager) HandleStopPostgres(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if container exists
	if m.container == nil {
		return mcp.NewToolResultError("No PostgreSQL container is running."), nil
	}

	// Terminate container
	if err := testcontainers.TerminateContainer(m.container); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to stop container: %v", err)), nil
	}

	// Clear container state
	m.container = nil
	m.connStr = ""
	m.startedAt = time.Time{}

	return mcp.NewToolResultText("PostgreSQL container stopped and removed."), nil
}

// HandlePostgresStatus returns the current status of the managed PostgreSQL container.
// Returns detailed information if a container is running, or a message indicating
// no container is managed.
func (m *ContainerManager) HandlePostgresStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if container exists
	if m.container == nil {
		return mcp.NewToolResultText("Status: No container managed.\nNo PostgreSQL container has been started."), nil
	}

	// Get container state
	state, err := m.container.State(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get container state: %v", err)), nil
	}

	// Calculate uptime
	uptime := time.Since(m.startedAt)

	// Get container ID
	containerID := m.container.GetContainerID()
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	// Format status message
	status := fmt.Sprintf("Status: Container running\nContainer ID: %s\nConnection string: %s\nUptime: %s\nRunning: %t",
		containerID,
		m.connStr,
		uptime.Round(time.Second),
		state.Running,
	)

	return mcp.NewToolResultText(status), nil
}
