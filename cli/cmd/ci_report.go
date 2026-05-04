package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

const ciRunArtifactSchemaVersion = "2026-05-04"

type ciRunReportOutputs struct {
	SummaryFiles []string            `json:"summary_files,omitempty" yaml:"summary_files,omitempty"`
	ArtifactDir  string              `json:"artifact_dir,omitempty" yaml:"artifact_dir,omitempty"`
	Artifacts    []ciRunArtifactFile `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
}

type ciRunArtifactFile struct {
	Kind string `json:"kind" yaml:"kind"`
	Path string `json:"path" yaml:"path"`
}

type ciRunArtifactEnvelope struct {
	SchemaVersion         string                  `json:"schema_version"`
	Kind                  string                  `json:"kind"`
	GeneratedAt           string                  `json:"generated_at"`
	ManifestPath          string                  `json:"manifest_path"`
	WorkspaceID           string                  `json:"workspace_id"`
	ChallengePackVersion  string                  `json:"challenge_pack_version_id"`
	Candidate             ciRunCandidateResult    `json:"candidate"`
	Baseline              ciBaselineRunResolution `json:"baseline"`
	GateVerdict           string                  `json:"gate_verdict,omitempty"`
	GatePolicyKey         string                  `json:"gate_policy_key,omitempty"`
	GatePolicyVersion     string                  `json:"gate_policy_version,omitempty"`
	GatePolicyFingerprint string                  `json:"gate_policy_fingerprint,omitempty"`
	Payload               any                     `json:"payload"`
}

type ciRunSummaryTarget struct {
	Path   string
	Append bool
}

func writeCIRunReports(cmd *cobra.Command, result ciRunResult, manifest ciManifest, createdRun, completedRun, scorecard, comparison, gateEnvelope map[string]any) (*ciRunReportOutputs, error) {
	targets := ciRunSummaryTargets(cmd)
	artifactDir, _ := cmd.Flags().GetString("artifact-dir")
	artifactDir = strings.TrimSpace(artifactDir)
	if len(targets) == 0 && artifactDir == "" {
		return nil, nil
	}

	generatedAt := time.Now().UTC()
	releaseGate := mapObject(gateEnvelope, "release_gate")
	outputs := &ciRunReportOutputs{}

	if len(targets) > 0 {
		summary := renderCIRunMarkdownSummary(result, manifest, scorecard, comparison, releaseGate)
		for _, target := range targets {
			if err := writeCIRunSummaryFile(target, summary); err != nil {
				return outputs, err
			}
			outputs.SummaryFiles = append(outputs.SummaryFiles, target.Path)
		}
	}

	if artifactDir != "" {
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			return outputs, fmt.Errorf("create ci artifact directory: %w", err)
		}
		outputs.ArtifactDir = artifactDir
		files := []ciRunArtifactFile{
			{Kind: "agentclash.ci.result", Path: filepath.Join(artifactDir, "result.json")},
			{Kind: "agentclash.ci.run", Path: filepath.Join(artifactDir, "run.json")},
			{Kind: "agentclash.ci.scorecard", Path: filepath.Join(artifactDir, "scorecard.json")},
			{Kind: "agentclash.ci.comparison", Path: filepath.Join(artifactDir, "comparison.json")},
			{Kind: "agentclash.ci.gate", Path: filepath.Join(artifactDir, "gate.json")},
		}
		outputs.Artifacts = append(outputs.Artifacts, files...)
		resultWithReports := result
		resultWithReports.Reports = outputs
		payloads := []any{
			resultWithReports,
			map[string]any{"created_run": createdRun, "completed_run": completedRun},
			scorecard,
			comparison,
			gateEnvelope,
		}
		for i, file := range files {
			envelope := ciRunArtifactEnvelope{
				SchemaVersion:         ciRunArtifactSchemaVersion,
				Kind:                  file.Kind,
				GeneratedAt:           generatedAt.Format(time.RFC3339),
				ManifestPath:          result.ManifestPath,
				WorkspaceID:           result.WorkspaceID,
				ChallengePackVersion:  manifest.Evaluation.ChallengePackVersionID,
				Candidate:             result.Candidate,
				Baseline:              result.Baseline,
				GateVerdict:           result.GateVerdict,
				GatePolicyKey:         mapString(releaseGate, "policy_key"),
				GatePolicyVersion:     mapString(releaseGate, "policy_version"),
				GatePolicyFingerprint: mapString(releaseGate, "policy_fingerprint"),
				Payload:               payloads[i],
			}
			if err := writeCIRunJSONArtifact(file.Path, envelope); err != nil {
				return outputs, err
			}
		}
	}

	return outputs, nil
}

func ciRunSummaryTargets(cmd *cobra.Command) []ciRunSummaryTarget {
	var targets []ciRunSummaryTarget
	add := func(path string, appendMode bool) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		for i := range targets {
			if targets[i].Path == path {
				targets[i].Append = targets[i].Append || appendMode
				return
			}
		}
		targets = append(targets, ciRunSummaryTarget{Path: path, Append: appendMode})
	}

	if summaryFile, _ := cmd.Flags().GetString("summary-file"); summaryFile != "" {
		add(summaryFile, false)
	}
	if enabled, _ := cmd.Flags().GetBool("github-step-summary"); enabled {
		add(os.Getenv("GITHUB_STEP_SUMMARY"), true)
	}
	return targets
}

func ciRunReportsEnabled(cmd *cobra.Command) bool {
	if len(ciRunSummaryTargets(cmd)) > 0 {
		return true
	}
	artifactDir, _ := cmd.Flags().GetString("artifact-dir")
	return strings.TrimSpace(artifactDir) != ""
}

func writeCIRunSummaryFile(target ciRunSummaryTarget, summary string) error {
	if parent := filepath.Dir(target.Path); parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return fmt.Errorf("create ci summary directory: %w", err)
		}
	}
	flag := os.O_CREATE | os.O_WRONLY
	if target.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	file, err := os.OpenFile(target.Path, flag, 0o644)
	if err != nil {
		return fmt.Errorf("open ci summary file: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(summary); err != nil {
		return fmt.Errorf("write ci summary file: %w", err)
	}
	return nil
}

func writeCIRunJSONArtifact(path string, envelope ciRunArtifactEnvelope) error {
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ci artifact %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write ci artifact %s: %w", path, err)
	}
	return nil
}

func renderCIRunMarkdownSummary(result ciRunResult, manifest ciManifest, scorecard, comparison, releaseGate map[string]any) string {
	var b strings.Builder
	verdict := strings.TrimSpace(result.GateVerdict)
	if verdict == "" {
		verdict = "unknown"
	}
	fmt.Fprintf(&b, "## AgentClash CI Gate: %s\n\n", strings.ToUpper(output.SanitizeLine(verdict)))
	writeCIRunSummaryTable(&b, [][2]string{
		{"Verdict", verdict},
		{"Workspace", result.WorkspaceID},
		{"Manifest", result.ManifestPath},
		{"Challenge Pack Version", manifest.Evaluation.ChallengePackVersionID},
		{"Baseline Run", result.Baseline.RunID},
		{"Baseline Agent", result.Baseline.RunAgentID},
		{"Candidate Run", result.Candidate.RunID},
		{"Candidate Agent", result.Candidate.RunAgentID},
		{"Gate Policy", ciRunGatePolicyLabel(releaseGate)},
		{"Reason", mapString(releaseGate, "reason_code")},
		{"Evidence", mapString(releaseGate, "evidence_status")},
	})

	if links := ciRunSummaryLinks(result, scorecard, comparison, releaseGate); len(links) > 0 {
		b.WriteString("\n### Links\n\n")
		for _, link := range links {
			fmt.Fprintf(&b, "- %s\n", link)
		}
	}

	b.WriteString("\n### Gate Evidence\n\n")
	for _, line := range ciRunTopFailureLines(result.GateVerdict, scorecard, comparison, releaseGate) {
		fmt.Fprintf(&b, "- %s\n", ciMarkdownText(line))
	}
	b.WriteByte('\n')
	return b.String()
}

func writeCIRunSummaryTable(b *strings.Builder, rows [][2]string) {
	b.WriteString("| Field | Value |\n")
	b.WriteString("| --- | --- |\n")
	for _, row := range rows {
		if strings.TrimSpace(row[1]) == "" {
			continue
		}
		fmt.Fprintf(b, "| %s | %s |\n", ciMarkdownText(row[0]), ciMarkdownText(row[1]))
	}
}

func ciRunGatePolicyLabel(releaseGate map[string]any) string {
	key := mapString(releaseGate, "policy_key")
	version := mapString(releaseGate, "policy_version")
	fingerprint := mapString(releaseGate, "policy_fingerprint")
	parts := make([]string, 0, 3)
	if key != "" {
		parts = append(parts, key)
	}
	if version != "" {
		parts = append(parts, "v"+version)
	}
	if fingerprint != "" {
		parts = append(parts, fingerprint)
	}
	return strings.Join(parts, " / ")
}

func ciRunSummaryLinks(result ciRunResult, scorecard, comparison, releaseGate map[string]any) []string {
	type link struct {
		label string
		raw   string
	}
	candidates := []link{
		{label: "Candidate run", raw: result.Candidate.RunURL},
		{label: "CI workflow", raw: str(mapValue(result.Candidate.CIMetadata, "workflow_run_url"))},
		{label: "Scorecard", raw: mapString(scorecard, "url", "web_url", "scorecard_url")},
		{label: "Replay", raw: mapString(scorecard, "replay_url")},
		{label: "Comparison", raw: mapString(comparison, "url", "web_url", "comparison_url")},
		{label: "Release gate", raw: mapString(releaseGate, "url", "web_url")},
	}
	links := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if safe := ciSafeHTTPURL(candidate.raw); safe != "" {
			links = append(links, fmt.Sprintf("[%s](%s)", ciMarkdownText(candidate.label), safe))
		}
	}
	return links
}

func ciRunTopFailureLines(verdict string, scorecard, comparison, releaseGate map[string]any) []string {
	var lines []string
	add := func(line string) {
		line = output.SanitizeLine(strings.TrimSpace(line))
		if line == "" {
			return
		}
		for _, existing := range lines {
			if existing == line {
				return
			}
		}
		lines = append(lines, line)
	}
	addSlice := func(prefix string, values []any) {
		for _, value := range values {
			if len(lines) >= 6 {
				return
			}
			add(prefix + str(value))
		}
	}

	if summary := mapString(releaseGate, "summary"); summary != "" {
		add(summary)
	}
	addSlice("Regression: ", mapSlice(comparison, "regression_reasons"))
	if evidence := mapObject(comparison, "evidence_quality"); evidence != nil {
		addSlice("Evidence warning: ", mapSlice(evidence, "warnings"))
		addSlice("Missing evidence: ", mapSlice(evidence, "missing_fields"))
	}
	if details := mapObject(releaseGate, "evaluation_details"); details != nil {
		addSlice("Condition: ", mapSlice(details, "triggered_conditions"))
		addSlice("Warning: ", mapSlice(details, "warnings"))
		addSlice("Missing: ", mapSlice(details, "missing_required_dimensions"))
	}
	for _, line := range ciRunFailedScorecardDimensions(scorecard) {
		if len(lines) >= 6 {
			break
		}
		add(line)
	}
	if len(lines) == 0 {
		if verdict == "pass" {
			add("No blocking failures reported.")
		} else if reason := mapString(releaseGate, "reason_code"); reason != "" {
			add("Gate reason: " + reason)
		} else {
			add("Gate returned " + verdict + ".")
		}
	}
	if len(lines) > 6 {
		return lines[:6]
	}
	return lines
}

func ciRunFailedScorecardDimensions(scorecard map[string]any) []string {
	document := mapObject(scorecard, "scorecard")
	dimensions := mapObject(document, "dimensions")
	if len(dimensions) == 0 {
		dimensions = mapObject(scorecard, "dimensions")
	}
	if len(dimensions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(dimensions))
	for key := range dimensions {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		dimension, _ := dimensions[key].(map[string]any)
		if dimension == nil {
			continue
		}
		passed, hasPassed := dimension["passed"].(bool)
		outcome := mapString(dimension, "outcome", "status", "state")
		if hasPassed && passed {
			continue
		}
		if !hasPassed && outcome != "fail" && outcome != "failed" && outcome != "regression" {
			continue
		}
		detail := firstNonEmptyString(mapString(dimension, "reason"), mapString(dimension, "summary"), outcome)
		if detail == "" {
			detail = "failed"
		}
		lines = append(lines, fmt.Sprintf("Scorecard %s: %s", key, detail))
	}
	return lines
}

func ciMarkdownText(value string) string {
	value = output.SanitizeLine(value)
	value = strings.ReplaceAll(value, "|", "\\|")
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func ciSafeHTTPURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}
