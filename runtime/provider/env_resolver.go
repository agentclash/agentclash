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
// A workspace-scoped context never falls back to process credentials, even
// when its vault is nil or empty.
func WithWorkspaceSecrets(ctx context.Context, secrets map[string]string) context.Context {
	return context.WithValue(ctx, workspaceSecretsKey{}, secrets)
}

func PrepareCredentialContext(ctx context.Context, credentialReference string, loadWorkspaceSecrets func() (map[string]string, error)) (context.Context, error) {
	if err := ValidateWorkspaceCredentialReference(credentialReference); err != nil {
		return nil, err
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
	// Workspace contexts are an exclusive credential boundary, not a fallback.
	_, workspaceScoped := ctx.Value(workspaceSecretsKey{}).(map[string]string)
	if workspaceScoped || strings.HasPrefix(credentialReference, "workspace-secret://") {
		if err := ValidateWorkspaceCredentialReference(credentialReference); err != nil {
			return "", err
		}
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

	candidates, err := CandidateEnvVars(credentialReference)
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

// CandidateEnvVars returns the process-environment variable names to try for
// env:// and secret:// credential references. Local and hosted resolvers share
// this expansion so secret-candidate forms stay in lockstep.
//
// For secret://name the candidates are AGENTCLASH_SECRET_<NORM>, <NORM>, and
// <NORM>_API_KEY where <NORM> is the uppercased alphanumeric form of name.
func CandidateEnvVars(credentialReference string) ([]string, error) {
	switch {
	case strings.HasPrefix(credentialReference, "env://"):
		return []string{strings.TrimPrefix(credentialReference, "env://")}, nil
	case strings.HasPrefix(credentialReference, "secret://"):
		secretName := strings.TrimPrefix(credentialReference, "secret://")
		normalized := NormalizeSecretName(secretName)
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

// NormalizeSecretName uppercases a secret name and replaces non-alphanumeric
// runs with underscores (used by secret:// env-candidate expansion).
func NormalizeSecretName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(nonAlnum.ReplaceAllString(strings.ToUpper(trimmed), "_"), "_")
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]+`)

// ValidateWorkspaceCredentialReference accepts only a named workspace vault key.
// Its key syntax matches the workspace secret API and database constraint.
func ValidateWorkspaceCredentialReference(reference string) error {
	const scheme = "workspace-secret://"
	key := strings.TrimPrefix(reference, scheme)
	if !strings.HasPrefix(reference, scheme) || len(key) == 0 || len(key) > 128 || !workspaceSecretName.MatchString(key) {
		return NewFailure("", FailureCodeCredentialUnavailable, "a valid workspace-secret:// credential reference is required", false, ErrCredentialUnavailable)
	}
	return nil
}

var workspaceSecretName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
