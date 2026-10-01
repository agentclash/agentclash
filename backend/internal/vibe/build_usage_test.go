package vibe

import (
	"context"
	"encoding/json"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/google/uuid"
	"testing"
)

func TestReportedAttemptTokens(t *testing.T) {
	for _, tc := range []struct {
		name    string
		receipt []byte
		tokens  int64
		known   bool
	}{
		{"response", raw(provider.Response{Usage: provider.Usage{InputTokens: 123, OutputTokens: 42, TotalTokens: 165}}), 165, true},
		{"missing total", raw(provider.Response{Usage: provider.Usage{InputTokens: 123, OutputTokens: 42}}), 165, true},
		{"reconciled", []byte(`{"data":{"native_tokens_prompt":123,"native_tokens_completion":42}}`), 165, true},
		{"cost only", []byte(`{"data":{"total_cost":0.01}}`), 0, false},
		{"empty", []byte(`{}`), 0, false},
		{"corrupt", []byte(`{`), 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, known := reportedAttemptTokens(tc.receipt)
			if n != tc.tokens || known != tc.known {
				t.Fatalf("tokens=%d known=%t", n, known)
			}
		})
	}
}

func TestIntegrationBuildQuotaRecheckedBeforePaidWork(t *testing.T) {
	s, v := materialService(t)
	ctx := context.Background()
	request := BuildQuoteRequest{Content: "Summarize meeting notes.", Models: DefaultModels()}
	quote, err := s.QuoteBuild(ctx, v.Actor, v.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Store.CreateAgent(ctx, v.Actor, nil, uuid.New(), "build", DefaultModels())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Store.DB.Exec(ctx, "DELETE FROM vibe_sessions WHERE id=$1", other.ID) })
	// Another tab uses the initial check after our quote, before our submission.
	sub := Submission{ClientID: uuid.New(), Revision: other.Revision, Kind: "check", Models: DefaultModels()}
	op, err := s.Store.Submit(ctx, other.Actor, other.ID, sub, Plan{Submission: sub, Anonymous: true, Calls: 1, MaxCost: 1}, s.Config)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Store.Start(ctx, op.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.Finish(ctx, op.ID, nil); err != nil {
		t.Fatal(err)
	}
	_, err = s.QuoteBuild(ctx, v.Actor, v.ID, request)
	requireFault(t, err, "trial_limit")
	_, err = s.Prepare(ctx, v.Actor, v.ID, Submission{ClientID: uuid.New(), Revision: v.Revision, Kind: "message", Content: request.Content, Models: request.Models, TestJourney: true, CycleID: &quote.ID})
	requireFault(t, err, "trial_limit")
	current, err := s.Store.GetSession(ctx, v.Actor, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Operations) != 0 {
		b, _ := json.Marshal(current.Operations)
		t.Fatalf("quota failure admitted work: %s", b)
	}
}
