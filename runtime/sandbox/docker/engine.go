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

"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
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
	ContainerExecCreate(ctx context.Context, id string, cfg container.ExecOptions) (string, error)
	ContainerExecAttach(ctx context.Context, execID string) (execAttach, error)
	ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error)
	Close() error
}

type execAttach interface {
	Reader() io.Reader
	Close() error
}

type dockerEngine struct {
	cli *client.Client
}

func newDockerEngine() (*dockerEngine, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, wrapDockerUnavailable(err)
	}
	return &dockerEngine{cli: cli}, nil
}

func (e *dockerEngine) Ping(ctx context.Context) error {
	_, err := e.cli.Ping(ctx)
	if err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return err
	}
	return nil
}

func (e *dockerEngine) ImagePull(ctx context.Context, ref string) error {
	reader, err := e.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("pull image %q: %w", ref, err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("consume image pull for %q: %w", ref, err)
	}
	return nil
}

func (e *dockerEngine) ContainerCreate(ctx context.Context, cfg container.Config, hostCfg container.HostConfig, name string) (string, error) {
	resp, err := e.cli.ContainerCreate(ctx, &cfg, &hostCfg, nil, nil, name)
	if err != nil {
		if isDaemonUnavailable(err) {
			return "", wrapDockerUnavailable(err)
		}
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

func (e *dockerEngine) ContainerStart(ctx context.Context, id string) error {
	if err := e.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("start container %s: %w", id, err)
	}
	return nil
}

func (e *dockerEngine) ContainerStop(ctx context.Context, id string, timeout *time.Duration) error {
	opts := container.StopOptions{}
	if timeout != nil {
		seconds := int(timeout.Round(time.Second) / time.Second)
		opts.Timeout = &seconds
	}
	if err := e.cli.ContainerStop(ctx, id, opts); err != nil {
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
	if err := e.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: force}); err != nil {
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
	if err := e.cli.CopyToContainer(ctx, id, destPath, content, container.CopyToContainerOptions{}); err != nil {
		if isDaemonUnavailable(err) {
			return wrapDockerUnavailable(err)
		}
		return fmt.Errorf("copy to container %s:%s: %w", id, destPath, err)
	}
	return nil
}

func (e *dockerEngine) CopyFromContainer(ctx context.Context, id, srcPath string) (io.ReadCloser, error) {
	reader, _, err := e.cli.CopyFromContainer(ctx, id, srcPath)
	if err != nil {
		if isDaemonUnavailable(err) {
			return nil, wrapDockerUnavailable(err)
		}
		return nil, err
	}
	return reader, nil
}

func (e *dockerEngine) ContainerExecCreate(ctx context.Context, id string, cfg container.ExecOptions) (string, error) {
	resp, err := e.cli.ContainerExecCreate(ctx, id, cfg)
	if err != nil {
		if isDaemonUnavailable(err) {
			return "", wrapDockerUnavailable(err)
		}
		return "", fmt.Errorf("exec create in %s: %w", id, err)
	}
	return resp.ID, nil
}

type dockerExecAttach struct {
	hijacked types.HijackedResponse
}

func (a dockerExecAttach) Reader() io.Reader { return a.hijacked.Reader }
func (a dockerExecAttach) Close() error {
	a.hijacked.Close()
	return nil
}

func (e *dockerEngine) ContainerExecAttach(ctx context.Context, execID string) (execAttach, error) {
	hijacked, err := e.cli.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{})
	if err != nil {
		if isDaemonUnavailable(err) {
			return nil, wrapDockerUnavailable(err)
		}
		return nil, fmt.Errorf("exec attach %s: %w", execID, err)
	}
	return dockerExecAttach{hijacked: hijacked}, nil
}

func (e *dockerEngine) ContainerExecInspect(ctx context.Context, execID string) (container.ExecInspect, error) {
	inspect, err := e.cli.ContainerExecInspect(ctx, execID)
	if err != nil {
		if isDaemonUnavailable(err) {
			return container.ExecInspect{}, wrapDockerUnavailable(err)
		}
		return container.ExecInspect{}, fmt.Errorf("exec inspect %s: %w", execID, err)
	}
	return inspect, nil
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
	return io.ReadAll(tr)
}

func demuxDockerOutput(r io.Reader) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, r); err != nil && err != io.EOF {
		return "", "", err
	}
	return outBuf.String(), errBuf.String(), nil
}
