package workflow

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GitHubPullRequestClient interface {
	CreateInstallationToken(ctx context.Context, installationID int64) (string, error)
	CreatePullRequest(ctx context.Context, input CreateGitHubPullRequestInput) (GitHubPullRequest, error)
}

type GitHubAppClientConfig struct {
	AppID         int64
	PrivateKeyPEM string
	APIBaseURL    string
	HTTPClient    *http.Client
}

type CreateGitHubPullRequestInput struct {
	Token string
	Owner string
	Repo  string
	Title string
	Head  string
	Base  string
	Body  string
	Draft bool
}

type GitHubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Draft   bool   `json:"draft"`
}

type githubAppClient struct {
	appID      int64
	privateKey *rsa.PrivateKey
	apiBaseURL string
	httpClient *http.Client
}

type githubAccessTokenResponse struct {
	Token string `json:"token"`
}

func NewGitHubAppClient(config GitHubAppClientConfig) (GitHubPullRequestClient, error) {
	if config.AppID <= 0 {
		return nil, errors.New("github app id is required")
	}
	key, err := parseGitHubAppPrivateKey(config.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	apiBaseURL := strings.TrimRight(config.APIBaseURL, "/")
	if apiBaseURL == "" {
		apiBaseURL = "https://api.github.com"
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &githubAppClient{
		appID:      config.AppID,
		privateKey: key,
		apiBaseURL: apiBaseURL,
		httpClient: httpClient,
	}, nil
}

func parseGitHubAppPrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, errors.New("github app private key is not PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app private key is not RSA")
	}
	return key, nil
}

func (c *githubAppClient) CreateInstallationToken(ctx context.Context, installationID int64) (string, error) {
	var response githubAccessTokenResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installationID), "", bytes.NewReader([]byte(`{}`)), &response); err != nil {
		return "", err
	}
	if response.Token == "" {
		return "", errors.New("github installation token response was empty")
	}
	return response.Token, nil
}

func (c *githubAppClient) CreatePullRequest(ctx context.Context, input CreateGitHubPullRequestInput) (GitHubPullRequest, error) {
	var response GitHubPullRequest
	body, err := json.Marshal(map[string]any{
		"title": input.Title,
		"head":  input.Head,
		"base":  input.Base,
		"body":  input.Body,
		"draft": input.Draft,
	})
	if err != nil {
		return GitHubPullRequest{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls", input.Owner, input.Repo)
	if err := c.doJSON(ctx, http.MethodPost, path, input.Token, bytes.NewReader(body), &response); err != nil {
		return GitHubPullRequest{}, err
	}
	return response, nil
}

func (c *githubAppClient) doJSON(ctx context.Context, method string, path string, bearerToken string, body io.Reader, out any) error {
	token := bearerToken
	if token == "" {
		appToken, err := c.appJWT()
		if err != nil {
			return err
		}
		token = appToken
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiBaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("github api %s %s returned %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *githubAppClient) appJWT() (string, error) {
	now := time.Now().UTC()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": c.appID,
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claims)
	signingInput := header + "." + payload
	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
