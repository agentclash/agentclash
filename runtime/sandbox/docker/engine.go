package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
)

// engine is the subset of Docker Engine API used by the provider. Tests inject fakes.
type engine interface {
	Ping(ctx context.Context) error
	ImagePull(ctx context.Context, ref string) error
	ContainerCreate(ctx context.Context, cfg container.Config, hostCfg container.HostConfig, name string) (string, error)
	ContainerStart(ctx context.Context, id string) error
	ContainerStop(ctx context.Context, id string, timeout *time.Duration) error
	ContainerRemove(ctx context.Context, id string, force bool) error
	CopyToContainer(ctx context.Context, id, destPath string, content io.Reader) error
	CopyFromContainer(ctx context.Context, id, srcPath string) (io.ReadCloser, error)
	ContainerExecCreate(ctx context.Context, id string, cfg client.ExecCreateOptions) (string, error)
	ContainerExecAttach(ctx context.Context, execID string) (execAttach, error)
	ContainerExecInspect(ctx context.Context, execID string) (client.ExecInspectResult, error)
	ContainerInspectLabels(ctx context.Context, ref string) (map[string]string, error)
	Close() error
}

type execAttach interface {
	Reader() io.Reader
	Close() error
}

type dockerEngine struct {
	cli *client.Client
}

func newDockerEngine(host string) (*dockerEngine, error) {
	opts := []client.Opt{client.FromEnv}
	if strings.TrimSpace(host) != "" {
		opts = append(opts, client.WithHost(strings.TrimSpace(host)))
	}
	cli, err := client.New(opts...)
	if err != nil {
		return nil, wrapDockerUnavailable(err)
	}
	return &dockerEngine{cli: cli}, nil
}

func (e *dockerEngine) Ping(ctx context.Context) error {
	_, err := e.cli.Ping(ctx, client.PingOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return err
	}
	return nil
}

func (e *dockerEngine) ImagePull(ctx context.Context, ref string) error {
	reader, err := e.cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("pull image %q: %w", ref, err)
	}
	defer reader.Close()
	if err := consumePullStream(reader); err != nil {
		return fmt.Errorf("pull image %q: %w", ref, err)
	}
	return nil
}

// consumePullStream drains the pull progress stream and surfaces in-stream
// errors, which the daemon reports as JSON messages inside a 200 response
// (auth failures, missing manifests, disk full, ...).
func consumePullStream(r io.Reader) error {
	return jsonmessage.DisplayJSONMessagesStream(r, io.Discard, 0, false, nil)
}

func (e *dockerEngine) ContainerCreate(ctx context.Context, cfg container.Config, hostCfg container.HostConfig, name string) (string, error) {
	resp, err := e.cli.ContainerCreate(ctx, client.ContainerCreateOptions{Config: &cfg, HostConfig: &hostCfg, Name: name})
	if err != nil {
		if isDaemonUnavailable(err) {
			return "", wrapDockerUnavailable(err)
		}
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

func (e *dockerEngine) ContainerStart(ctx context.Context, id string) error {
	if _, err := e.cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("start container %s: %w", id, err)
	}
	return nil
}

func (e *dockerEngine) ContainerStop(ctx context.Context, id string, timeout *time.Duration) error {
	opts := client.ContainerStopOptions{}
	if timeout != nil {
		seconds := int(timeout.Round(time.Second) / time.Second)
		if seconds < 1 {
			// Zero means immediate SIGKILL; keep a minimal grace period.
			seconds = 1
		}
		opts.Timeout = &seconds
	}
	if _, err := e.cli.ContainerStop(ctx, id, opts); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("stop container %s: %w", id, err)
	}
	return nil
}

func (e *dockerEngine) ContainerRemove(ctx context.Context, id string, force bool) error {
	if _, err := e.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: force}); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("remove container %s: %w", id, err)
	}
	return nil
}

func (e *dockerEngine) CopyToContainer(ctx context.Context, id, destPath string, content io.Reader) error {
	if _, err := e.cli.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: destPath, Content: content}); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("copy to container %s:%s: %w", id, destPath, err)
	}
	return nil
}

func (e *dockerEngine) CopyFromContainer(ctx context.Context, id, srcPath string) (io.ReadCloser, error) {
	result, err := e.cli.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{SourcePath: srcPath})
	if err != nil {
		if isDaemonUnavailable(err) {
			return nil, wrapDockerUnavailable(err)
		}
		return nil, err
	}
	return result.Content, nil
}

func (e *dockerEngine) ContainerExecCreate(ctx context.Context, id string, cfg client.ExecCreateOptions) (string, error) {
	resp, err := e.cli.ExecCreate(ctx, id, cfg)
	if err != nil {
		if isDaemonUnavailable(err) {
			return "", wrapDockerUnavailable(err)
		}
		return "", fmt.Errorf("exec create in %s: %w", id, err)
	}
	return resp.ID, nil
}

type dockerExecAttach struct {
	hijacked client.ExecAttachResult
}

func (a dockerExecAttach) Reader() io.Reader { return a.hijacked.Reader }
func (a dockerExecAttach) Close() error {
	a.hijacked.Close()
	return nil
}

func (e *dockerEngine) ContainerExecAttach(ctx context.Context, execID string) (execAttach, error) {
	hijacked, err := e.cli.ExecAttach(ctx, execID, client.ExecAttachOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return nil, wrapDockerUnavailable(err)
		}
		return nil, fmt.Errorf("exec attach %s: %w", execID, err)
	}
	return dockerExecAttach{hijacked: hijacked}, nil
}

func (e *dockerEngine) ContainerExecInspect(ctx context.Context, execID string) (client.ExecInspectResult, error) {
	inspect, err := e.cli.ExecInspect(ctx, execID, client.ExecInspectOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return client.ExecInspectResult{}, wrapDockerUnavailable(err)
		}
		return client.ExecInspectResult{}, fmt.Errorf("exec inspect %s: %w", execID, err)
	}
	return inspect, nil
}

func (e *dockerEngine) ContainerInspectLabels(ctx context.Context, ref string) (map[string]string, error) {
	inspect, err := e.cli.ContainerInspect(ctx, ref, client.ContainerInspectOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return nil, wrapDockerUnavailable(err)
		}
		return nil, fmt.Errorf("inspect container %s: %w", ref, err)
	}
	if inspect.Container.Config == nil {
		return map[string]string{}, nil
	}
	return inspect.Container.Config.Labels, nil
}

func (e *dockerEngine) Close() error {
	return e.cli.Close()
}

func writeTarFile(filePath string, content []byte) (io.Reader, error) {
	cleaned := path.Clean(strings.TrimSpace(filePath))
	if cleaned == "." || cleaned == "/" {
		return nil, fmt.Errorf("invalid file path %q", filePath)
	}
	base := path.Base(cleaned)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name: base,
		Mode: 0o644,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(content); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func readTarFile(r io.Reader) ([]byte, error) {
	tr := tar.NewReader(r)
	hdr, err := tr.Next()
	if err != nil {
		return nil, err
	}
	if hdr.Typeflag == tar.TypeDir {
		return nil, fmt.Errorf("path is a directory")
	}
	// Symlinks come back as zero-byte link entries; reading one silently as
	// empty content would corrupt whatever consumes the file.
	if hdr.Typeflag != tar.TypeReg {
		return nil, fmt.Errorf("path is not a regular file (tar typeflag %d)", hdr.Typeflag)
	}
	return io.ReadAll(tr)
}
