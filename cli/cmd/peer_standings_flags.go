package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	peerStandingsFlagName        = "peer-standings"
	peerStandingsCadenceFlagName = "peer-standings-cadence"
	legacyRaceContextFlagName    = "race-context"
	legacyRaceContextCadenceName = "race-context-cadence"
)

func registerPeerStandingsFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.Bool(
		peerStandingsFlagName,
		false,
		"Enable live peer-standings injection during the run (requires 2+ agents)",
	)
	flags.Int(
		peerStandingsCadenceFlagName,
		0,
		"Override peer-standings cadence; minimum steps between standings injections, [1, 10]. 0 uses the backend default.",
	)
	flags.Bool(legacyRaceContextFlagName, false, "Deprecated: use --peer-standings")
	flags.Int(legacyRaceContextCadenceName, 0, "Deprecated: use --peer-standings-cadence")
	_ = flags.MarkDeprecated(legacyRaceContextFlagName, "use --peer-standings instead")
	_ = flags.MarkDeprecated(legacyRaceContextCadenceName, "use --peer-standings-cadence instead")
}

func peerStandingsFromFlags(cmd *cobra.Command) (enabled bool, cadence int) {
	flags := cmd.Flags()
	if flags.Changed(peerStandingsFlagName) {
		enabled, _ = flags.GetBool(peerStandingsFlagName)
	} else if legacy, _ := flags.GetBool(legacyRaceContextFlagName); legacy {
		enabled = true
	}

	if flags.Changed(peerStandingsCadenceFlagName) {
		cadence, _ = flags.GetInt(peerStandingsCadenceFlagName)
	} else if flags.Changed(legacyRaceContextCadenceName) {
		cadence, _ = flags.GetInt(legacyRaceContextCadenceName)
	}
	return enabled, cadence
}

func validatePeerStandingsCadence(cadence int) error {
	if cadence == 0 || (cadence >= 1 && cadence <= 10) {
		return nil
	}
	return fmt.Errorf(
		"--%s must be 0 (backend default) or between 1 and 10, got %d",
		peerStandingsCadenceFlagName,
		cadence,
	)
}

func peerStandingsFlagsUnsupportedError(context string) error {
	return fmt.Errorf(
		"--%s / --%s are not supported with %s",
		peerStandingsFlagName,
		peerStandingsCadenceFlagName,
		context,
	)
}
