package docker

import (
	"errors"
	"fmt"
	"strings"
)

// ErrDockerUnavailable is returned when the Docker daemon cannot be reached
// (daemon not running, socket missing, permission denied, etc.).
var ErrDockerUnavailable = errors.New("docker daemon unavailable")

func wrapDockerUnavailable(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrDockerUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrDockerUnavailable, err)
}

func isDaemonUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDockerUnavailable) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, needle := range []string{
		"cannot connect to the docker daemon",
		"is the docker daemon running",
		"error during connect",
		"docker desktop is not running",
		"dial unix",
		"connect: connection refused",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
