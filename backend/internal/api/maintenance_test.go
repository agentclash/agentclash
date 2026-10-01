package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/pubsub"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/domain"
	"github.com/google/uuid"
)

func TestRunEventsCursorSurvivesPersistedPolling(t *testing.T) {
	old := runEventStreamPollInterval
	runEventStreamPollInterval = time.Millisecond
	t.Cleanup(func() { runEventStreamPollInterval = old })
	runID, agentID := uuid.New(), uuid.New()
	var events []repository.RunEvent
	for i := int64(1); i <= 4; i++ {
		events = append(events, persistedTestRunEvent(runID, agentID, i, time.Now()))
	}
	for _, cursor := range []string{persistedStreamEventID(agentID, 2), "unknown-cursor"} {
		svc := &fakeSSERunReadService{streamResults: []ListRunEventStreamResult{
			{Run: persistedStreamRun(runID, domain.RunStatusRunning), Events: events[:3]},
			{Run: persistedStreamRun(runID, domain.RunStatusCompleted), Events: events},
		}}
		rec := serveRunEventsSSE(t, &capturingSSEAuthenticator{}, svc, pubsub.NoopSubscriber{}, runID, func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer test-token")
			r.Header.Set("Last-Event-ID", cursor)
		})
		for i := int64(1); i <= 4; i++ {
			want := 1
			if cursor != "unknown-cursor" && i <= 2 {
				want = 0
			}
			if got := strings.Count(rec.Body.String(), "id: "+persistedStreamEventID(agentID, i)+"\n"); got != want {
				t.Fatalf("cursor=%s sequence=%d count=%d want=%d", cursor, i, got, want)
			}
		}
	}
}

type maintenanceGate struct{ active atomic.Int32 }

func (g *maintenanceGate) TryAcquire(context.Context) bool { g.active.Add(1); return true }
func (g *maintenanceGate) Release(context.Context)         { g.active.Add(-1) }

type maintenanceSubscriber struct{ done chan struct{} }

func (s *maintenanceSubscriber) Close() error { return nil }

func (s *maintenanceSubscriber) Subscribe(ctx context.Context, _ uuid.UUID) (<-chan []byte, error) {
	ch := make(chan []byte)
	go func() { <-ctx.Done(); close(ch); close(s.done) }()
	return ch, nil
}

type idleReadConn struct{ net.Conn }

func (c idleReadConn) Read(p []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	return c.Conn.Read(p)
}

func TestQuietSSEThroughIdleProxyAndShutdown(t *testing.T) {
	runID := uuid.New()
	streamCtx, stop := context.WithCancel(context.Background())
	defer stop()
	gate := &maintenanceGate{}
	subscriber := &maintenanceSubscriber{done: make(chan struct{})}
	router := buildRouter(routerOptions{
		logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		authenticator:   &capturingSSEAuthenticator{},
		runReadService:  &fakeSSERunReadService{streamResults: []ListRunEventStreamResult{{Run: persistedStreamRun(runID, domain.RunStatusRunning)}}},
		eventSubscriber: subscriber, sseGate: gate,
		streamOptions: runEventStreamOptions{heartbeatInterval: 25 * time.Millisecond, shutdownContext: streamCtx},
	})
	origin := httptest.NewServer(router)
	defer origin.Close()
	target, _ := url.Parse(origin.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return idleReadConn{conn}, nil
	}}
	defer transport.CloseIdleConnections()
	proxy.Transport = transport
	proxy.ErrorLog = slog.NewLogLogger(slog.NewTextHandler(io.Discard, nil), slog.LevelError)
	edge := httptest.NewServer(proxy)
	defer edge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, edge.URL+"/v1/runs/"+runID.String()+"/events/stream", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := edge.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	r := bufio.NewReader(resp.Body)
	// Remain quiet for longer than the proxy's idle deadline. Every frame
	// must be a comment; it must not change the client's Last-Event-ID.
	for range 12 {
		line, err := r.ReadString('\n')
		if err != nil || line != ": keepalive\n" {
			t.Fatalf("heartbeat: %q %v", line, err)
		}
		if line, err = r.ReadString('\n'); err != nil || line != "\n" {
			t.Fatalf("frame end: %q %v", line, err)
		}
	}
	server := &Server{httpServer: origin.Config, stopStreams: stop}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := server.shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r); err != nil {
		t.Fatalf("stream shutdown: %v", err)
	}
	select {
	case <-subscriber.done:
	case <-time.After(time.Second):
		t.Fatal("subscriber was not cancelled")
	}
	if gate.active.Load() != 0 {
		t.Fatal("SSE capacity leaked")
	}
}

func TestAPIShutdownFinishesRequestsOrForceCloses(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "graceful", true: "deadline"}[force], func(t *testing.T) {
			entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-r.Context().Done():
				case <-release:
					_, _ = w.Write([]byte("complete"))
				}
				close(finished)
			}))
			defer origin.Close()
			response := make(chan error, 1)
			go func() {
				resp, err := origin.Client().Get(origin.URL)
				if resp != nil {
					_, err = io.ReadAll(resp.Body)
					resp.Body.Close()
				}
				response <- err
			}()
			<-entered
			s := &Server{httpServer: origin.Config, stopStreams: func() {
				if !force {
					close(release)
				}
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := s.shutdown(ctx)
			if force && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("shutdown: %v", err)
			}
			if !force && err != nil {
				t.Fatal(err)
			}
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("handler context still active")
			}
			if err := <-response; !force && err != nil {
				t.Fatalf("ordinary request interrupted: %v", err)
			}
		})
	}
}

func TestReadinessRedisAndDrainingIngress(t *testing.T) {
	for _, redisState := range []string{"disabled", "ok", "unreachable"} {
		var ping func(context.Context) error
		if redisState != "disabled" {
			ping = func(context.Context) error {
				if redisState == "unreachable" {
					return errors.New("private connection details")
				}
				return nil
			}
		}
		rec := httptest.NewRecorder()
		healthzReadyHandler(fakeDBPinger{}, fakeTemporalHealthChecker{}, readinessOptions{redis: ping})(rec, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))
		want := http.StatusOK
		if redisState == "unreachable" {
			want = http.StatusServiceUnavailable
		}
		if rec.Code != want || !strings.Contains(rec.Body.String(), `"redis":"`+redisState+`"`) || strings.Contains(rec.Body.String(), "private") {
			t.Fatalf("readiness: %d %s", rec.Code, rec.Body.String())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ready := httptest.NewRecorder()
	healthzReadyHandler(nil, nil, readinessOptions{shutdownContext: ctx})(ready, httptest.NewRequest(http.MethodGet, "/healthz/ready", nil))
	if ready.Code != http.StatusServiceUnavailable || !strings.Contains(ready.Body.String(), "draining") {
		t.Fatal("draining server reported ready")
	}
	for _, path := range []string{"/v1/workspaces", "/v1/runs/example/events/stream", "/healthz"} {
		called := false
		h := shutdownIngress(ctx)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(http.StatusOK) }))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if (path == "/healthz") != called {
			t.Fatalf("ingress reached application: %s", path)
		}
	}
}

type failingStreamWriter struct {
	http.ResponseWriter
	writeErr, flushErr error
}

func (w failingStreamWriter) Write(p []byte) (int, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	return len(p), nil
}
func (w failingStreamWriter) Flush()            {}
func (w failingStreamWriter) FlushError() error { return w.flushErr }

func TestSSEWriteFailuresDoNotMarkEventsDelivered(t *testing.T) {
	broken := errors.New("broken stream")
	for _, w := range []failingStreamWriter{{writeErr: broken}, {flushErr: broken}} {
		delivered := map[string]struct{}{}
		event := persistedTestRunEvent(uuid.New(), uuid.New(), 1, time.Now())
		if err := emitPersistedRunEvents(w, w, []repository.RunEvent{event}, delivered); !errors.Is(err, broken) {
			t.Fatalf("write result: %v", err)
		}
		if len(delivered) != 0 {
			t.Fatal("failed event marked delivered")
		}
	}
}

func TestMaintenanceDurationConfig(t *testing.T) {
	for _, key := range []string{"API_SHUTDOWN_TIMEOUT", "SSE_HEARTBEAT_INTERVAL"} {
		for _, value := range []string{"", "bad", "0s", "-1s"} {
			t.Run(key+"/"+value, func(t *testing.T) {
				t.Setenv(key, value)
				if _, err := LoadConfigFromEnv(); !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("config: %v", err)
				}
			})
		}
	}
	t.Setenv("API_SHUTDOWN_TIMEOUT", "40s")
	t.Setenv("SSE_HEARTBEAT_INTERVAL", "12s")
	cfg, err := LoadConfigFromEnv()
	if err != nil || cfg.ShutdownTimeout != 40*time.Second || cfg.SSEHeartbeatInterval != 12*time.Second {
		t.Fatalf("duration configuration: %v", err)
	}
}
