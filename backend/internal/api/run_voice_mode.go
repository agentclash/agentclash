package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/runtime/challengepack"
)

const (
	runModeTextSim      = "text-sim"
	runModeAudioSim     = "audio-sim"
	runModeLiveCall     = "live-call"
	runModeReplayImport = "replay-import"

	runVoiceTransportTextSim = "text_sim"
)

type runVoiceMode struct {
	Mode      string
	Modality  string
	Transport string
}

type runVoiceMetadataResponse struct {
	Mode      string `json:"mode"`
	Modality  string `json:"modality"`
	Transport string `json:"transport,omitempty"`
}

func normalizeRunMode(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validateRequestedRunMode(mode string) error {
	switch mode {
	case "":
		return nil
	case runModeTextSim:
		return nil
	case runModeAudioSim, runModeLiveCall, runModeReplayImport:
		return RunCreationValidationError{
			Code:    "unsupported_mode",
			Message: fmt.Sprintf("mode %q is reserved for future voice eval support; supported mode is %s", mode, runModeTextSim),
		}
	default:
		return RunCreationValidationError{
			Code:    "unsupported_mode",
			Message: fmt.Sprintf("mode must be %s; %s, %s, and %s are reserved for future voice eval support", runModeTextSim, runModeAudioSim, runModeLiveCall, runModeReplayImport),
		}
	}
}

func resolveRunVoiceMode(mode string, manifest json.RawMessage) (*runVoiceMode, error) {
	if err := validateRequestedRunMode(mode); err != nil {
		return nil, err
	}
	if mode == "" {
		return nil, nil
	}

	var document struct {
		Modality      string `json:"modality"`
		InterfaceSpec struct {
			Transports []string `json:"transports"`
		} `json:"interface_spec"`
	}
	if len(manifest) > 0 {
		if err := json.Unmarshal(manifest, &document); err != nil {
			return nil, fmt.Errorf("decode challenge pack manifest voice metadata: %w", err)
		}
	}

	if strings.TrimSpace(document.Modality) != challengepack.ModalityVoice {
		return nil, RunCreationValidationError{
			Code:    "incompatible_mode",
			Message: fmt.Sprintf("mode %s requires a voice challenge pack", runModeTextSim),
		}
	}
	if !containsTrimmedString(document.InterfaceSpec.Transports, runVoiceTransportTextSim) {
		return nil, RunCreationValidationError{
			Code:    "incompatible_mode",
			Message: fmt.Sprintf("mode %s requires voice transport %s", runModeTextSim, runVoiceTransportTextSim),
		}
	}

	return &runVoiceMode{
		Mode:      mode,
		Modality:  challengepack.ModalityVoice,
		Transport: runVoiceTransportTextSim,
	}, nil
}

func containsTrimmedString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func runMetadataFromExecutionPlan(executionPlan json.RawMessage) (string, string, *runVoiceMetadataResponse) {
	var plan struct {
		Mode     string                    `json:"mode"`
		Modality string                    `json:"modality"`
		Voice    *runVoiceMetadataResponse `json:"voice"`
	}
	if len(executionPlan) == 0 {
		return "", "", nil
	}
	if err := json.Unmarshal(executionPlan, &plan); err != nil {
		return "", "", nil
	}

	mode := strings.TrimSpace(plan.Mode)
	modality := strings.TrimSpace(plan.Modality)
	if plan.Voice == nil {
		if mode == "" && modality == "" {
			return "", "", nil
		}
		return mode, modality, nil
	}

	voice := *plan.Voice
	voice.Mode = strings.TrimSpace(voice.Mode)
	voice.Modality = strings.TrimSpace(voice.Modality)
	voice.Transport = strings.TrimSpace(voice.Transport)
	if mode == "" {
		mode = voice.Mode
	}
	if modality == "" {
		modality = voice.Modality
	}
	return mode, modality, &voice
}
