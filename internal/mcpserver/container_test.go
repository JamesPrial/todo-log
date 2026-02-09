package mcpserver

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// NewContainerManager: basic construction
// ---------------------------------------------------------------------------

func Test_NewContainerManager_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	cm := NewContainerManager()
	if cm == nil {
		t.Fatal("NewContainerManager() returned nil")
	}
}

// ---------------------------------------------------------------------------
// NewContainerManager: initial state has no container running
// ---------------------------------------------------------------------------

func Test_NewContainerManager_InitialState_NoContainerRunning(t *testing.T) {
	t.Parallel()

	cm := NewContainerManager()
	if cm == nil {
		t.Fatal("NewContainerManager() returned nil")
	}

	// A freshly created ContainerManager should have an empty connection
	// string, indicating no container is running.
	connStr := cm.ConnStr()
	if connStr != "" {
		t.Errorf("NewContainerManager().ConnStr() = %q, want empty string for initial state", connStr)
	}
}

// ---------------------------------------------------------------------------
// NewContainerManager: multiple instances are independent
// ---------------------------------------------------------------------------

func Test_NewContainerManager_MultipleInstancesAreIndependent(t *testing.T) {
	t.Parallel()

	cm1 := NewContainerManager()
	cm2 := NewContainerManager()

	if cm1 == nil || cm2 == nil {
		t.Fatal("NewContainerManager() returned nil")
	}

	if cm1 == cm2 {
		t.Error("NewContainerManager() returned the same pointer for two calls, expected independent instances")
	}
}

// ===========================================================================
// Stage 2: Container Lifecycle Handler Tests
// ===========================================================================

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dockerAvailable checks whether Docker is available on the host.
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// newTestContainerManager creates a ContainerManager for testing and registers
// a cleanup function that stops any running container when the test finishes.
// It skips the test if Docker is not available.
func newTestContainerManager(t *testing.T) *ContainerManager {
	t.Helper()
	if !dockerAvailable() {
		t.Skip("Docker not available, skipping container tests")
	}
	cm := NewContainerManager()
	t.Cleanup(func() {
		ctx := context.Background()
		req := mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name: "stop_postgres",
			},
		}
		_, _ = cm.HandleStopPostgres(ctx, req)
	})
	return cm
}

// makeStartRequest creates a CallToolRequest for HandleStartPostgres with the
// given optional arguments. Pass nil for defaults.
func makeStartRequest(args map[string]any) mcp.CallToolRequest {
	if args == nil {
		args = map[string]any{}
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "start_postgres",
			Arguments: args,
		},
	}
}

// makeStopRequest creates a CallToolRequest for HandleStopPostgres.
func makeStopRequest() mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "stop_postgres",
		},
	}
}

// makeStatusRequest creates a CallToolRequest for HandlePostgresStatus.
func makeStatusRequest() mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "postgres_status",
		},
	}
}

// resultText extracts the text content from the first Content element of a
// CallToolResult. It calls t.Fatal if the result is nil or has no content.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result has no Content elements")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
	return tc.Text
}

// assertTextContains checks that the result text contains the given substring.
func assertTextContains(t *testing.T, result *mcp.CallToolResult, substr string) {
	t.Helper()
	text := resultText(t, result)
	if !strings.Contains(text, substr) {
		t.Errorf("result text = %q, want it to contain %q", text, substr)
	}
}

// ---------------------------------------------------------------------------
// HandleStartPostgres: start with defaults
// ---------------------------------------------------------------------------

func Test_HandleStartPostgres_WithDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, result))
	}

	assertTextContains(t, result, "started successfully")
	assertTextContains(t, result, "Connection string:")

	connStr := cm.ConnStr()
	if connStr == "" {
		t.Error("ConnStr() is empty after successful start, want non-empty connection string")
	}
}

// ---------------------------------------------------------------------------
// HandleStartPostgres: start with custom password
// ---------------------------------------------------------------------------

func Test_HandleStartPostgres_WithCustomPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := cm.HandleStartPostgres(ctx, makeStartRequest(map[string]any{
		"password": "custom123",
	}))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, result))
	}

	assertTextContains(t, result, "started successfully")
}

// ---------------------------------------------------------------------------
// HandleStartPostgres: start with custom database
// ---------------------------------------------------------------------------

func Test_HandleStartPostgres_WithCustomDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := cm.HandleStartPostgres(ctx, makeStartRequest(map[string]any{
		"database": "mydb",
	}))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, result))
	}

	assertTextContains(t, result, "started successfully")
}

// ---------------------------------------------------------------------------
// HandleStartPostgres: already running (double start)
// ---------------------------------------------------------------------------

func Test_HandleStartPostgres_AlreadyRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// First start
	result1, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("first HandleStartPostgres() returned error: %v", err)
	}
	if result1.IsError {
		t.Fatalf("first HandleStartPostgres() IsError = true, text = %q", resultText(t, result1))
	}

	connStr1 := cm.ConnStr()
	if connStr1 == "" {
		t.Fatal("ConnStr() is empty after first start")
	}

	// Second start (already running)
	result2, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("second HandleStartPostgres() returned error: %v", err)
	}

	assertTextContains(t, result2, "already running")

	connStr2 := cm.ConnStr()
	if connStr2 != connStr1 {
		t.Errorf("ConnStr() changed after double start: first = %q, second = %q", connStr1, connStr2)
	}
}

// ---------------------------------------------------------------------------
// HandleStartPostgres: context cancelled
// ---------------------------------------------------------------------------

func Test_HandleStartPostgres_ContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))

	// The handler may return an error directly or return a result with IsError.
	// Either way, we need to verify it surfaces the cancellation.
	if err != nil {
		// If it returns a Go error, that is acceptable for context cancellation
		if !strings.Contains(err.Error(), "cancel") && !strings.Contains(err.Error(), "context") {
			t.Errorf("HandleStartPostgres() error = %v, want context-related error", err)
		}
		return
	}

	// If no Go error, the result should indicate an error condition
	if result == nil {
		t.Fatal("HandleStartPostgres() returned nil result and nil error")
	}
	text := resultText(t, result)
	if !result.IsError && !strings.Contains(strings.ToLower(text), "cancel") && !strings.Contains(strings.ToLower(text), "context") {
		t.Errorf("HandleStartPostgres() with cancelled context: IsError = %v, text = %q; want error indication", result.IsError, text)
	}
}

// ---------------------------------------------------------------------------
// HandleStopPostgres: stop running container
// ---------------------------------------------------------------------------

func Test_HandleStopPostgres_StopRunningContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start a container first
	startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, startResult))
	}

	if cm.ConnStr() == "" {
		t.Fatal("ConnStr() is empty after start")
	}

	// Stop the container
	stopResult, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}
	if stopResult.IsError {
		t.Fatalf("HandleStopPostgres() IsError = true, text = %q", resultText(t, stopResult))
	}

	assertTextContains(t, stopResult, "stopped and removed")

	connStr := cm.ConnStr()
	if connStr != "" {
		t.Errorf("ConnStr() = %q after stop, want empty string", connStr)
	}
}

// ---------------------------------------------------------------------------
// HandleStopPostgres: stop when nothing running
// ---------------------------------------------------------------------------

func Test_HandleStopPostgres_NothingRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}

	if !result.IsError {
		t.Errorf("HandleStopPostgres() IsError = false when no container running, want true")
	}

	assertTextContains(t, result, "No PostgreSQL container")
}

// ---------------------------------------------------------------------------
// HandleStopPostgres: double stop
// ---------------------------------------------------------------------------

func Test_HandleStopPostgres_DoubleStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start then stop
	startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, startResult))
	}

	// First stop
	stop1, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("first HandleStopPostgres() returned error: %v", err)
	}
	if stop1.IsError {
		t.Fatalf("first HandleStopPostgres() IsError = true, text = %q", resultText(t, stop1))
	}

	// Second stop (nothing running)
	stop2, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("second HandleStopPostgres() returned error: %v", err)
	}

	if !stop2.IsError {
		t.Error("second HandleStopPostgres() IsError = false, want true (nothing to stop)")
	}

	text := resultText(t, stop2)
	if !strings.Contains(text, "No PostgreSQL container") {
		t.Errorf("second HandleStopPostgres() text = %q, want it to contain %q", text, "No PostgreSQL container")
	}
}

// ---------------------------------------------------------------------------
// HandleStopPostgres: start after stop (full lifecycle)
// ---------------------------------------------------------------------------

func Test_HandleStopPostgres_StartAfterStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Start
	start1, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("first HandleStartPostgres() returned error: %v", err)
	}
	if start1.IsError {
		t.Fatalf("first HandleStartPostgres() IsError = true, text = %q", resultText(t, start1))
	}

	connStr1 := cm.ConnStr()
	if connStr1 == "" {
		t.Fatal("ConnStr() is empty after first start")
	}

	// Stop
	stopResult, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}
	if stopResult.IsError {
		t.Fatalf("HandleStopPostgres() IsError = true, text = %q", resultText(t, stopResult))
	}

	if cm.ConnStr() != "" {
		t.Errorf("ConnStr() = %q after stop, want empty", cm.ConnStr())
	}

	// Start again
	start2, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("second HandleStartPostgres() returned error: %v", err)
	}
	if start2.IsError {
		t.Fatalf("second HandleStartPostgres() IsError = true, text = %q", resultText(t, start2))
	}

	assertTextContains(t, start2, "started successfully")

	connStr2 := cm.ConnStr()
	if connStr2 == "" {
		t.Error("ConnStr() is empty after second start, want non-empty")
	}
}

// ---------------------------------------------------------------------------
// HandlePostgresStatus: no container managed
// ---------------------------------------------------------------------------

func Test_HandlePostgresStatus_NoContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandlePostgresStatus(ctx, makeStatusRequest())
	if err != nil {
		t.Fatalf("HandlePostgresStatus() returned error: %v", err)
	}

	assertTextContains(t, result, "No container managed")
}

// ---------------------------------------------------------------------------
// HandlePostgresStatus: container running
// ---------------------------------------------------------------------------

func Test_HandlePostgresStatus_ContainerRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start a container
	startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, startResult))
	}

	// Check status
	statusResult, err := cm.HandlePostgresStatus(ctx, makeStatusRequest())
	if err != nil {
		t.Fatalf("HandlePostgresStatus() returned error: %v", err)
	}
	if statusResult.IsError {
		t.Fatalf("HandlePostgresStatus() IsError = true, text = %q", resultText(t, statusResult))
	}

	text := resultText(t, statusResult)

	// Should indicate running state
	textLower := strings.ToLower(text)
	if !strings.Contains(textLower, "running") {
		t.Errorf("HandlePostgresStatus() text = %q, want it to contain 'running' (case-insensitive)", text)
	}

	// Should include connection string
	connStr := cm.ConnStr()
	if connStr == "" {
		t.Fatal("ConnStr() is empty while container is running")
	}
	if !strings.Contains(text, connStr) && !strings.Contains(textLower, "connection") {
		t.Errorf("HandlePostgresStatus() text = %q, want it to contain connection string or connection info", text)
	}
}

// ---------------------------------------------------------------------------
// HandlePostgresStatus: after stop
// ---------------------------------------------------------------------------

func Test_HandlePostgresStatus_AfterStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start
	startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, startResult))
	}

	// Stop
	stopResult, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}
	if stopResult.IsError {
		t.Fatalf("HandleStopPostgres() IsError = true, text = %q", resultText(t, stopResult))
	}

	// Check status after stop
	statusResult, err := cm.HandlePostgresStatus(ctx, makeStatusRequest())
	if err != nil {
		t.Fatalf("HandlePostgresStatus() returned error: %v", err)
	}

	assertTextContains(t, statusResult, "No container managed")
}

// ===========================================================================
// Table-Driven: HandleStartPostgres parameter variations
// ===========================================================================

func Test_HandleStartPostgres_ParameterVariations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}
	if !dockerAvailable() {
		t.Skip("Docker not available, skipping container tests")
	}

	tests := []struct {
		name string
		args map[string]any
	}{
		{
			name: "defaults only",
			args: nil,
		},
		{
			name: "custom password only",
			args: map[string]any{"password": "testpw456"},
		},
		{
			name: "custom database only",
			args: map[string]any{"database": "testdb"},
		},
		{
			name: "custom password and database",
			args: map[string]any{"password": "pw789", "database": "customdb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// NOT parallel: each subtest creates and tears down a container
			cm := NewContainerManager()
			t.Cleanup(func() {
				ctx := context.Background()
				_, _ = cm.HandleStopPostgres(ctx, makeStopRequest())
			})

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			result, err := cm.HandleStartPostgres(ctx, makeStartRequest(tt.args))
			if err != nil {
				t.Fatalf("HandleStartPostgres() returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, result))
			}

			assertTextContains(t, result, "started successfully")

			connStr := cm.ConnStr()
			if connStr == "" {
				t.Error("ConnStr() is empty after successful start")
			}
		})
	}
}

// ===========================================================================
// Table-Driven: HandleStopPostgres scenarios
// ===========================================================================

func Test_HandleStopPostgres_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}
	if !dockerAvailable() {
		t.Skip("Docker not available, skipping container tests")
	}

	tests := []struct {
		name        string
		startFirst  bool
		wantIsError bool
		wantContain string
	}{
		{
			name:        "stop running container succeeds",
			startFirst:  true,
			wantIsError: false,
			wantContain: "stopped and removed",
		},
		{
			name:        "stop when nothing running returns error",
			startFirst:  false,
			wantIsError: true,
			wantContain: "No PostgreSQL container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewContainerManager()
			t.Cleanup(func() {
				ctx := context.Background()
				_, _ = cm.HandleStopPostgres(ctx, makeStopRequest())
			})

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			if tt.startFirst {
				startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
				if err != nil {
					t.Fatalf("HandleStartPostgres() setup returned error: %v", err)
				}
				if startResult.IsError {
					t.Fatalf("HandleStartPostgres() setup IsError = true, text = %q", resultText(t, startResult))
				}
			}

			result, err := cm.HandleStopPostgres(ctx, makeStopRequest())
			if err != nil {
				t.Fatalf("HandleStopPostgres() returned error: %v", err)
			}

			if result.IsError != tt.wantIsError {
				t.Errorf("HandleStopPostgres() IsError = %v, want %v; text = %q",
					result.IsError, tt.wantIsError, resultText(t, result))
			}

			assertTextContains(t, result, tt.wantContain)

			if tt.startFirst {
				// After stopping, ConnStr should be empty
				if cm.ConnStr() != "" {
					t.Errorf("ConnStr() = %q after stop, want empty", cm.ConnStr())
				}
			}
		})
	}
}

// ===========================================================================
// Table-Driven: HandlePostgresStatus scenarios
// ===========================================================================

func Test_HandlePostgresStatus_Cases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}
	if !dockerAvailable() {
		t.Skip("Docker not available, skipping container tests")
	}

	tests := []struct {
		name        string
		setup       string // "none", "start", "start_then_stop"
		wantContain string
	}{
		{
			name:        "no container managed",
			setup:       "none",
			wantContain: "No container managed",
		},
		{
			name:        "container running",
			setup:       "start",
			wantContain: "unning", // matches "Running" or "running"
		},
		{
			name:        "after stop",
			setup:       "start_then_stop",
			wantContain: "No container managed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewContainerManager()
			t.Cleanup(func() {
				ctx := context.Background()
				_, _ = cm.HandleStopPostgres(ctx, makeStopRequest())
			})

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			switch tt.setup {
			case "start":
				startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
				if err != nil {
					t.Fatalf("HandleStartPostgres() setup returned error: %v", err)
				}
				if startResult.IsError {
					t.Fatalf("HandleStartPostgres() setup IsError = true, text = %q", resultText(t, startResult))
				}
			case "start_then_stop":
				startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
				if err != nil {
					t.Fatalf("HandleStartPostgres() setup returned error: %v", err)
				}
				if startResult.IsError {
					t.Fatalf("HandleStartPostgres() setup IsError = true, text = %q", resultText(t, startResult))
				}
				stopResult, err := cm.HandleStopPostgres(ctx, makeStopRequest())
				if err != nil {
					t.Fatalf("HandleStopPostgres() setup returned error: %v", err)
				}
				if stopResult.IsError {
					t.Fatalf("HandleStopPostgres() setup IsError = true, text = %q", resultText(t, stopResult))
				}
			case "none":
				// No setup needed
			}

			result, err := cm.HandlePostgresStatus(ctx, makeStatusRequest())
			if err != nil {
				t.Fatalf("HandlePostgresStatus() returned error: %v", err)
			}

			assertTextContains(t, result, tt.wantContain)
		})
	}
}

// ===========================================================================
// ConnStr state transitions
// ===========================================================================

func Test_ConnStr_StateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 1. Initially empty
	if got := cm.ConnStr(); got != "" {
		t.Errorf("step 1: ConnStr() = %q, want empty (initial state)", got)
	}

	// 2. After start: non-empty
	startResult, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}
	if startResult.IsError {
		t.Fatalf("HandleStartPostgres() IsError = true, text = %q", resultText(t, startResult))
	}

	connAfterStart := cm.ConnStr()
	if connAfterStart == "" {
		t.Fatal("step 2: ConnStr() is empty after start, want non-empty")
	}

	// 3. After stop: empty again
	stopResult, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}
	if stopResult.IsError {
		t.Fatalf("HandleStopPostgres() IsError = true, text = %q", resultText(t, stopResult))
	}

	if got := cm.ConnStr(); got != "" {
		t.Errorf("step 3: ConnStr() = %q after stop, want empty", got)
	}

	// 4. After restart: non-empty again
	restart, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() restart returned error: %v", err)
	}
	if restart.IsError {
		t.Fatalf("HandleStartPostgres() restart IsError = true, text = %q", resultText(t, restart))
	}

	if got := cm.ConnStr(); got == "" {
		t.Error("step 4: ConnStr() is empty after restart, want non-empty")
	}
}

// ===========================================================================
// Result structure verification
// ===========================================================================

func Test_HandleStartPostgres_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := cm.HandleStartPostgres(ctx, makeStartRequest(nil))
	if err != nil {
		t.Fatalf("HandleStartPostgres() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleStartPostgres() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleStartPostgres() result has no Content")
	}

	// Verify content is TextContent type
	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("HandleStartPostgres() result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

func Test_HandleStopPostgres_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop without starting (error case)
	result, err := cm.HandleStopPostgres(ctx, makeStopRequest())
	if err != nil {
		t.Fatalf("HandleStopPostgres() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("HandleStopPostgres() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandleStopPostgres() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("HandleStopPostgres() result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}

func Test_HandlePostgresStatus_ResultHasContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-dependent test in short mode")
	}

	cm := newTestContainerManager(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := cm.HandlePostgresStatus(ctx, makeStatusRequest())
	if err != nil {
		t.Fatalf("HandlePostgresStatus() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("HandlePostgresStatus() returned nil result")
	}
	if len(result.Content) == 0 {
		t.Fatal("HandlePostgresStatus() result has no Content")
	}

	_, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Errorf("HandlePostgresStatus() result.Content[0] is %T, want mcp.TextContent", result.Content[0])
	}
}
