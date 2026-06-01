package api

import (
	"encoding/json"
	"sort"
	"strings"
)

type agentBuildVersionTemplate struct {
	Key             string
	Name            string
	Description     string
	AgentKind       string
	InterfaceSpec   json.RawMessage
	PolicySpec      json.RawMessage
	ReasoningSpec   json.RawMessage
	MemorySpec      json.RawMessage
	WorkflowSpec    json.RawMessage
	GuardrailSpec   json.RawMessage
	ModelSpec       json.RawMessage
	OutputSchema    json.RawMessage
	TraceContract   json.RawMessage
	PublicationSpec json.RawMessage
}

type agentBuildVersionTemplateResponse struct {
	Key             string          `json:"key"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	AgentKind       string          `json:"agent_kind"`
	InterfaceSpec   json.RawMessage `json:"interface_spec"`
	PolicySpec      json.RawMessage `json:"policy_spec"`
	ReasoningSpec   json.RawMessage `json:"reasoning_spec"`
	MemorySpec      json.RawMessage `json:"memory_spec"`
	WorkflowSpec    json.RawMessage `json:"workflow_spec"`
	GuardrailSpec   json.RawMessage `json:"guardrail_spec"`
	ModelSpec       json.RawMessage `json:"model_spec"`
	OutputSchema    json.RawMessage `json:"output_schema"`
	TraceContract   json.RawMessage `json:"trace_contract"`
	PublicationSpec json.RawMessage `json:"publication_spec"`
}

var agentBuildVersionTemplates = map[string]agentBuildVersionTemplate{
	"code-reviewer": {
		Key:         "code-reviewer",
		Name:        "Code Reviewer",
		Description: "Reviews source changes for correctness, regressions, security risks, and missing tests.",
		AgentKind:   "llm_agent",
		PolicySpec: json.RawMessage(`{
			"instructions": "You are a senior code reviewer. Prioritize correctness bugs, security and privacy risks, behavioral regressions, and missing tests. Lead with actionable findings and cite concrete files or evidence when possible."
		}`),
		ReasoningSpec: json.RawMessage(`{
			"mode": "deliberate",
			"review_focus": ["correctness", "security", "regressions", "tests"]
		}`),
		ModelSpec: json.RawMessage(`{
			"temperature": 0.1
		}`),
		OutputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"findings": {
					"type": "array",
					"items": {
						"type": "object",
						"properties": {
							"severity": {"type": "string"},
							"title": {"type": "string"},
							"location": {"type": "string"},
							"recommendation": {"type": "string"}
						}
					}
				},
				"summary": {"type": "string"}
			}
		}`),
		TraceContract: json.RawMessage(`{
			"required_events": ["analysis", "finding_summary"]
		}`),
		PublicationSpec: json.RawMessage(`{
			"display": "review_findings"
		}`),
	},
	"honest-agent": {
		Key:         "honest-agent",
		Name:        "Honest Agent",
		Description: "Answers directly, states uncertainty, and refuses to invent facts when evidence is missing.",
		AgentKind:   "llm_agent",
		PolicySpec: json.RawMessage(`{
			"instructions": "Answer honestly and directly. State uncertainty, ask for missing context when it matters, and never invent facts, tool results, citations, or successful actions."
		}`),
		ReasoningSpec: json.RawMessage(`{
			"mode": "careful",
			"uncertainty_policy": "state_limits"
		}`),
		ModelSpec: json.RawMessage(`{
			"temperature": 0.2
		}`),
		OutputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"answer": {"type": "string"},
				"confidence": {"type": "string"},
				"open_questions": {
					"type": "array",
					"items": {"type": "string"}
				}
			}
		}`),
		TraceContract: json.RawMessage(`{
			"required_events": ["answer"]
		}`),
		PublicationSpec: json.RawMessage(`{
			"display": "answer"
		}`),
	},
}

func listAgentBuildVersionTemplateResponses() []agentBuildVersionTemplateResponse {
	keys := make([]string, 0, len(agentBuildVersionTemplates))
	for key := range agentBuildVersionTemplates {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	items := make([]agentBuildVersionTemplateResponse, 0, len(keys))
	for _, key := range keys {
		items = append(items, buildAgentBuildVersionTemplateResponse(agentBuildVersionTemplates[key]))
	}
	return items
}

func buildAgentBuildVersionTemplateResponse(template agentBuildVersionTemplate) agentBuildVersionTemplateResponse {
	return agentBuildVersionTemplateResponse{
		Key:             template.Key,
		Name:            template.Name,
		Description:     template.Description,
		AgentKind:       template.AgentKind,
		InterfaceSpec:   defaultJSON(template.InterfaceSpec),
		PolicySpec:      defaultJSON(template.PolicySpec),
		ReasoningSpec:   defaultJSON(template.ReasoningSpec),
		MemorySpec:      defaultJSON(template.MemorySpec),
		WorkflowSpec:    defaultJSON(template.WorkflowSpec),
		GuardrailSpec:   defaultJSON(template.GuardrailSpec),
		ModelSpec:       defaultJSON(template.ModelSpec),
		OutputSchema:    defaultJSON(template.OutputSchema),
		TraceContract:   defaultJSON(template.TraceContract),
		PublicationSpec: defaultJSON(template.PublicationSpec),
	}
}

func applyAgentBuildVersionTemplate(input CreateAgentBuildVersionInput) (CreateAgentBuildVersionInput, error) {
	key := strings.TrimSpace(input.Template)
	if key == "" {
		return input, nil
	}

	template, ok := agentBuildVersionTemplates[key]
	if !ok {
		return CreateAgentBuildVersionInput{}, AgentBuildValidationError{
			Code:    "invalid_template",
			Message: "template must be one of: " + strings.Join(agentBuildVersionTemplateKeys(), ", "),
		}
	}

	if strings.TrimSpace(input.AgentKind) == "" {
		input.AgentKind = template.AgentKind
	}
	input.InterfaceSpec = defaultRawMessage(input.InterfaceSpec, template.InterfaceSpec)
	input.PolicySpec = defaultRawMessage(input.PolicySpec, template.PolicySpec)
	input.ReasoningSpec = defaultRawMessage(input.ReasoningSpec, template.ReasoningSpec)
	input.MemorySpec = defaultRawMessage(input.MemorySpec, template.MemorySpec)
	input.WorkflowSpec = defaultRawMessage(input.WorkflowSpec, template.WorkflowSpec)
	input.GuardrailSpec = defaultRawMessage(input.GuardrailSpec, template.GuardrailSpec)
	input.ModelSpec = defaultRawMessage(input.ModelSpec, template.ModelSpec)
	input.OutputSchema = defaultRawMessage(input.OutputSchema, template.OutputSchema)
	input.TraceContract = defaultRawMessage(input.TraceContract, template.TraceContract)
	input.PublicationSpec = defaultRawMessage(input.PublicationSpec, template.PublicationSpec)
	return input, nil
}

func agentBuildVersionTemplateKeys() []string {
	keys := make([]string, 0, len(agentBuildVersionTemplates))
	for key := range agentBuildVersionTemplates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func defaultRawMessage(value, fallback json.RawMessage) json.RawMessage {
	if len(value) == 0 || strings.EqualFold(strings.TrimSpace(string(value)), "null") {
		return fallback
	}
	return value
}
