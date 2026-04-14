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

// NewClient creates an API client.
func NewClient(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
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
}

// DecodeJSON unmarshals the response body into v.
func (r *Response) DecodeJSON(v any) error {
	return json.Unmarshal(r.Body, v)
}

// APIError represents a structured API error.
type APIError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.StatusCode)
}

// ParseError attempts to parse an API error from a response. Returns nil if status < 400.
func (r *Response) ParseError() *APIError {
	if r.StatusCode < 400 {
		return nil
	}
	var envelope struct {
		Error APIError `json:"error"`
	}
	if json.Unmarshal(r.Body, &envelope) == nil && envelope.Error.Code != "" {
		envelope.Error.StatusCode = r.StatusCode
		return &envelope.Error
	}
	return &APIError{
		StatusCode: r.StatusCode,
		Code:       http.StatusText(r.StatusCode),
		Message:    string(r.Body),
	}
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

// PostMultipart performs a multipart/form-data POST.
func (c *Client) PostMultipart(ctx context.Context, path string, fields map[string]string, files map[string]FileUpload) (*Response, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("writing field %s: %w", k, err)
		}
	}

	for field, file := range files {
		part, err := writer.CreateFormFile(field, file.Filename)
		if err != nil {
			return nil, fmt.Errorf("creating form file %s: %w", field, err)
		}
		if _, err := io.Copy(part, file.Reader); err != nil {
			return nil, fmt.Errorf("copying file %s: %w", field, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return c.PostRaw(ctx, path, writer.FormDataContentType(), &buf)
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

func (c *Client) executeWithRetry(req *http.Request) (*Response, error) {
	var resp *Response
	var err error

	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; attempt <= len(delays); attempt++ {
		resp, err = c.execute(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 500 || resp.StatusCode == http.StatusUnprocessableEntity {
			return resp, nil
		}
		if attempt < len(delays) {
			if c.verbose {
				c.logger.Debug("retrying request", "status", resp.StatusCode, "attempt", attempt+1, "delay", delays[attempt])
			}
			time.Sleep(delays[attempt])
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
	}, nil
}
