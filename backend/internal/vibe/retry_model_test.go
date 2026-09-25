package vibe

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIntegrationVibeRetryWithAnotherModel(t *testing.T) {
	svc, v, cfg := memoryService(t)
	ctx := context.Background()
	failed, original := memoryOperation(t, svc, v, "Prepare three tests for my returns agent.")
	until := timestamp().Add(time.Minute)
	if err := svc.Store.Finish(ctx, failed.ID, &Fault{Code: "provider_rate_limit", Message: "Busy", RetryAvailableAt: &until}); err != nil {
		t.Fatal(err)
	}
	v, _ = svc.Store.GetSession(ctx, v.Actor, v.ID)
	request := RetryRequest{ClientID: uuid.New(), Revision: v.Revision}
	_, err := svc.Retry(ctx, v.Actor, v.ID, failed.ID, request)
	requireFault(t, err, "retry_cooldown")
	request.AssistantModel = "openai/gpt-4o-mini"
	p := cfg.Profiles[request.AssistantModel]
	p.StructuredOutputs = true
	svc.Config.Profiles[p.ID] = p
	admitted, err := svc.Retry(ctx, v.Actor, v.ID, failed.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	var plan Plan
	if err = json.Unmarshal(admitted.Input, &plan); err != nil {
		t.Fatal(err)
	}
	if admitted.Models.Assistant != p.ID || admitted.Models.Target != failed.Models.Target || admitted.Models.Evaluator != failed.Models.Evaluator || plan.Submission.Content != original.Submission.Content || plan.sourceMessageID() != original.sourceMessageID() || plan.Retry.AssistantModel != p.ID || plan.Conversation.Profile.ID != p.ID {
		t.Fatal("model switch changed source/target/evaluator or reused the old profile")
	}
	current, _ := svc.Store.GetSession(ctx, v.Actor, v.ID)
	if len(current.Document.Messages) != 1 {
		t.Fatal("model switch duplicated the message")
	}
	again, err := svc.Retry(ctx, v.Actor, v.ID, failed.ID, request)
	if err != nil || again.ID != admitted.ID {
		t.Fatal("lost acknowledgement duplicated work", err)
	}
	request.AssistantModel = "openai/gpt-4.1"
	if _, err = svc.Retry(ctx, v.Actor, v.ID, failed.ID, request); err == nil {
		t.Fatal("same request ID changed model")
	}
}

func TestIntegrationVibeRetryModelRejectsUnapprovedOrHiddenChanges(t *testing.T) {
	for _, kind := range []string{"unknown", "unrecorded assistant", "target", "evaluator", "content"} {
		t.Run(kind, func(t *testing.T) {
			s := integrationStore(t)
			v, failed, plan := failedRetrySource(t, s)
			cfg := testConfig()
			svc := &Service{Store: s, Config: cfg, Gate: testGate(t)}
			request := RetryRequest{ClientID: uuid.New(), Revision: v.Revision, AssistantModel: "openai/gpt-4o-mini"}
			if kind == "unknown" {
				request.AssistantModel = "unapproved/model"
				if _, err := svc.Retry(context.Background(), v.Actor, v.ID, failed.ID, request); err == nil {
					t.Fatal("unapproved retry model admitted")
				}
				return
			}
			sub := retrySubmission(plan.Submission, failed.ID, request)
			retry := retryContext(failed, plan)
			retry.AssistantModel = request.AssistantModel
			switch kind {
			case "unrecorded assistant":
				retry.AssistantModel = ""
			case "target":
				sub.Models.Target = "openai/gpt-4.1"
			case "evaluator":
				sub.Models.Evaluator = "openai/gpt-4.1"
			case "content":
				sub.Content = "A different request"
			}
			plan.Submission, plan.Retry = sub, &retry
			_, err := s.Submit(context.Background(), v.Actor, v.ID, sub, plan, cfg)
			requireFault(t, err, "retry_not_allowed")
		})
	}
}
