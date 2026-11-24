package cmd

import (
	"errors"
	"io"
	"slices"
	"strconv"
	"sync"

	"github.com/agentclash/agentclash/cli/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type cobraUsageError struct {
	error
	jsonOutput bool
}

func (e *cobraUsageError) Unwrap() error { return e.error }

func wrapCobraUsageError(err error) error {
	if err == nil || errors.Is(err, pflag.ErrHelp) {
		return err
	}
	return &cliError{Code: "invalid_argument", Err: &cobraUsageError{error: err}}
}

var wrapPositionalValidatorsOnce sync.Once

func wrapPositionalValidators(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(cmd *cobra.Command, args []string) error {
			return wrapCobraUsageError(validate(cmd, args))
		}
	}
	for _, child := range cmd.Commands() {
		wrapPositionalValidators(child)
	}
}

func executeCLI(args []string) error {
	wrapPositionalValidatorsOnce.Do(func() { wrapPositionalValidators(rootCmd) })
	rootCmd.SetArgs(args)
	cmd, err := rootCmd.ExecuteC()
	if err == nil {
		return nil
	}

	_, flagArgs, lookupErr := rootCmd.Find(args)
	var usageErr *cobraUsageError
	if !errors.As(err, &usageErr) {
		if !isCobraValidationError(cmd, flagArgs, lookupErr, err) {
			return err
		}
		usageErr = &cobraUsageError{error: err}
		err = &cliError{Code: "invalid_argument", Err: usageErr}
	}
	usageErr.jsonOutput = usageErrorJSONOutput(cmd, flagArgs)
	return err
}

// Cobra exposes no hook or error type for command lookup and flag validation.
func isCobraValidationError(cmd *cobra.Command, args []string, lookupErr, err error) bool {
	matches := func(validationErr error) bool {
		return validationErr != nil && validationErr.Error() == err.Error()
	}
	if matches(lookupErr) || matches(cmd.ValidateRequiredFlags()) || matches(cmd.ValidateFlagGroups()) {
		return true
	}
	// Cobra creates this command at execution time.
	return cmd.Name() == cobra.ShellCompRequestCmd && matches(cmd.ValidateArgs(args))
}

func usageErrorJSONOutput(cmd *cobra.Command, args []string) bool {
	// Parse without applying values so an invalid value cannot stop the scan.
	flags := pflag.NewFlagSet("usage output", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.SetNormalizeFunc(cmd.GlobalNormalizationFunc())
	flags.ParseErrorsAllowlist.UnknownFlagsHandling = pflag.PassUnknownFlagToArgs
	cmd.InitDefaultHelpFlag()
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		copy := *flag
		flags.AddFlag(&copy)
	})

	var jsonOutput bool
	var format string
	for {
		err := flags.ParseAll(args, func(flag *pflag.Flag, value string) error {
			switch flag.Name {
			case "json":
				if enabled, err := strconv.ParseBool(value); err == nil {
					jsonOutput = enabled
				}
			case "output":
				format = value
			}
			return nil
		})
		var syntaxErr *pflag.InvalidSyntaxError
		if !errors.As(err, &syntaxErr) {
			break
		}
		// Skip the malformed token and look for output flags after it.
		index := slices.Index(args, syntaxErr.GetSpecifiedFlag())
		if index < 0 {
			break
		}
		args = args[index+1:]
	}
	if jsonOutput || format == "json" || format == "yaml" {
		return true
	}
	if format != "" {
		return false
	}
	cfg, err := config.Load()
	return err == nil && cfg.Output == "json"
}
