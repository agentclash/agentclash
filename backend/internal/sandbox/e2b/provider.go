package e2b

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/filesystem/filesystemconnect"
	"github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/process/processconnect"
)

type Provider struct {
	client *apiClient
	config Config
}

func NewProvider(config Config) *Provider {
	return &Provider{
		client: newAPIClient(config),
		config: config,
	}
}

func (p *Provider) Create(ctx context.Context, request sandbox.CreateRequest) (sandbox.Session, error) {
	startedAt := time.Now()
	record, err := p.client.createSandbox(ctx, createSandboxRequest{
		TemplateID:          p.config.TemplateID,
		Timeout:             int(request.Timeout.Round(time.Second) / time.Second),
		Metadata:            request.Labels,
		Secure:              true,
		AllowInternetAccess: request.ToolPolicy.AllowNetwork,
	})
	if err != nil {
		slog.Default().Error("sandbox create failed", "run_id", request.RunID, "run_agent_id", request.RunAgentID, "template_id", p.config.TemplateID, "outcome", "failed_create", "duration", time.Since(startedAt), "error", err)
		return nil, err
	}
	slog.Default().Info("sandbox created", "sandbox_id", record.SandboxID, "run_id", request.RunID, "run_agent_id", request.RunAgentID, "template_id", record.TemplateID, "sandbox_url", p.client.envdBaseURL(record), "outcome", "created", "duration", time.Since(startedAt))
	return &session{
		client: clientSession{
			api:           p.client,
			record:        record,
			processClient: p.client.processClient(record),
			filesClient:   p.client.filesystemClient(record),
		},
	}, nil
}

func (p *Provider) Reconnect(_ context.Context, metadata json.RawMessage) (sandbox.Session, error) {
	var record sandboxRecord
	if err := json.Unmarshal(metadata, &record); err != nil {
		return nil, fmt.Errorf("unmarshal sandbox metadata: %w", err)
	}
	if record.SandboxID == "" {
		return nil, fmt.Errorf("sandbox metadata missing sandboxID")
	}
	slog.Default().Info("sandbox reconnected", "sandbox_id", record.SandboxID, "template_id", record.TemplateID, "sandbox_url", p.client.envdBaseURL(record))
	return &session{
		client: clientSession{
			api:           p.client,
			record:        record,
			processClient: p.client.processClient(record),
			filesClient:   p.client.filesystemClient(record),
		},
	}, nil
}

type clientSession struct {
	api           *apiClient
	record        sandboxRecord
	processClient processconnect.ProcessClient
	filesClient   filesystemconnect.FilesystemClient
}
