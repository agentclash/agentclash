package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const arenaTaskLimit = 20

var arenaWinnerValues = []string{"left", "right"}

func init() {
	rootCmd.AddCommand(arenaCmd)
	arenaCmd.AddCommand(arenaTasksCmd)
	arenaCmd.AddCommand(arenaVoteCmd)

	arenaVoteCmd.Flags().String("winner", "", "Side of the winner (left or right)")
	arenaVoteCmd.Flags().String("winner-run-agent", "", "Winner run agent UUID")
	arenaVoteCmd.Flags().StringArray("rubric-score", nil, "Rubric score as key=value (repeat for each key)")
}

var arenaCmd = &cobra.Command{
	Use:     "arena",
	Short:   "Review arena tasks and submit votes",
	Example: `  agentclash arena tasks`,
}

var arenaTasksCmd = &cobra.Command{
	Use:     "tasks",
	Short:   "List pending arena tasks",
	Long:    `The server returns up to 20 pending tasks, oldest first.`,
	Example: `  agentclash arena tasks`,
	Args:    cobra.NoArgs,
	RunE:    runArenaTasks,
}

var arenaVoteCmd = &cobra.Command{
	Use:   "vote <task-id>",
	Short: "Submit a vote for an arena task",
	Long: `Use --winner to select the left or right run agent from the task list.
If the task is absent from the list, use --winner-run-agent with the winner's UUID.
Use exactly one of these flags.`,
	Example: `  agentclash arena vote <task-id> --winner left --rubric-score recovery=4.5
  agentclash arena vote <task-id> --winner-run-agent <run-agent-id>`,
	Args: cobra.ExactArgs(1),
	RunE: runArenaVote,
}

type arenaVoteRequest struct {
	TaskID           uuid.UUID          `json:"task_id"`
	WinnerRunAgentID string             `json:"winner_run_agent_id"`
	RubricScores     map[string]float64 `json:"rubric_scores,omitempty"`
}

func getArenaTasks(cmd *cobra.Command, rc *RunContext, workspaceID string) (map[string]any, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/arena/tasks", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}
	var body map[string]any
	if err := resp.DecodeJSON(&body); err != nil {
		return nil, fmt.Errorf("cannot decode arena tasks response: %w", err)
	}
	return body, nil
}

func runArenaTasks(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	body, err := getArenaTasks(cmd, rc, workspaceID)
	if err != nil {
		return err
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}

	items := mapSlice(body, "items")
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		task, _ := item.(map[string]any)
		// SanitizeLine keeps a newline in a server value from making a false row.
		rows = append(rows, []string{
			output.SanitizeLine(mapString(task, "id")),
			output.SanitizeLine(mapString(task, "case_key")),
			output.SanitizeLine(mapString(task, "left_run_agent_id")),
			output.SanitizeLine(mapString(task, "right_run_agent_id")),
			output.SanitizeLine(mapString(task, "status")),
		})
	}
	rc.Output.PrintTable(arenaTaskColumns, rows)
	if len(items) == arenaTaskLimit {
		rc.Output.PrintWarning(fmt.Sprintf("The server returns at most %d tasks. More pending tasks can exist.", arenaTaskLimit))
	}
	return nil
}

func runArenaVote(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	if err := requireUUID("task ID", args[0]); err != nil {
		return err
	}
	taskID, _ := uuid.Parse(args[0])

	hasSide := cmd.Flags().Changed("winner")
	hasWinnerID := cmd.Flags().Changed("winner-run-agent")
	if hasSide == hasWinnerID {
		return &cliError{
			Code:    "invalid_argument",
			Message: "Use exactly one of --winner and --winner-run-agent.",
			Details: map[string]any{"flags": []string{"--winner", "--winner-run-agent"}},
		}
	}
	side, _ := cmd.Flags().GetString("winner")
	winnerID, _ := cmd.Flags().GetString("winner-run-agent")
	if hasSide {
		if err := requireOneOf("winner", side, arenaWinnerValues); err != nil {
			return err
		}
	} else if err := requireUUID("--winner-run-agent", winnerID); err != nil {
		return err
	}
	scores, _ := cmd.Flags().GetStringArray("rubric-score")
	rubricScores, err := parseArenaRubricScores(scores)
	if err != nil {
		return err
	}
	if hasSide {
		winnerID, err = resolveArenaWinner(cmd, rc, workspaceID, taskID.String(), side)
		if err != nil {
			return err
		}
	}

	resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+workspaceID+"/arena/votes", arenaVoteRequest{
		TaskID:           taskID,
		WinnerRunAgentID: winnerID,
		RubricScores:     rubricScores,
	})
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	if rc.Output.IsStructured() {
		var body map[string]any
		if err := resp.DecodeJSON(&body); err != nil {
			return fmt.Errorf("cannot decode arena vote response: %w", err)
		}
		return rc.Output.PrintRaw(body)
	}

	rc.Output.PrintSuccess("Arena vote recorded")
	return nil
}

func parseArenaRubricScores(scores []string) (map[string]float64, error) {
	result := make(map[string]float64, len(scores))
	for _, score := range scores {
		// The value is a number, so the last "=" separates the key and the
		// value. A key can contain "=", for example "status=refunded=4".
		separator := strings.LastIndex(score, "=")
		found := separator >= 0
		key, value := score, ""
		if found {
			key, value = score[:separator], score[separator+1:]
		}
		key = strings.TrimSpace(key)
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if !found || key == "" || err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("--rubric-score requires a key and a finite number in key=value form, got %q", score),
				Details: map[string]any{"flag": "--rubric-score", "value": score},
			}
		}
		// The server keeps one value for each key. A second value for the same
		// key is usually a mistake.
		if _, seen := result[key]; seen {
			return nil, &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("--rubric-score sets the key %q more than one time", key),
				Details: map[string]any{"flag": "--rubric-score", "key": key},
			}
		}
		result[key] = number
	}
	return result, nil
}

func resolveArenaWinner(cmd *cobra.Command, rc *RunContext, workspaceID, taskID, side string) (string, error) {
	body, err := getArenaTasks(cmd, rc, workspaceID)
	if err != nil {
		return "", err
	}
	for _, item := range mapSlice(body, "items") {
		task, _ := item.(map[string]any)
		if task["id"] != taskID {
			continue
		}
		winnerID, _ := task[side+"_run_agent_id"].(string)
		if _, err := uuid.Parse(winnerID); err != nil {
			return "", fmt.Errorf("arena task %s has no valid UUID for its %s run agent", taskID, side)
		}
		return winnerID, nil
	}

	nextStep := "Use --winner-run-agent with the winner's run agent UUID."
	return "", &cliError{
		Code:     "invalid_argument",
		Message:  fmt.Sprintf("Arena task %s is absent from the pending task list. %s", taskID, nextStep),
		Details:  map[string]any{"task_id": taskID},
		NextStep: nextStep,
	}
}

var arenaTaskColumns = []output.Column{
	{Header: "Task ID"},
	{Header: "Case Key"},
	{Header: "Left Run Agent ID"},
	{Header: "Right Run Agent ID"},
	{Header: "Status"},
}
