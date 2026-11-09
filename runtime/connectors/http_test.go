package connectors

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

type fixtureResolver []netip.Addr

func (r fixtureResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r, nil
}

type fixtureDialer func(context.Context, string, string) (net.Conn, error)

func (f fixtureDialer) DialContext(c context.Context, n, a string) (net.Conn, error) {
	return f(c, n, a)
}
func localClient(t *testing.T, handler http.HandlerFunc) (Client, *int) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	dials := new(int)
	c := Client{HTTP: server.Client(), Resolver: fixtureResolver{netip.MustParseAddr("93.184.216.34")}}
	c.Dialer = fixtureDialer(func(ctx context.Context, n, a string) (net.Conn, error) {
		*dials++
		if a != "93.184.216.34:443" {
			t.Errorf("dial not pinned: %s", a)
		}
		return (&net.Dialer{}).DialContext(ctx, n, server.Listener.Addr().String())
	})
	return c, dials
}
func fixtureConfig() Config {
	return Config{Adapter: HTTPJSON, EndpointURL: "https://example.com/invoke", SecretKey: "APP_TOKEN", Auth: "bearer", InputFields: map[string]string{"question": "/input/text"}, LiteralJSON: map[string]json.RawMessage{"version": json.RawMessage(`1`)}, ResponsePointer: "/result/a~1b/0", TimeoutSeconds: 30}
}
func TestHTTPJSONUsesExplicitMappingAndPinnedVerifiedTLS(t *testing.T) {
	c, dials := localClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" || r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Errorf("request contract: %s %s", r.URL.Path, r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		var v map[string]any
		if err := json.Unmarshal(b, &v); err != nil || v["question"] != "hello" || v["version"] != float64(1) || len(v) != 2 {
			t.Errorf("mapping=%s err=%v", b, err)
		}
		io.WriteString(w, `{"result":{"a/b":["answer"]},"ignored":true}`)
	})
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	got, err := c.InvokeHTTPJSON(context.Background(), fixtureConfig(), "fixture-secret", json.RawMessage(`{"input":{"text":"hello"},"private":"not mapped"}`), time.Now().Add(time.Second))
	if err != nil || string(got) != `"answer"` || *dials != 1 {
		t.Fatalf("output=%s dials=%d err=%v", got, *dials, err)
	}
}
func TestHTTPJSONRefusesRedirectsAndUnboundedOrTrailingBodies(t *testing.T) {
	for _, name := range []string{"redirect_same", "redirect_cross", "oversize", "trailing", "bad_pointer"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			c, _ := localClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch name {
				case "redirect_same":
					w.Header().Set("Location", "https://example.com/next")
					w.WriteHeader(307)
				case "redirect_cross":
					w.Header().Set("Location", "https://other.example.com/next")
					w.WriteHeader(302)
				case "oversize":
					io.WriteString(w, `"`+strings.Repeat("x", MaxBodyBytes)+`"`)
				case "trailing":
					io.WriteString(w, `{} {}`)
				default:
					io.WriteString(w, `{"result":{}}`)
				}
			})
			_, err := c.InvokeHTTPJSON(context.Background(), fixtureConfig(), "fixture-secret", json.RawMessage(`{"input":{"text":"hello"}}`), time.Now().Add(time.Second))
			if err == nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if strings.Contains(err.Error(), "fixture-secret") {
				t.Fatal("credential leaked in error")
			}
		})
	}
}
func TestHTTPJSONRevalidatesAllDNSAnswersAndExpiredDeadline(t *testing.T) {
	c, dials := localClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
	for _, addresses := range []fixtureResolver{{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1")}, {netip.MustParseAddr("::ffff:10.0.0.1")}, {netip.MustParseAddr("169.254.169.254")}} {
		c.Resolver = addresses
		_, err := c.DoJSON(context.Background(), "https://example.com", "bearer", "token", []byte(`{}`), time.Now().Add(time.Second))
		if err == nil {
			t.Fatal("private DNS answer accepted")
		}
	}
	if *dials != 0 {
		t.Fatalf("private address reached dialer: %d", *dials)
	}
	c.Resolver = fixtureResolver{netip.MustParseAddr("93.184.216.34")}
	if _, err := c.DoJSON(context.Background(), "https://example.com", "bearer", "token", []byte(`{}`), time.Now().Add(-time.Second)); err == nil || *dials != 0 {
		t.Fatal("expired request sent")
	}
}
func TestExplicitMappingRejectsMissingFieldsAndInvalidPointers(t *testing.T) {
	c := fixtureConfig()
	if _, err := c.Request(json.RawMessage(`{"other":"hello"}`)); err == nil {
		t.Fatal("missing mapped input accepted")
	}
	for _, p := range []string{"answer", "/~2", "/~"} {
		c.ResponsePointer = p
		if c.Validate() == nil {
			t.Fatalf("invalid pointer %q accepted", p)
		}
	}
	c = fixtureConfig()
	c.SecretKey = "env://TOKEN"
	if c.Validate() == nil {
		t.Fatal("environment credential reference accepted")
	}
}
