package vibe

import (
	"encoding/json"
	"fmt"
	"github.com/agentclash/agentclash/runtime/provider"
)

type ContextDiagnostic struct {
	UpperBound     int            `json:"upper_bound"`
	Limit          int            `json:"limit"`
	Sections       map[string]int `json:"sections"`
	LargestSection string         `json:"largest_section"`
}

func contextFault(req provider.Request, count ContextCount, p ModelProfile, l Limits) error {
	d := &ContextDiagnostic{UpperBound: count.UpperBound, Limit: min(l.ContextTokens, p.Context-req.MaxOutputTokens), Sections: map[string]int{"response_schema": len(req.ResponseFormat), "provider_framing": p.FramingAllowance}}
	for _, m := range req.Messages {
		name := m.Role + "_messages"
		if m.Role == "user" {
			var data map[string]json.RawMessage
			if json.Unmarshal([]byte(m.Content), &data) == nil {
				for k, v := range data {
					if k == "conversation_data" {
						var sections map[string]json.RawMessage
						if json.Unmarshal(v, &sections) == nil {
							for name, content := range sections {
								d.Sections[name] += len(raw(string(content)))
							}
							continue
						}
					}
					d.Sections[k] += len(raw(string(v)))
				}
				continue
			}
		}
		d.Sections[name] += len(raw(m.Content))
	}
	largest := 0
	for name, n := range d.Sections {
		if n > largest || (n == largest && name < d.LargestSection) {
			largest = n
			d.LargestSection = name
		}
	}
	label := map[string]string{"conversation": "setup conversation", "provided_chats": "provided chats", "observations": "evaluation evidence", "current_check": "current expectations", "user_request": "current message", "requirements": "requirements", "messages": "recent conversation", "latest_proposal": "latest draft or test plan", "accepted_agent": "accepted draft and evaluation", "current_message": "current message", "observed_evaluation_data": "evaluation evidence", "system_messages": "authoring instructions", "response_schema": "response contract", "provider_framing": "provider framing", "user_messages": "message"}[d.LargestSection]
	if label == "" {
		label = conversationContextLabels()[d.LargestSection]
	}
	if label == "" {
		label = "conversation context"
	}
	return &Fault{Code: "context_limit", Message: fmt.Sprintf("This request is too large to send in one go. The largest part is %s. Try fewer chats or a shorter message. Your saved work is unchanged, and no model request was sent.", label), Context: d}
}
