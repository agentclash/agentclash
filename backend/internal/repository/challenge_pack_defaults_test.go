package repository

import (
	"strings"
	"testing"
)

func TestChallengePackDeploymentDefaultsExtractsVersionDefaults(t *testing.T) {
	defaults, err := challengePackDeploymentDefaults([]byte(`{
		"schema_version": 1,
		"version": {
			"deployment_defaults": {
				"aliases": {"candidate": "Candidate Agent"},
				"lineups": {"default": ["candidate"]}
			}
		}
	}`))
	if err != nil {
		t.Fatalf("challengePackDeploymentDefaults returned error: %v", err)
	}
	if !strings.Contains(string(defaults), `"candidate"`) {
		t.Fatalf("defaults = %s, want candidate alias", defaults)
	}
}

func TestChallengePackDeploymentDefaultsRejectsMalformedManifest(t *testing.T) {
	_, err := challengePackDeploymentDefaults([]byte(`{"version":`))
	if err == nil {
		t.Fatal("challengePackDeploymentDefaults returned nil error")
	}
	if !strings.Contains(err.Error(), "decode challenge pack manifest") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}
