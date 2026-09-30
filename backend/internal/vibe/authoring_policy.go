package vibe

import "strings"

// This describes execution, not the persisted wire format. Retained plans keep
// their original bytes, stage keys and journal versions across deployments.
type authoringPolicy struct {
	Version   int
	Contract  string
	Calls     int
	Prototype bool
	Materials bool
}

func authoringPolicyFor(version int) (authoringPolicy, bool) {
	switch version {
	case interpretedAuthoringVersion:
		return authoringPolicy{Version: 15, Contract: "vibe-v15", Calls: 8}, true
	case buildAuthoringVersion:
		return authoringPolicy{Version: 18, Contract: "vibe-v18", Calls: 10, Prototype: true}, true
	case groundedBuildAuthoringVersion:
		return authoringPolicy{Version: 20, Contract: "vibe-v20", Calls: 10, Prototype: true, Materials: true}, true
	}
	return authoringPolicy{}, false
}
func (p authoringPolicy) stageAllowed(step string, manual bool) bool {
	if manual {
		return step == "review"
	}
	if step == "candidate:patch" || step == "candidate:review" {
		return true
	}
	stage := strings.TrimSuffix(strings.TrimSuffix(step, ":repair"), ":fallback")
	return (step == stage || step == stage+":repair" || step == stage+":fallback") && (stage == "route" || stage == "handler" || stage == "review" || stage == "prototype" && p.Prototype)
}
func (p authoringPolicy) allowance(primary int64, fallback *AssistantRecovery) (int, int64) {
	calls, cost := p.Calls, int64(p.Calls)*primary
	if fallback != nil {
		calls++
		cost += fallback.MaxCost
	}
	return calls, cost
}
func recoveryModel(primary string) string {
	if strings.HasPrefix(primary, "deepseek/") {
		return "openai/gpt-5.4-mini"
	}
	if primary == "openai/gpt-5.4-mini" {
		return "deepseek/deepseek-v4.1-flash"
	}
	return ""
}

func retainedAuthoring(version int) bool { _, ok := authoringPolicyFor(version); return ok }

func authoringStageAllowed(p Plan, step string) bool {
	policy, ok := authoringPolicyFor(p.AuthoringVersion)
	return ok && policy.stageAllowed(step, p.Conversation != nil && p.Conversation.Manual != nil)
}
