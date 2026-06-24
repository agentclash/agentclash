package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

const voiceEvalModality = "voice"

func normalizeVoiceValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func humanVoiceMode(value string) string {
	switch normalizeVoiceValue(value) {
	case runCreateModeTextSim:
		return "Text simulation"
	case runCreateModeAudioSim:
		return "Audio simulation"
	case runCreateModeLiveCall:
		return "Live call"
	case runCreateModeReplayImport:
		return "Replay import"
	default:
		if strings.TrimSpace(value) == "" {
			return "Voice"
		}
		return strings.TrimSpace(value)
	}
}

func voiceModeSummary(mode, transport string) string {
	parts := []string{voiceEvalModality}
	if strings.TrimSpace(mode) != "" {
		parts = append(parts, humanVoiceMode(mode))
	}
	if strings.TrimSpace(transport) != "" {
		parts = append(parts, strings.TrimSpace(transport))
	}
	return strings.Join(parts, " / ")
}

func voiceRunModality(run map[string]any) string {
	if voice := mapObject(run, "voice"); voice != nil {
		if modality := mapString(voice, "modality"); modality != "" {
			return modality
		}
	}
	return mapString(run, "modality")
}

func voiceRunMode(run map[string]any) string {
	if voice := mapObject(run, "voice"); voice != nil {
		if mode := mapString(voice, "mode"); mode != "" {
			return mode
		}
	}
	return mapString(run, "mode")
}

func voiceRunTransport(run map[string]any) string {
	if voice := mapObject(run, "voice"); voice != nil {
		return mapString(voice, "transport")
	}
	return ""
}

func voiceRunSummary(run map[string]any) string {
	modality := voiceRunModality(run)
	mode := voiceRunMode(run)
	transport := voiceRunTransport(run)
	if normalizeVoiceValue(modality) != voiceEvalModality && mode == "" && transport == "" {
		return ""
	}
	if normalizeVoiceValue(modality) != voiceEvalModality {
		parts := []string{strings.TrimSpace(modality)}
		if mode != "" {
			parts = append(parts, humanVoiceMode(mode))
		}
		if transport != "" {
			parts = append(parts, strings.TrimSpace(transport))
		}
		return strings.Join(compactNonEmptyStrings(parts), " / ")
	}
	return voiceModeSummary(mode, transport)
}

func runModeSummary(run map[string]any) string {
	executionMode := mapString(run, "execution_mode")
	voiceSummary := voiceRunSummary(run)
	switch {
	case executionMode != "" && voiceSummary != "":
		return fmt.Sprintf("%s; %s", executionMode, voiceSummary)
	case voiceSummary != "":
		return voiceSummary
	case executionMode != "":
		return executionMode
	default:
		return "-"
	}
}

func latestEvalPackVersionMap(pack map[string]any) map[string]any {
	versions := mapSlice(pack, "versions")
	if len(versions) == 0 {
		return nil
	}
	var latest map[string]any
	for _, raw := range versions {
		version, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if latest == nil || versionNumber(version["version_number"]) > versionNumber(latest["version_number"]) {
			latest = version
		}
	}
	return latest
}

func evalPackMapModalitySummary(pack map[string]any) string {
	version := latestEvalPackVersionMap(pack)
	if version == nil {
		return "-"
	}
	modality := mapString(version, "modality")
	transports := mapStringSlice(version, "interface_transports")
	if normalizeVoiceValue(modality) != voiceEvalModality {
		if strings.TrimSpace(modality) != "" {
			return strings.TrimSpace(modality)
		}
		return "-"
	}
	if len(transports) == 0 {
		return voiceEvalModality
	}
	return voiceEvalModality + " / " + strings.Join(transports, ", ")
}

func versionNumber(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case float64:
		return typed
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		return 0
	}
}

func evalPackHasVoiceVersion(pack evalPackSummary) bool {
	for _, version := range pack.Versions {
		if normalizeVoiceValue(version.Modality) == voiceEvalModality {
			return true
		}
	}
	return false
}

func evalPackTransportSummary(pack evalPackSummary) string {
	seen := map[string]bool{}
	var transports []string
	for _, version := range pack.Versions {
		if normalizeVoiceValue(version.Modality) != voiceEvalModality {
			continue
		}
		for _, transport := range version.InterfaceTransports {
			normalized := strings.TrimSpace(transport)
			if normalized == "" || seen[normalized] {
				continue
			}
			seen[normalized] = true
			transports = append(transports, normalized)
		}
	}
	return strings.Join(transports, ", ")
}

func evalPackPickerLabel(pack evalPackSummary) string {
	if evalPackHasVoiceVersion(pack) {
		return strings.TrimSpace(pack.Name) + " (voice)"
	}
	return pack.Name
}

func evalPackPickerDescription(pack evalPackSummary) string {
	description := fmt.Sprintf("%d runnable version(s) • %s", len(pack.Versions), pack.ID)
	if transports := evalPackTransportSummary(pack); transports != "" {
		description += " • " + transports
	}
	return description
}

func evalPackVersionPickerLabel(version evalPackVersionBrief) string {
	label := fmt.Sprintf("v%d", version.VersionNumber)
	if normalizeVoiceValue(version.Modality) == voiceEvalModality {
		label += " (voice)"
	}
	return label
}

func evalPackVersionPickerDescription(version evalPackVersionBrief) string {
	description := fmt.Sprintf("status: %s • %s", version.LifecycleStatus, version.ID)
	if len(version.InterfaceTransports) > 0 {
		description += " • " + strings.Join(version.InterfaceTransports, ", ")
	}
	return description
}

func versionSupportsTextSimBrief(version evalPackVersionBrief) bool {
	if normalizeVoiceValue(version.Modality) != voiceEvalModality {
		return false
	}
	for _, transport := range version.InterfaceTransports {
		if normalizeVoiceValue(transport) == "text_sim" {
			return true
		}
	}
	return false
}

func suggestedRunModeForVersion(version evalPackVersionBrief) string {
	if versionSupportsTextSimBrief(version) {
		return runCreateModeTextSim
	}
	return ""
}
