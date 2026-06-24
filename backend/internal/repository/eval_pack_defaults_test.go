package repository

import (
	"strings"
	"testing"
)

func TestEvalPackDeploymentDefaultsExtractsVersionDefaults(t *testing.T) {
	defaults, err := evalPackDeploymentDefaults([]byte(`{
		"schema_version": 1,
		"version": {
			"deployment_defaults": {
				"aliases": {"candidate": "Candidate Agent"},
				"lineups": {"default": ["candidate"]}
			}
		}
	}`))
	if err != nil {
		t.Fatalf("evalPackDeploymentDefaults returned error: %v", err)
	}
	if !strings.Contains(string(defaults), `"candidate"`) {
		t.Fatalf("defaults = %s, want candidate alias", defaults)
	}
}

func TestEvalPackDeploymentDefaultsRejectsMalformedManifest(t *testing.T) {
	_, err := evalPackDeploymentDefaults([]byte(`{"version":`))
	if err == nil {
		t.Fatal("evalPackDeploymentDefaults returned nil error")
	}
	if !strings.Contains(err.Error(), "decode eval pack manifest") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
}

func TestEvalPackVersionMetadataExtractsVoiceHints(t *testing.T) {
	metadata, err := evalPackVersionMetadata([]byte(`{
		"schema_version": 1,
		"modality": " voice ",
		"interface_spec": {
			"transports": [" text_sim ", "sip", ""]
		},
		"version": {
			"deployment_defaults": {
				"aliases": {"candidate": "Candidate Agent"}
			}
		}
	}`))
	if err != nil {
		t.Fatalf("evalPackVersionMetadata returned error: %v", err)
	}
	if metadata.Modality != "voice" {
		t.Fatalf("modality = %q, want voice", metadata.Modality)
	}
	if len(metadata.InterfaceTransports) != 2 || metadata.InterfaceTransports[0] != "text_sim" || metadata.InterfaceTransports[1] != "sip" {
		t.Fatalf("interface transports = %#v, want [text_sim sip]", metadata.InterfaceTransports)
	}
	if !strings.Contains(string(metadata.DeploymentDefaults), `"candidate"`) {
		t.Fatalf("deployment defaults = %s, want candidate alias", metadata.DeploymentDefaults)
	}
}
