package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/agentclash/agentclash/cli/internal/output"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// shareResourceTypeValues are the resource types that the share-link API
// accepts (validPublicShareResourceType in backend/internal/api/public_shares.go).
var shareResourceTypeValues = []string{
	"run_scorecard",
	"run_agent_scorecard",
	"run_agent_replay",
	"challenge_pack_version",
	"agent_tryout",
	"report_snapshot",
}

// shareResourceReportSnapshot needs an expiry and cannot use search indexing
// (PublicShareManager.CreateShareLink).
const shareResourceReportSnapshot = "report_snapshot"

// shareClock gives the current time for the expiry check. Tests replace it.
var shareClock = time.Now

func init() {
	rootCmd.AddCommand(shareCmd)
	shareCmd.AddCommand(shareCreateCmd)
	shareCmd.AddCommand(shareUpdateCmd)
	shareCmd.AddCommand(shareRevokeCmd)

	shareCreateCmd.Flags().String("resource-type", "", "Type of the resource to share: run_scorecard, run_agent_scorecard, run_agent_replay, challenge_pack_version, agent_tryout or report_snapshot")
	shareCreateCmd.Flags().String("resource-id", "", "ID of the resource to share; the help text gives the ID that each resource type takes")
	shareCreateCmd.Flags().Bool("search-indexing", false, "Also publish the resource on a publication page that search engines can index (needs the permission to publish public artifacts)")
	shareCreateCmd.Flags().String("expires-at", "", "Time when the link stops, in RFC 3339 format, for example 2026-12-31T00:00:00Z")
	shareCreateCmd.Flags().Duration("expires-in", 0, "Time from now when the link stops, for example 168h")
	shareCreateCmd.MarkFlagRequired("resource-type")
	shareCreateCmd.MarkFlagRequired("resource-id")

	shareUpdateCmd.Flags().Bool("search-indexing", false, "Turn the indexable publication page of the link on (true) or off (false)")
	shareUpdateCmd.MarkFlagRequired("search-indexing")

	shareRevokeCmd.Flags().Bool("yes", false, "Revoke the link with no confirmation prompt; necessary in non-interactive mode")
}

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Create, change and revoke public share links",
	Long: `A share link gives a public URL for one resource. Anyone who has the URL can
see the resource with no login, until the link expires or you revoke it.

The CLI has no list command for share links. Keep the share ID that
'agentclash share create' prints: 'share update' and 'share revoke' need it.`,
}

var shareCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Make a public share link for a scorecard, replay, pack version, tryout or report",
	Long: `Make a public share link for one resource.

--resource-id takes this ID for each --resource-type:

  run_scorecard           run ID (the run must have a scorecard)
  run_agent_scorecard     run agent ID (the run agent must have a scorecard)
  run_agent_replay        run agent ID (the run agent must have a replay)
  challenge_pack_version  challenge pack version ID
  agent_tryout            agent tryout ID (the redaction of its evidence must be complete)
  report_snapshot         report snapshot ID (needs --expires-at or --expires-in)

Without --expires-at or --expires-in, the link does not expire.

The server keeps one active link for each resource. If the resource already
has an active link, the command returns that link with its share ID, URL and
expiry, and a warning shows when the expiry is not the one that you asked for.
If that link expired, the server gives it a new URL and the new expiry.

Search engines never index the share URL. --search-indexing also publishes
the resource on a separate publication page that search engines can index,
and the response gives its URL. It needs the permission to publish public
artifacts, and report_snapshot links cannot use it.

Anyone who has the URL can see the resource. Keep the share ID from the output
to revoke the link later.`,
	Example: `  # Share the scorecard of a run for one week
  agentclash share create --resource-type run_scorecard --resource-id <run-id> --expires-in 168h

  # Share the replay of one run agent, and get the URL as JSON
  agentclash share create --resource-type run_agent_replay --resource-id <run-agent-id> --json --query '.url'`,
	Args: cobra.NoArgs,
	RunE: runShareCreate,
}

var shareUpdateCmd = &cobra.Command{
	Use:   "update <share-id>",
	Short: "Turn search indexing of a share link on or off",
	Long: `Turn search indexing of a share link on or off. When it is on, a separate
publication page of the resource is public, and search engines can index it.
Search engines never index the share URL itself.

The server returns no body. The command writes a confirmation line to stderr
and nothing to stdout, also with --json.`,
	Example: `  # Stop search engines from indexing a shared page
  agentclash share update <share-id> --search-indexing=false`,
	Args: cobra.ExactArgs(1),
	RunE: runShareUpdate,
}

var shareRevokeCmd = &cobra.Command{
	Use:   "revoke <share-id>",
	Short: "Revoke a public share link",
	Long: `Revoke a public share link. The URL stops at once for every person who has it.
You cannot undo a revoke: a new link has a new URL.

On a terminal, the command asks for confirmation. In non-interactive mode
(--non-interactive, AGENTCLASH_NONINTERACTIVE, CI or no TTY), pass --yes.
A link that is already revoked exits 66.`,
	Example: `  # Revoke a link after a confirmation prompt
  agentclash share revoke <share-id>

  # Revoke a link in a script
  agentclash share revoke <share-id> --yes`,
	Args: cobra.ExactArgs(1),
	RunE: runShareRevoke,
}

func runShareCreate(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	resourceType, _ := cmd.Flags().GetString("resource-type")
	resourceID, _ := cmd.Flags().GetString("resource-id")
	searchIndexing, _ := cmd.Flags().GetBool("search-indexing")
	if err := requireOneOf("resource-type", resourceType, shareResourceTypeValues); err != nil {
		return err
	}
	if err := requireUUID("--resource-id", resourceID); err != nil {
		return err
	}
	if parsed, _ := uuid.Parse(resourceID); parsed == uuid.Nil {
		return &cliError{
			Code:    "invalid_argument",
			Message: "--resource-id must not be the zero UUID",
			Details: map[string]any{"flag": "--resource-id", "value": resourceID},
		}
	}
	expiresAt, err := shareExpiry(cmd)
	if err != nil {
		return err
	}
	if resourceType == shareResourceReportSnapshot {
		if expiresAt == nil {
			return &cliError{
				Code:     "invalid_argument",
				Message:  "a report_snapshot link needs --expires-at or --expires-in",
				Details:  map[string]any{"flag": "--expires-in", "resource_type": resourceType},
				NextStep: "Pass --expires-in, for example --expires-in 168h.",
			}
		}
		if searchIndexing {
			return &cliError{
				Code:    "invalid_argument",
				Message: "a report_snapshot link cannot use --search-indexing",
				Details: map[string]any{"flag": "--search-indexing", "resource_type": resourceType},
			}
		}
	}

	body := map[string]any{
		"resource_type":   resourceType,
		"resource_id":     resourceID,
		"search_indexing": searchIndexing,
	}
	if expiresAt != nil {
		body["expires_at"] = expiresAt.UTC().Format(time.RFC3339Nano)
	}
	resp, err := rc.Client.Post(cmd.Context(), "/v1/share-links", body)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	var result map[string]any
	if err := resp.DecodeJSON(&result); err != nil {
		return fmt.Errorf("decoding share link response: %w", err)
	}
	share := mapObject(result, "share")
	shareID := output.SanitizeLine(mapString(share, "id"))
	// The warnings go to stderr, so --json output stays the server response.
	for _, warning := range shareExistingLinkWarnings(share, expiresAt, searchIndexing) {
		rc.Output.PrintWarning(warning)
	}
	if rc.Output.IsStructured() {
		return rc.Output.PrintRaw(result)
	}

	// SanitizeLine keeps a newline in a server value from making a false row.
	rc.Output.PrintSuccess("Share link created")
	rc.Output.PrintDetail("Share ID", shareID)
	rc.Output.PrintDetail("Resource", output.SanitizeLine(mapString(share, "resource_type")+" "+mapString(share, "resource_id")))
	rc.Output.PrintDetail("URL", output.SanitizeLine(mapString(result, "url")))
	if publicationURL := mapString(result, "publication_url"); publicationURL != "" {
		rc.Output.PrintDetail("Publication URL", output.SanitizeLine(publicationURL))
	}
	rc.Output.PrintDetail("Search indexing", onOff(share["search_indexing"] == true))
	if expires := mapString(share, "expires_at"); expires != "" {
		rc.Output.PrintDetail("Expires", output.SanitizeLine(expires))
	} else {
		rc.Output.PrintDetail("Expires", "never")
	}
	rc.Output.PrintWarning("Anyone who has the URL can see this resource with no login.")
	rc.Output.PrintDetail("Revoke", "agentclash share revoke "+shareID)
	return nil
}

// shareExistingLinkWarnings compares the returned link with the request. The
// server keeps one active link for each resource. If an active link exists,
// create returns that link and keeps its expiry, and it turns indexing on if
// the old link or the request has it on (CreatePublicShareLink in
// backend/internal/repository/public_share_links.go).
func shareExistingLinkWarnings(share map[string]any, requestedExpiry *time.Time, requestedIndexing bool) []string {
	var warnings []string
	returned := output.SanitizeLine(mapString(share, "expires_at"))
	expiryDiffers := false
	switch {
	case requestedExpiry == nil && returned != "":
		expiryDiffers = true
		warnings = append(warnings, fmt.Sprintf("The resource already has an active link. The server returned that link, and it expires at %s.", returned))
	case requestedExpiry != nil && returned == "":
		expiryDiffers = true
		warnings = append(warnings, "The resource already has an active link that does not expire. The server returned that link without the expiry that you asked for.")
	case requestedExpiry != nil:
		parsed, err := time.Parse(time.RFC3339Nano, returned)
		// Postgres keeps microseconds, so a smaller difference is the same time.
		if diff := parsed.Sub(*requestedExpiry); err != nil || diff > time.Microsecond || diff < -time.Microsecond {
			expiryDiffers = true
			warnings = append(warnings, fmt.Sprintf("The resource already has an active link. The server returned that link, and it expires at %s, not at the time that you asked for.", returned))
		}
	}
	if !requestedIndexing && share["search_indexing"] == true {
		warnings = append(warnings, "Search indexing of this link is on, because the active link already had it on.")
	}
	if expiryDiffers {
		warnings = append(warnings, fmt.Sprintf("To set a different expiry, revoke the link (agentclash share revoke %s), then create a new link.", output.SanitizeLine(mapString(share, "id"))))
	}
	return warnings
}

// shareExpiry reads --expires-at or --expires-in. It returns nil if neither
// flag is set.
func shareExpiry(cmd *cobra.Command) (*time.Time, error) {
	atSet := cmd.Flags().Changed("expires-at")
	inSet := cmd.Flags().Changed("expires-in")
	if atSet && inSet {
		return nil, &cliError{
			Code:    "invalid_argument",
			Message: "use --expires-at or --expires-in, not both",
			Details: map[string]any{"flags": []string{"--expires-at", "--expires-in"}},
		}
	}
	now := shareClock()
	var expiresAt time.Time
	switch {
	case atSet:
		raw, _ := cmd.Flags().GetString("expires-at")
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
		if err != nil {
			return nil, &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("--expires-at must be an RFC 3339 time, for example 2026-12-31T00:00:00Z, got %q", raw),
				Details: map[string]any{"flag": "--expires-at", "value": raw},
			}
		}
		expiresAt = parsed
	case inSet:
		duration, _ := cmd.Flags().GetDuration("expires-in")
		if duration <= 0 {
			return nil, &cliError{
				Code:    "invalid_argument",
				Message: fmt.Sprintf("--expires-in must be more than 0, got %s", duration),
				Details: map[string]any{"flag": "--expires-in", "value": duration.String()},
			}
		}
		expiresAt = now.Add(duration)
	default:
		return nil, nil
	}
	if !expiresAt.After(now) {
		return nil, &cliError{
			Code:    "invalid_argument",
			Message: fmt.Sprintf("--expires-at must be in the future, got %s", expiresAt.UTC().Format(time.RFC3339)),
			Details: map[string]any{"flag": "--expires-at", "value": expiresAt.UTC().Format(time.RFC3339)},
		}
	}
	return &expiresAt, nil
}

func runShareUpdate(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	shareID := args[0]
	if err := requireUUID("share ID", shareID); err != nil {
		return err
	}
	enabled, _ := cmd.Flags().GetBool("search-indexing")

	resp, err := rc.Client.Patch(cmd.Context(), "/v1/share-links/"+shareID, map[string]any{"search_indexing": enabled})
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	rc.Output.PrintSuccess(fmt.Sprintf("Search indexing of share link %s is now %s", shareID, onOff(enabled)))
	return nil
}

func runShareRevoke(cmd *cobra.Command, args []string) error {
	rc := GetRunContext(cmd)
	shareID := args[0]
	if err := requireUUID("share ID", shareID); err != nil {
		return err
	}
	if err := confirmLargeEffect(cmd, rc, fmt.Sprintf("Revoke share link %s? The URL stops at once for every person who has it.", shareID)); err != nil {
		return err
	}

	resp, err := rc.Client.Delete(cmd.Context(), "/v1/share-links/"+shareID)
	if err != nil {
		return err
	}
	if apiErr := resp.ParseError(); apiErr != nil {
		return apiErr
	}
	rc.Output.PrintSuccess(fmt.Sprintf("Share link %s revoked", shareID))
	return nil
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}
