package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/hostedruns"
)

type HostedRunClient struct {
	httpClient      *http.Client
	transport       connectors.Client
	credentials     ConnectorCredentialSource
	callbackBaseURL string
	callbackSigner  hostedruns.CallbackTokenSigner
}

func NewHostedRunClient(httpClient *http.Client, callbackBaseURL string, callbackSecret string) HostedRunClient {
	return HostedRunClient{
		httpClient:      httpClient,
		transport:       connectors.Client{HTTP: httpClient},
		callbackBaseURL: strings.TrimRight(callbackBaseURL, "/"),
		callbackSigner:  hostedruns.NewCallbackTokenSigner(callbackSecret),
	}
}

type ConnectorCredentialSource interface {
	ResolveApprovedConnector(context.Context, uuid.UUID, uuid.UUID) (connectors.Binding, string, error)
}

func (c HostedRunClient) WithConnectorRepository(repo ConnectorCredentialSource) HostedRunClient {
	c.credentials = repo
	return c
}

func (c HostedRunClient) Start(ctx context.Context, input workflowpkg.HostedRunStartInput) (hostedruns.StartResponse, error) {
	if c.credentials == nil {
		return hostedruns.StartResponse{}, errors.New("hosted connector approval is unavailable")
	}
	binding, secret, err := c.credentials.ResolveApprovedConnector(ctx, input.ExecutionContext.Deployment.AgentDeploymentSnapshotID, input.ExecutionContext.Run.WorkspaceID)
	if err != nil {
		return hostedruns.StartResponse{}, err
	}
	if binding.Config.Adapter != connectors.Hosted {
		return hostedruns.StartResponse{}, errors.New("snapshot is not a hosted connector")
	}
	endpointURL, err := hostedEndpointURL(binding.Config.EndpointURL)
	if err != nil {
		return hostedruns.StartResponse{}, err
	}

	callbackToken, err := c.callbackSigner.Sign(input.ExecutionContext.Run.ID, input.ExecutionContext.RunAgent.ID)
	if err != nil {
		return hostedruns.StartResponse{}, err
	}

	requestBody := hostedruns.StartRequest{
		RunID:                  input.ExecutionContext.Run.ID,
		RunAgentID:             input.ExecutionContext.RunAgent.ID,
		ChallengePackVersionID: input.ExecutionContext.Run.ChallengePackVersionID,
		TaskPayload:            input.TaskPayload,
		TraceLevel:             input.TraceLevel,
		CallbackURL:            fmt.Sprintf("%s/v1/integrations/hosted-runs/%s/events", c.callbackBaseURL, input.ExecutionContext.Run.ID),
		CallbackToken:          callbackToken,
		DeadlineAt:             input.DeadlineAt,
	}
	if err := requestBody.Validate(); err != nil {
		return hostedruns.StartResponse{}, err
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return hostedruns.StartResponse{}, fmt.Errorf("marshal hosted run request: %w", err)
	}

	deadline := input.DeadlineAt
	if bound := time.Now().Add(time.Duration(binding.Config.TimeoutSeconds) * time.Second); bound.Before(deadline) {
		deadline = bound
	}
	transport := c.transport
	transport.ResolveSecret = func(ctx context.Context) (string, error) {
		_, secret, err := c.credentials.ResolveApprovedConnector(ctx, input.ExecutionContext.Deployment.AgentDeploymentSnapshotID, input.ExecutionContext.Run.WorkspaceID)
		return secret, err
	}
	response, err := transport.DoJSON(ctx, endpointURL, binding.Config.Auth, secret, payload, deadline)
	if err != nil {
		return hostedruns.StartResponse{}, err
	}

	var startResponse hostedruns.StartResponse
	decoder := json.NewDecoder(strings.NewReader(string(response)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&startResponse); err != nil {
		return hostedruns.StartResponse{}, fmt.Errorf("decode hosted run response: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF {
		return hostedruns.StartResponse{}, fmt.Errorf("decode hosted run response: response must contain exactly one JSON object")
	}
	if !startResponse.Accepted {
		return startResponse, nil
	}
	if startResponse.ExternalRunID == "" {
		return hostedruns.StartResponse{}, fmt.Errorf("hosted deployment response is missing external_run_id")
	}
	return startResponse, nil
}

func hostedEndpointURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse hosted deployment endpoint_url: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/agentclash/runs"
	return parsed.String(), nil
}
