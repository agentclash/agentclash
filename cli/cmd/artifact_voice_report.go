package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/cobra"
)

const (
	voiceSchemaLiveContinuityFile   = "voice-live-continuity-report.schema.json"
	voiceSchemaVideoSyncFile        = "voice-video-sync-report.schema.json"
	voiceSchemaSourceSeparationFile = "voice-source-separation-report.schema.json"
)

//go:embed voice_schemas/*.json
var embeddedVoiceSchemas embed.FS

var artifactValidateVoiceReportSchema string

func init() {
	artifactCmd.AddCommand(artifactValidateVoiceReportCmd)
	artifactValidateVoiceReportCmd.Flags().StringVar(&artifactValidateVoiceReportSchema, "schema", "", "JSON Schema path (required when report type cannot be auto-detected)")
}

var artifactValidateVoiceReportCmd = &cobra.Command{
	Use:   "validate-voice-report <file>",
	Short: "Validate a voice report JSON file against an AgentClash schema",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := validateVoiceReportFile(args[0], artifactValidateVoiceReportSchema)
		if err != nil {
			return err
		}

		rc := GetRunContext(cmd)
		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
		}
		rc.Output.PrintDetail("Report", result.Path)
		rc.Output.PrintDetail("Schema", result.Schema)
		rc.Output.PrintDetail("Valid", fmt.Sprintf("%t", result.Valid))
		return nil
	},
}

type voiceReportValidationResult struct {
	Path   string `json:"path"`
	Schema string `json:"schema"`
	Valid  bool   `json:"valid"`
}

func validateVoiceReportFile(reportPath, schemaOverride string) (voiceReportValidationResult, error) {
	report, err := readJSONDocument(reportPath)
	if err != nil {
		return voiceReportValidationResult{}, fmt.Errorf("read report: %w", err)
	}

	schemaPath := strings.TrimSpace(schemaOverride)
	embeddedSchema := false
	if schemaPath == "" {
		schemaPath, err = schemaPathForVoiceReport(report)
		if err != nil {
			return voiceReportValidationResult{}, err
		}
		embeddedSchema = true
	}

	schema, err := readJSONSchema(schemaPath, embeddedSchema)
	if err != nil {
		return voiceReportValidationResult{}, fmt.Errorf("read schema: %w", err)
	}
	if err := schema.Validate(report); err != nil {
		return voiceReportValidationResult{}, fmt.Errorf("voice report schema validation failed: %w", err)
	}

	return voiceReportValidationResult{
		Path:   reportPath,
		Schema: schemaPath,
		Valid:  true,
	}, nil
}

func schemaPathForVoiceReport(report any) (string, error) {
	object, ok := report.(map[string]any)
	if !ok {
		return "", fmt.Errorf("voice report must be a JSON object")
	}
	reportType, _ := object["type"].(string)
	reportType = strings.TrimSpace(reportType)

	var schemaFile string
	switch reportType {
	case "agentclash.voice.live_continuity_eval.v1", "voicey.live_continuity_eval":
		schemaFile = voiceSchemaLiveContinuityFile
	case "agentclash.voice.video_sync_eval.v1", "voicey.video_sync_eval":
		schemaFile = voiceSchemaVideoSyncFile
	case "agentclash.voice.source_separation_eval.v1", "voicey.source_separation_eval":
		schemaFile = voiceSchemaSourceSeparationFile
	case "":
		return "", fmt.Errorf("report type is empty; pass --schema for omitted-type reports")
	default:
		return "", fmt.Errorf("unsupported voice report type %q; pass --schema to validate with an explicit schema", reportType)
	}

	return schemaFile, nil
}

func readJSONSchema(path string, embeddedSchema bool) (*jsonschema.Resolved, error) {
	var (
		data []byte
		err  error
	)
	if embeddedSchema {
		data, err = fs.ReadFile(embeddedVoiceSchemas, filepath.ToSlash(filepath.Join("voice_schemas", path)))
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema.Resolve(nil)
}

func readJSONDocument(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return document, nil
}
