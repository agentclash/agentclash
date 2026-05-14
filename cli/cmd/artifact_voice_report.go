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
		rc := GetRunContext(cmd)
		if err != nil {
			if rc.Output.IsStructured() {
				if printErr := rc.Output.PrintRaw(result); printErr != nil {
					return printErr
				}
			}
			return err
		}

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
	Path   string   `json:"path"`
	Schema string   `json:"schema,omitempty"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors,omitempty"`
}

func validateVoiceReportFile(reportPath, schemaOverride string) (voiceReportValidationResult, error) {
	result := voiceReportValidationResult{Path: reportPath}
	report, err := readJSONDocument(reportPath)
	if err != nil {
		err = fmt.Errorf("read report: %w", err)
		result.Errors = []string{err.Error()}
		return result, err
	}

	schemaPath := strings.TrimSpace(schemaOverride)
	embeddedSchema := false
	if schemaPath == "" {
		schemaPath, err = schemaPathForVoiceReport(report)
		if err != nil {
			result.Errors = []string{err.Error()}
			return result, err
		}
		embeddedSchema = true
	}
	result.Schema = schemaPath

	schema, err := readJSONSchema(schemaPath, embeddedSchema)
	if err != nil {
		err = fmt.Errorf("read schema: %w", err)
		result.Errors = []string{err.Error()}
		return result, err
	}
	if err := schema.Validate(report); err != nil {
		err = fmt.Errorf("voice report schema validation failed: %w", err)
		result.Errors = []string{err.Error()}
		return result, err
	}

	result.Valid = true
	return result, nil
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
