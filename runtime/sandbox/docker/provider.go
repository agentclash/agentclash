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
	"maps"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/agentclash/agentclash/runtime/maputil"
	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
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
	if len(request.AdditionalPackages) > 0 && !request.ToolPolicy.AllowNetwork {
		return nil, fmt.Errorf("additional packages require network access, but the tool policy disables it (apt-get cannot reach mirrors with NetworkMode=none)")
	}
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
	labels := make(map[string]string, len(request.Labels)+3)
	maps.Copy(labels, request.Labels)
	// Reserved labels last so caller labels cannot mask managed containers.
	labels[labelManagedBy] = labelManagedByValue
	labels[labelRunID] = request.RunID.String()
	labels[labelRunAgentID] = request.RunAgentID.String()

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
		// Entrypoint (not Cmd) so images with their own ENTRYPOINT don't turn
		// this into `<entrypoint> sleep infinity` and exit immediately.
		Entrypoint: []string{"sleep", "infinity"},
		Tty:        false,
	}

	name := containerName(request.RunAgentID)
	id, err := p.createContainer(ctx, cfg, hostCfg, name)
	if err != nil {
		if p.config.pullMissing() && isImageMissing(err) {
			if pullErr := p.engine.ImagePull(ctx, imageRef); pullErr != nil {
				slog.Default().Error("docker sandbox image pull failed", "image", imageRef, "error", pullErr, "duration", time.Since(startedAt))
				return nil, pullErr
			}
			// Retry after pull; reassigns both id and err.
			id, err = p.createContainer(ctx, cfg, hostCfg, name)
		}
		// err is either the original non-image-missing error, or the post-pull retry error.
		if err != nil {
			slog.Default().Error("docker sandbox create failed", "image", imageRef, "error", err, "duration", time.Since(startedAt))
			return nil, err
		}
	}

	if err := p.engine.ContainerStart(ctx, id); err != nil {
		// Cleanup must survive a canceled request context or the container leaks.
		_ = p.engine.ContainerRemove(context.WithoutCancel(ctx), id, true)
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
		maxOutputBytes:     p.config.maxExecOutputBytes(),
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

// createContainer creates the container, recovering from a name conflict with
// a stale container leaked by a crashed prior attempt (Temporal retries reuse
// the same RunAgentID, hence the same deterministic name).
func (p *Provider) createContainer(ctx context.Context, cfg container.Config, hostCfg container.HostConfig, name string) (string, error) {
	id, err := p.engine.ContainerCreate(ctx, cfg, hostCfg, name)
	if err == nil || !isNameConflict(err) {
		return id, err
	}
	labels, inspectErr := p.engine.ContainerInspectLabels(ctx, name)
	if inspectErr != nil || labels[labelManagedBy] != labelManagedByValue {
		// Not ours (or can't prove it is) — don't remove someone else's container.
		return "", err
	}
	if rmErr := p.engine.ContainerRemove(ctx, name, true); rmErr != nil && !isNotFoundErr(rmErr) {
		return "", fmt.Errorf("remove stale sandbox container %s: %w", name, rmErr)
	}
	slog.Default().Warn("docker sandbox removed stale container from prior attempt", "container_name", name)
	return p.engine.ContainerCreate(ctx, cfg, hostCfg, name)
}

func isNameConflict(err error) bool {
	if err == nil {
		return false
	}
	if errdefs.IsConflict(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "is already in use")
}

func (p *Provider) installAdditionalPackages(ctx context.Context, sess *session, request sandbox.CreateRequest) error {
	packages, err := validateDebianPackageNames(request.AdditionalPackages)
	if err != nil {
		return err
	}

	// Provider infrastructure: argv form only (no shell) so package names cannot inject.
	update, err := sess.execInternal(ctx, sandbox.ExecRequest{
		Command:     []string{"apt-get", "update"},
		Environment: map[string]string{"DEBIAN_FRONTEND": "noninteractive"},
		Timeout:     packageInstallTimeout,
	})
	if err != nil {
		return fmt.Errorf("additional packages apt-get update: %w", err)
	}
	if update.ExitCode != 0 {
		return fmt.Errorf("additional packages apt-get update failed: exit=%d stderr=%s", update.ExitCode, update.Stderr)
	}

	installCmd := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, packages...)
	result, err := sess.execInternal(ctx, sandbox.ExecRequest{
		Command:     installCmd,
		Environment: map[string]string{"DEBIAN_FRONTEND": "noninteractive"},
		Timeout:     packageInstallTimeout,
	})
	if err != nil {
		return fmt.Errorf("additional packages install: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("additional packages install failed: exit=%d stderr=%s", result.ExitCode, result.Stderr)
	}
	return nil
}

// debianPackageNamePattern matches Debian package names (see Debian Policy §5.6.1).
var debianPackageNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9+.\-]*$`)

func validateDebianPackageNames(packages []string) ([]string, error) {
	if len(packages) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(packages))
	for _, raw := range packages {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, fmt.Errorf("additional package name is empty")
		}
		if !debianPackageNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid additional package name %q", raw)
		}
		out = append(out, name)
	}
	return out, nil
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
		(strings.Contains(msg, "not found") && strings.Contains(msg, "image")) ||
		strings.Contains(msg, "repository does not exist")
}
