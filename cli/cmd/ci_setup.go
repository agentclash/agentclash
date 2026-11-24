package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/spf13/cobra"
)

// ciSetupMaxFileBytes is the limit of the server for each file
// (validateCISetupPullRequestFiles in backend/internal/api/github_integrations.go).
const ciSetupMaxFileBytes = 1 << 20

// ciSetupRepoFullNamePattern accepts a GitHub owner/name pair.
var ciSetupRepoFullNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$`)

func init() {
	ciCmd.AddCommand(ciSetupCmd)

	ciSetupCmd.Flags().String("repo", "", "GitHub repository as owner/name, or its numeric GitHub repository ID")
	ciSetupCmd.Flags().String("manifest", ".agentclash/ci.yaml", "Local CI manifest to add; the same relative path is used in the repository")
	ciSetupCmd.Flags().String("workflow", ".github/workflows/agentclash.yml", "Local GitHub Actions workflow to add; pass an empty value to leave it out")
	ciSetupCmd.Flags().StringArray("file", nil, "Another local file to add at the same relative path; may be repeated")
	ciSetupCmd.Flags().String("base", "", "Base branch of the pull request (default: the default branch of the repository)")
	ciSetupCmd.Flags().String("title", "Set up AgentClash CI", "Title of the pull request")
	ciSetupCmd.Flags().String("body", "", "Body of the pull request (default: a list of the added files)")
	ciSetupCmd.Flags().Bool("draft", true, "Open the pull request as a draft")
	ciSetupCmd.Flags().Bool("check-only", false, "Only report the files that already exist on the base branch; open no pull request")
	ciSetupCmd.Flags().Bool("overwrite", false, "Replace files that already exist on the base branch")
	ciSetupCmd.MarkFlagRequired("repo")
}

var ciSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Open a pull request that adds the AgentClash CI files to a GitHub repository",
	Long: `Open a pull request that adds the AgentClash CI files to a GitHub repository.

The command reads the local CI manifest, the GitHub Actions workflow and any
--file paths, and sends them to the server. The server uses the AgentClash
GitHub App of the workspace to make a branch agentclash/ci-setup/<id> and a
pull request. Each file goes into the repository at the same relative path,
so run the command from the root of your local clone.

The AgentClash GitHub App must be installed on the repository, and you need
the permission to manage integrations in the workspace.

If a file already exists on the base branch, the server opens no pull
request. The command then lists the files and exits 1. Use --check-only to
list them first, or --overwrite to replace them.

A pull request is not idempotent. If the command exits 75, look for an
agentclash/ci-setup/* branch in the repository before you run it again.`,
	Example: `  # Write and check the manifest, then open the setup pull request
  agentclash ci init .agentclash/ci.yaml
  agentclash ci validate .agentclash/ci.yaml
  agentclash ci setup --repo acme/support-agent

  # List the files that already exist on the base branch; open no pull request
  agentclash ci setup --repo acme/support-agent --check-only

  # Replace the existing files and add the agent spec too
  agentclash ci setup --repo acme/support-agent --overwrite --file .agentclash/agent.json

  # Print only the URL of the pull request
  agentclash ci setup --repo acme/support-agent --json --query '.pull_request.html_url'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		repo, _ := cmd.Flags().GetString("repo")
		manifestPath, _ := cmd.Flags().GetString("manifest")
		workflowPath, _ := cmd.Flags().GetString("workflow")
		extraPaths, _ := cmd.Flags().GetStringArray("file")
		base, _ := cmd.Flags().GetString("base")
		title, _ := cmd.Flags().GetString("title")
		prBody, _ := cmd.Flags().GetString("body")
		draft, _ := cmd.Flags().GetBool("draft")
		checkOnly, _ := cmd.Flags().GetBool("check-only")
		overwrite, _ := cmd.Flags().GetBool("overwrite")

		if checkOnly && overwrite {
			return &cliError{
				Code:    "invalid_argument",
				Message: "--check-only and --overwrite cannot be used together",
				Details: map[string]any{"flags": []string{"--check-only", "--overwrite"}},
			}
		}
		repoID, repoFullName, err := parseCISetupRepo(repo)
		if err != nil {
			return err
		}

		files, err := collectCISetupFiles(rc, cmd.Flags().Changed("workflow"), manifestPath, workflowPath, extraPaths)
		if err != nil {
			return err
		}

		wsID := RequireWorkspace(cmd)

		if !rc.Output.IsStructured() {
			if _, statErr := os.Stat(".git"); statErr != nil {
				rc.Output.PrintWarning("The current directory is not the root of a git repository. Each file goes into the repository at the path that you gave.")
			}
		}

		var installationID int64
		if repoID == 0 {
			repoID, installationID, err = resolveCISetupRepository(cmd, rc, wsID, repoFullName)
			if err != nil {
				return err
			}
		}

		body := map[string]any{
			"github_repository_id": repoID,
			"files":                files,
			"draft":                draft,
		}
		if installationID > 0 {
			body["github_installation_id"] = installationID
		}
		if base = strings.TrimSpace(base); base != "" {
			body["base_branch"] = base
		}
		if title = strings.TrimSpace(title); title != "" {
			body["title"] = title
		}
		if strings.TrimSpace(prBody) == "" {
			prBody = defaultCISetupBody(files)
		}
		body["body"] = prBody
		if checkOnly {
			body["check_only"] = true
		}
		if overwrite {
			body["overwrite_existing"] = true
		}

		spinnerMsg := "Opening the setup pull request..."
		if checkOnly {
			spinnerMsg = "Checking the repository for existing files..."
		}
		sp := output.NewSpinner(spinnerMsg, flagQuiet)
		resp, err := rc.Client.WithRequestTimeout(longRequestTimeout).Post(cmd.Context(), "/v1/workspaces/"+wsID+"/github/ci-setup-pull-request", body)
		if err == nil {
			if apiErr := resp.ParseError(); apiErr != nil {
				err = apiErr
			}
		}
		if err != nil {
			sp.StopWithError("CI setup request failed")
			if !checkOnly && !rc.Output.IsStructured() && ciSetupMayHaveOpenedPullRequest(err) {
				warnCISetupMayHaveOpenedPullRequest(rc)
			}
			return err
		}
		sp.Stop()

		var result map[string]any
		if err := resp.DecodeJSON(&result); err != nil {
			// The server accepted the request, so the pull request can exist.
			if !checkOnly && !rc.Output.IsStructured() {
				warnCISetupMayHaveOpenedPullRequest(rc)
			}
			return fmt.Errorf("decoding ci setup response: %w", err)
		}

		pullRequest := mapObject(result, "pull_request")
		conflicts := mapSlice(result, "conflicts")
		var outcomeErr error
		if !checkOnly && pullRequest == nil {
			outcomeErr = &ExitCodeError{
				Code:    1,
				Message: fmt.Sprintf("%d file(s) already exist on branch %s, so the server opened no pull request. Pass --overwrite to replace them.", len(conflicts), mapString(result, "base_branch")),
			}
		}

		if rc.Output.IsStructured() {
			if err := rc.Output.PrintRaw(result); err != nil {
				return err
			}
			return outcomeErr
		}

		switch {
		case pullRequest != nil:
			renderCISetupPullRequest(rc, result, pullRequest, repoFullName)
		case len(conflicts) == 0:
			rc.Output.PrintSuccess(fmt.Sprintf("No conflicts: none of the %d file(s) exist on branch %s", len(mapSlice(result, "files")), mapString(result, "base_branch")))
			rc.Output.PrintDetail("Next", "agentclash ci setup --repo "+repo)
		default:
			rc.Output.PrintWarning(fmt.Sprintf("%d file(s) already exist on branch %s", len(conflicts), mapString(result, "base_branch")))
			renderCISetupConflicts(rc, conflicts)
			if checkOnly {
				rc.Output.PrintDetail("Next", "agentclash ci setup --repo "+repo+" --overwrite")
			}
		}
		return outcomeErr
	},
}

// parseCISetupRepo returns the numeric GitHub repository ID, or the owner/name
// pair that the command must look up.
func parseCISetupRepo(repo string) (int64, string, error) {
	repo = strings.TrimSpace(repo)
	if id, err := strconv.ParseInt(repo, 10, 64); err == nil {
		if id > 0 {
			return id, "", nil
		}
	} else if ciSetupRepoFullNamePattern.MatchString(repo) {
		return 0, repo, nil
	}
	return 0, "", &cliError{
		Code:    "invalid_argument",
		Message: fmt.Sprintf("--repo must be owner/name or a positive GitHub repository ID, got %q", repo),
		Details: map[string]any{"flag": "--repo", "value": repo},
	}
}

// collectCISetupFiles reads the manifest, the workflow and the extra files,
// and applies the file rules of the server before any request.
func collectCISetupFiles(rc *RunContext, workflowChanged bool, manifestPath, workflowPath string, extraPaths []string) ([]map[string]string, error) {
	if strings.TrimSpace(manifestPath) == "" {
		return nil, &cliError{
			Code:    "invalid_argument",
			Message: "--manifest must not be empty",
			Details: map[string]any{"flag": "--manifest"},
		}
	}
	validation, err := validateCIManifestFile(manifestPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w; make one with 'agentclash ci init %s'", err, manifestPath)
		}
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			return nil, err
		}
		return nil, &cliError{
			Code:     "invalid_argument",
			Message:  fmt.Sprintf("CI manifest %s is not valid: %s", manifestPath, strings.Join(validation.Errors, "; ")),
			Details:  map[string]any{"path": manifestPath, "errors": validation.Errors},
			NextStep: "Run `agentclash ci validate " + manifestPath + "`.",
		}
	}

	paths := []string{manifestPath}
	switch {
	case strings.TrimSpace(workflowPath) == "":
		// An explicit empty value leaves the workflow out.
	case !workflowChanged:
		if _, statErr := os.Stat(workflowPath); errors.Is(statErr, fs.ErrNotExist) {
			if !rc.Output.IsStructured() {
				rc.Output.PrintWarning(fmt.Sprintf("No workflow file at %s; the pull request adds no workflow. Pass --workflow to add one.", workflowPath))
			}
		} else {
			paths = append(paths, workflowPath)
		}
	default:
		paths = append(paths, workflowPath)
	}
	paths = append(paths, extraPaths...)

	files := make([]map[string]string, 0, len(paths))
	seen := make(map[string]string, len(paths))
	for _, localPath := range paths {
		repoPath, err := ciSetupRepositoryPath(localPath)
		if err != nil {
			return nil, err
		}
		if first, ok := seen[repoPath]; ok {
			return nil, &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("%s and %s are the same repository path %s", first, localPath, repoPath),
				Details: map[string]any{"path": repoPath},
			}
		}
		seen[repoPath] = localPath

		if err := ciSetupCheckInsideDirectory(localPath); err != nil {
			return nil, err
		}
		data, err := os.ReadFile(localPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", localPath, err)
		}
		if err := checkCISetupFileContent(localPath, data); err != nil {
			return nil, err
		}
		files = append(files, map[string]string{"path": repoPath, "content": string(data)})
	}
	return files, nil
}

// ciSetupRepositoryPath turns a local relative path into the repository path
// that the server accepts: relative, slash-separated, without "." or "..".
func ciSetupRepositoryPath(localPath string) (string, error) {
	invalid := func(reason string) error {
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("file path %q %s", localPath, reason),
			Details: map[string]any{"path": localPath},
		}
	}
	if strings.TrimSpace(localPath) == "" {
		return "", invalid("is empty")
	}
	// The server trims the path, so " a.txt" and "a.txt" are the same path.
	if strings.TrimSpace(localPath) != localPath {
		return "", invalid("must not start or end with a space")
	}
	if filepath.IsAbs(localPath) || strings.HasPrefix(localPath, "/") {
		return "", invalid("must be relative to the root of the repository")
	}
	if strings.Contains(localPath, `\`) && filepath.Separator != '\\' {
		return "", invalid("must not contain a backslash")
	}
	repoPath := filepath.ToSlash(filepath.Clean(localPath))
	if repoPath == "." || repoPath == ".." || strings.HasPrefix(repoPath, "../") {
		return "", invalid("must stay inside the repository")
	}
	return repoPath, nil
}

// ciSetupCheckInsideDirectory rejects a local file that resolves, through a
// symbolic link, to a location outside the current directory. The command
// uploads the content of the link target, so a link in a cloned repository
// must not send a file from another location, for example a credentials file.
func ciSetupCheckInsideDirectory(localPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("finding the current directory: %w", err)
	}
	root, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		return fmt.Errorf("resolving the current directory: %w", err)
	}
	target, err := filepath.EvalSymlinks(localPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", localPath, err)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return &cliError{
			Code:     "invalid_argument",
			Message:  fmt.Sprintf("file %s is a link to %s, which is outside the current directory", localPath, target),
			Details:  map[string]any{"path": localPath, "target": target},
			NextStep: "Replace the link with a regular file, or run the command from the root of the repository that contains the target.",
		}
	}
	return nil
}

func checkCISetupFileContent(localPath string, data []byte) error {
	invalid := func(reason string) error {
		return &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("file %s %s", localPath, reason),
			Details: map[string]any{"path": localPath, "bytes": len(data)},
		}
	}
	switch {
	case strings.TrimSpace(string(data)) == "":
		return invalid("is empty")
	case len(data) > ciSetupMaxFileBytes:
		return invalid("is larger than 1 MiB")
	case !utf8.Valid(data):
		return invalid("is not valid UTF-8 text")
	}
	return nil
}

// resolveCISetupRepository finds the installed repository with the owner/name
// pair, and returns its GitHub repository ID and installation ID.
func resolveCISetupRepository(cmd *cobra.Command, rc *RunContext, wsID, fullName string) (int64, int64, error) {
	resp, err := rc.Client.Get(cmd.Context(), "/v1/workspaces/"+wsID+"/github/repositories", url.Values{"query": {fullName}})
	if err != nil {
		return 0, 0, err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return 0, 0, apiErr
	}
	var list struct {
		Items []struct {
			FullName             string `json:"full_name"`
			GitHubRepositoryID   int64  `json:"github_repository_id"`
			GitHubInstallationID int64  `json:"github_installation_id"`
		} `json:"items"`
	}
	if err := resp.DecodeJSON(&list); err != nil {
		return 0, 0, fmt.Errorf("decoding github repositories: %w", err)
	}

	candidates := make([]string, 0, len(list.Items))
	var repoID, installationID int64
	ambiguous := false
	for _, item := range list.Items {
		candidates = append(candidates, item.FullName)
		if !strings.EqualFold(item.FullName, fullName) {
			continue
		}
		switch {
		case repoID == 0:
			repoID, installationID = item.GitHubRepositoryID, item.GitHubInstallationID
		case repoID != item.GitHubRepositoryID:
			ambiguous = true
		}
	}
	if repoID > 0 && !ambiguous {
		return repoID, installationID, nil
	}

	message := fmt.Sprintf("no repository %s is installed for the AgentClash GitHub App of this workspace", fullName)
	if ambiguous {
		message = fmt.Sprintf("two or more installed repositories have the name %s", fullName)
	}
	return 0, 0, &cliError{
		Code:     "invalid_argument",
		Message:  message,
		Details:  map[string]any{"flag": "--repo", "value": fullName, "candidates": candidates},
		NextStep: "Install the AgentClash GitHub App on the repository from the CI setup page of the workspace in the web app, or pass the numeric GitHub repository ID with --repo.",
	}
}

func defaultCISetupBody(files []map[string]string) string {
	var b strings.Builder
	b.WriteString("Adds the AgentClash CI configuration from `agentclash ci setup`.\n\nFiles:\n")
	for _, file := range files {
		b.WriteString("- `" + file["path"] + "`\n")
	}
	return b.String()
}

// ciSetupMayHaveOpenedPullRequest reports whether a failed request can have
// opened a pull request on the server before the failure.
func ciSetupMayHaveOpenedPullRequest(err error) bool {
	var apiErr *cliapi.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 && apiErr.Code != "github_app_not_configured"
	}
	// A failed dial means that the request did not reach the server.
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return false
	}
	// Any other failure, for example a timeout, a reset connection or a
	// truncated response, can happen after the server opened the pull request.
	return true
}

func warnCISetupMayHaveOpenedPullRequest(rc *RunContext) {
	rc.Output.PrintWarning("The server can have opened the pull request before the failure. Look for an agentclash/ci-setup/* branch in the repository before you run the command again.")
}

func renderCISetupPullRequest(rc *RunContext, result, pullRequest map[string]any, repoFullName string) {
	number := str(pullRequest["number"])
	if n, ok := pullRequest["number"].(float64); ok {
		number = strconv.FormatInt(int64(n), 10)
	}
	rc.Output.PrintSuccess("Opened pull request #" + number)
	rc.Output.PrintDetail("Pull request", str(pullRequest["html_url"]))
	rc.Output.PrintDetail("Draft", str(pullRequest["draft"]))
	rc.Output.PrintDetail("Branch", str(result["branch"]))
	rc.Output.PrintDetail("Base branch", str(result["base_branch"]))
	paths := make([]string, 0)
	for _, item := range mapSlice(result, "files") {
		if file, ok := item.(map[string]any); ok {
			paths = append(paths, str(file["path"]))
		}
	}
	rc.Output.PrintDetail("Files", strings.Join(paths, ", "))
	if conflicts := mapSlice(result, "conflicts"); len(conflicts) > 0 {
		rc.Output.PrintDetail("Replaced", fmt.Sprintf("%d existing file(s)", len(conflicts)))
	}
	secretsTarget := "the repository"
	if repoFullName != "" {
		secretsTarget = repoFullName
	}
	rc.Output.PrintDetail("Next", "Add the AGENTCLASH_TOKEN and AGENTCLASH_WORKSPACE secrets that the workflow reads to "+secretsTarget+", then merge the pull request")
}

func renderCISetupConflicts(rc *RunContext, conflicts []any) {
	rows := make([][]string, 0, len(conflicts))
	for _, item := range conflicts {
		conflict, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, []string{str(conflict["path"]), str(conflict["sha"])})
	}
	rc.Output.PrintTable([]output.Column{{Header: "Path"}, {Header: "SHA"}}, rows)
}
