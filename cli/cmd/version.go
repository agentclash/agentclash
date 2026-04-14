package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	cliVersion = "dev"
	cliCommit  = "none"
	cliDate    = "unknown"
)

// SetVersionInfo sets the version information (called from main).
func SetVersionInfo(version, commit, date string) {
	cliVersion = version
	cliCommit = commit
	cliDate = date
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show CLI version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		rc := GetRunContext(cmd)
		if rc != nil && rc.Output.IsJSON() {
			return rc.Output.PrintJSON(map[string]string{
				"version": cliVersion,
				"commit":  cliCommit,
				"date":    cliDate,
				"go":      runtime.Version(),
				"os":      runtime.GOOS,
				"arch":    runtime.GOARCH,
			})
		}

		fmt.Printf("agentclash %s\n", cliVersion)
		fmt.Printf("  commit:  %s\n", cliCommit)
		fmt.Printf("  built:   %s\n", cliDate)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
