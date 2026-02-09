package mcpserver

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// toolSpec describes the expected shape of a tool definition for table-driven
// testing. requiredParams lists parameter names that MUST appear in the
// schema's "required" array. allParams lists every parameter name that MUST
// exist in the schema's "properties" map.
type toolSpec struct {
	name           string
	wantName       string
	buildFunc      func() mcp.Tool
	requiredParams []string
	allParams      []string
}

// assertToolSpec is a test helper that verifies a tool matches its spec.
func assertToolSpec(t *testing.T, tool mcp.Tool, spec toolSpec) {
	t.Helper()

	// 1. Name
	if tool.Name != spec.wantName {
		t.Errorf("tool Name = %q, want %q", tool.Name, spec.wantName)
	}

	// 2. Description must be non-empty
	if tool.Description == "" {
		t.Errorf("tool %q has empty Description", tool.Name)
	}

	// 3. InputSchema type should be "object"
	if tool.InputSchema.Type != "object" {
		t.Errorf("tool %q InputSchema.Type = %q, want %q", tool.Name, tool.InputSchema.Type, "object")
	}

	// 4. All expected params exist in Properties
	for _, param := range spec.allParams {
		if _, ok := tool.InputSchema.Properties[param]; !ok {
			t.Errorf("tool %q missing expected parameter %q in Properties", tool.Name, param)
		}
	}

	// 5. Required params are in the Required array
	requiredSet := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		requiredSet[r] = true
	}
	for _, param := range spec.requiredParams {
		if !requiredSet[param] {
			t.Errorf("tool %q: parameter %q should be required but is not in Required array %v",
				tool.Name, param, tool.InputSchema.Required)
		}
	}

	// 6. Params that are NOT in requiredParams should NOT be in Required
	optionalParams := make(map[string]bool)
	for _, p := range spec.allParams {
		optionalParams[p] = true
	}
	for _, r := range spec.requiredParams {
		delete(optionalParams, r)
	}
	for param := range optionalParams {
		if requiredSet[param] {
			t.Errorf("tool %q: parameter %q should be optional but appears in Required array %v",
				tool.Name, param, tool.InputSchema.Required)
		}
	}
}

// ---------------------------------------------------------------------------
// Tool definition tests: table-driven
// ---------------------------------------------------------------------------

func Test_ToolDefinitions_Cases(t *testing.T) {
	t.Parallel()

	tests := []toolSpec{
		{
			name:           "startPostgresTool",
			wantName:       "start_postgres",
			buildFunc:      startPostgresTool,
			requiredParams: nil,
			allParams:      []string{"password", "database", "image"},
		},
		{
			name:           "stopPostgresTool",
			wantName:       "stop_postgres",
			buildFunc:      stopPostgresTool,
			requiredParams: nil,
			allParams:      nil,
		},
		{
			name:           "postgresStatusTool",
			wantName:       "postgres_status",
			buildFunc:      postgresStatusTool,
			requiredParams: nil,
			allParams:      nil,
		},
		{
			name:           "executeQueryTool",
			wantName:       "execute_query",
			buildFunc:      executeQueryTool,
			requiredParams: []string{"query"},
			allParams:      []string{"query"},
		},
		{
			name:           "listTablesTool",
			wantName:       "list_tables",
			buildFunc:      listTablesTool,
			requiredParams: nil,
			allParams:      []string{"schema"},
		},
		{
			name:           "describeTableTool",
			wantName:       "describe_table",
			buildFunc:      describeTableTool,
			requiredParams: []string{"table"},
			allParams:      []string{"table", "schema"},
		},
		{
			name:           "insertRowsTool",
			wantName:       "insert_rows",
			buildFunc:      insertRowsTool,
			requiredParams: []string{"table", "columns", "values"},
			allParams:      []string{"table", "columns", "values"},
		},
		{
			name:           "updateRowsTool",
			wantName:       "update_rows",
			buildFunc:      updateRowsTool,
			requiredParams: []string{"table", "set", "where"},
			allParams:      []string{"table", "set", "where"},
		},
		{
			name:           "deleteRowsTool",
			wantName:       "delete_rows",
			buildFunc:      deleteRowsTool,
			requiredParams: []string{"table", "where"},
			allParams:      []string{"table", "where"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tool := tt.buildFunc()
			assertToolSpec(t, tool, tt)
		})
	}
}

// ---------------------------------------------------------------------------
// Individual tool definition smoke tests: verify each returns a valid Tool
// ---------------------------------------------------------------------------

func Test_startPostgresTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := startPostgresTool()
	if tool.Description == "" {
		t.Error("startPostgresTool() Description is empty")
	}
}

func Test_stopPostgresTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := stopPostgresTool()
	if tool.Description == "" {
		t.Error("stopPostgresTool() Description is empty")
	}
}

func Test_postgresStatusTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := postgresStatusTool()
	if tool.Description == "" {
		t.Error("postgresStatusTool() Description is empty")
	}
}

func Test_executeQueryTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := executeQueryTool()
	if tool.Description == "" {
		t.Error("executeQueryTool() Description is empty")
	}
}

func Test_listTablesTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := listTablesTool()
	if tool.Description == "" {
		t.Error("listTablesTool() Description is empty")
	}
}

func Test_describeTableTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := describeTableTool()
	if tool.Description == "" {
		t.Error("describeTableTool() Description is empty")
	}
}

func Test_insertRowsTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := insertRowsTool()
	if tool.Description == "" {
		t.Error("insertRowsTool() Description is empty")
	}
}

func Test_updateRowsTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := updateRowsTool()
	if tool.Description == "" {
		t.Error("updateRowsTool() Description is empty")
	}
}

func Test_deleteRowsTool_HasDescription(t *testing.T) {
	t.Parallel()
	tool := deleteRowsTool()
	if tool.Description == "" {
		t.Error("deleteRowsTool() Description is empty")
	}
}

// ---------------------------------------------------------------------------
// Tool schema type verification
// ---------------------------------------------------------------------------

func Test_AllTools_InputSchemaTypeIsObject(t *testing.T) {
	t.Parallel()

	builders := []struct {
		name      string
		buildFunc func() mcp.Tool
	}{
		{"startPostgresTool", startPostgresTool},
		{"stopPostgresTool", stopPostgresTool},
		{"postgresStatusTool", postgresStatusTool},
		{"executeQueryTool", executeQueryTool},
		{"listTablesTool", listTablesTool},
		{"describeTableTool", describeTableTool},
		{"insertRowsTool", insertRowsTool},
		{"updateRowsTool", updateRowsTool},
		{"deleteRowsTool", deleteRowsTool},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			t.Parallel()
			tool := b.buildFunc()
			if tool.InputSchema.Type != "object" {
				t.Errorf("%s InputSchema.Type = %q, want %q", b.name, tool.InputSchema.Type, "object")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tools with no required params should have nil or empty Required
// ---------------------------------------------------------------------------

func Test_ToolsWithNoRequiredParams_Cases(t *testing.T) {
	t.Parallel()

	noRequiredTools := []struct {
		name      string
		buildFunc func() mcp.Tool
	}{
		{"startPostgresTool", startPostgresTool},
		{"stopPostgresTool", stopPostgresTool},
		{"postgresStatusTool", postgresStatusTool},
		{"listTablesTool", listTablesTool},
	}

	for _, tt := range noRequiredTools {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool := tt.buildFunc()
			if len(tool.InputSchema.Required) > 0 {
				t.Errorf("%s has Required = %v, want empty or nil", tt.name, tool.InputSchema.Required)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tools with required params: verify exact required sets
// ---------------------------------------------------------------------------

func Test_executeQueryTool_RequiredParams(t *testing.T) {
	t.Parallel()
	tool := executeQueryTool()

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	if !required["query"] {
		t.Errorf("executeQueryTool() Required = %v, want 'query' to be required", tool.InputSchema.Required)
	}
}

func Test_describeTableTool_RequiredParams(t *testing.T) {
	t.Parallel()
	tool := describeTableTool()

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	if !required["table"] {
		t.Errorf("describeTableTool() Required = %v, want 'table' to be required", tool.InputSchema.Required)
	}

	// "schema" should NOT be required
	if required["schema"] {
		t.Errorf("describeTableTool() 'schema' should be optional, but found in Required = %v", tool.InputSchema.Required)
	}
}

func Test_insertRowsTool_RequiredParams(t *testing.T) {
	t.Parallel()
	tool := insertRowsTool()

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	for _, param := range []string{"table", "columns", "values"} {
		if !required[param] {
			t.Errorf("insertRowsTool() Required = %v, want %q to be required", tool.InputSchema.Required, param)
		}
	}
}

func Test_updateRowsTool_RequiredParams(t *testing.T) {
	t.Parallel()
	tool := updateRowsTool()

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	for _, param := range []string{"table", "set", "where"} {
		if !required[param] {
			t.Errorf("updateRowsTool() Required = %v, want %q to be required", tool.InputSchema.Required, param)
		}
	}
}

func Test_deleteRowsTool_RequiredParams(t *testing.T) {
	t.Parallel()
	tool := deleteRowsTool()

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	for _, param := range []string{"table", "where"} {
		if !required[param] {
			t.Errorf("deleteRowsTool() Required = %v, want %q to be required", tool.InputSchema.Required, param)
		}
	}
}

// ---------------------------------------------------------------------------
// Tool names: uniqueness across all 9 tools
// ---------------------------------------------------------------------------

func Test_AllToolNames_AreUnique(t *testing.T) {
	t.Parallel()

	builders := []func() mcp.Tool{
		startPostgresTool,
		stopPostgresTool,
		postgresStatusTool,
		executeQueryTool,
		listTablesTool,
		describeTableTool,
		insertRowsTool,
		updateRowsTool,
		deleteRowsTool,
	}

	seen := make(map[string]bool, len(builders))
	for _, build := range builders {
		tool := build()
		if seen[tool.Name] {
			t.Errorf("duplicate tool name: %q", tool.Name)
		}
		seen[tool.Name] = true
	}
}

// ---------------------------------------------------------------------------
// Tool count: exactly 9 tools
// ---------------------------------------------------------------------------

func Test_ToolCount_IsNine(t *testing.T) {
	t.Parallel()

	builders := []func() mcp.Tool{
		startPostgresTool,
		stopPostgresTool,
		postgresStatusTool,
		executeQueryTool,
		listTablesTool,
		describeTableTool,
		insertRowsTool,
		updateRowsTool,
		deleteRowsTool,
	}

	if len(builders) != 9 {
		t.Errorf("expected 9 tool builders, got %d", len(builders))
	}
}

// ---------------------------------------------------------------------------
// startPostgresTool: optional params have string type
// ---------------------------------------------------------------------------

func Test_startPostgresTool_OptionalParamTypes(t *testing.T) {
	t.Parallel()
	tool := startPostgresTool()

	for _, param := range []string{"password", "database", "image"} {
		prop, ok := tool.InputSchema.Properties[param]
		if !ok {
			t.Errorf("startPostgresTool() missing property %q", param)
			continue
		}

		propMap, ok := prop.(map[string]any)
		if !ok {
			t.Errorf("startPostgresTool() property %q is not map[string]any, got %T", param, prop)
			continue
		}

		propType, ok := propMap["type"]
		if !ok {
			t.Errorf("startPostgresTool() property %q has no 'type' field", param)
			continue
		}

		if propType != "string" {
			t.Errorf("startPostgresTool() property %q type = %v, want %q", param, propType, "string")
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func Benchmark_ToolDefinitions(b *testing.B) {
	builders := []func() mcp.Tool{
		startPostgresTool,
		stopPostgresTool,
		postgresStatusTool,
		executeQueryTool,
		listTablesTool,
		describeTableTool,
		insertRowsTool,
		updateRowsTool,
		deleteRowsTool,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, build := range builders {
			_ = build()
		}
	}
}
