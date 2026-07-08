package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
)

type fakeEngine struct {
	mu sync.Mutex

	pingErr    error
	pullErr    error
	createErr  error
	startErr   error
	stopErr    error
	removeErr  error
	inspectErr error

	// createFailOnce fails the first ContainerCreate, then succeeds (image-pull retry path).
	createFailOnce error

	// inspectLabels backs ContainerInspectLabels, keyed by container name or ID.
	inspectLabels map[string]map[string]string

	// aptGetExitCode / aptGetStderr script apt-get failures in simulateExec.
	aptGetExitCode int
	aptGetStderr   string

	// forceExitCode, when set, overrides the exit code of the next exec.
	forceExitCode *int

	pulled        []string
	created       []fakeCreate
	started       []string
	stopped       []string
	removed       []string
	execCreates   []container.ExecOptions
	files         map[string]map[string][]byte // containerID -> path -> content
	execResults   map[string]container.ExecInspect
	execOutputs   map[string]struct{ stdout, stderr string }
	nextContainer string
	nextExecID    int
}

type fakeCreate struct {
	cfg     container.Config
	hostCfg container.HostConfig
	name    string
	id      string
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		files:         map[string]map[string][]byte{},
		execResults:   map[string]container.ExecInspect{},
		execOutputs:   map[string]struct{ stdout, stderr string }{},
		inspectLabels: map[string]map[string]string{},
	}
}

func (f *fakeEngine) Ping(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pingErr
}

func (f *fakeEngine) ImagePull(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pullErr != nil {
		return f.pullErr
	}
	f.pulled = append(f.pulled, ref)
	return nil
}

func (f *fakeEngine) ContainerCreate(_ context.Context, cfg container.Config, hostCfg container.HostConfig, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createFailOnce != nil {
		err := f.createFailOnce
		f.createFailOnce = nil
		return "", err
	}
	if f.createErr != nil {
		return "", f.createErr
	}
	id := f.nextContainer
	if id == "" {
		id = fmt.Sprintf("ctr-%d", len(f.created)+1)
	}
	f.nextContainer = ""
	f.created = append(f.created, fakeCreate{cfg: cfg, hostCfg: hostCfg, name: name, id: id})
	f.files[id] = map[string][]byte{}
	return id, nil
}

func (f *fakeEngine) ContainerStart(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return f.startErr
	}
	f.started = append(f.started, id)
	return nil
}

func (f *fakeEngine) ContainerStop(_ context.Context, id string, _ *time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopErr != nil {
		return f.stopErr
	}
	f.stopped = append(f.stopped, id)
	return nil
}

func (f *fakeEngine) ContainerRemove(_ context.Context, id string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.removeErr != nil {
		return f.removeErr
	}
	f.removed = append(f.removed, id)
	delete(f.files, id)
	return nil
}

func (f *fakeEngine) CopyToContainer(_ context.Context, id, destPath string, content io.Reader) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	files, ok := f.files[id]
	if !ok {
		return fmt.Errorf("no such container: %s", id)
	}
	tr := tar.NewReader(content)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		full := path.Join(destPath, hdr.Name)
		files[normalizeAbsPath(full)] = data
	}
	return nil
}

func (f *fakeEngine) CopyFromContainer(_ context.Context, id, srcPath string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	files, ok := f.files[id]
	if !ok {
		return nil, fmt.Errorf("no such container: %s", id)
	}
	content, ok := files[normalizeAbsPath(srcPath)]
	if !ok {
		return nil, fmt.Errorf("no such file or directory: %s", srcPath)
	}
	archive, err := writeTarFile(srcPath, content)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(archive)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeEngine) ContainerExecCreate(_ context.Context, id string, cfg container.ExecOptions) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.execCreates = append(f.execCreates, cfg)
	f.nextExecID++
	execID := fmt.Sprintf("exec-%d", f.nextExecID)

	stdout, stderr, exitCode := f.simulateExec(id, cfg)
	if f.forceExitCode != nil {
		exitCode = *f.forceExitCode
		f.forceExitCode = nil
	}
	f.execResults[execID] = container.ExecInspect{ExecID: execID, ExitCode: exitCode}
	f.execOutputs[execID] = struct{ stdout, stderr string }{stdout: stdout, stderr: stderr}
	return execID, nil
}

func (f *fakeEngine) simulateExec(id string, cfg container.ExecOptions) (stdout, stderr string, exitCode int) {
	if len(cfg.Cmd) == 0 {
		return "", "empty command", 1
	}
	// Strip the in-container timeout wrapper (`timeout -k 2 <secs> cmd...`)
	// added by execInternal when ExecRequest.Timeout is set.
	if cfg.Cmd[0] == "timeout" && len(cfg.Cmd) > 4 {
		cfg.Cmd = cfg.Cmd[4:]
	}
	cmd := cfg.Cmd[0]
	switch cmd {
	case "mkdir":
		return "", "", 0
	case "find":
		files := f.files[id]
		prefix := "/"
		if len(cfg.Cmd) > 1 {
			prefix = normalizeAbsPath(cfg.Cmd[1])
		}
		var lines []string
		for name, content := range files {
			if prefix != "/" && name != prefix && !strings.HasPrefix(name, prefix+"/") {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s\t%d", name, len(content)))
		}
		return strings.Join(lines, "\n"), "", 0
	case "echo":
		return strings.Join(cfg.Cmd[1:], " ") + "\n", "", 0
	case "apt-get":
		return "", f.aptGetStderr, f.aptGetExitCode
	case "sh", "bash":
		// Support `sh -c 'printf ...'` style used in smoke; keep simple.
		if len(cfg.Cmd) >= 3 && (cfg.Cmd[1] == "-c" || cfg.Cmd[1] == "-lc") {
			script := cfg.Cmd[2]
			if strings.HasPrefix(script, "echo ") {
				return strings.TrimPrefix(script, "echo ") + "\n", "", 0
			}
			if strings.Contains(script, "apt-get") {
				return "", "", 0
			}
		}
		return "", "", 0
	default:
		return "", "", 0
	}
}

type fakeAttach struct {
	r io.Reader
}

func (a fakeAttach) Reader() io.Reader { return a.r }
func (a fakeAttach) Close() error      { return nil }

func (f *fakeEngine) ContainerExecAttach(_ context.Context, execID string) (execAttach, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.execOutputs[execID]
	// Frame stdout/stderr with stdcopy headers so demuxDockerOutput can split them.
	framed := frameStdcopy(1, []byte(out.stdout))
	framed = append(framed, frameStdcopy(2, []byte(out.stderr))...)
	return fakeAttach{r: bytes.NewReader(framed)}, nil
}

func frameStdcopy(stream byte, payload []byte) []byte {
	if len(payload) == 0 {
		return nil
	}
	header := make([]byte, 8)
	header[0] = stream
	header[4] = byte(len(payload) >> 24)
	header[5] = byte(len(payload) >> 16)
	header[6] = byte(len(payload) >> 8)
	header[7] = byte(len(payload))
	return append(header, payload...)
}

func (f *fakeEngine) ContainerExecInspect(_ context.Context, execID string) (container.ExecInspect, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inspect, ok := f.execResults[execID]
	if !ok {
		return container.ExecInspect{}, fmt.Errorf("unknown exec %s", execID)
	}
	return inspect, nil
}

func (f *fakeEngine) ContainerInspectLabels(_ context.Context, ref string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.inspectErr != nil {
		return nil, f.inspectErr
	}
	labels, ok := f.inspectLabels[ref]
	if !ok {
		return nil, fmt.Errorf("no such container: %s", ref)
	}
	return labels, nil
}

func (f *fakeEngine) Close() error { return nil }

func (f *fakeEngine) lastCreate() fakeCreate {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.created) == 0 {
		return fakeCreate{}
	}
	return f.created[len(f.created)-1]
}

func (f *fakeEngine) lastExec() container.ExecOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.execCreates) == 0 {
		return container.ExecOptions{}
	}
	return f.execCreates[len(f.execCreates)-1]
}
