package mcpserver

import (
	"testing"
)

// ---------------------------------------------------------------------------
// NewServer: basic construction
// ---------------------------------------------------------------------------

func Test_NewServer_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer() returned nil server without error")
	}
}

func Test_NewServer_ReturnsNilError(t *testing.T) {
	t.Parallel()

	_, err := NewServer()
	if err != nil {
		t.Errorf("NewServer() error = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// NewServer: does not require Docker
// ---------------------------------------------------------------------------

func Test_NewServer_DoesNotRequireDocker(t *testing.T) {
	t.Parallel()

	// NewServer should succeed without Docker being available.
	// Container management is deferred to tool handler invocation,
	// so server creation itself must not probe for Docker.
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer() should not require Docker, got error: %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer() returned nil server")
	}
}

// ---------------------------------------------------------------------------
// NewServer: independent instances
// ---------------------------------------------------------------------------

func Test_NewServer_MultipleCallsCreateIndependentInstances(t *testing.T) {
	t.Parallel()

	srv1, err1 := NewServer()
	if err1 != nil {
		t.Fatalf("NewServer() first call error: %v", err1)
	}
	if srv1 == nil {
		t.Fatal("NewServer() first call returned nil")
	}

	srv2, err2 := NewServer()
	if err2 != nil {
		t.Fatalf("NewServer() second call error: %v", err2)
	}
	if srv2 == nil {
		t.Fatal("NewServer() second call returned nil")
	}

	if srv1 == srv2 {
		t.Error("NewServer() returned the same pointer for two calls, expected independent instances")
	}
}

func Test_NewServer_ThreeInstancesAllDistinct(t *testing.T) {
	t.Parallel()

	servers := make([]interface{}, 3)
	for i := range servers {
		srv, err := NewServer()
		if err != nil {
			t.Fatalf("NewServer() call %d error: %v", i, err)
		}
		if srv == nil {
			t.Fatalf("NewServer() call %d returned nil", i)
		}
		servers[i] = srv
	}

	// All three must be distinct pointers
	if servers[0] == servers[1] {
		t.Error("servers[0] and servers[1] are the same pointer")
	}
	if servers[0] == servers[2] {
		t.Error("servers[0] and servers[2] are the same pointer")
	}
	if servers[1] == servers[2] {
		t.Error("servers[1] and servers[2] are the same pointer")
	}
}
