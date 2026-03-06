package tools

import (
	"fmt"
	"os/exec"
)

// BashTool is the escape hatch. Agents CAN use it, but structured tools
// are preferred and score better. This is tracked in telemetry.
type BashTool struct{}

func (t *BashTool) Name() string        { return "bash" }
func (t *BashTool) Category() Category  { return CatEscape }
func (t *BashTool) Description() string {
	return "Run an arbitrary shell command. Use this ONLY when no structured tool (read_file, search_text, build, run_tests) can accomplish the task. Using structured tools is faster and scores better."
}
func (t *BashTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

func (t *BashTool) Execute(workDir string, args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()

	result := string(output)

	// Cap output to prevent context blowup
	if len(result) > 10000 {
		result = result[:10000] + "\n... (output truncated)"
	}

	if err != nil {
		result += "\nexit error: " + err.Error()
	}
	return result, nil
}
