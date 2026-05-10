package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

var runTranscriptCmd = &cobra.Command{
	Use:   "transcript <runId>",
	Short: "Export a Markdown run transcript",
	Long:  "Export a readable, secret-safe Markdown transcript from persisted run events.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		transcript, err := buildRunMarkdownTranscript(cmd, rc, args[0])
		if err != nil {
			return err
		}
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(map[string]any{
				"run_id":     args[0],
				"format":     "markdown",
				"transcript": transcript,
			})
		}
		_, err = fmt.Fprint(rc.Output.Writer(), transcript)
		return err
	},
}

type transcriptAgent struct {
	ID     string
	Label  string
	Status string
}

type transcriptEvent struct {
	EventID        string         `json:"event_id"`
	RunID          string         `json:"run_id"`
	RunAgentID     string         `json:"run_agent_id"`
	SequenceNumber int64          `json:"sequence_number"`
	EventType      string         `json:"event_type"`
	Source         string         `json:"source"`
	OccurredAt     string         `json:"occurred_at"`
	Payload        map[string]any `json:"payload"`
	Summary        map[string]any `json:"summary"`
}

func buildRunMarkdownTranscript(cmd *cobra.Command, rc *RunContext, runID string) (string, error) {
	agents, err := fetchTranscriptAgents(cmd, rc, runID)
	if err != nil {
		return "", err
	}
	events, err := fetchTranscriptEvents(cmd, rc, runID)
	if err != nil {
		return "", err
	}
	return renderRunMarkdownTranscript(runID, agents, events), nil
}

func fetchTranscriptAgents(cmd *cobra.Command, rc *RunContext, runID string) (map[string]transcriptAgent, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID+"/agents", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	var result struct {
		Items []map[string]any `json:"items"`
	}
	if err := resp.DecodeJSON(&result); err != nil {
		return nil, err
	}

	agents := make(map[string]transcriptAgent, len(result.Items))
	for _, item := range result.Items {
		id := str(item["id"])
		if id == "" {
			continue
		}
		agents[id] = transcriptAgent{
			ID:     id,
			Label:  firstNonEmpty(mapString(item, "label"), mapString(item, "agent_deployment_name"), id),
			Status: str(item["status"]),
		}
	}
	return agents, nil
}

func fetchTranscriptEvents(cmd *cobra.Command, rc *RunContext, runID string) ([]transcriptEvent, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/runs/"+runID+"/events/export", nil)
	if err != nil {
		return nil, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return nil, apiErr
	}

	decoder := json.NewDecoder(bytes.NewReader(resp.Body))
	var events []transcriptEvent
	for {
		var event transcriptEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode run event JSONL: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}

func renderRunMarkdownTranscript(runID string, agents map[string]transcriptAgent, events []transcriptEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Run Transcript\n\n")
	fmt.Fprintf(&b, "- Run ID: `%s`\n", markdownInline(runID))
	fmt.Fprintf(&b, "- Events considered: %d\n", len(events))
	fmt.Fprintf(&b, "- Raw payloads and tool arguments are intentionally omitted.\n\n")

	writeTranscriptAgents(&b, agents)

	b.WriteString("## Transcript\n\n")
	wrote := false
	for _, event := range events {
		if writeTranscriptEvent(&b, event, agents) {
			wrote = true
		}
	}
	if !wrote {
		b.WriteString("_No transcript-safe events found._\n")
	}
	return b.String()
}

func writeTranscriptAgents(b *strings.Builder, agents map[string]transcriptAgent) {
	if len(agents) == 0 {
		return
	}
	b.WriteString("## Agents\n\n")
	ids := make([]string, 0, len(agents))
	for id := range agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		agent := agents[id]
		fmt.Fprintf(b, "- `%s`", markdownInline(id))
		if agent.Label != "" && agent.Label != id {
			fmt.Fprintf(b, " - %s", markdownInline(agent.Label))
		}
		if agent.Status != "" {
			fmt.Fprintf(b, " (%s)", markdownInline(agent.Status))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeTranscriptEvent(b *strings.Builder, event transcriptEvent, agents map[string]transcriptAgent) bool {
	title, details, block := transcriptEventSummary(event)
	if title == "" {
		return false
	}

	agent := transcriptAgentLabel(event.RunAgentID, agents)
	fmt.Fprintf(b, "### %s", markdownInline(title))
	if event.OccurredAt != "" {
		fmt.Fprintf(b, " - %s", markdownInline(event.OccurredAt))
	}
	b.WriteString("\n\n")
	if agent != "" {
		fmt.Fprintf(b, "- Agent: %s\n", markdownInline(agent))
	}
	if event.SequenceNumber > 0 {
		fmt.Fprintf(b, "- Sequence: %d\n", event.SequenceNumber)
	}
	if len(details) > 0 {
		for _, detail := range details {
			fmt.Fprintf(b, "- %s\n", markdownInline(detail))
		}
	}
	if block != "" {
		b.WriteString("\n")
		b.WriteString(markdownFence(block))
	}
	b.WriteString("\n")
	return true
}

func transcriptEventSummary(event transcriptEvent) (string, []string, string) {
	payload := event.Payload
	switch event.EventType {
	case "system.run.started":
		return "Run started", nil, ""
	case "system.run.completed":
		return "Run completed", nil, ""
	case "system.run.failed":
		return "Run failed", transcriptFailureDetails(payload), ""
	case "system.output.finalized":
		return "Final output", nil, safePayloadString(payload, "final_output", "output_text", "output", "message")
	case "model.call.started":
		return "Model call started", compactDetails(
			labelValue("Provider", safePayloadString(payload, "provider_key")),
			labelValue("Model", firstNonEmpty(safePayloadString(payload, "provider_model_id"), safePayloadString(payload, "model"))),
			labelValue("Messages", safePayloadString(payload, "message_count")),
		), ""
	case "model.call.completed":
		return "Model call completed", compactDetails(
			labelValue("Finish", safePayloadString(payload, "finish_reason")),
			labelValue("Usage", transcriptUsage(payload)),
			labelValue("Tool calls", transcriptToolCallNames(payload)),
		), safePayloadString(payload, "output_text")
	case "model.tool_calls.proposed":
		return "Tool calls proposed", compactDetails(labelValue("Tool calls", transcriptToolCallNames(payload))), ""
	case "tool.call.started":
		return "Tool call started", compactDetails(labelValue("Tool", transcriptToolName(payload))), ""
	case "tool.call.completed":
		return "Tool call completed", compactDetails(labelValue("Tool", transcriptToolName(payload))), ""
	case "tool.call.failed":
		return "Tool call failed", compactDetails(
			labelValue("Tool", transcriptToolName(payload)),
			labelValue("Code", safePayloadString(payload, "error_code", "code", "failure_code")),
		), ""
	case "sandbox.command.started":
		return "Sandbox command started", compactDetails(labelValue("Action", safePayloadString(payload, "sandbox_action", "action"))), ""
	case "sandbox.command.completed":
		return "Sandbox command completed", compactDetails(
			labelValue("Action", safePayloadString(payload, "sandbox_action", "action")),
			labelValue("Exit code", safePayloadString(payload, "exit_code")),
		), ""
	case "sandbox.command.failed":
		return "Sandbox command failed", compactDetails(
			labelValue("Action", safePayloadString(payload, "sandbox_action", "action")),
			labelValue("Exit code", safePayloadString(payload, "exit_code")),
			labelValue("Code", safePayloadString(payload, "error_code", "code", "failure_code")),
		), ""
	case "scoring.started":
		return "Scoring started", nil, ""
	case "scoring.completed":
		return "Scoring completed", transcriptStatusDetails(payload), ""
	case "scoring.failed":
		return "Scoring failed", transcriptFailureDetails(payload), ""
	default:
		return "", nil, ""
	}
}

func transcriptStatusDetails(payload map[string]any) []string {
	return compactDetails(
		labelValue("Status", safePayloadString(payload, "status")),
	)
}

func transcriptFailureDetails(payload map[string]any) []string {
	return compactDetails(
		labelValue("Status", safePayloadString(payload, "status")),
		labelValue("Class", safePayloadString(payload, "failure_class", "error_class")),
		labelValue("Code", safePayloadString(payload, "failure_code", "error_code", "code")),
	)
}

func transcriptUsage(payload map[string]any) string {
	usage := mapObject(payload, "usage")
	if usage == nil {
		return ""
	}
	parts := compactDetails(
		labelValue("input", safePayloadString(usage, "input_tokens", "prompt_tokens")),
		labelValue("output", safePayloadString(usage, "output_tokens", "completion_tokens")),
		labelValue("total", safePayloadString(usage, "total_tokens")),
	)
	return strings.Join(parts, ", ")
}

func transcriptToolCallNames(payload map[string]any) string {
	calls := mapSlice(payload, "tool_calls")
	names := make([]string, 0, len(calls))
	for _, raw := range calls {
		call, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := safePayloadString(call, "name", "tool_name")
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func transcriptToolName(payload map[string]any) string {
	return firstNonEmpty(
		safePayloadString(payload, "tool_name"),
		safePayloadString(payload, "name"),
		safePayloadString(mapObject(payload, "tool_call"), "name", "tool_name"),
	)
}

func safePayloadString(payload map[string]any, keys ...string) string {
	if payload == nil {
		return ""
	}
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return strings.TrimSpace(output.SanitizeControl(typed))
		case float64:
			if typed == float64(int64(typed)) {
				return fmt.Sprintf("%d", int64(typed))
			}
			return fmt.Sprint(typed)
		case bool:
			return fmt.Sprint(typed)
		default:
			return ""
		}
	}
	return ""
}

func transcriptAgentLabel(runAgentID string, agents map[string]transcriptAgent) string {
	if runAgentID == "" {
		return ""
	}
	agent, ok := agents[runAgentID]
	if !ok {
		return runAgentID
	}
	if agent.Label == "" || agent.Label == agent.ID {
		return agent.ID
	}
	return fmt.Sprintf("%s (%s)", agent.Label, agent.ID)
}

func compactDetails(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func labelValue(label, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", label, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func markdownInline(value string) string {
	value = output.SanitizeLine(value)
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"|", "\\|",
		"#", "\\#",
	)
	return replacer.Replace(value)
}

func markdownFence(value string) string {
	value = strings.TrimRight(output.SanitizeControl(value), "\n")
	if value == "" {
		return ""
	}
	fence := strings.Repeat("`", maxBacktickRun(value)+1)
	if len(fence) < 3 {
		fence = "```"
	}
	return fmt.Sprintf("%s\n%s\n%s\n", fence, value, fence)
}

func maxBacktickRun(value string) int {
	maxRun := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > maxRun {
				maxRun = current
			}
			continue
		}
		current = 0
	}
	return maxRun
}
