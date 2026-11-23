package cmd

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

const calibrationReviewLimit = 100

// decimalIntValue is an int flag that reads base 10 only. The pflag Int
// flag reads base 0, so "010" becomes 8 and a review goes to the wrong turn.
type decimalIntValue int

func (v *decimalIntValue) Set(s string) error {
	n, err := strconv.ParseInt(s, 10, strconv.IntSize)
	if err != nil {
		return errors.New("must be a base-10 integer")
	}
	*v = decimalIntValue(n)
	return nil
}

func (v *decimalIntValue) String() string { return strconv.Itoa(int(*v)) }

// Type returns "int", so GetInt and the schema treat the flag as an int.
func (v *decimalIntValue) Type() string { return "int" }

func init() {
	rootCmd.AddCommand(calibrationCmd)
	calibrationCmd.AddCommand(calibrationCreateCmd)
	calibrationCmd.AddCommand(calibrationListCmd)

	calibrationCreateCmd.Flags().String("run-agent", "", "Run agent UUID for the review")
	calibrationCreateCmd.Flags().Var(new(decimalIntValue), "turn", "Turn index (0 or more)")
	calibrationCreateCmd.Flags().Float64("score", 0, "Calibration score from 1 to 5")
	calibrationCreateCmd.Flags().String("rubric-key", "", "Rubric key for the review")
	calibrationCreateCmd.Flags().String("notes", "", "Notes for the review")
	calibrationCreateCmd.MarkFlagRequired("run-agent")
	calibrationCreateCmd.MarkFlagRequired("turn")
	calibrationCreateCmd.MarkFlagRequired("score")
}

var calibrationCmd = &cobra.Command{
	Use:     "calibration",
	Short:   "Manage calibration reviews",
	Example: `  agentclash calibration list`,
}

var calibrationCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Record a calibration review for a run agent turn",
	Long: `Record a score for a turn of a run agent.
Use a turn index of 0 or more and a score from 1 to 5.
The server keeps two decimal places of the score.`,
	Example: `  agentclash calibration create --run-agent <run-agent-id> --turn 0 --score 4.5
  agentclash calibration create --run-agent <run-agent-id> --turn 1 --score 5 --rubric-key recovery --notes "The agent resumed the task."`,
	Args: cobra.NoArgs,
	RunE: runCalibrationCreate,
}

var calibrationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent calibration reviews",
	Long: `The server returns up to 100 reviews, newest first.
To select reviews for one run agent, use --json and --query.
Use the expression '.items[] | select(.run_agent_id == "<run-agent-id>")'.`,
	Example: `  agentclash calibration list
  agentclash calibration list --json --query '.items[] | select(.run_agent_id == "<run-agent-id>")'`,
	Args: cobra.NoArgs,
	RunE: runCalibrationList,
}

type calibrationReviewRequest struct {
	RunAgentID string  `json:"run_agent_id"`
	TurnIndex  int     `json:"turn_index"`
	Score      float64 `json:"score"`
	RubricKey  string  `json:"rubric_key,omitempty"`
	Notes      string  `json:"notes,omitempty"`
}

func runCalibrationCreate(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	runAgentID, _ := cmd.Flags().GetString("run-agent")
	if err := requireUUID("--run-agent", runAgentID); err != nil {
		return err
	}
	turn, _ := cmd.Flags().GetInt("turn")
	// The database column is a 32-bit integer.
	if turn < 0 || turn > math.MaxInt32 {
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("--turn must be from 0 to %d, got %d", math.MaxInt32, turn),
			Details: map[string]any{"flag": "--turn", "value": turn},
		}
	}
	score, _ := cmd.Flags().GetFloat64("score")
	if math.IsNaN(score) || score < 1 || score > 5 {
		value := strconv.FormatFloat(score, 'g', -1, 64)
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("--score must be a finite number from 1 to 5, got %s", value),
			Details: map[string]any{"flag": "--score", "value": value},
		}
	}
	rubricKey, _ := cmd.Flags().GetString("rubric-key")
	notes, _ := cmd.Flags().GetString("notes")
	request := calibrationReviewRequest{
		RunAgentID: runAgentID,
		TurnIndex:  turn,
		Score:      score,
		RubricKey:  rubricKey,
		Notes:      notes,
	}

	resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+workspaceID+"/calibration-reviews", request)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	if rc.Output.IsStructured() {
		var body map[string]any
		if err := resp.DecodeJSON(&body); err != nil {
			return fmt.Errorf("cannot decode calibration review response: %w", err)
		}
		return rc.Output.PrintRaw(body)
	}

	rc.Output.PrintSuccess("Calibration review created")
	return nil
}

func runCalibrationList(cmd *cobra.Command, _ []string) error {
	rc := GetRunContext(cmd)
	workspaceID := RequireWorkspace(cmd)
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/calibration-reviews", nil)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var body map[string]any
	if err := resp.DecodeJSON(&body); err != nil {
		return fmt.Errorf("cannot decode calibration reviews response: %w", err)
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(body)
	}

	items := mapSlice(body, "items")
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		review, _ := item.(map[string]any)
		// SanitizeLine keeps a newline in a server value from making a false row.
		rows = append(rows, []string{
			output.SanitizeLine(mapString(review, "id")),
			output.SanitizeLine(mapString(review, "run_agent_id")),
			output.SanitizeLine(mapString(review, "turn_index")),
			output.SanitizeLine(mapString(review, "score")),
			output.SanitizeLine(mapString(review, "rubric_key")),
			output.SanitizeLine(mapString(review, "created_at")),
		})
	}
	rc.Output.PrintTable(calibrationReviewColumns, rows)
	if len(items) == calibrationReviewLimit {
		rc.Output.PrintWarning(fmt.Sprintf("The server returns at most %d reviews. Older reviews can exist.", calibrationReviewLimit))
	}
	return nil
}

var calibrationReviewColumns = []output.Column{
	{Header: "Review ID"},
	{Header: "Run Agent ID"},
	{Header: "Turn", Align: 1},
	{Header: "Score", Align: 1},
	{Header: "Rubric Key"},
	{Header: "Created At"},
}
