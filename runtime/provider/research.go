package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ResearchRequest describes a single OpenAI Responses API / deep-research call.
// AgentClash uses this for challenge packs with execution_mode: responses.
type ResearchRequest struct {
	ProviderKey         string
	ProviderAccountID   string
	CredentialReference string
	Model               string
	TraceMode           string
	RunTimeout          time.Duration
	Instructions        string
	Input               string
	OutputSchema        json.RawMessage
	Metadata            json.RawMessage
	Background          bool
}

type ResearchClient interface {
	InvokeResearch(ctx context.Context, request ResearchRequest) (Response, error)
}

func (r Router) InvokeResearch(ctx context.Context, request ResearchRequest) (Response, error) {
	adapter, ok := r.adapters[request.ProviderKey]
	if !ok {
		return Response{}, NewFailure(
			request.ProviderKey,
			FailureCodeUnsupportedProvider,
			fmt.Sprintf("provider %q is not supported", request.ProviderKey),
			false,
			ErrUnsupportedProvider,
		)
	}

	researchClient, ok := adapter.(ResearchClient)
	if !ok {
		return Response{}, NewFailure(
			request.ProviderKey,
			FailureCodeUnsupportedCapability,
			fmt.Sprintf("provider %q does not support OpenAI Responses / deep research", request.ProviderKey),
			false,
			nil,
		)
	}

	return researchClient.InvokeResearch(ctx, request)
}
