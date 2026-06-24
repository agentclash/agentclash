package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	evaltestCmd.AddCommand(evaltestPromoteFailuresCmd)
	evaltestPromoteFailuresCmd.Flags().String("from", "agentclash-results/results.json", "Local eval JSON report")
	evaltestPromoteFailuresCmd.Flags().String("to", ".agentclash/challenge-packs/local-regressions.yaml", "Draft challenge-pack output path")
	evaltestPromoteFailuresCmd.Flags().Bool("dry-run", false, "Print draft YAML without writing")
	evaltestPromoteFailuresCmd.Flags().Bool("append", false, "Append cases to an existing draft pack")
}

var evaltestPromoteFailuresCmd = &cobra.Command{
	Use:   "promote-failures",
	Short: "Promote failed local eval cases into a draft challenge pack",
	RunE: func(cmd *cobra.Command, args []string) error {
		fromPath, _ := cmd.Flags().GetString("from")
		toPath, _ := cmd.Flags().GetString("to")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		appendMode, _ := cmd.Flags().GetBool("append")

		report, err := readEvaltestReport(fromPath)
		if err != nil {
			evaltestExit(evaltestExitConfigError)
			return err
		}

		failures := evaltestFailureRows(report)
		if len(failures) == 0 {
			fmt.Fprintln(os.Stdout, "No failed eval cases to promote.")
			return nil
		}

		draft, err := buildPromotedChallengePack(report, failures, toPath, appendMode)
		if err != nil {
			evaltestExit(evaltestExitInternalError)
			return err
		}
		encoded, err := yaml.Marshal(draft)
		if err != nil {
			evaltestExit(evaltestExitInternalError)
			return fmt.Errorf("marshal draft pack: %w", err)
		}

		if dryRun {
			fmt.Printf("%s", encoded)
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
			evaltestExit(evaltestExitInternalError)
			return err
		}
		if err := os.WriteFile(toPath, encoded, 0o644); err != nil {
			evaltestExit(evaltestExitInternalError)
			return err
		}
		fmt.Fprintf(os.Stdout, "Wrote draft challenge pack with %d failure(s) to %s\n", len(failures), toPath)
		return nil
	},
}

func readEvaltestReport(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read report %s: %w", path, err)
	}
	var report map[string]any
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("parse report %s: %w", path, err)
	}
	return report, nil
}

func evaltestFailureRows(report map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, item := range evaltestCases(report) {
		row, _ := item.(map[string]any)
		if row == nil {
			continue
		}
		if mapString(row, "status") == "passed" {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func buildPromotedChallengePack(report map[string]any, failures []map[string]any, targetPath string, appendMode bool) (map[string]any, error) {
	slug := "local-regressions"
	if appendMode {
		if existing, err := readExistingPromotedPack(targetPath); err == nil {
			return mergePromotedPack(existing, failures), nil
		}
	}

	cases := make([]map[string]any, 0, len(failures))
	challenges := make([]map[string]any, 0, len(failures))
	for index, failure := range failures {
		caseObj, _ := failure["case"].(map[string]any)
		caseID := mapString(caseObj, "case_id")
		if caseID == "" {
			caseID = fmt.Sprintf("failure-%d", index+1)
		}
		input := mapString(caseObj, "input")
		if input == "" {
			input = mapString(failure, "input")
		}
		reason := evaltestFailureReason(failure)
		challengeKey := slugifyChallengePackName(caseID)
		challenges = append(challenges, map[string]any{
			"key":          challengeKey,
			"title":        mapString(caseObj, "name"),
			"category":     "regression",
			"difficulty":   "medium",
			"instructions": reason + "\n",
			"tags":         []string{"local-eval", "promoted"},
		})
		cases = append(cases, map[string]any{
			"challenge_key": challengeKey,
			"case_key":      caseID,
			"inputs": []map[string]any{
				{"key": "prompt", "kind": "text", "value": input},
			},
			"expectations": []map[string]any{
				{"key": "metric_evidence", "kind": "text", "source": "literal:" + reason},
			},
			"metadata": map[string]any{
				"promoted_from": "evaltest",
				"metric_reason": reason,
			},
		})
	}

	return map[string]any{
		"pack": map[string]any{
			"slug":   slug,
			"name":   "Local Eval Regressions",
			"family": "regression",
		},
		"version": map[string]any{
			"number":         1,
			"execution_mode": "native",
			"evaluation_spec": map[string]any{
				"name":           slug + "-v1",
				"version_number": 1,
				"judge_mode":     "deterministic",
				"validators": []map[string]any{
					{
						"key":           "contains_expected",
						"type":          "contains",
						"target":        "final_output",
						"expected_from": "challenge_input",
					},
				},
				"scorecard": map[string]any{
					"dimensions": []string{"correctness"},
				},
			},
		},
		"challenges": challenges,
		"input_sets": []map[string]any{
			{
				"key":   "promoted",
				"name":  "Promoted Local Failures",
				"cases": cases,
			},
		},
		"metadata": map[string]any{
			"promoted_at": mapString(report, "generated_at"),
			"source_report_id": mapString(report, "report_id"),
		},
	}, nil
}

func readExistingPromotedPack(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var draft map[string]any
	if err := yaml.Unmarshal(data, &draft); err != nil {
		return nil, err
	}
	return draft, nil
}

func mergePromotedPack(existing map[string]any, failures []map[string]any) map[string]any {
	newDraft, _ := buildPromotedChallengePack(map[string]any{}, failures, "", false)
	newCases := extractInputSetCases(newDraft)
	newChallenges := extractChallenges(newDraft)

	inputSets, _ := existing["input_sets"].([]any)
	if len(inputSets) == 0 {
		existing["input_sets"] = newDraft["input_sets"]
	} else {
		first, _ := inputSets[0].(map[string]any)
		if first != nil {
			existingCases, _ := first["cases"].([]any)
			first["cases"] = append(existingCases, anySlice(newCases)...)
			inputSets[0] = first
			existing["input_sets"] = inputSets
		}
	}

	challenges, _ := existing["challenges"].([]any)
	existing["challenges"] = append(challenges, anySlice(newChallenges)...)
	return existing
}

func extractInputSetCases(draft map[string]any) []map[string]any {
	inputSets, _ := draft["input_sets"].([]any)
	if len(inputSets) == 0 {
		return nil
	}
	first, _ := inputSets[0].(map[string]any)
	cases, _ := first["cases"].([]any)
	out := make([]map[string]any, 0, len(cases))
	for _, item := range cases {
		if row, ok := item.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}

func extractChallenges(draft map[string]any) []map[string]any {
	challenges, _ := draft["challenges"].([]any)
	out := make([]map[string]any, 0, len(challenges))
	for _, item := range challenges {
		if row, ok := item.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}

func anySlice(rows []map[string]any) []any {
	out := make([]any, len(rows))
	for i, row := range rows {
		out[i] = row
	}
	return out
}
