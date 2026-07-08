// Package docker implements runtime/sandbox.Provider using a local Docker Engine.
//
// Unit tests use a fake engine and do not require a daemon. Optional live smoke:
//
//	AGENTCLASH_DOCKER_SMOKE=1 go test -tags dockersmoke -count=1 ./sandbox/docker -run TestDockerSmokeLifecycle
package docker

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/maputil"
	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
)

// Provider creates Docker-backed sandbox sessions.
type Provider struct {
	engine engine
	config Config
	owns   bool
}

// NewProvider connects to the local Docker daemon via DOCKER_* env / default socket.
func NewProvider(config Config) (*Provider, error) {
	eng, err := newDockerEngine()
	if err != nil {
		return nil, err
	}
	return &Provider{engine: eng, config: config, owns: true}, nil
}

// NewProviderWithEngine is for tests that inject a fake Docker engine.
func NewProviderWithEngine(eng engine, config Config) *Provider {
	return &Provider{engine: eng, config: config, owns: false}
}

// Close releases the underlying Docker client when this provider owns it.
func (p *Provider) Close() error {
	if p == nil || !p.owns || p.engine == nil {
		return nil
	}
	return p.engine.Close()
}

func (p *Provider) Create(ctx context.Context, request sandbox.CreateRequest) (sandbox.Session, error) {
	startedAt := time.Now()
	if err := p.engine.Ping(ctx); err != nil {
		return nil, err
	}

	imageRef := p.config.image()
	if strings.TrimSpace(request.TemplateID) != "" {
		imageRef = strings.TrimSpace(request.TemplateID)
	}

	workingDir := strings.TrimSpace(request.Filesystem.WorkingDirectory)
	if workingDir == "" {
		workingDir = defaultWorkingDirectory
	}
	workingDir = path.Clean(workingDir)

	env := envSlice(request.EnvVars)
	labels := map[string]string{
		labelManagedBy:  labelManagedByValue,
		labelRunID:      request.RunID.String(),
		labelRunAgentID: request.RunAgentID.String(),
	}
	for key, value := range request.Labels {
		labels[key] = value
	}

	hostCfg := container.HostConfig{
		AutoRemove: false,
	}
	if !request.ToolPolicy.AllowNetwork {
		hostCfg.NetworkMode = "none"
	}

	cfg := container.Config{
		Image:      imageRef,
		Env:        env,
		WorkingDir: workingDir,
		Labels:     labels,
		Cmd:        []string{"sleep", "infinity"},
		Tty:        false,
	}

	name := containerName(request.RunAgentID)
	id, err := p.engine.ContainerCreate(ctx, cfg, hostCfg, name)
	if err != nil {
		if p.config.pullMissing() && isImageMissing(err) {
			if pullErr := p.engine.ImagePull(ctx, imageRef); pullErr != nil {
				slog.Default().Error("docker sandbox image pull failed", "image", imageRef, "error", pullErr, "duration", time.Since(startedAt))
				return nil, pullErr
			}
			id, err = p.engine.ContainerCreate(ctx, cfg, hostCfg, name)
		}
		if err != nil {
			slog.Default().Error("docker sandbox create failed", "image", imageRef, "error", err, "duration", time.Since(startedAt))
			return nil, err
		}
	}

	if err := p.engine.ContainerStart(ctx, id); err != nil {
		_ = p.engine.ContainerRemove(ctx, id, true)
		slog.Default().Error("docker sandbox start failed", "container_id", id, "error", err, "duration", time.Since(startedAt))
		return nil, err
	}

	sess := &session{
		engine:             p.engine,
		id:                 id,
		allowShell:         request.ToolPolicy.AllowShell,
		workingDirectory:   workingDir,
		defaultEnvironment: maputil.CloneStringMap(request.EnvVars),
		stopTimeout:        p.config.stopTimeout(),
	}

	if err := sess.ensureWorkingDirectory(ctx); err != nil {
		_ = sess.Destroy(ctx)
		return nil, err
	}

	if len(request.AdditionalPackages) > 0 {
		if err := p.installAdditionalPackages(ctx, sess, request); err != nil {
			_ = sess.Destroy(ctx)
			return nil, err
		}
	}

	slog.Default().Info("docker sandbox created", "container_id", id, "image", imageRef, "run_id", request.RunID, "run_agent_id", request.RunAgentID, "duration", time.Since(startedAt))
	return sess, nil
}

func (p *Provider) installAdditionalPackages(ctx context.Context, sess *session, request sandbox.CreateRequest) error {
	installCmd := "apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends " + strings.Join(request.AdditionalPackages, " ")
	// Bypass AllowShell: package install is provider infrastructure, not agent shell.
	result, err := sess.execInternal(ctx, sandbox.ExecRequest{
		Command: []string{"sh", "-c", installCmd},
		Timeout: 120 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("additional packages install: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("additional packages install failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	return nil
}

func containerName(runAgentID uuid.UUID) string {
	id := runAgentID.String()
	if id == uuid.Nil.String() {
		id = uuid.New().String()
	}
	// Docker names: [a-zA-Z0-9][a-zA-Z0-9_.-]+
	return "agentclash-" + strings.ReplaceAll(id, "-", "")
}

func envSlice(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}

func isImageMissing(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such image") ||
		strings.Contains(msg, "not found") && strings.Contains(msg, "image") ||
		strings.Contains(msg, "repository does not exist")
}
