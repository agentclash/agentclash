package docker

import (
	"bytes"
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
	"github.com/docker/docker/pkg/stdcopy"
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
	maxOutputBytes     int
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

// execTimeoutGrace is added to the client-side backstop deadline so the
// in-container `timeout` wrapper (which enforces the real limit) gets a
// chance to kill the process and report exit code 124 before the client
// gives up on a hung daemon.
const execTimeoutGrace = 5 * time.Second

// timeoutExitCode is what coreutils/busybox `timeout` exits with when the
// wrapped command exceeds its deadline.
const timeoutExitCode = 124

func (s *session) execInternal(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	command := request.Command
	execCtx := ctx
	cancel := func() {}
	if request.Timeout > 0 {
		// Docker has no exec-kill API and hijacked attach reads ignore context
		// cancellation, so the deadline is enforced in-container via `timeout`.
		// The client context is only a backstop for a hung daemon.
		command = append([]string{"timeout", "-k", "2", strconv.Itoa(ceilSeconds(request.Timeout))}, command...)
		execCtx, cancel = context.WithTimeout(ctx, request.Timeout+execTimeoutGrace)
	}
	defer cancel()

	workingDir := strings.TrimSpace(request.WorkingDirectory)
	if workingDir == "" {
		workingDir = s.workingDirectory
	}

	env := mergeEnvironment(s.defaultEnvironment, request.Environment)
	execID, err := s.engine.ContainerExecCreate(execCtx, s.id, container.ExecOptions{
		Cmd:          command,
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

	stdoutW := newCappedStreamWriter(s.maxOutputBytes, request.OnStdout)
	stderrW := newCappedStreamWriter(s.maxOutputBytes, request.OnStderr)

	// The hijacked reader is not interruptible by context; run the copy in a
	// goroutine and close the attach to unblock it if the backstop fires.
	demuxDone := make(chan error, 1)
	go func() {
		_, copyErr := stdcopy.StdCopy(stdoutW, stderrW, attach.Reader())
		if errors.Is(copyErr, io.EOF) {
			copyErr = nil
		}
		demuxDone <- copyErr
	}()

	timedOut := false
	var demuxErr error
	select {
	case demuxErr = <-demuxDone:
	case <-execCtx.Done():
		timedOut = true
		_ = attach.Close() // unblock the reader
		<-demuxDone        // read error from the closed conn is expected; discard
	}
	if demuxErr != nil && !timedOut {
		var cbErr *callbackError
		if errors.As(demuxErr, &cbErr) {
			return sandbox.ExecResult{}, cbErr.err
		}
		return sandbox.ExecResult{}, fmt.Errorf("demux exec output: %w", demuxErr)
	}

	result := sandbox.ExecResult{
		Stdout:   stdoutW.String(),
		Stderr:   stderrW.String(),
		Metadata: map[string]string{},
	}
	if stdoutW.truncated {
		result.Metadata["stdout_truncated"] = "true"
	}
	if stderrW.truncated {
		result.Metadata["stderr_truncated"] = "true"
	}
	if timedOut {
		return result, fmt.Errorf("exec timed out after %s: %w", request.Timeout, context.DeadlineExceeded)
	}

	// Inspect with a fresh context: the caller's context may already be
	// canceled/expired, and losing the exit code here discards the run output.
	inspectCtx, inspectCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer inspectCancel()
	inspect, err := s.engine.ContainerExecInspect(inspectCtx, execID)
	if err != nil {
		return sandbox.ExecResult{}, err
	}
	if request.Timeout > 0 && inspect.ExitCode == timeoutExitCode {
		result.ExitCode = inspect.ExitCode
		return result, fmt.Errorf("exec timed out after %s: %w", request.Timeout, context.DeadlineExceeded)
	}
	if inspect.Running {
		return sandbox.ExecResult{}, fmt.Errorf("exec %s still running after output stream ended", execID)
	}
	result.ExitCode = inspect.ExitCode
	return result, nil
}

func ceilSeconds(d time.Duration) int {
	seconds := int((d + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

// callbackError distinguishes an OnStdout/OnStderr abort from a transport error.
type callbackError struct{ err error }

func (e *callbackError) Error() string { return e.err.Error() }
func (e *callbackError) Unwrap() error { return e.err }

// cappedStreamWriter forwards chunks to the caller's streaming callback as
// they arrive and retains up to limit bytes for the final ExecResult.
type cappedStreamWriter struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
	onChunk   func([]byte) error
}

func newCappedStreamWriter(limit int, onChunk func([]byte) error) *cappedStreamWriter {
	return &cappedStreamWriter{limit: limit, onChunk: onChunk}
}

func (w *cappedStreamWriter) Write(p []byte) (int, error) {
	if w.onChunk != nil && len(p) > 0 {
		if err := w.onChunk(p); err != nil {
			return 0, &callbackError{err: err}
		}
	}
	if remaining := w.limit - w.buf.Len(); remaining > 0 {
		n := min(len(p), remaining)
		w.buf.Write(p[:n])
		if n < len(p) {
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	return len(p), nil
}

func (w *cappedStreamWriter) String() string { return w.buf.String() }

func (s *session) Destroy(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Cleanup must survive caller cancellation: Destroy typically runs during
	// workflow teardown, when the request context is already canceled.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.stopTimeout+30*time.Second)
	defer cancel()

	timeout := s.stopTimeout
	if err := s.engine.ContainerStop(cleanupCtx, s.id, &timeout); err != nil && !isNotFoundErr(err) {
		if rmErr := s.engine.ContainerRemove(cleanupCtx, s.id, true); rmErr != nil && !isNotFoundErr(rmErr) {
			// Session stays open so a retried Destroy attempts cleanup again.
			return err
		}
		s.markClosed()
		return nil
	}
	if err := s.engine.ContainerRemove(cleanupCtx, s.id, true); err != nil && !isNotFoundErr(err) {
		return err
	}
	s.markClosed()
	return nil
}

func (s *session) markClosed() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
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
