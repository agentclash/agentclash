package docker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/google/uuid"
)

func TestProviderCreateUsesDefaultsAndNetworkNone(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})

	session, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		EnvVars:    map[string]string{"FOO": "bar"},
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true, AllowNetwork: false},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer session.Destroy(context.Background())

	created := eng.lastCreate()
	if created.cfg.Image != defaultImage {
		t.Fatalf("image = %q, want %q", created.cfg.Image, defaultImage)
	}
	if created.cfg.WorkingDir != defaultWorkingDirectory {
		t.Fatalf("working dir = %q, want %q", created.cfg.WorkingDir, defaultWorkingDirectory)
	}
	if created.hostCfg.NetworkMode != "none" {
		t.Fatalf("network mode = %q, want none", created.hostCfg.NetworkMode)
	}
	if !containsEnv(created.cfg.Env, "FOO=bar") {
		t.Fatalf("env = %#v, want FOO=bar", created.cfg.Env)
	}
	if created.cfg.Labels[labelManagedBy] != labelManagedByValue {
		t.Fatalf("managed-by label missing: %#v", created.cfg.Labels)
	}
	if len(eng.started) != 1 {
		t.Fatalf("started = %d, want 1", len(eng.started))
	}
}

func TestProviderCreateAllowsNetworkWhenRequested(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true, AllowNetwork: true},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if eng.lastCreate().hostCfg.NetworkMode != "" {
		t.Fatalf("network mode = %q, want empty default", eng.lastCreate().hostCfg.NetworkMode)
	}
}

func TestProviderCreateUsesTemplateIDAsImage(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{Image: "ignored:latest"})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		TemplateID: "custom:tag",
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if eng.lastCreate().cfg.Image != "custom:tag" {
		t.Fatalf("image = %q, want custom:tag", eng.lastCreate().cfg.Image)
	}
}

func TestProviderCreatePullsMissingImage(t *testing.T) {
	eng := newFakeEngine()
	eng.createFailOnce = errors.New("no such image: missing:latest")
	provider := NewProviderWithEngine(eng, Config{})

	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		TemplateID: "missing:latest",
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(eng.pulled) != 1 || eng.pulled[0] != "missing:latest" {
		t.Fatalf("pulled = %#v", eng.pulled)
	}
	if len(eng.created) != 1 {
		t.Fatalf("created = %d, want 1 after retry", len(eng.created))
	}
}

func TestProviderCreateMapsDaemonUnavailable(t *testing.T) {
	eng := newFakeEngine()
	eng.pingErr = wrapDockerUnavailable(errors.New("cannot connect to the Docker daemon"))
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
	})
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("error = %v, want ErrDockerUnavailable", err)
	}
}

func TestSessionFileLifecycle(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})
	session, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
		Filesystem: sandbox.FilesystemSpec{WorkingDirectory: "/workspace"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := session.UploadFile(context.Background(), "/workspace/input.txt", []byte("hello")); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if err := session.WriteFile(context.Background(), "/workspace/out.txt", []byte("world")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	content, err := session.ReadFile(context.Background(), "/workspace/input.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("content = %q", content)
	}
	downloaded, err := session.DownloadFile(context.Background(), "/workspace/out.txt")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(downloaded) != "world" {
		t.Fatalf("downloaded = %q", downloaded)
	}
	files, err := session.ListFiles(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListFiles count = %d, want 2 (%#v)", len(files), files)
	}
}

func TestSessionExecMergesEnvAndRejectsShell(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})
	session, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		EnvVars:    map[string]string{"BASE": "1"},
		ToolPolicy: sandbox.ToolPolicy{AllowShell: false},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = session.Exec(context.Background(), sandbox.ExecRequest{Command: []string{"bash", "-lc", "echo hi"}})
	if !errors.Is(err, sandbox.ErrShellNotAllowed) {
		t.Fatalf("shell exec error = %v, want ErrShellNotAllowed", err)
	}

	result, err := session.Exec(context.Background(), sandbox.ExecRequest{
		Command:     []string{"echo", "ok"},
		Environment: map[string]string{"EXTRA": "2"},
		Timeout:     time.Second,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "ok" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
	last := eng.lastExec()
	if !containsEnv(last.Env, "BASE=1") || !containsEnv(last.Env, "EXTRA=2") {
		t.Fatalf("exec env = %#v", last.Env)
	}
}

func TestSessionRejectsAfterDestroy(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})
	session, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:      uuid.New(),
		RunAgentID: uuid.New(),
		ToolPolicy: sandbox.ToolPolicy{AllowShell: true},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("second Destroy: %v", err)
	}
	if err := session.WriteFile(context.Background(), "/workspace/x", []byte("x")); !errors.Is(err, sandbox.ErrSessionDestroyed) {
		t.Fatalf("WriteFile after destroy = %v", err)
	}
	if len(eng.stopped) != 1 || len(eng.removed) != 1 {
		t.Fatalf("stop/remove = %d/%d", len(eng.stopped), len(eng.removed))
	}
}

func TestIsDaemonUnavailable(t *testing.T) {
	if !isDaemonUnavailable(errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?")) {
		t.Fatal("expected daemon unavailable detection")
	}
	if isDaemonUnavailable(errors.New("image pull rate limited")) {
		t.Fatal("did not expect daemon unavailable")
	}
}

func containsEnv(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}
