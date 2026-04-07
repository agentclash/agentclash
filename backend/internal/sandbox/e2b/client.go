package e2b

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/filesystem/filesystemconnect"
	"github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/process/processconnect"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

type apiClient struct {
	httpClient *http.Client
	config     Config
}

type sandboxRecord struct {
	SandboxID        string  `json:"sandboxID"`
	TemplateID       string  `json:"templateID"`
	EnvdVersion      string  `json:"envdVersion"`
	Domain           *string `json:"domain"`
	EnvdAccessToken  string  `json:"envdAccessToken"`
	TrafficAuthToken *string `json:"trafficAccessToken"`
}

func newAPIClient(config Config) *apiClient {
	return &apiClient{
		httpClient: &http.Client{Timeout: config.requestTimeout()},
		config:     config,
	}
}

func (c *apiClient) createSandbox(ctx context.Context, request createSandboxRequest) (sandboxRecord, error) {
	var record sandboxRecord
	if err := c.doJSON(ctx, http.MethodPost, c.config.apiBaseURL()+"/sandboxes", request, &record, nil, nil); err != nil {
		return sandboxRecord{}, err
	}
	return record, nil
}

func (c *apiClient) destroySandbox(ctx context.Context, sandboxID string) error {
	return c.doJSON(ctx, http.MethodDelete, c.config.apiBaseURL()+"/sandboxes/"+sandboxID, nil, nil, map[int]struct{}{http.StatusNoContent: {}}, sandbox.ErrSandboxNotFound)
}

func (c *apiClient) envdBaseURL(record sandboxRecord) string {
	domain := defaultDomain
	if record.Domain != nil && strings.TrimSpace(*record.Domain) != "" {
		domain = strings.TrimSpace(*record.Domain)
	}
	return fmt.Sprintf("https://%d-%s.%s", defaultEnvdPort, record.SandboxID, domain)
}

func (c *apiClient) filesystemClient(record sandboxRecord) filesystemconnect.FilesystemClient {
	return filesystemconnect.NewFilesystemClient(c.httpClient, c.envdBaseURL(record))
}

func (c *apiClient) processClient(record sandboxRecord) processconnect.ProcessClient {
	return processconnect.NewProcessClient(c.httpClient, c.envdBaseURL(record))
}

func (c *apiClient) readFile(ctx context.Context, record sandboxRecord, filePath string) ([]byte, error) {
	values := url.Values{}
	values.Set("path", filePath)
	values.Set("username", defaultSandboxUser)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.envdBaseURL(record)+"/files?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c.setEnvdHeaders(req.Header, record)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, normalizeHTTPError(resp.StatusCode, string(body), sandbox.ErrFileNotFound)
	}
	return body, nil
}

func (c *apiClient) writeFile(ctx context.Context, record sandboxRecord, filePath string, content []byte) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", path.Base(strings.TrimSpace(filePath)))
	if err != nil {
		return err
	}
	if _, err := part.Write(content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	values := url.Values{}
	values.Set("path", filePath)
	values.Set("username", defaultSandboxUser)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.envdBaseURL(record)+"/files?"+values.Encode(), &body)
	if err != nil {
		return err
	}
	c.setEnvdHeaders(req.Header, record)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return normalizeHTTPError(resp.StatusCode, string(respBody), nil)
	}
	return nil
}

func (c *apiClient) setEnvdHeaders(header http.Header, record sandboxRecord) {
	header.Set("X-Access-Token", record.EnvdAccessToken)
	header.Set("E2b-Sandbox-Id", record.SandboxID)
	header.Set("E2b-Sandbox-Port", strconv.Itoa(defaultEnvdPort))
}

func (c *apiClient) authHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(defaultSandboxUser+":"))
}

func (c *apiClient) doJSON(ctx context.Context, method string, rawURL string, requestBody any, responseBody any, allowedEmptyStatuses map[int]struct{}, notFoundErr error) error {
	var body io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return err
		}
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-KEY", c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if _, ok := allowedEmptyStatuses[resp.StatusCode]; ok {
		return nil
	}
	if resp.StatusCode >= 300 {
		return normalizeHTTPError(resp.StatusCode, string(respBytes), notFoundErr)
	}
	if responseBody == nil {
		return nil
	}
	return json.Unmarshal(respBytes, responseBody)
}

type createSandboxRequest struct {
	TemplateID          string            `json:"templateID"`
	Timeout             int               `json:"timeout"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	Secure              bool              `json:"secure"`
	AllowInternetAccess bool              `json:"allowInternetAccess"`
	EnvVars             map[string]string `json:"envVars,omitempty"`
	Network             *networkConfig    `json:"network,omitempty"`
}

type networkConfig struct {
	AllowOut []string `json:"allowOut,omitempty"`
}
