package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Atharva-Kanherkar/agentclash/cli/internal/api"
	"github.com/Atharva-Kanherkar/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(artifactCmd)
	artifactCmd.AddCommand(artifactUploadCmd)
	artifactCmd.AddCommand(artifactDownloadCmd)

	artifactUploadCmd.Flags().String("type", "", "Artifact type (required)")
	artifactUploadCmd.Flags().String("run", "", "Run ID (optional)")
	artifactUploadCmd.Flags().String("run-agent", "", "Run agent ID (optional)")
	artifactUploadCmd.Flags().String("metadata", "", "JSON metadata (optional)")
	artifactUploadCmd.MarkFlagRequired("type")

	artifactDownloadCmd.Flags().StringP("output", "O", "", "Output file path (defaults to stdout)")
}

var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Upload and download artifacts",
}

var artifactUploadCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload an artifact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		wsID := RequireWorkspace(cmd)

		filePath := args[0]
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()

		artifactType, _ := cmd.Flags().GetString("type")
		fields := map[string]string{
			"artifact_type": artifactType,
		}
		if v, _ := cmd.Flags().GetString("run"); v != "" {
			fields["run_id"] = v
		}
		if v, _ := cmd.Flags().GetString("run-agent"); v != "" {
			fields["run_agent_id"] = v
		}
		if v, _ := cmd.Flags().GetString("metadata"); v != "" {
			fields["metadata"] = v
		}

		sp := output.NewSpinner("Uploading artifact...", flagQuiet)

		files := map[string]api.FileUpload{
			"file": {
				Filename: filepath.Base(filePath),
				Reader:   f,
			},
		}

		resp, err := rc.Client.PostMultipart(cmd.Context(), "/v1/workspaces/"+wsID+"/artifacts", fields, files)
		if err != nil {
			sp.StopWithError("Upload failed")
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			sp.StopWithError("Upload failed")
			return apiErr
		}

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			return err
		}

		sp.StopWithSuccess("Uploaded")

		if rc.Output.IsJSON() {
			return rc.Output.PrintJSON(result)
		}

		rc.Output.PrintDetail("Artifact ID", str(result["id"]))
		rc.Output.PrintDetail("Type", artifactType)
		rc.Output.PrintDetail("Size", str(result["size_bytes"]))
		return nil
	},
}

var artifactDownloadCmd = &cobra.Command{
	Use:   "download <artifactId>",
	Short: "Download an artifact",
	Long:  "Downloads an artifact. Outputs to stdout by default (pipeable).\nUse -O to save to a file.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)

		// Step 1: Get the signed download URL.
		resp, err := rc.Client.Get(cmd.Context(), "/v1/artifacts/"+args[0]+"/download", nil)
		if err != nil {
			return err
		}
		if apiErr := resp.ParseError(); apiErr != nil {
			return apiErr
		}

		var dlResp struct {
			URL       string `json:"url"`
			ExpiresAt string `json:"expires_at"`
		}
		if err := resp.DecodeJSON(&dlResp); err != nil {
			return err
		}

		// Step 2: Download the content from the signed URL.
		httpResp, err := http.Get(dlResp.URL)
		if err != nil {
			return fmt.Errorf("downloading artifact: %w", err)
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode != http.StatusOK {
			return fmt.Errorf("download failed: HTTP %d", httpResp.StatusCode)
		}

		// Step 3: Write to output.
		outPath, _ := cmd.Flags().GetString("output")
		var w io.Writer
		if outPath != "" {
			f, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("creating output file: %w", err)
			}
			defer f.Close()
			w = f
		} else {
			w = os.Stdout
		}

		n, err := io.Copy(w, httpResp.Body)
		if err != nil {
			return fmt.Errorf("writing artifact: %w", err)
		}

		if outPath != "" {
			rc.Output.PrintSuccess(fmt.Sprintf("Downloaded %d bytes to %s", n, outPath))
		}
		return nil
	},
}
