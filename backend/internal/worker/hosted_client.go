package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/hostedruns"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
)

type HostedRunClient struct {
	httpClient      *http.Client
	callbackBaseURL string
	callbackSigner  hostedruns.CallbackTokenSigner
}

func NewHostedRunClient(httpClient *http.Client, callbackBaseURL string, callbackSecret string) HostedRunClient {
	return HostedRunClient{
		httpClient:      httpClient,
		callbackBaseURL: strings.TrimRight(callbackBaseURL, "/"),
		callbackSigner:  hostedruns.NewCallbackTokenSigner(callbackSecret),
	}
}

func (c HostedRunClient) Start(ctx context.Context, input workflowpkg.HostedRunStartInput) (hostedruns.StartResponse, error) {
	endpointURL, err := hostedEndpointURL(*input.ExecutionContext.Deployment.EndpointURL)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(payload))
	if err != nil {
		return hostedruns.StartResponse{}, fmt.Errorf("build hosted run request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return hostedruns.StartResponse{}, fmt.Errorf("post hosted run request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return hostedruns.StartResponse{}, fmt.Errorf("hosted run request returned status %d", resp.StatusCode)
	}

	var startResponse hostedruns.StartResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&startResponse); err != nil {
		return hostedruns.StartResponse{}, fmt.Errorf("decode hosted run response: %w", err)
	}
	if decoder.More() {
		return hostedruns.StartResponse{}, fmt.Errorf("decode hosted run response: response must contain exactly one JSON object")
	}
	if !startResponse.Accepted {
		return hostedruns.StartResponse{}, fmt.Errorf("hosted deployment rejected run")
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
