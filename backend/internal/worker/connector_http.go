package worker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/connectors"
	"github.com/google/uuid"
	"time"
)

type ConnectorExecutionRepository interface {
	ConnectorCredentialSource
	ConnectorDeadline(context.Context, uuid.UUID, time.Time) (time.Time, error)
	ClaimConnectorDispatch(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string, string, []byte, time.Time) (repository.ConnectorDispatch, bool, error)
	CommitConnectorResponse(context.Context, uuid.UUID, json.RawMessage) error
	UnknownConnectorDispatch(context.Context, uuid.UUID) error
}
type HTTPJSONInvoker struct {
	Repository ConnectorExecutionRepository
	Client     connectors.Client
}

func (i HTTPJSONInvoker) InvokeConnector(ctx context.Context, e repository.RunAgentExecutionContext) error {
	if i.Repository == nil {
		return errors.New("connector repository unavailable")
	}
	if e.ChallengeInputSet == nil || len(e.ChallengeInputSet.Cases) == 0 {
		return errors.New("http_json requires explicitly selected canonical cases")
	}
	if e.ValidateControlledDispatch != nil {
		if err := e.ValidateControlledDispatch(ctx); err != nil {
			return err
		}
	}
	// One run deadline bounds every case, including network and response parsing.
	deadline, err := i.Repository.ConnectorDeadline(ctx, e.RunAgent.ID, time.Now().Add(time.Duration(e.Deployment.RuntimeProfile.RunTimeoutSeconds)*time.Second))
	if err != nil {
		return err
	}
	for _, item := range e.ChallengeInputSet.Cases {
		binding, secret, err := i.Repository.ResolveApprovedConnector(ctx, e.Deployment.AgentDeploymentSnapshotID, e.Run.WorkspaceID)
		if err != nil {
			return err
		}
		if binding.Config.Adapter != connectors.HTTPJSON {
			return errors.New("snapshot is not an http_json connector")
		}
		mapped, err := binding.Config.Request(item.Payload)
		if err != nil {
			return err
		}
		caseDeadline := deadline
		if d := time.Now().Add(time.Duration(binding.Config.TimeoutSeconds) * time.Second); d.Before(caseDeadline) {
			caseDeadline = d
		}
		if item.CaseTimeoutSeconds > 0 {
			if d := time.Now().Add(time.Duration(item.CaseTimeoutSeconds) * time.Second); d.Before(caseDeadline) {
				caseDeadline = d
			}
		}
		dispatch, claimed, err := i.Repository.ClaimConnectorDispatch(ctx, e.RunAgent.ID, binding.SnapshotID, item.ID, item.CaseKey, item.ChallengeKey, mapped, caseDeadline)
		if err != nil {
			return err
		}
		if !claimed {
			if dispatch.State == "completed" {
				continue
			}
			return repository.ErrConnectorOutcomeUnknown
		}
		transport := i.Client
		transport.BeforeSend = e.AcknowledgeControlledDispatch
		transport.ResolveSecret = func(ctx context.Context) (string, error) {
			if e.ValidateControlledDispatch != nil {
				if err := e.ValidateControlledDispatch(ctx); err != nil {
					return "", err
				}
			}
			_, secret, err := i.Repository.ResolveApprovedConnector(ctx, e.Deployment.AgentDeploymentSnapshotID, e.Run.WorkspaceID)
			return secret, err
		}
		output, err := transport.InvokeHTTPJSON(ctx, binding.Config, secret, item.Payload, dispatch.DeadlineAt)
		persist, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		if err != nil {
			markErr := i.Repository.UnknownConnectorDispatch(persist, dispatch.ID)
			cancel()
			if markErr != nil {
				return markErr
			}
			return repository.ErrConnectorOutcomeUnknown
		}
		err = i.Repository.CommitConnectorResponse(persist, dispatch.ID, output)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}
