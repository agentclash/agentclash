package sandbox

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"sync"
)

type FakeProvider struct {
	mu             sync.Mutex
	CreateErr      error
	NextSession    *FakeSession
	CreateRequests []CreateRequest
	Sessions       []*FakeSession
}

func (p *FakeProvider) Create(_ context.Context, request CreateRequest) (Session, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.CreateErr != nil {
		return nil, p.CreateErr
	}

	session := p.NextSession
	if session == nil {
		session = NewFakeSession("fake-sandbox")
	}
	p.NextSession = nil
	session.attachCreateRequest(request)

	p.CreateRequests = append(p.CreateRequests, cloneCreateRequest(request))
	p.Sessions = append(p.Sessions, session)

	return session, nil
}

type FakeSession struct {
	mu            sync.Mutex
	id            string
	createRequest CreateRequest
	files         map[string][]byte
	execCalls     []ExecRequest
	execResult    ExecResult
	execErr       error
	execFn        func(ExecRequest, map[string][]byte) (ExecResult, error)
	destroyCalls  int
	destroyErr    error
	destroyed     bool
}

func NewFakeSession(id string) *FakeSession {
	if strings.TrimSpace(id) == "" {
		id = "fake-sandbox"
	}
	return &FakeSession{
		id:    id,
		files: map[string][]byte{},
	}
}

func (s *FakeSession) ID() string {
	return s.id
}

func (s *FakeSession) UploadFile(_ context.Context, name string, content []byte) error {
	return s.writeFile(name, content)
}

func (s *FakeSession) ReadFile(_ context.Context, name string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureActive(); err != nil {
		return nil, err
	}

	normalized := normalizePath(name)
	content, ok := s.files[normalized]
	if !ok {
		return nil, ErrFileNotFound
	}
	return cloneBytes(content), nil
}

func (s *FakeSession) WriteFile(_ context.Context, name string, content []byte) error {
	return s.writeFile(name, content)
}

func (s *FakeSession) ListFiles(_ context.Context, prefix string) ([]FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureActive(); err != nil {
		return nil, err
	}

	normalizedPrefix := normalizePath(prefix)
	items := make([]FileInfo, 0, len(s.files))
	for name, content := range s.files {
		if normalizedPrefix != "/" && !strings.HasPrefix(name, normalizedPrefix) {
			continue
		}
		items = append(items, FileInfo{
			Path: name,
			Size: int64(len(content)),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func (s *FakeSession) Exec(_ context.Context, request ExecRequest) (ExecResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureActive(); err != nil {
		return ExecResult{}, err
	}
	if !s.createRequest.ToolPolicy.AllowShell {
		return ExecResult{}, ErrShellNotAllowed
	}

	s.execCalls = append(s.execCalls, cloneExecRequest(request))

	if s.execFn != nil {
		return s.execFn(request, cloneFileMap(s.files))
	}
	if s.execErr != nil {
		return ExecResult{}, s.execErr
	}
	return s.execResult, nil
}

func (s *FakeSession) DownloadFile(ctx context.Context, name string) ([]byte, error) {
	return s.ReadFile(ctx, name)
}

func (s *FakeSession) Destroy(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.destroyCalls++
	s.destroyed = true
	return s.destroyErr
}

func (s *FakeSession) SetExecResult(result ExecResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execResult = result
}

func (s *FakeSession) SetExecError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execErr = err
}

func (s *FakeSession) SetExecFunc(execFn func(ExecRequest, map[string][]byte) (ExecResult, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execFn = execFn
}

func (s *FakeSession) SetDestroyError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.destroyErr = err
}

func (s *FakeSession) ExecCalls() []ExecRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	calls := make([]ExecRequest, 0, len(s.execCalls))
	for _, call := range s.execCalls {
		calls = append(calls, cloneExecRequest(call))
	}
	return calls
}

func (s *FakeSession) DestroyCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.destroyCalls
}

func (s *FakeSession) Files() map[string][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneFileMap(s.files)
}

func (s *FakeSession) attachCreateRequest(request CreateRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.createRequest = cloneCreateRequest(request)
	if s.files == nil {
		s.files = map[string][]byte{}
	}
}

func (s *FakeSession) writeFile(name string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureActive(); err != nil {
		return err
	}

	s.files[normalizePath(name)] = cloneBytes(content)
	return nil
}

func (s *FakeSession) ensureActive() error {
	if s.destroyed {
		return ErrSessionDestroyed
	}
	return nil
}

func normalizePath(raw string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func cloneCreateRequest(request CreateRequest) CreateRequest {
	cloned := request
	cloned.ToolPolicy.AllowedToolKinds = append([]string(nil), request.ToolPolicy.AllowedToolKinds...)
	cloned.Filesystem.ReadableRoots = append([]string(nil), request.Filesystem.ReadableRoots...)
	cloned.Filesystem.WritableRoots = append([]string(nil), request.Filesystem.WritableRoots...)
	cloned.NetworkAllowlist = append([]string(nil), request.NetworkAllowlist...)
	cloned.AdditionalPackages = append([]string(nil), request.AdditionalPackages...)
	if request.Labels != nil {
		cloned.Labels = make(map[string]string, len(request.Labels))
		for key, value := range request.Labels {
			cloned.Labels[key] = value
		}
	}
	if request.EnvVars != nil {
		cloned.EnvVars = make(map[string]string, len(request.EnvVars))
		for key, value := range request.EnvVars {
			cloned.EnvVars[key] = value
		}
	}
	return cloned
}

func cloneExecRequest(request ExecRequest) ExecRequest {
	cloned := request
	cloned.Command = append([]string(nil), request.Command...)
	if request.Environment != nil {
		cloned.Environment = make(map[string]string, len(request.Environment))
		for key, value := range request.Environment {
			cloned.Environment[key] = value
		}
	}
	return cloned
}

func cloneFileMap(files map[string][]byte) map[string][]byte {
	cloned := make(map[string][]byte, len(files))
	for name, content := range files {
		cloned[name] = cloneBytes(content)
	}
	return cloned
}

func cloneBytes(content []byte) []byte {
	if len(content) == 0 {
		return nil
	}
	cloned := make([]byte, len(content))
	copy(cloned, content)
	return cloned
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrFileNotFound)
}
