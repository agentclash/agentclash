package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- search_text (structured grep) ---

type SearchText struct{}

func (t *SearchText) Name() string        { return "search_text" }
func (t *SearchText) Category() Category  { return CatSearch }
func (t *SearchText) Description() string {
	return "Search for a text pattern across files in the workspace. Like grep, but structured."
}
func (t *SearchText) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Text pattern to search for (case-insensitive substring match)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in, relative to workspace root (use '.' for all)",
			},
			"glob": map[string]any{
				"type":        "string",
				"description": "File glob pattern to filter files (e.g. '*.go', '*.py'). Optional.",
			},
		},
		"required": []string{"pattern", "path"},
	}
}

func (t *SearchText) Execute(workDir string, args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	searchDir, err := safePath(workDir, args["path"])
	if err != nil {
		return "", err
	}

	glob, _ := args["glob"].(string)
	patternLower := strings.ToLower(pattern)

	var matches []string
	maxMatches := 100

	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if len(matches) >= maxMatches {
			return filepath.SkipAll
		}

		// Apply glob filter
		if glob != "" {
			matched, _ := filepath.Match(glob, info.Name())
			if !matched {
				return nil
			}
		}

		// Skip binary / large files
		if info.Size() > 1<<20 { // 1MB
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		relPath, _ := filepath.Rel(workDir, path)
		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", relPath, lineNum, line))
				if len(matches) >= maxMatches {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search_text: %w", err)
	}

	if len(matches) == 0 {
		return "no matches found", nil
	}

	result := strings.Join(matches, "\n")
	if len(matches) == maxMatches {
		result += fmt.Sprintf("\n... (limited to %d matches)", maxMatches)
	}
	return result, nil
}

// --- search_files (structured find) ---

type SearchFiles struct{}

func (t *SearchFiles) Name() string        { return "search_files" }
func (t *SearchFiles) Category() Category  { return CatSearch }
func (t *SearchFiles) Description() string {
	return "Find files by name pattern in the workspace. Like find, but structured."
}
func (t *SearchFiles) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "File name glob pattern (e.g. '*.go', 'test_*', 'main.*')",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in, relative to workspace root (use '.' for all)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *SearchFiles) Execute(workDir string, args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	searchPath := "."
	if p, ok := args["path"].(string); ok && p != "" {
		searchPath = p
	}

	searchDir, err := safePath(workDir, searchPath)
	if err != nil {
		return "", err
	}

	var found []string
	maxResults := 200

	err = filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if len(found) >= maxResults {
			return filepath.SkipAll
		}

		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			relPath, _ := filepath.Rel(workDir, path)
			suffix := ""
			if info.IsDir() {
				suffix = "/"
			}
			found = append(found, relPath+suffix)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("search_files: %w", err)
	}

	if len(found) == 0 {
		return "no files found", nil
	}

	result := strings.Join(found, "\n")
	if len(found) == maxResults {
		result += fmt.Sprintf("\n... (limited to %d results)", maxResults)
	}
	return result, nil
}
