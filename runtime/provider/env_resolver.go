package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// workspaceSecretsKey is the context key for workspace secrets.
type workspaceSecretsKey struct{}

// WithWorkspaceSecrets injects decrypted workspace secrets into the context
// so the credential resolver can access them for workspace-secret:// references.
func WithWorkspaceSecrets(ctx context.Context, secrets map[string]string) context.Context {
	return context.WithValue(ctx, workspaceSecretsKey{}, secrets)
}

func PrepareCredentialContext(ctx context.Context, credentialReference string, loadWorkspaceSecrets func() (map[string]string, error)) (context.Context, error) {
	if !strings.HasPrefix(credentialReference, "workspace-secret://") {
		return ctx, nil
	}
	if loadWorkspaceSecrets == nil {
		return nil, NewFailure(
			"",
			FailureCodeCredentialUnavailable,
			fmt.Sprintf("workspace secrets not available for %q", credentialReference),
			false,
			ErrCredentialUnavailable,
		)
	}

	secrets, err := loadWorkspaceSecrets()
	if err != nil {
		return nil, err
	}
	return WithWorkspaceSecrets(ctx, secrets), nil
}

func workspaceSecretsFromContext(ctx context.Context) map[string]string {
	if s, ok := ctx.Value(workspaceSecretsKey{}).(map[string]string); ok {
		return s
	}
	return nil
}

type EnvCredentialResolver struct{}

func (EnvCredentialResolver) Resolve(ctx context.Context, credentialReference string) (string, error) {
	// workspace-secret:// resolves from workspace secrets stored in context.
	if strings.HasPrefix(credentialReference, "workspace-secret://") {
		key := strings.TrimPrefix(credentialReference, "workspace-secret://")
		secrets := workspaceSecretsFromContext(ctx)
		if secrets == nil {
			return "", NewFailure(
				"",
				FailureCodeCredentialUnavailable,
				fmt.Sprintf("workspace secrets not available for %q", credentialReference),
				false,
				ErrCredentialUnavailable,
			)
		}
		value, ok := secrets[key]
		if !ok || value == "" {
			return "", NewFailure(
				"",
				FailureCodeCredentialUnavailable,
				fmt.Sprintf("workspace secret %q not found", key),
				false,
				ErrCredentialUnavailable,
			)
		}
		return value, nil
	}

	candidates, err := candidateEnvVars(credentialReference)
	if err != nil {
		return "", err
	}

	for _, envVar := range candidates {
		value, ok := os.LookupEnv(envVar)
		if ok && value != "" {
			return value, nil
		}
	}

	return "", NewFailure(
		"",
		FailureCodeCredentialUnavailable,
		fmt.Sprintf("credential env var for %q is not set; tried %s", credentialReference, strings.Join(candidates, ", ")),
		false,
		ErrCredentialUnavailable,
	)
}

func candidateEnvVars(credentialReference string) ([]string, error) {
	switch {
	case strings.HasPrefix(credentialReference, "env://"):
		return []string{strings.TrimPrefix(credentialReference, "env://")}, nil
	case strings.HasPrefix(credentialReference, "secret://"):
		secretName := strings.TrimPrefix(credentialReference, "secret://")
		normalized := normalizeSecretName(secretName)
		return []string{
			"AGENTCLASH_SECRET_" + normalized,
			normalized,
			normalized + "_API_KEY",
		}, nil
	default:
		return nil, NewFailure(
			"",
			FailureCodeCredentialUnavailable,
			fmt.Sprintf("credential reference %q is not supported by the env resolver", credentialReference),
			false,
			ErrCredentialUnavailable,
		)
	}
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

func normalizeSecretName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(trimmed), "_"), "_")
}
