package tools

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/internal/provider"
)

// Category groups tools for challenge configuration and scoring.
type Category string

const (
	CatFile   Category = "file"
	CatSearch Category = "search"
	CatBuild  Category = "build"
	CatEscape Category = "escape"
	CatMeta   Category = "meta"
)

// Result captures what happened when a tool was executed.
type Result struct {
	Tool     string        `json:"tool"`
	Category Category      `json:"category"`
	Input    string        `json:"input"`
	Output   string        `json:"output"`
	Error    string        `json:"error,omitempty"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration_ms"`
}

// Tool is the interface every tool implements.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Category() Category
	Execute(workDir string, args map[string]any) (string, error)
}

// Registry holds all available tools and executes them.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Defs returns tool definitions formatted for LLM tool calling.
func (r *Registry) Defs() []provider.ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]provider.ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, provider.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// Execute runs a tool by name with raw JSON arguments.
func (r *Registry) Execute(workDir, name, argsJSON string) Result {
	start := time.Now()

	tool, ok := r.Get(name)
	if !ok {
		return Result{
			Tool:     name,
			Category: CatEscape,
			Input:    argsJSON,
			Error:    fmt.Sprintf("unknown tool: %s", name),
			Duration: time.Since(start),
		}
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return Result{
			Tool:     name,
			Category: tool.Category(),
			Input:    argsJSON,
			Error:    fmt.Sprintf("invalid arguments: %v", err),
			Duration: time.Since(start),
		}
	}

	output, err := tool.Execute(workDir, args)
	dur := time.Since(start)

	res := Result{
		Tool:     name,
		Category: tool.Category(),
		Input:    argsJSON,
		Output:   output,
		Success:  err == nil,
		Duration: dur,
	}
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

// DefaultRegistry creates a registry with all standard tools registered.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// File tools
	r.Register(&ReadFile{})
	r.Register(&WriteFile{})
	r.Register(&ListFiles{})

	// Search tools
	r.Register(&SearchText{})
	r.Register(&SearchFiles{})

	// Build tools
	r.Register(&Build{})
	r.Register(&RunTests{})

	// Escape hatch
	r.Register(&BashTool{})

	// Meta
	r.Register(&SubmitSolution{})

	return r
}
