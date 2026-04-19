package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSecretSetPreservesPipedStdinExactly(t *testing.T) {
	input := "  line1\nline2\n  \n"
	var gotValue string
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PUT /v1/workspaces/ws-1/secrets/API_KEY": func(w http.ResponseWriter, r *http.Request) {
			called = true
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			gotValue = body.Value
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	withStdin(t, input, func() {
		t.Setenv("AGENTCLASH_TOKEN", "test-tok")
		err := executeCommand(t, []string{"secret", "set", "API_KEY", "-w", "ws-1"}, srv.URL)
		if err != nil {
			t.Fatalf("secret set error: %v", err)
		}
	})

	if !called {
		t.Fatal("PUT /v1/workspaces/ws-1/secrets/API_KEY was not called")
	}
	if gotValue != input {
		t.Fatalf("secret value = %q, want exact %q", gotValue, input)
	}
}

func TestSecretSetPreservesValueFlagExactly(t *testing.T) {
	input := "  literal\nline2\n  "
	var gotValue string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PUT /v1/workspaces/ws-1/secrets/API_KEY": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			gotValue = body.Value
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"secret", "set", "API_KEY", "-w", "ws-1", "--value", input}, srv.URL)
	if err != nil {
		t.Fatalf("secret set error: %v", err)
	}

	if gotValue != input {
		t.Fatalf("secret value = %q, want exact %q", gotValue, input)
	}
}

func TestSecretSetAllowsWhitespaceOnlyNonEmptyValue(t *testing.T) {
	input := "  \n"
	var gotValue string
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PUT /v1/workspaces/ws-1/secrets/API_KEY": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Value string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			gotValue = body.Value
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"secret", "set", "API_KEY", "-w", "ws-1", "--value", input}, srv.URL)
	if err != nil {
		t.Fatalf("secret set error: %v", err)
	}

	if gotValue != input {
		t.Fatalf("secret value = %q, want exact %q", gotValue, input)
	}
}

func TestSecretSetRejectsZeroLengthValue(t *testing.T) {
	var called bool
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"PUT /v1/workspaces/ws-1/secrets/API_KEY": func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"secret", "set", "API_KEY", "-w", "ws-1", "--value", ""}, srv.URL)
	if err == nil {
		t.Fatal("expected zero-length secret value to fail")
	}
	if !strings.Contains(err.Error(), "secret value cannot be empty") {
		t.Fatalf("error = %v, want empty value message", err)
	}
	if called {
		t.Fatal("secret API should not be called for zero-length value")
	}
}

func TestReadSecretSetValueFromInputUsesHiddenTerminalInput(t *testing.T) {
	oldReadSecretPassword := readSecretPassword
	defer func() { readSecretPassword = oldReadSecretPassword }()

	var gotFD int
	readSecretPassword = func(fd int) ([]byte, error) {
		gotFD = fd
		return []byte("hidden-value"), nil
	}

	var stderr bytes.Buffer
	got, err := readSecretSetValueFromInput(strings.NewReader("visible-value"), 42, true, &stderr)
	if err != nil {
		t.Fatalf("read secret value: %v", err)
	}
	if got != "hidden-value" {
		t.Fatalf("secret value = %q, want hidden-value", got)
	}
	if gotFD != 42 {
		t.Fatalf("fd = %d, want 42", gotFD)
	}
	if !strings.Contains(stderr.String(), "Enter secret value: ") {
		t.Fatalf("stderr = %q, want prompt", stderr.String())
	}
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	if _, err := fmt.Fprint(writer, input); err != nil {
		t.Fatalf("write stdin pipe: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}

	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		reader.Close()
	}()

	fn()
}
