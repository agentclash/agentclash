package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client is the HTTP client for the AgentClash API.
type Client struct {
	baseURL    string
	token      string
	devUserID  string
	devOrgMem  string
	devWSMem   string
	userAgent  string
	httpClient *http.Client
	verbose    bool
	logger     *slog.Logger
}

// Option configures the client.
type Option func(*Client)

// WithDevMode sets dev auth headers.
func WithDevMode(userID, orgMemberships, wsMemberships string) Option {
	return func(c *Client) {
		c.devUserID = userID
		c.devOrgMem = orgMemberships
		c.devWSMem = wsMemberships
	}
}

// WithVerbose enables debug logging.
func WithVerbose(v bool) Option {
	return func(c *Client) { c.verbose = v }
}

// WithUserAgent sets the User-Agent header on every outbound request.
// Use BuildUserAgent to compose a value gated by the resolved API host —
// self-hosted backends only receive the neutral form (no command name leak).
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// NewClient creates an API client.
func NewClient(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: StrictRedirectPolicy,
		},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// StrictRedirectPolicy refuses to follow HTTP redirects that change scheme,
// host, or port. Go's default net/http client strips Authorization across
// cross-host redirects but retains it across cross-port, cross-scheme, and
// cross-subdomain hops — a compromised or misconfigured origin could exploit
// any of those to exfiltrate a bearer token. Loopback hops between localhost
// aliases (localhost / 127.0.0.1 / ::1) on the same port are permitted to
// keep the local dev loop working.
func StrictRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	origin := via[0].URL
	dest := req.URL
	if origin.Scheme != dest.Scheme {
		return fmt.Errorf("refusing redirect: scheme changed %s → %s", origin.Scheme, dest.Scheme)
	}
	if !sameHostPort(origin, dest) {
		return fmt.Errorf("refusing redirect: host/port changed %s → %s", origin.Host, dest.Host)
	}
	return nil
}

func sameHostPort(a, b *url.URL) bool {
	ha, hb := a.Hostname(), b.Hostname()
	pa, pb := a.Port(), b.Port()
	if pa == "" {
		pa = defaultPortForScheme(a.Scheme)
	}
	if pb == "" {
		pb = defaultPortForScheme(b.Scheme)
	}
	if pa != pb {
		return false
	}
	if ha == hb {
		return true
	}
	return isLoopback(ha) && isLoopback(hb)
}

func defaultPortForScheme(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}

func isLoopback(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// NewDownloadClient returns a fresh http.Client that shares this client's
// transport (so corporate proxies / TLS config still apply) but has no
// request timeout — artifact downloads can legitimately last minutes — and
// reuses the strict redirect policy. Callers must pass a cancellable context
// to the request so Ctrl+C cancels the download.
func (c *Client) NewDownloadClient() *http.Client {
	return &http.Client{
		Transport:     c.httpClient.Transport,
		CheckRedirect: StrictRedirectPolicy,
	}
}

// Token returns the client's auth token.
func (c *Client) Token() string {
	return c.token
}

// BaseURL returns the client's base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Response wraps an HTTP response.
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// DecodeJSON unmarshals the response body into v.
func (r *Response) DecodeJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// APIError represents a structured API error.
type APIError struct {
	StatusCode    int        `json:"-"`
	Code          string     `json:"code"`
	Message       string     `json:"message"`
	PlanKey       string     `json:"plan_key,omitempty"`
	UpgradeTarget string     `json:"upgrade_target,omitempty"`
	Limit         *int       `json:"limit,omitempty"`
	Used          *int       `json:"used,omitempty"`
	Remaining     *int       `json:"remaining,omitempty"`
	ResetAt       *time.Time `json:"reset_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	// RetryAfterSeconds is derived from the Retry-After response header (not
	// the error envelope body), normalized to whole seconds.
	RetryAfterSeconds *int `json:"-"`
}

// Retryable reports whether the failure is transient and the same request can
// be retried as-is: rate limiting (429) and server-side failures (5xx). This
// is the single definition the client's own retry loop and the CLI error
// envelope both derive from.
func (e *APIError) Retryable() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}

func (e *APIError) Error() string {
	if e.IsBillingGate() {
		return e.BillingMessage()
	}
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
}

func (e *APIError) IsBillingGate() bool {
	switch e.Code {
	case "plan_limit_exceeded",
		"quota_exceeded",
		"concurrency_limit_exceeded",
		"seat_limit_exceeded",
		"workspace_limit_exceeded",
		"feature_not_entitled",
		"entitlement_expired",
		"entitlement_inactive":
		return true
	default:
		return false
	}
}

func (e *APIError) BillingMessage() string {
	current := planLabel(e.PlanKey)
	upgrade := planLabel(e.UpgradeTarget)
	action := "Open the organization billing page in the AgentClash web app to start a trial or upgrade."
	if e.UpgradeTarget == "" {
		action = "Open the organization billing page in the AgentClash web app to update billing."
	}

	var detail string
	switch e.Code {
	case "feature_not_entitled":
		detail = fmt.Sprintf("%s does not include this feature.", current)
		if e.UpgradeTarget != "" {
			detail = fmt.Sprintf("%s Upgrade target: %s.", detail, upgrade)
		}
	case "quota_exceeded":
		detail = fmt.Sprintf("%s monthly run quota is exhausted.", current)
		if e.Used != nil && e.Limit != nil {
			detail = fmt.Sprintf("%s Used %d of %d.", detail, *e.Used, *e.Limit)
		}
		if e.ResetAt != nil {
			detail = fmt.Sprintf("%s Resets %s.", detail, e.ResetAt.UTC().Format("2006-01-02"))
		}
	case "concurrency_limit_exceeded":
		detail = fmt.Sprintf("%s concurrent run limit is reached.", current)
		if e.Limit != nil {
			detail = fmt.Sprintf("%s Limit: %d.", detail, *e.Limit)
		}
	case "seat_limit_exceeded":
		detail = fmt.Sprintf("%s seat limit is reached.", current)
		if e.Limit != nil {
			detail = fmt.Sprintf("%s Limit: %d.", detail, *e.Limit)
		}
	case "workspace_limit_exceeded", "plan_limit_exceeded":
		detail = fmt.Sprintf("%s plan limit is reached.", current)
		if e.Limit != nil {
			detail = fmt.Sprintf("%s Limit: %d.", detail, *e.Limit)
		}
	case "entitlement_expired":
		detail = fmt.Sprintf("%s access has expired.", current)
		if e.ExpiresAt != nil {
			detail = fmt.Sprintf("%s Expired %s.", detail, e.ExpiresAt.UTC().Format("2006-01-02"))
		}
	case "entitlement_inactive":
		detail = fmt.Sprintf("%s billing is inactive.", current)
	default:
		detail = e.Message
	}
	return fmt.Sprintf("%s %s (HTTP %d)", detail, action, e.StatusCode)
}

func planLabel(planKey string) string {
	switch strings.TrimSpace(planKey) {
	case "free":
		return "Free"
	case "pro":
		return "Pro"
	case "team":
		return "Team"
	case "enterprise":
		return "Enterprise"
	case "":
		return "This organization"
	default:
		return planKey
	}
}

// ParseError attempts to parse an API error from a response. Returns nil if status < 400.
func (r *Response) ParseError() *APIError {
	if r.StatusCode < 400 {
		return nil
	}
	apiErr := &APIError{
		StatusCode: r.StatusCode,
		Code:       http.StatusText(r.StatusCode),
		Message:    string(r.Body),
	}
	var envelope struct {
		Error APIError `json:"error"`
	}
	if json.Unmarshal(r.Body, &envelope) == nil && envelope.Error.Code != "" {
		envelope.Error.StatusCode = r.StatusCode
		apiErr = &envelope.Error
	}
	if delay, ok := parseRetryAfter(r.Header); ok {
		seconds := int(delay / time.Second)
		apiErr.RetryAfterSeconds = &seconds
	}
	return apiErr
}

// parseRetryAfter reads a Retry-After header in either RFC 9110 form —
// delta-seconds or an HTTP-date — and returns it as a non-negative duration.
func parseRetryAfter(h http.Header) (time.Duration, bool) {
	value := strings.TrimSpace(h.Get("Retry-After"))
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		delay := time.Until(at)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

// Get performs a GET request.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (*Response, error) {
	return c.do(ctx, http.MethodGet, path, query, nil)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error) {
	return c.doJSON(ctx, http.MethodPost, path, body)
}

// Patch performs a PATCH request with a JSON body.
func (c *Client) Patch(ctx context.Context, path string, body any) (*Response, error) {
	return c.doJSON(ctx, http.MethodPatch, path, body)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error) {
	return c.doJSON(ctx, http.MethodPut, path, body)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*Response, error) {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// PostRaw performs a POST with a raw body and custom content type.
func (c *Client) PostRaw(ctx context.Context, path string, contentType string, body io.Reader) (*Response, error) {
	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	c.setAuth(req)
	return c.execute(req)
}

// FileUpload describes a file to upload.
type FileUpload struct {
	Filename string
	Reader   io.Reader
}

// PostMultipart performs a multipart/form-data POST. The body is first
// spooled to a temp file on disk, then replayed from that file as the
// request body. This gives us three properties the original buffered-in-RAM
// implementation couldn't:
//
//  1. Bounded memory use even for replay bundles that exceed 100MB.
//  2. A real Content-Length header — some gateways and WAFs reject
//     `Transfer-Encoding: chunked` uploads with 411/400.
//  3. A `GetBody` implementation so Go can replay the body on same-origin
//     307/308 redirects instead of silently failing.
func (c *Client) PostMultipart(ctx context.Context, path string, fields map[string]string, files map[string]FileUpload) (*Response, error) {
	spool, err := os.CreateTemp("", "agentclash-multipart-*")
	if err != nil {
		return nil, fmt.Errorf("creating multipart spool: %w", err)
	}
	spoolPath := spool.Name()
	// Best-effort cleanup: unlink now so the fd keeps the bytes alive while
	// in-flight, then the OS reclaims on close. On Windows, Remove-while-open
	// fails, so fall back to a deferred Remove after the request completes.
	defer func() {
		_ = os.Remove(spoolPath)
	}()

	writer := multipart.NewWriter(spool)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			_ = spool.Close()
			return nil, fmt.Errorf("writing field %s: %w", k, err)
		}
	}
	for field, file := range files {
		part, err := writer.CreateFormFile(field, file.Filename)
		if err != nil {
			_ = spool.Close()
			return nil, fmt.Errorf("creating form file %s: %w", field, err)
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			_ = spool.Close()
			return nil, fmt.Errorf("copying file %s: %w", field, err)
		}
	}
	if err := writer.Close(); err != nil {
		_ = spool.Close()
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}
	size, err := spool.Seek(0, io.SeekEnd)
	if err != nil {
		_ = spool.Close()
		return nil, fmt.Errorf("measuring spooled body: %w", err)
	}
	if _, err := spool.Seek(0, io.SeekStart); err != nil {
		_ = spool.Close()
		return nil, fmt.Errorf("rewinding spooled body: %w", err)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, spool)
	if err != nil {
		_ = spool.Close()
		return nil, err
	}
	req.ContentLength = size
	// GetBody lets net/http replay the body on a 307/308 redirect. Each
	// invocation re-opens the spool file so concurrent replays don't share
	// a file offset.
	req.GetBody = func() (io.ReadCloser, error) {
		return os.Open(spoolPath)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.setAuth(req)
	resp, err := c.execute(req)
	_ = spool.Close()
	return resp, err
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) (*Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuth(req)
	return c.executeWithRetry(req)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) (*Response, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	return c.executeWithRetry(req)
}

func (c *Client) setAuth(req *http.Request) {
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.devUserID != "" {
		req.Header.Set("X-Agentclash-User-Id", c.devUserID)
		if c.devOrgMem != "" {
			req.Header.Set("X-Agentclash-Org-Memberships", c.devOrgMem)
		}
		if c.devWSMem != "" {
			req.Header.Set("X-Agentclash-Workspace-Memberships", c.devWSMem)
		}
		return
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
}

// maxRetryAfterAutoWait bounds how long the automatic GET retry loop will
// honor a server's Retry-After before giving up and surfacing the 429 to the
// caller (whose error envelope still carries details.retry_after_seconds).
const maxRetryAfterAutoWait = 30 * time.Second

func (c *Client) executeWithRetry(req *http.Request) (*Response, error) {
	if req.Method != http.MethodGet {
		return c.execute(req)
	}

	var resp *Response
	var err error

	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; attempt <= len(delays); attempt++ {
		resp, err = c.execute(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		if attempt < len(delays) {
			delay := delays[attempt]
			if resp.StatusCode == http.StatusTooManyRequests {
				if retryAfter, ok := parseRetryAfter(resp.Header); ok {
					if retryAfter > maxRetryAfterAutoWait {
						// Too long to block on — let the caller decide when
						// to come back (RFC 6585 §4 / RFC 9110 §10.2.3).
						return resp, nil
					}
					if retryAfter > delay {
						delay = retryAfter
					}
				}
			}
			if c.verbose {
				c.logger.Debug("retrying request", "status", resp.StatusCode, "attempt", attempt+1, "delay", delay)
			}
			time.Sleep(delay)
		}
	}
	return resp, nil
}

func (c *Client) execute(req *http.Request) (*Response, error) {
	if c.verbose {
		c.logger.Debug("request", "method", req.Method, "url", req.URL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if c.verbose {
		c.logger.Debug("response", "status", resp.StatusCode, "body_len", len(body))
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Header:     resp.Header,
	}, nil
}
