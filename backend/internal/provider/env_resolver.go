package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type EnvCredentialResolver struct{}

func (EnvCredentialResolver) Resolve(_ context.Context, credentialReference string) (string, error) {
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
