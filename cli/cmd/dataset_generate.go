package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	datasetCmd.AddCommand(datasetGenerateCmd)
	datasetGenerateCmd.Flags().String("strategy", "self-instruct", "Generation strategy (self-instruct, agentic-self-instruct)")
	datasetGenerateCmd.Flags().Int("count", 0, "Target number of accepted synthetic examples")
	datasetGenerateCmd.Flags().String("provider-account", "", "Provider account ID")
	datasetGenerateCmd.Flags().String("model", "", "Provider model ID")
	datasetGenerateCmd.Flags().String("judge-provider-account", "", "Judge provider account ID for agentic generation")
	datasetGenerateCmd.Flags().String("judge-model", "", "Judge provider model ID for agentic generation")
	datasetGenerateCmd.Flags().Int("max-rounds-per-example", 0, "Maximum judge/improve rounds per generated example for agentic generation")
	datasetGenerateCmd.Flags().String("acceptance-mode", "", "Agentic acceptance mode (judge or threshold)")
	datasetGenerateCmd.Flags().Float64("min-gap", 0, "Minimum strong-minus-weak score gap for agentic threshold guardrails")
	datasetGenerateCmd.Flags().Float64("max-weak-score", 0, "Maximum weak score for agentic threshold guardrails")
	datasetGenerateCmd.Flags().Float64("min-strong-score", 0, "Minimum strong score for agentic threshold guardrails")
	datasetGenerateCmd.Flags().String("seeds-tag", "", "Only use seed examples with this tag")
	datasetGenerateCmd.Flags().Bool("create-version", false, "Snapshot a dataset version when generation completes")
	datasetGenerateCmd.Flags().String("version-label", "", "Optional label for the generated dataset version")
	datasetGenerateCmd.Flags().Bool("follow", false, "Poll generation job status until it finishes")
	datasetGenerateCmd.Flags().Duration("poll-interval", 2*time.Second, "Polling interval for --follow")
	datasetGenerateCmd.Flags().Duration("timeout", 0, "Give up on --follow after this duration (0 = wait indefinitely)")
}

var datasetGenerateCmd = &cobra.Command{
	Use:   "generate <datasetId>",
	Short: "Start in-house synthetic dataset generation",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)
		datasetID := args[0]

		count, err := cmd.Flags().GetInt("count")
		if err != nil {
			return err
		}
		providerAccount, err := cmd.Flags().GetString("provider-account")
		if err != nil {
			return err
		}
		model, err := cmd.Flags().GetString("model")
		if err != nil {
			return err
		}
		if count <= 0 || providerAccount == "" || model == "" {
			return fmt.Errorf("--count, --provider-account, and --model are required")
		}

		strategy, _ := cmd.Flags().GetString("strategy")
		seedsTag, _ := cmd.Flags().GetString("seeds-tag")
		createVersion, _ := cmd.Flags().GetBool("create-version")
		versionLabel, _ := cmd.Flags().GetString("version-label")
		follow, _ := cmd.Flags().GetBool("follow")
		judgeProviderAccount, _ := cmd.Flags().GetString("judge-provider-account")
		judgeModel, _ := cmd.Flags().GetString("judge-model")
		maxRoundsPerExample, _ := cmd.Flags().GetInt("max-rounds-per-example")
		acceptanceMode, _ := cmd.Flags().GetString("acceptance-mode")

		body := map[string]any{
			"strategy":            strategy,
			"target_count":        count,
			"provider_account_id": providerAccount,
			"model":               model,
			"create_version":      createVersion,
		}
		if seedsTag != "" {
			body["seeds_tag"] = seedsTag
		}
		if versionLabel != "" {
			body["version_label"] = versionLabel
		}
		if judgeProviderAccount != "" {
			body["judge_provider_account_id"] = judgeProviderAccount
		}
		if judgeModel != "" {
			body["judge_model"] = judgeModel
		}
		if maxRoundsPerExample > 0 {
			body["max_rounds_per_example"] = maxRoundsPerExample
		}
		if acceptanceMode != "" {
			body["acceptance_mode"] = acceptanceMode
		}
		if cmd.Flags().Changed("min-gap") {
			minGap, _ := cmd.Flags().GetFloat64("min-gap")
			body["min_gap"] = minGap
		}
		if cmd.Flags().Changed("max-weak-score") {
			maxWeakScore, _ := cmd.Flags().GetFloat64("max-weak-score")
			body["max_weak_score"] = maxWeakScore
		}
		if cmd.Flags().Changed("min-strong-score") {
			minStrongScore, _ := cmd.Flags().GetFloat64("min-strong-score")
			body["min_strong_score"] = minStrongScore
		}

		resp, err := rc.Client.Post(cmd.Context(), "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/generate", body)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}
		var job map[string]any
		if err := resp.DecodeJSON(&job); err != nil {
			return err
		}

		jobID, _ := job["id"].(string)
		if !follow || jobID == "" {
			if rc.Output.IsStructured() {
				return rc.Output.PrintRaw(job)
			}
			fmt.Fprintf(rc.Output.Writer(), "generation job %s queued\n", jobID)
			return nil
		}

		pollInterval, _ := cmd.Flags().GetDuration("poll-interval")
		if pollInterval <= 0 {
			pollInterval = 2 * time.Second
		}
		timeout, _ := cmd.Flags().GetDuration("timeout")
		ctx := cmd.Context()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		followTimeoutErr := func() error {
			return &cliError{
				Code:    "follow_timeout",
				Message: fmt.Sprintf("generation job %s did not reach a terminal status within %s; it keeps running server-side", jobID, timeout),
			}
		}

		for {
			statusResp, pollErr := rc.Client.Get(ctx, "/v1/workspaces/"+wsID+"/datasets/"+datasetID+"/generations/"+jobID, nil)
			if pollErr != nil {
				if timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return followTimeoutErr()
				}
				return pollErr
			}
			if apiErr := statusResp.ParseError(); apiErr != nil {
				return apiErr
			}
			var current map[string]any
			if err := statusResp.DecodeJSON(&current); err != nil {
				return err
			}
			status, _ := current["status"].(string)
			if !rc.Output.IsStructured() {
				fmt.Fprintf(rc.Output.Writer(), "status: %s accepted=%v rejected=%v\n", status, current["accepted_count"], current["rejected_count"])
			}
			// Shared terminal set — the old hand-rolled switch was missing
			// "cancelled" and looped forever on a cancelled job.
			if isTerminalRunStatus(status) {
				if rc.Output.IsStructured() {
					return rc.Output.PrintRaw(current)
				}
				return nil
			}

			timer := time.NewTimer(pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				if timeout > 0 && errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return followTimeoutErr()
				}
				return ctx.Err()
			case <-timer.C:
			}
		}
	},
}
