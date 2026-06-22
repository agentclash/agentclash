// Package connection is the single, safe interface for connecting LLM providers.
//
// It owns the provider-connection lifecycle end to end: creating a connection
// stores the raw API key in the encrypted workspace-secret vault and records
// only a credential reference; testing, credential resolution, and live model
// listing all go through this one service rather than being smeared across the
// API manager, the credential resolver, the repository, and the provider
// clients. Storage stays in the existing `provider_accounts` table — "connection"
// is the vocabulary of this service layer.
package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// Repository is the narrow slice of persistence the connection service needs.
// *repository.Repository satisfies it.
type Repository interface {
	GetOrganizationIDByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) (uuid.UUID, error)
	CreateProviderAccount(ctx context.Context, p repository.CreateProviderAccountParams) (repository.ProviderAccountRow, error)
	GetProviderAccountByID(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error)
	ListProviderAccountsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error)
	ArchiveProviderAccount(ctx context.Context, id uuid.UUID) error
	UpsertWorkspaceSecret(ctx context.Context, params repository.UpsertWorkspaceSecretParams) error
	LoadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error)
}

// ProviderRouter is the provider-side capability the service depends on.
// provider.Router satisfies it.
type ProviderRouter interface {
	InvokeModel(ctx context.Context, request provider.Request) (provider.Response, error)
	ListModels(ctx context.Context, request provider.ListModelsRequest) ([]provider.ModelInfo, error)
}

// Service is the single entry point for provider connections.
type Service struct {
	repo   Repository
	router ProviderRouter
	cache  *modelsCache
}

func NewService(repo Repository, router ProviderRouter) *Service {
	return &Service{repo: repo, router: router, cache: newModelsCache()}
}

// CreateConnectionInput describes a new provider connection. Exactly one of
// APIKey (a raw key to store in the vault) or CredentialReference (a pointer to
// an existing secret/env reference) must be set.
type CreateConnectionInput struct {
	ProviderKey         string
	Name                string
	CredentialReference string
	APIKey              string
	LimitsConfig        json.RawMessage
	ActorUserID         *uuid.UUID
}

// TestInput parameterizes a connection smoke test.
type TestInput struct {
	Model              string
	StepTimeoutSeconds int32
}

// TestResult is the provider-agnostic outcome of a connection smoke test.
type TestResult struct {
	AccountID       uuid.UUID
	ProviderKey     string
	Model           string
	ProviderModelID string
	Passed          bool
	Status          string
	Code            string
	Message         string
	Retryable       bool
	DurationMS      int64
}

// Create stores a connection. A raw API key is written to the encrypted
// workspace-secret vault and replaced with a credential reference pointing at it.
func (s *Service) Create(ctx context.Context, workspaceID uuid.UUID, input CreateConnectionInput) (repository.ProviderAccountRow, error) {
	orgID, err := s.repo.GetOrganizationIDByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return repository.ProviderAccountRow{}, fmt.Errorf("resolve org: %w", err)
	}

	credRef := input.CredentialReference
	if input.APIKey != "" {
		secretKey := providerSecretKey(input.ProviderKey)
		if err := s.repo.UpsertWorkspaceSecret(ctx, repository.UpsertWorkspaceSecretParams{
			WorkspaceID: workspaceID,
			Key:         secretKey,
			Value:       input.APIKey,
			ActorUserID: input.ActorUserID,
		}); err != nil {
			return repository.ProviderAccountRow{}, fmt.Errorf("store api key as workspace secret: %w", err)
		}
		credRef = workspaceSecretScheme + secretKey
	}

	return s.repo.CreateProviderAccount(ctx, repository.CreateProviderAccountParams{
		OrganizationID:      orgID,
		WorkspaceID:         workspaceID,
		ProviderKey:         input.ProviderKey,
		Name:                input.Name,
		CredentialReference: credRef,
		LimitsConfig:        input.LimitsConfig,
	})
}

func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]repository.ProviderAccountRow, error) {
	return s.repo.ListProviderAccountsByWorkspaceID(ctx, workspaceID)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (repository.ProviderAccountRow, error) {
	return s.repo.GetProviderAccountByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	s.cache.invalidate(id)
	return s.repo.ArchiveProviderAccount(ctx, id)
}

// ResolveCredentialContext returns a context with the connection's workspace
// secrets injected, ready for a provider call. Provider clients resolve the
// connection's credential reference against these secrets.
func (s *Service) ResolveCredentialContext(ctx context.Context, account repository.ProviderAccountRow) (context.Context, error) {
	resolved, _, err := s.credentialContext(ctx, account)
	return resolved, err
}

// credentialContext loads the workspace secrets a connection's credential
// reference needs and injects them into the returned context. It also returns
// the set of secret values for log redaction.
func (s *Service) credentialContext(ctx context.Context, account repository.ProviderAccountRow) (context.Context, []string, error) {
	secrets := map[string]string{}
	if account.WorkspaceID != nil && strings.HasPrefix(account.CredentialReference, workspaceSecretScheme) {
		loaded, err := s.repo.LoadWorkspaceSecrets(ctx, *account.WorkspaceID)
		if err != nil {
			return nil, nil, fmt.Errorf("load workspace secrets: %w", err)
		}
		secrets = loaded
	}
	resolved := provider.WithWorkspaceSecrets(ctx, secrets)
	return resolved, redactionValues(resolved, secrets, account.CredentialReference), nil
}

// ListModels returns the models reachable with a connection's credential. Results
// are cached per account (~1h); on a live fetch failure a stale cached list is
// served when available.
func (s *Service) ListModels(ctx context.Context, account repository.ProviderAccountRow) ([]provider.ModelInfo, error) {
	if cached, ok := s.cache.get(account.ID); ok {
		return cached, nil
	}
	resolvedCtx, _, err := s.credentialContext(ctx, account)
	if err != nil {
		return nil, err
	}
	models, err := s.router.ListModels(resolvedCtx, provider.ListModelsRequest{
		ProviderKey:         account.ProviderKey,
		CredentialReference: account.CredentialReference,
	})
	if err != nil {
		if stale, ok := s.cache.getStale(account.ID); ok {
			return stale, nil
		}
		return nil, err
	}
	s.cache.set(account.ID, models)
	return models, nil
}

// Test runs a minimal provider round-trip to confirm a connection works.
func (s *Service) Test(ctx context.Context, account repository.ProviderAccountRow, input TestInput) (TestResult, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = defaultSmokeModel(account.ProviderKey)
	}
	result := TestResult{
		AccountID:   account.ID,
		ProviderKey: account.ProviderKey,
		Model:       model,
		Passed:      false,
		Status:      "failed",
	}

	if s.router == nil {
		result.Code = string(provider.FailureCodeUnsupportedProvider)
		result.Message = "provider smoke test client is not configured"
		return result, nil
	}
	if model == "" {
		result.Code = string(provider.FailureCodeUnsupportedProvider)
		result.Message = fmt.Sprintf("no default smoke-test model is configured for provider %q; pass --model", account.ProviderKey)
		return result, nil
	}
	if account.WorkspaceID == nil {
		result.Code = "invalid_provider_account"
		result.Message = "provider account is not attached to a workspace"
		return result, nil
	}
	if account.Status != "" && account.Status != "active" {
		result.Code = "inactive_provider_account"
		result.Message = fmt.Sprintf("provider account status is %q", account.Status)
		return result, nil
	}

	timeout := testTimeout(input.StepTimeoutSeconds)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resolvedCtx, redaction, err := s.credentialContext(runCtx, account)
	if err != nil {
		return TestResult{}, err
	}

	startedAt := time.Now()
	response, err := s.router.InvokeModel(resolvedCtx, provider.Request{
		ProviderKey:         account.ProviderKey,
		ProviderAccountID:   account.ID.String(),
		CredentialReference: account.CredentialReference,
		Model:               model,
		TraceMode:           "optional",
		StepTimeout:         timeout,
		Messages: []provider.Message{
			{Role: "user", Content: "Reply with exactly: agentclash-smoke-ok"},
		},
	})
	result.DurationMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		return failureResult(result, err, redaction, account.CredentialReference), nil
	}

	result.Passed = true
	result.Status = "passed"
	result.ProviderModelID = response.ProviderModelID
	result.Message = "provider account smoke test passed"
	return result, nil
}

const workspaceSecretScheme = "workspace-secret://"

func providerSecretKey(providerKey string) string {
	return fmt.Sprintf("PROVIDER_%s_API_KEY", strings.ToUpper(strings.ReplaceAll(providerKey, "-", "_")))
}

func testTimeout(seconds int32) time.Duration {
	if seconds <= 0 {
		return 20 * time.Second
	}
	if seconds > 30 {
		return 30 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func defaultSmokeModel(providerKey string) string {
	switch strings.TrimSpace(providerKey) {
	case "openai":
		return "gpt-4.1-mini"
	case "anthropic":
		return "claude-haiku-4-5-20251001"
	case "gemini":
		return "gemini-2.0-flash"
	case "xai":
		return "grok-4-1-fast-reasoning"
	case "openrouter":
		return "openai/gpt-4.1-mini"
	case "mistral":
		return "mistral-small-latest"
	default:
		return ""
	}
}

func failureResult(result TestResult, err error, redaction []string, credentialReference string) TestResult {
	result.Passed = false
	result.Status = "failed"
	if errors.Is(err, context.DeadlineExceeded) {
		result.Code = string(provider.FailureCodeTimeout)
		result.Message = "provider account smoke test timed out"
		return result
	}
	if failure, ok := provider.AsFailure(err); ok {
		result.Code = string(failure.Code)
		result.Message = sanitizeMessage(failure.Message, redaction, credentialReference)
		result.Retryable = failure.Retryable
		return result
	}
	result.Code = string(provider.FailureCodeUnknown)
	result.Message = sanitizeMessage(err.Error(), redaction, credentialReference)
	return result
}

func redactionValues(ctx context.Context, secrets map[string]string, credentialReference string) []string {
	values := make([]string, 0, len(secrets)+1)
	for _, value := range secrets {
		if value != "" {
			values = append(values, value)
		}
	}
	if resolved, err := (provider.EnvCredentialResolver{}).Resolve(ctx, credentialReference); err == nil && resolved != "" {
		values = append(values, resolved)
	}
	return values
}

func sanitizeMessage(message string, redaction []string, credentialReference string) string {
	sanitized := message
	for _, value := range redaction {
		sanitized = strings.ReplaceAll(sanitized, value, "[redacted]")
	}
	if credentialReference != "" {
		sanitized = strings.ReplaceAll(sanitized, credentialReference, "[credential-reference]")
	}
	return sanitized
}
