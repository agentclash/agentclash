package auth

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"time"
)

// CallbackResult holds the result from the localhost callback server.
type CallbackResult struct {
	Token string
	Error string
}

// StartCallbackServer starts a temporary HTTP server on 127.0.0.1 with a random port.
// It waits for a single GET /callback request, extracts the token, and shuts down.
// The expected state parameter is validated against the received state.
func StartCallbackServer(ctx context.Context, expectedState string) (<-chan CallbackResult, int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("binding localhost: %w", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	resultCh := make(chan CallbackResult, 1)

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		state := query.Get("state")
		token := query.Get("token")
		errMsg := query.Get("error")

		if errMsg != "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, callbackHTML("Authorization Denied", "The CLI login was denied. You can close this tab."))
			resultCh <- CallbackResult{Error: errMsg}
			go server.Close()
			return
		}

		if state != expectedState {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, callbackHTML("Invalid State", "The state parameter does not match. This may be a CSRF attack."))
			resultCh <- CallbackResult{Error: "state mismatch"}
			go server.Close()
			return
		}

		if token == "" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, callbackHTML("Missing Token", "No token was provided in the callback."))
			resultCh <- CallbackResult{Error: "missing token"}
			go server.Close()
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, callbackHTML("Logged In", "You are now authenticated. You can close this tab and return to your terminal."))
		resultCh <- CallbackResult{Token: token}
		go server.Close()
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			resultCh <- CallbackResult{Error: fmt.Sprintf("callback server: %s", err)}
		}
	}()

	// Shutdown on context cancellation or timeout.
	go func() {
		select {
		case <-ctx.Done():
			server.Close()
		case <-time.After(5 * time.Minute):
			resultCh <- CallbackResult{Error: "login timed out (5 minutes)"}
			server.Close()
		}
	}()

	return resultCh, port, nil
}

func callbackHTML(title, message string) string {
	title = html.EscapeString(title)
	message = html.EscapeString(message)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>AgentClash CLI - %s</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #0a0a0a; color: #e5e5e5; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
    .card { text-align: center; max-width: 400px; padding: 2rem; }
    h1 { font-size: 1.25rem; margin-bottom: 0.5rem; }
    p { color: #999; font-size: 0.875rem; }
  </style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>%s</p>
  </div>
</body>
</html>`, title, title, message)
}
