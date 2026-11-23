package cmd

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// isolateCLIEnv gives a test an empty HOME, an empty XDG config directory and
// an empty working directory, so that stored credentials, a saved default
// workspace or a project .agentclash.yaml on the machine cannot change the
// result. Only AGENTCLASH_TOKEN is set. A past release broke because tests
// passed locally with leaked stored credentials.
func isolateCLIEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, key := range []string{
		"AGENTCLASH_API_URL",
		"AGENTCLASH_WORKSPACE",
		"AGENTCLASH_ORG",
		"AGENTCLASH_DEV_USER_ID",
		"AGENTCLASH_DEV_ORG_MEMBERSHIPS",
		"AGENTCLASH_DEV_WORKSPACE_MEMBERSHIPS",
		"AGENTCLASH_NONINTERACTIVE",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	t.Chdir(home)
	return home
}

// countingFakeAPI is fakeAPI with a request counter. A test of a local check
// asserts that the counter stays at zero, which proves that no request left
// the CLI.
func countingFakeAPI(t *testing.T, routes map[string]http.HandlerFunc) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var count atomic.Int64
	router := fakeAPIRouter(t, routes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		router(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// assertExitCode checks the exit code that main would use for err.
func assertExitCode(t *testing.T, err error, want int) {
	t.Helper()
	if got := exitCodeForError(err); got != want {
		t.Fatalf("exit code = %d, want %d (err: %v)", got, want, err)
	}
}

// assertCLIErrorCode checks that err is a local *cliError with the code.
func assertCLIErrorCode(t *testing.T, err error, want string) *cliError {
	t.Helper()
	var local *cliError
	if !errors.As(err, &local) {
		t.Fatalf("error = %v (%T), want *cliError with code %q", err, err, want)
	}
	if local.Code != want {
		t.Fatalf("cliError code = %q, want %q (message: %s)", local.Code, want, local.Message)
	}
	return local
}

// stubConfirmAnswer makes confirmLargeEffect read answer instead of stdin.
func stubConfirmAnswer(t *testing.T, answer string) {
	t.Helper()
	previous := confirmReader
	confirmReader = func() io.Reader { return strings.NewReader(answer) }
	t.Cleanup(func() { confirmReader = previous })
}
