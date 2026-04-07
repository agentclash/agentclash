package e2b

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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

	templateID := p.config.TemplateID
	if request.TemplateID != "" {
		templateID = request.TemplateID
	}

	var network *networkConfig
	if len(request.NetworkAllowlist) > 0 {
		network = &networkConfig{AllowOut: request.NetworkAllowlist}
	}

	record, err := p.client.createSandbox(ctx, createSandboxRequest{
		TemplateID:          templateID,
		Timeout:             int(request.Timeout.Round(time.Second) / time.Second),
		Metadata:            request.Labels,
		Secure:              true,
		AllowInternetAccess: request.ToolPolicy.AllowNetwork,
		EnvVars:             request.EnvVars,
		Network:             network,
	})
	if err != nil {
		slog.Default().Error("sandbox create failed", "run_id", request.RunID, "run_agent_id", request.RunAgentID, "template_id", templateID, "outcome", "failed_create", "duration", time.Since(startedAt), "error", err)
		return nil, err
	}
	slog.Default().Info("sandbox created", "sandbox_id", record.SandboxID, "run_id", request.RunID, "run_agent_id", request.RunAgentID, "template_id", record.TemplateID, "sandbox_url", p.client.envdBaseURL(record), "outcome", "created", "duration", time.Since(startedAt))

	sess := &session{
		client: clientSession{
			api:           p.client,
			record:        record,
			processClient: p.client.processClient(record),
			filesClient:   p.client.filesystemClient(record),
		},
		allowShell: request.ToolPolicy.AllowShell,
	}

	if len(request.AdditionalPackages) > 0 {
		if err := p.installAdditionalPackages(ctx, sess, request); err != nil {
			_ = sess.Destroy(ctx)
			return nil, err
		}
	}

	return sess, nil
}

func (p *Provider) installAdditionalPackages(ctx context.Context, sess *session, request sandbox.CreateRequest) error {
	startedAt := time.Now()
	installCmd := "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " + strings.Join(request.AdditionalPackages, " ")
	result, err := sess.Exec(ctx, sandbox.ExecRequest{
		Command: []string{"sh", "-c", installCmd},
		Timeout: 120 * time.Second,
	})
	if err != nil {
		slog.Default().Error("sandbox additional packages install failed", "sandbox_id", sess.ID(), "run_id", request.RunID, "packages", request.AdditionalPackages, "duration", time.Since(startedAt), "error", err)
		return fmt.Errorf("additional packages install: %w", err)
	}
	if result.ExitCode != 0 {
		slog.Default().Error("sandbox additional packages install exited non-zero", "sandbox_id", sess.ID(), "run_id", request.RunID, "packages", request.AdditionalPackages, "exit_code", result.ExitCode, "stderr", result.Stderr, "duration", time.Since(startedAt))
		return fmt.Errorf("additional packages install failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	slog.Default().Info("sandbox additional packages installed", "sandbox_id", sess.ID(), "run_id", request.RunID, "packages", request.AdditionalPackages, "duration", time.Since(startedAt))
	return nil
}

type clientSession struct {
	api           *apiClient
	record        sandboxRecord
	processClient processconnect.ProcessClient
	filesClient   filesystemconnect.FilesystemClient
}
