package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agentclash/agentclash/runtime/maputil"
	"github.com/agentclash/agentclash/runtime/sandbox"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
)

type session struct {
	mu                 sync.Mutex
	engine             engine
	id                 string
	closed             bool
	allowShell         bool
	workingDirectory   string
	defaultEnvironment map[string]string
	stopTimeout        time.Duration
}

func (s *session) ID() string {
	return s.id
}

func (s *session) UploadFile(ctx context.Context, filePath string, content []byte) error {
	return s.WriteFile(ctx, filePath, content)
}

func (s *session) DownloadFile(ctx context.Context, filePath string) ([]byte, error) {
	return s.ReadFile(ctx, filePath)
}

func (s *session) WriteFile(ctx context.Context, filePath string, content []byte) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	cleaned := normalizeAbsPath(filePath)
	dir := path.Dir(cleaned)
	if err := s.mkdirp(ctx, dir); err != nil {
		return err
	}
	archive, err := writeTarFile(cleaned, content)
	if err != nil {
		return fmt.Errorf("build tar for %s: %w", cleaned, err)
	}
	if err := s.engine.CopyToContainer(ctx, s.id, dir, archive); err != nil {
		return err
	}
	return nil
}

func (s *session) ReadFile(ctx context.Context, filePath string) ([]byte, error) {
	if err := s.ensureActive(); err != nil {
		return nil, err
	}
	cleaned := normalizeAbsPath(filePath)
	reader, err := s.engine.CopyFromContainer(ctx, s.id, cleaned)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, sandbox.ErrFileNotFound
		}
		return nil, fmt.Errorf("copy from container %s:%s: %w", s.id, cleaned, err)
	}
	defer reader.Close()
	content, err := readTarFile(reader)
	if err != nil {
		if errors.Is(err, io.EOF) || isNotFoundErr(err) {
			return nil, sandbox.ErrFileNotFound
		}
		return nil, fmt.Errorf("read tar from %s: %w", cleaned, err)
	}
	return content, nil
}

func (s *session) ListFiles(ctx context.Context, prefix string) ([]sandbox.FileInfo, error) {
	if err := s.ensureActive(); err != nil {
		return nil, err
	}
	search := strings.TrimSpace(prefix)
	if search == "" {
		search = s.workingDirectory
	}
	search = normalizeAbsPath(search)

	result, err := s.execInternal(ctx, sandbox.ExecRequest{
		Command: []string{"find", search, "-type", "f", "-printf", "%p\t%s\n"},
	})
	if err != nil {
		return nil, err
	}
	if result.ExitCode != 0 {
		if strings.Contains(result.Stderr, "No such file or directory") {
			return nil, sandbox.ErrFileNotFound
		}
		return nil, fmt.Errorf("find exited with code %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return []sandbox.FileInfo{}, nil
	}
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	items := make([]sandbox.FileInfo, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected find output line %q", line)
		}
		size, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse listed file size for %q: %w", parts[0], err)
		}
		items = append(items, sandbox.FileInfo{
			Path: strings.TrimSpace(parts[0]),
			Size: size,
		})
	}
	return items, nil
}

func (s *session) Exec(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	if err := s.ensureActive(); err != nil {
		return sandbox.ExecResult{}, err
	}
	if len(request.Command) == 0 {
		return sandbox.ExecResult{}, fmt.Errorf("exec command is required")
	}
	if !s.allowShell && isShellCommand(request.Command) {
		return sandbox.ExecResult{}, sandbox.ErrShellNotAllowed
	}
	return s.execInternal(ctx, request)
}

func (s *session) execInternal(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	execCtx := ctx
	cancel := func() {}
	if request.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, request.Timeout)
	}
	defer cancel()

	workingDir := strings.TrimSpace(request.WorkingDirectory)
	if workingDir == "" {
		workingDir = s.workingDirectory
	}

	env := mergeEnvironment(s.defaultEnvironment, request.Environment)
	execID, err := s.engine.ContainerExecCreate(execCtx, s.id, container.ExecOptions{
		Cmd:          request.Command,
		Env:          envSlice(env),
		WorkingDir:   workingDir,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return sandbox.ExecResult{}, err
	}

	attach, err := s.engine.ContainerExecAttach(execCtx, execID)
	if err != nil {
		return sandbox.ExecResult{}, err
	}
	defer attach.Close()

	stdout, stderr, demuxErr := demuxDockerOutput(attach.Reader())
	if demuxErr != nil && !errors.Is(demuxErr, context.Canceled) && !errors.Is(demuxErr, context.DeadlineExceeded) {
		return sandbox.ExecResult{}, fmt.Errorf("demux exec output: %w", demuxErr)
	}

	inspect, err := s.engine.ContainerExecInspect(execCtx, execID)
	if err != nil {
		return sandbox.ExecResult{}, err
	}

	result := sandbox.ExecResult{
		ExitCode: inspect.ExitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Metadata: map[string]string{},
	}
	if request.OnStdout != nil && stdout != "" {
		if err := request.OnStdout([]byte(stdout)); err != nil {
			return sandbox.ExecResult{}, err
		}
	}
	if request.OnStderr != nil && stderr != "" {
		if err := request.OnStderr([]byte(stderr)); err != nil {
			return sandbox.ExecResult{}, err
		}
	}
	return result, nil
}

func (s *session) Destroy(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	timeout := s.stopTimeout
	if err := s.engine.ContainerStop(ctx, s.id, &timeout); err != nil && !isNotFoundErr(err) {
		_ = s.engine.ContainerRemove(ctx, s.id, true)
		return err
	}
	if err := s.engine.ContainerRemove(ctx, s.id, true); err != nil && !isNotFoundErr(err) {
		return err
	}
	return nil
}

func (s *session) ensureWorkingDirectory(ctx context.Context) error {
	return s.mkdirp(ctx, s.workingDirectory)
}

func (s *session) mkdirp(ctx context.Context, dir string) error {
	dir = normalizeAbsPath(dir)
	if dir == "/" {
		return nil
	}
	result, err := s.execInternal(ctx, sandbox.ExecRequest{
		Command: []string{"mkdir", "-p", dir},
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("mkdir %s failed: exit=%d stderr=%s", dir, result.ExitCode, result.Stderr)
	}
	return nil
}

func (s *session) ensureActive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return sandbox.ErrSessionDestroyed
	}
	return nil
}

func normalizeAbsPath(raw string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func mergeEnvironment(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := maputil.CloneStringMap(base)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func isShellCommand(command []string) bool {
	if len(command) == 0 {
		return false
	}
	cmd := path.Base(strings.TrimSpace(command[0]))
	switch cmd {
	case "sh", "bash", "ash", "zsh", "dash":
		return true
	default:
		return false
	}
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errdefs.IsNotFound(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "could not find") ||
		strings.Contains(msg, "no such container")
}
