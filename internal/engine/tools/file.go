package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- read_file ---

type ReadFile struct{}

func (t *ReadFile) Name() string        { return "read_file" }
func (t *ReadFile) Category() Category  { return CatFile }
func (t *ReadFile) Description() string { return "Read the contents of a file in the workspace" }
func (t *ReadFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to workspace root",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFile) Execute(workDir string, args map[string]any) (string, error) {
	path, err := safePath(workDir, args["path"])
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	return string(data), nil
}

// --- write_file ---

type WriteFile struct{}

func (t *WriteFile) Name() string        { return "write_file" }
func (t *WriteFile) Category() Category  { return CatFile }
func (t *WriteFile) Description() string { return "Write content to a file (creates or overwrites)" }
func (t *WriteFile) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "File path relative to workspace root",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Full file content to write",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFile) Execute(workDir string, args map[string]any) (string, error) {
	path, err := safePath(workDir, args["path"])
	if err != nil {
		return "", err
	}
	content, _ := args["content"].(string)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("write_file: mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}
	return "file written successfully", nil
}

// --- list_files ---

type ListFiles struct{}

func (t *ListFiles) Name() string        { return "list_files" }
func (t *ListFiles) Category() Category  { return CatFile }
func (t *ListFiles) Description() string { return "List all files in a directory" }
func (t *ListFiles) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path relative to workspace root (use '.' for root)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ListFiles) Execute(workDir string, args map[string]any) (string, error) {
	path, err := safePath(workDir, args["path"])
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list_files: %w", err)
	}
	var lines []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	return strings.Join(lines, "\n"), nil
}

// safePath resolves a relative path within workDir and prevents escape.
func safePath(workDir string, raw any) (string, error) {
	relPath, _ := raw.(string)
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	abs := filepath.Join(workDir, filepath.Clean(relPath))
	if !strings.HasPrefix(abs, workDir) {
		return "", fmt.Errorf("path escapes workspace: %s", relPath)
	}
	return abs, nil
}
