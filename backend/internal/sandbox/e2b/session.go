package e2b

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	filesystempb "github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/filesystem"
	processpb "github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/process"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

type session struct {
	mu     sync.Mutex
	client clientSession
	closed bool
}

func (s *session) ID() string {
	return s.client.record.SandboxID
}

func (s *session) UploadFile(ctx context.Context, path string, content []byte) error {
	return s.WriteFile(ctx, path, content)
}

func (s *session) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureActive(); err != nil {
		return nil, err
	}
	return s.client.api.readFile(ctx, s.client.record, path)
}

func (s *session) WriteFile(ctx context.Context, path string, content []byte) error {
	if err := s.ensureActive(); err != nil {
		return err
	}
	return s.client.api.writeFile(ctx, s.client.record, path, content)
}

func (s *session) ListFiles(ctx context.Context, prefix string) ([]sandbox.FileInfo, error) {
	if err := s.ensureActive(); err != nil {
		return nil, err
	}
	req := connect.NewRequest(&filesystempb.ListDirRequest{
		Path:  prefix,
		Depth: 32,
	})
	req.Header().Set("Authorization", s.client.api.authHeader())
	s.client.api.setEnvdHeaders(req.Header(), s.client.record)
	resp, err := s.client.filesClient.ListDir(ctx, req)
	if err != nil {
		return nil, normalizeRPCError(err)
	}
	items := make([]sandbox.FileInfo, 0, len(resp.Msg.Entries))
	for _, entry := range resp.Msg.Entries {
		if entry.GetType() != filesystempb.FileType_FILE_TYPE_FILE {
			continue
		}
		items = append(items, sandbox.FileInfo{
			Path: entry.GetPath(),
			Size: entry.GetSize(),
		})
	}
	return items, nil
}

func (s *session) Exec(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	if err := s.ensureActive(); err != nil {
		return sandbox.ExecResult{}, err
	}
	execCtx := ctx
	cancel := func() {}
	if request.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, request.Timeout)
	}
	defer cancel()

	stdin := false
	req := connect.NewRequest(&processpb.StartRequest{
		Process: &processpb.ProcessConfig{
			Cmd:  request.Command[0],
			Args: request.Command[1:],
			Envs: request.Environment,
			Cwd:  stringPtr(request.WorkingDirectory),
		},
		Stdin: &stdin,
	})
	req.Header().Set("Authorization", s.client.api.authHeader())
	req.Header().Set("Keepalive-Ping-Interval", "50")
	s.client.api.setEnvdHeaders(req.Header(), s.client.record)

	stream, err := s.client.processClient.Start(execCtx, req)
	if err != nil {
		return sandbox.ExecResult{}, normalizeRPCError(err)
	}
	defer stream.Close()

	result := sandbox.ExecResult{Metadata: map[string]string{}}
	var stdout strings.Builder
	var stderr strings.Builder
	for stream.Receive() {
		event := stream.Msg().GetEvent().GetEvent()
		switch e := event.(type) {
		case *processpb.ProcessEvent_Data:
			data := e.Data.GetOutput()
			switch out := data.(type) {
			case *processpb.ProcessEvent_DataEvent_Stdout:
				_, _ = stdout.Write(out.Stdout)
			case *processpb.ProcessEvent_DataEvent_Stderr:
				_, _ = stderr.Write(out.Stderr)
			}
		case *processpb.ProcessEvent_End:
			result.ExitCode = int(e.End.GetExitCode())
			if errorMessage := e.End.GetError(); errorMessage != "" {
				result.Metadata["error"] = errorMessage
			}
		}
	}
	if err := stream.Err(); err != nil {
		return sandbox.ExecResult{}, normalizeRPCError(err)
	}
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	return result, nil
}

func (s *session) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	return s.ReadFile(ctx, path)
}

func (s *session) Destroy(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	startedAt := time.Now()
	err := s.client.api.destroySandbox(ctx, s.client.record.SandboxID)
	if err != nil && !errors.Is(err, sandbox.ErrSandboxNotFound) {
		slog.Default().Error("sandbox destroy failed", "sandbox_id", s.client.record.SandboxID, "template_id", s.client.record.TemplateID, "sandbox_url", s.client.api.envdBaseURL(s.client.record), "outcome", "failed_destroy", "duration", time.Since(startedAt), "error", err)
		return err
	}
	slog.Default().Info("sandbox destroyed", "sandbox_id", s.client.record.SandboxID, "template_id", s.client.record.TemplateID, "sandbox_url", s.client.api.envdBaseURL(s.client.record), "outcome", "destroyed", "duration", time.Since(startedAt))
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

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
