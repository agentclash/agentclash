package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(quotaCmd)
}

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "Show workspace quota usage",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		workspaceID := RequireWorkspace(cmd)

		resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+workspaceID+"/quota", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var quota map[string]any
		if err := resp.DecodeJSON(&quota); err != nil {
			return err
		}

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(quota)
		}

		rc.Output.PrintDetail("Plan", mapString(quota, "plan_key"))
		if status := mapString(quota, "status"); status != "" {
			rc.Output.PrintDetail("Status", output.StatusColor(status))
		}
		rc.Output.PrintDetail("Monthly evals", quotaCounterLine(mapObject(quota, "monthly_races")))
		rc.Output.PrintDetail("Concurrent evals", quotaCounterLine(mapObject(quota, "concurrent_races")))
		if resetAt := mapString(mapObject(quota, "monthly_races"), "reset_at"); resetAt != "" {
			rc.Output.PrintDetail("Quota resets", resetAt)
		}
		return nil
	},
}

func quotaCounterLine(counter map[string]any) string {
	used := mapInt(counter, "used")
	limit, hasLimit := mapOptionalInt(counter, "limit")
	remaining, hasRemaining := mapOptionalInt(counter, "remaining")
	if !hasLimit {
		return fmt.Sprintf("%d used / unlimited", used)
	}
	if hasRemaining {
		return fmt.Sprintf("%d / %d used (%d remaining)", used, limit, remaining)
	}
	return fmt.Sprintf("%d / %d used", used, limit)
}

func mapInt(m map[string]any, key string) int {
	value, ok := mapOptionalInt(m, key)
	if !ok {
		return 0
	}
	return value
}

func mapOptionalInt(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	switch value := m[key].(type) {
	case int:
		return value, true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case json.Number:
		i, err := value.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
