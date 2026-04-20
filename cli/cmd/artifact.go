package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
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

		if rc.Output.IsStructured() {
			return rc.Output.PrintRaw(result)
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

		// Step 2: Validate scheme on the signed URL. Refuse plain http unless
		// (a) it points at a loopback host (local dev), or (b) it is
		// same-origin with the API base URL we were already willing to talk
		// to — the backend's requestBaseURL helper intentionally returns
		// http://<host> when TLS is terminated upstream without
		// X-Forwarded-Proto, so blocking all non-loopback http would break
		// perfectly legitimate deployments.
		parsed, err := url.Parse(dlResp.URL)
		if err != nil {
			return fmt.Errorf("parsing download URL: %w", err)
		}
		switch parsed.Scheme {
		case "https":
		case "http":
			// Accept plain http only when it points at loopback (local dev)
			// or at the exact same origin the CLI is already using for the
			// API. We deliberately do NOT accept http downloads when the
			// API base is https: that case happens when a TLS-terminating
			// proxy forwards to the backend without an X-Forwarded-Proto
			// header, and the server's requestBaseURL helper then emits
			// http://<host>/... from inside the proxy. Accepting that here
			// would let a network attacker downgrade an otherwise-secure
			// session to cleartext. The operator fix is to set
			// X-Forwarded-Proto on the proxy, not to weaken this check.
			if !isArtifactLoopbackHost(parsed.Hostname()) && !sameOriginAsAPI(parsed, rc.Client.BaseURL()) {
				return fmt.Errorf(
					"refusing plain-http download URL %q: expected https, or same origin as --api-url. "+
						"If your deployment terminates TLS upstream, ensure the proxy sets X-Forwarded-Proto: https",
					dlResp.URL,
				)
			}
		default:
			return fmt.Errorf("download URL scheme %q is not allowed", parsed.Scheme)
		}

		// Step 3: Download through a context-cancellable client that shares
		// the main client's transport and refuses cross-origin redirects.
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, dlResp.URL, nil)
		if err != nil {
			return fmt.Errorf("building download request: %w", err)
		}
		httpResp, err := rc.Client.NewDownloadClient().Do(req)
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

func isArtifactLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// sameOriginAsAPI reports whether dl is same-scheme/host/port as the API
// base URL the CLI was configured to use. That gives us "trust the backend
// to point at itself" semantics — e.g. if the user is already talking to
// http://devbox:8080, accepting http://devbox:8080/... artifact URLs is no
// worse than the API call we just made.
func sameOriginAsAPI(dl *url.URL, apiBase string) bool {
	if apiBase == "" {
		return false
	}
	base, err := url.Parse(apiBase)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return false
	}
	if base.Scheme != dl.Scheme {
		return false
	}
	bh, dh := base.Hostname(), dl.Hostname()
	bp, dp := base.Port(), dl.Port()
	if bp == "" {
		if base.Scheme == "https" {
			bp = "443"
		} else {
			bp = "80"
		}
	}
	if dp == "" {
		if dl.Scheme == "https" {
			dp = "443"
		} else {
			dp = "80"
		}
	}
	return bh == dh && bp == dp
}
