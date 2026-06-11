package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

// registerListPaginationFlags adds the standard --limit/--offset pair to a
// list command. These endpoints use limit/offset (not cursors) server-side;
// limit 0 means "let the server apply its default" (20, capped at 100).
func registerListPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", 0, "Maximum items to return (1-100; 0 = server default of 20)")
	cmd.Flags().Int("offset", 0, "Number of items to skip before the first result")
}

// listPaginationQuery validates --limit/--offset and encodes them as query
// parameters. Validation errors carry a stable code so agents can branch.
func listPaginationQuery(cmd *cobra.Command) (url.Values, error) {
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")
	if limit < 0 || limit > 100 {
		return nil, &cliError{Code: "invalid_argument", Message: "--limit must be 0 (server default) or between 1 and 100"}
	}
	if offset < 0 {
		return nil, &cliError{Code: "invalid_argument", Message: "--offset must be greater than or equal to 0"}
	}
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return q, nil
}

// paginatedList is the CLI's structured envelope for limit/offset list
// endpoints: the server's items/total/limit/offset passed through, plus a
// client-derived has_more so agents can page without arithmetic.
type paginatedList struct {
	Items   []map[string]any `json:"items"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

func newPaginatedList(items []map[string]any, total int64, limit, offset int) paginatedList {
	if items == nil {
		// An empty page must serialize as items: [], not items: null.
		items = []map[string]any{}
	}
	return paginatedList{
		Items:   items,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset)+int64(len(items)) < total,
	}
}

// printListPagingHint tells a human how to fetch the next page. No-op when
// everything already fit.
func printListPagingHint(rc *RunContext, list paginatedList) {
	if !list.HasMore || len(list.Items) == 0 {
		return
	}
	// Repeat --limit in the hint: the server echoes the effective page size,
	// and omitting it would silently change page sizes between iterations.
	rc.Output.PrintDetail("More", fmt.Sprintf("showing %d-%d of %d; next page: --limit %d --offset %d",
		list.Offset+1, list.Offset+len(list.Items), list.Total, list.Limit, list.Offset+len(list.Items)))
}
