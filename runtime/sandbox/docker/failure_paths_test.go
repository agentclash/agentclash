package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/google/uuid"
)

func createTestSession(t *testing.T, eng *fakeEngine, cfg Config, req sandbox.CreateRequest) sandbox.Session {
	t.Helper()
	if req.RunID == uuid.Nil {
		req.RunID = uuid.New()
	}
	if req.RunAgentID == uuid.Nil {
		req.RunAgentID = uuid.New()
	}
	provider := NewProviderWithEngine(eng, cfg)
	session, err := provider.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return session
}

func TestProviderCreateStartFailureRemovesContainer(t *testing.T) {
	eng := newFakeEngine()
	eng.startErr = errors.New("start failed")
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{RunID: uuid.New(), RunAgentID: uuid.New()})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(eng.removed) != 1 {
		t.Fatalf("removed = %d, want 1 (container leaked on start failure)", len(eng.removed))
	}
}

func TestSessionDestroyStopFailureForceRemoves(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{})
	eng.stopErr = errors.New("stop failed")
	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy after failed stop with successful force-remove = %v, want nil", err)
	}
	if len(eng.removed) != 1 {
		t.Fatalf("removed = %d, want 1", len(eng.removed))
	}
}

func TestSessionDestroyRetriesAfterCleanupFailure(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{})
	eng.stopErr = errors.New("stop failed")
	eng.removeErr = errors.New("remove failed")
	if err := session.Destroy(context.Background()); err == nil {
		t.Fatal("expected Destroy error when stop and remove both fail")
	}
	// A failed Destroy must not latch the session closed: the retry has to
	// reach the daemon again, or the container leaks forever.
	eng.mu.Lock()
	eng.stopErr = nil
	eng.removeErr = nil
	eng.mu.Unlock()
	if err := session.Destroy(context.Background()); err != nil {
		t.Fatalf("retried Destroy = %v, want nil", err)
	}
	if len(eng.removed) != 1 {
		t.Fatalf("removed = %d, want 1 (retry never removed the container)", len(eng.removed))
	}
}

func TestSessionDestroySurvivesCanceledContext(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Destroy(ctx); err != nil {
		t.Fatalf("Destroy with canceled ctx = %v, want nil (cleanup must use a detached context)", err)
	}
	if len(eng.removed) != 1 {
		t.Fatalf("removed = %d, want 1", len(eng.removed))
	}
}

func TestProviderCreatePullFailurePropagates(t *testing.T) {
	eng := newFakeEngine()
	eng.createFailOnce = errors.New("No such image: python:3.12-slim")
	eng.pullErr = errors.New("pull access denied")
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{RunID: uuid.New(), RunAgentID: uuid.New()})
	if err == nil || !strings.Contains(err.Error(), "pull access denied") {
		t.Fatalf("Create error = %v, want pull error", err)
	}
	if len(eng.started) != 0 {
		t.Fatalf("started = %d, want 0", len(eng.started))
	}
}

func TestProviderCreateNameConflictRemovesStaleManagedContainer(t *testing.T) {
	eng := newFakeEngine()
	runAgentID := uuid.New()
	name := containerName(runAgentID)
	eng.createFailOnce = fmt.Errorf("Conflict. The container name %q is already in use", name)
	eng.inspectLabels[name] = map[string]string{labelManagedBy: labelManagedByValue}
	provider := NewProviderWithEngine(eng, Config{})
	session, err := provider.Create(context.Background(), sandbox.CreateRequest{RunID: uuid.New(), RunAgentID: runAgentID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if !containsString(eng.removed, name) {
		t.Fatalf("removed = %#v, want stale container %q removed", eng.removed, name)
	}
}

func TestProviderCreateNameConflictLeavesUnmanagedContainer(t *testing.T) {
	eng := newFakeEngine()
	runAgentID := uuid.New()
	name := containerName(runAgentID)
	eng.createFailOnce = fmt.Errorf("Conflict. The container name %q is already in use", name)
	eng.inspectLabels[name] = map[string]string{"someone": "else"}
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{RunID: uuid.New(), RunAgentID: runAgentID})
	if err == nil {
		t.Fatal("expected conflict error for unmanaged container")
	}
	if containsString(eng.removed, name) {
		t.Fatalf("unmanaged container %q was removed", name)
	}
}

func TestProviderCreateRejectsPackagesWithoutNetwork(t *testing.T) {
	eng := newFakeEngine()
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:              uuid.New(),
		RunAgentID:         uuid.New(),
		ToolPolicy:         sandbox.ToolPolicy{AllowNetwork: false},
		AdditionalPackages: []string{"curl"},
	})
	if err == nil || !strings.Contains(err.Error(), "network") {
		t.Fatalf("Create error = %v, want network requirement error", err)
	}
	if len(eng.created) != 0 {
		t.Fatalf("created = %d, want 0 (validation should run before create)", len(eng.created))
	}
}

func TestInstallAdditionalPackagesFailureDestroysSession(t *testing.T) {
	eng := newFakeEngine()
	eng.aptGetExitCode = 100
	eng.aptGetStderr = "E: Unable to locate package nope"
	provider := NewProviderWithEngine(eng, Config{})
	_, err := provider.Create(context.Background(), sandbox.CreateRequest{
		RunID:              uuid.New(),
		RunAgentID:         uuid.New(),
		ToolPolicy:         sandbox.ToolPolicy{AllowNetwork: true},
		AdditionalPackages: []string{"nope"},
	})
	if err == nil || !strings.Contains(err.Error(), "apt-get") {
		t.Fatalf("Create error = %v, want apt-get failure", err)
	}
	if len(eng.removed) != 1 {
		t.Fatalf("removed = %d, want 1 (session must be destroyed on install failure)", len(eng.removed))
	}
}

func TestIsImageMissing(t *testing.T) {
	cases := map[string]bool{
		"Error: No such image: python:3.12-slim":         true,
		"image not found":                                true,
		"repository does not exist or may require login": true,
		"network not-found-but-unrelated":                false,
		"Conflict. The container name is already in use": false,
		"cannot connect to the docker daemon":            false,
	}
	for msg, want := range cases {
		if got := isImageMissing(errors.New(msg)); got != want {
			t.Errorf("isImageMissing(%q) = %v, want %v", msg, got, want)
		}
	}
	if isImageMissing(nil) {
		t.Error("isImageMissing(nil) = true")
	}
}

func TestIsShellCommand(t *testing.T) {
	cases := map[string]bool{
		"sh":            true,
		"bash":          true,
		"ash":           true,
		"zsh":           true,
		"dash":          true,
		"/bin/sh":       true,
		"/usr/bin/bash": true,
		"python3":       false,
		"echo":          false,
		"timeout":       false,
	}
	for cmd, want := range cases {
		if got := isShellCommand([]string{cmd}); got != want {
			t.Errorf("isShellCommand(%q) = %v, want %v", cmd, got, want)
		}
	}
	if isShellCommand(nil) {
		t.Error("isShellCommand(nil) = true")
	}
}

func TestExecTimeoutWrapsCommandAndMapsTimeoutExit(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{ToolPolicy: sandbox.ToolPolicy{AllowShell: true}})

	// In-container `timeout` exits 124 when the command exceeds the deadline.
	code := timeoutExitCode
	eng.mu.Lock()
	eng.forceExitCode = &code
	eng.mu.Unlock()
	_, err := session.Exec(context.Background(), sandbox.ExecRequest{
		Command: []string{"echo", "slow"},
		Timeout: time.Second,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Exec error = %v, want context.DeadlineExceeded", err)
	}
	last := eng.lastExec()
	if len(last.Cmd) < 5 || last.Cmd[0] != "timeout" {
		t.Fatalf("exec cmd = %#v, want in-container timeout wrapper", last.Cmd)
	}
	if last.Cmd[4] != "echo" {
		t.Fatalf("wrapped cmd = %#v, want original command after wrapper", last.Cmd)
	}
}

func TestExecWithoutTimeoutIsNotWrapped(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{ToolPolicy: sandbox.ToolPolicy{AllowShell: true}})
	if _, err := session.Exec(context.Background(), sandbox.ExecRequest{Command: []string{"echo", "hi"}}); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if last := eng.lastExec(); last.Cmd[0] != "echo" {
		t.Fatalf("exec cmd = %#v, want unwrapped command", last.Cmd)
	}
}

func TestExecStreamsCallbacks(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{ToolPolicy: sandbox.ToolPolicy{AllowShell: true}})
	var streamed bytes.Buffer
	result, err := session.Exec(context.Background(), sandbox.ExecRequest{
		Command: []string{"echo", "hello"},
		OnStdout: func(chunk []byte) error {
			streamed.Write(chunk)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if streamed.String() != result.Stdout {
		t.Fatalf("streamed = %q, result = %q", streamed.String(), result.Stdout)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestExecCallbackErrorAborts(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{}, sandbox.CreateRequest{ToolPolicy: sandbox.ToolPolicy{AllowShell: true}})
	wantErr := errors.New("consumer failed")
	_, err := session.Exec(context.Background(), sandbox.ExecRequest{
		Command:  []string{"echo", "hello"},
		OnStdout: func([]byte) error { return wantErr },
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Exec error = %v, want callback error", err)
	}
}

func TestExecOutputTruncatedAtCap(t *testing.T) {
	eng := newFakeEngine()
	session := createTestSession(t, eng, Config{MaxExecOutputBytes: 4}, sandbox.CreateRequest{ToolPolicy: sandbox.ToolPolicy{AllowShell: true}})
	var streamed bytes.Buffer
	result, err := session.Exec(context.Background(), sandbox.ExecRequest{
		Command: []string{"echo", "0123456789"},
		OnStdout: func(chunk []byte) error {
			streamed.Write(chunk)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(result.Stdout) != 4 {
		t.Fatalf("stdout len = %d, want capped at 4", len(result.Stdout))
	}
	if result.Metadata["stdout_truncated"] != "true" {
		t.Fatalf("metadata = %#v, want stdout_truncated", result.Metadata)
	}
	// Streaming callbacks still receive the full output.
	if !strings.Contains(streamed.String(), "0123456789") {
		t.Fatalf("streamed = %q, want full output", streamed.String())
	}
}

func TestReadTarFileRejectsSymlink(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "result.json",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	_, err := readTarFile(&buf)
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("readTarFile(symlink) = %v, want not-a-regular-file error", err)
	}
}

func TestConsumePullStreamSurfacesErrors(t *testing.T) {
	stream := strings.NewReader(
		`{"status":"Pulling from library/python"}` + "\n" +
			`{"error":"pull access denied for private/image","errorDetail":{"message":"pull access denied for private/image"}}` + "\n")
	err := consumePullStream(stream)
	if err == nil || !strings.Contains(err.Error(), "pull access denied") {
		t.Fatalf("consumePullStream = %v, want in-stream error surfaced", err)
	}
	if err := consumePullStream(strings.NewReader(`{"status":"ok"}` + "\n")); err != nil {
		t.Fatalf("consumePullStream(clean) = %v", err)
	}
}
