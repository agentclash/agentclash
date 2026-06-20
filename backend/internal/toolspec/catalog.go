// Package toolspec is the source of truth for the catalog of base primitive tools
// and the canonical schema for user-authored tool definitions (primitive and
// composed). It is dependency-free (stdlib only) so both the API server and the
// engine can reference it without import cycles.
//
// The primitive catalog here mirrors engine.nativePrimitiveTools. An engine test
// (TestPrimitiveCatalogMatchesNativePrimitives) asserts the two never drift.
package toolspec

import "encoding/json"

// PrimitiveKind groups primitives by the tool policy that gates them at run time.
type PrimitiveKind string

const (
	KindCore    PrimitiveKind = "core"
	KindFile    PrimitiveKind = "file"
	KindData    PrimitiveKind = "data"
	KindNetwork PrimitiveKind = "network"
	KindBuild   PrimitiveKind = "build"
	KindShell   PrimitiveKind = "shell"
)

// PrimitiveSpec describes a base primitive a custom tool can delegate to.
type PrimitiveSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Kind        PrimitiveKind   `json:"kind"`
	Parameters  json.RawMessage `json:"parameters"`
	// Delegatable reports whether a custom tool may delegate to this primitive.
	// "submit" is the agent's final-answer tool and is not a useful building block.
	Delegatable bool `json:"delegatable"`
}

// Primitive name constants, kept identical to the engine's executor_builders.go.
const (
	PrimitiveSubmit      = "submit"
	PrimitiveReadFile    = "read_file"
	PrimitiveWriteFile   = "write_file"
	PrimitiveListFiles   = "list_files"
	PrimitiveSearchFiles = "search_files"
	PrimitiveSearchText  = "search_text"
	PrimitiveQueryJSON   = "query_json"
	PrimitiveQuerySQL    = "query_sql"
	PrimitiveHTTPRequest = "http_request"
	PrimitiveRunTests    = "run_tests"
	PrimitiveBuild       = "build"
	PrimitiveExec        = "exec"
)

// Primitives returns the full catalog of base primitives in a stable order.
// The parameter schemas are byte-identical to the engine's definitions.
func Primitives() []PrimitiveSpec {
	return []PrimitiveSpec{
		{
			Name:        PrimitiveSubmit,
			Description: "Submit your final answer for the benchmark when you are finished.",
			Kind:        KindCore,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
			Delegatable: false,
		},
		{
			Name:        PrimitiveReadFile,
			Description: "Read a file from the sandbox workspace.",
			Kind:        KindFile,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveWriteFile,
			Description: "Write text content to a file in the sandbox workspace.",
			Kind:        KindFile,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveListFiles,
			Description: "List files in the sandbox workspace under an optional path prefix.",
			Kind:        KindFile,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"prefix":{"type":"string"}},"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveSearchFiles,
			Description: "Search for files in the sandbox workspace by name or glob pattern.",
			Kind:        KindFile,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveSearchText,
			Description: "Search file contents in the sandbox workspace using a regex pattern.",
			Kind:        KindFile,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"},"include":{"type":"string"},"case_sensitive":{"type":"boolean"},"max_results":{"type":"integer","minimum":1}},"required":["pattern"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveQueryJSON,
			Description: "Query JSON from a file or inline JSON string using jq.",
			Kind:        KindData,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"file_path":{"type":"string"},"json":{"type":"string"},"output_path":{"type":"string"}},"required":["query"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveQuerySQL,
			Description: "Run a SQL query against a supported database engine. Day-1 support is SQLite only.",
			Kind:        KindData,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"engine":{"type":"string"},"query":{"type":"string"},"database_path":{"type":"string"},"output_path":{"type":"string"}},"required":["engine","query"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveHTTPRequest,
			Description: "Make an HTTP request from inside the sandbox with structured response output.",
			Kind:        KindNetwork,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"method":{"type":"string"},"url":{"type":"string"},"headers":{"type":"object","additionalProperties":{"type":"string"}},"body":{"type":"string"},"timeout_seconds":{"type":"integer","minimum":1},"output_path":{"type":"string"}},"required":["method","url"],"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveRunTests,
			Description: "Run project tests in the sandbox workspace using an explicit or auto-detected command.",
			Kind:        KindBuild,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"},"minItems":1}]},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}},"timeout_seconds":{"type":"integer","minimum":1}},"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveBuild,
			Description: "Build the project in the sandbox workspace using an explicit or auto-detected command.",
			Kind:        KindBuild,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"},"minItems":1}]},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}},"timeout_seconds":{"type":"integer","minimum":1}},"additionalProperties":false}`),
			Delegatable: true,
		},
		{
			Name:        PrimitiveExec,
			Description: "Execute a shell command inside the sandbox workspace.",
			Kind:        KindShell,
			Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"array","items":{"type":"string"},"minItems":1},"working_directory":{"type":"string"},"environment":{"type":"object","additionalProperties":{"type":"string"}}},"required":["command"],"additionalProperties":false}`),
			Delegatable: true,
		},
	}
}

// PrimitiveByName returns the spec for a primitive name, or ok=false if unknown.
func PrimitiveByName(name string) (PrimitiveSpec, bool) {
	for _, p := range Primitives() {
		if p.Name == name {
			return p, true
		}
	}
	return PrimitiveSpec{}, false
}

// PrimitiveNames returns the set of all primitive names.
func PrimitiveNames() map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range Primitives() {
		out[p.Name] = struct{}{}
	}
	return out
}
