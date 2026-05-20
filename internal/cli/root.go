package cli

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type contextKey string

const (
	optionsContextKey            contextKey = "options"
	buildContextKey              contextKey = "build"
	rawResponseSupportAnnotation            = "authnet.raw_response"
)

type globalOptions struct {
	JSON        bool
	Automation  bool
	Profile     string
	Environment string
	RawResponse bool
	Color       string
	NoColor     bool
}

// NewRootCommand builds the root authnet command with local-only scaffold behavior.
func NewRootCommand(info BuildInfo) *cobra.Command {
	build := info.normalized()
	options := &globalOptions{}
	ctx := context.WithValue(context.Background(), optionsContextKey, options)
	ctx = context.WithValue(ctx, buildContextKey, build)

	root := &cobra.Command{
		Use:           "authnet",
		Short:         "Authorize.Net operations CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       build.Version,
	}
	root.SetContext(ctx)
	root.CompletionOptions.DisableDefaultCmd = true

	root.SetVersionTemplate("authnet {{.Version}}\n")
	root.PersistentFlags().BoolVar(&options.JSON, "json", false, "emit the stable JSON contract")
	root.PersistentFlags().BoolVar(&options.Automation, "automation", false, "enable deterministic non-interactive automation mode")
	root.PersistentFlags().StringVar(&options.Profile, "profile", "", "profile name to use for this command")
	root.PersistentFlags().BoolVar(&options.RawResponse, "raw-response", false, "allow sandbox-only raw gateway response output")
	root.PersistentFlags().StringVar(&options.Color, "color", "auto", "control color output: auto, always, never")
	root.PersistentFlags().BoolVar(&options.NoColor, "no-color", false, "disable color output")

	config, err := newCLIConfig(root.PersistentFlags())
	if err != nil {
		root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
			return err
		}
	} else {
		root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
			return validateGlobalOptions(cmd, config, options)
		}
	}

	root.AddCommand(newVersionCommand(build))
	root.AddCommand(newPathsCommand())
	root.AddCommand(newConfigCommand())
	root.AddCommand(newAuthCommand())
	root.AddCommand(newProfileCommand())
	root.AddCommand(newTransactionCommand())
	root.AddCommand(newCustomerProfileCommand())
	root.AddCommand(newResponseCodeCommand())
	root.AddCommand(newSandboxCommand())
	root.AddCommand(newCompletionCommand(root))

	return root
}

// Execute runs the root command and returns the mapped process exit code.
func Execute(command *cobra.Command) ExitCode {
	return execute(command)
}

// ExecuteWithArgs runs the root command with explicit raw arguments.
func ExecuteWithArgs(command *cobra.Command, args []string) ExitCode {
	command.SetArgs(args)
	applyOutputModeFromRawArgs(optionsFromCommand(command), args)
	return execute(command)
}

func execute(command *cobra.Command) ExitCode {
	executed, err := command.ExecuteC()
	if err == nil {
		return exitSuccess
	}

	target := executed
	if target == nil {
		target = command
	}
	err = normalizeCommandError(target, err)
	options := optionsFromCommand(target)
	if !options.JSON {
		var exiting exitingError
		if errors.As(err, &exiting) {
			_, _ = target.ErrOrStderr().Write([]byte(err.Error() + "\n"))
			return exiting.exitCode
		}
		var rendered renderedError
		if !errors.As(err, &rendered) {
			_, _ = target.ErrOrStderr().Write([]byte(err.Error() + "\n"))
		}
		return exitSuccess
	}

	var rendered renderedError
	if errors.As(err, &rendered) {
		return rendered.exitCode
	}
	if options.JSON {
		envelope, code := structuredFailure(target, err)
		if writeErr := writeOutputJSON(target.OutOrStdout(), envelope, colorEnabled(options)); writeErr != nil {
			_, _ = target.ErrOrStderr().Write([]byte(writeErr.Error() + "\n"))
			return exitGeneralFailure
		}
		return code
	}

	return exitCodeForError(err)
}

func applyOutputModeFromRawArgs(options *globalOptions, args []string) {
	for _, arg := range args {
		if arg == "--json" {
			options.JSON = true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--json="); ok {
			if parsed, err := strconv.ParseBool(value); err == nil && parsed {
				options.JSON = true
			}
			continue
		}
		if arg == "--automation" {
			options.Automation = true
			options.JSON = true
			continue
		}
		if value, ok := strings.CutPrefix(arg, "--automation="); ok {
			if parsed, err := strconv.ParseBool(value); err == nil && parsed {
				options.Automation = true
				options.JSON = true
			}
		}
	}
}

func normalizeCommandError(cmd *cobra.Command, err error) error {
	if isUnsupportedUnsettledDateRangeFlag(cmd, err) {
		return newExitingUsageError("no gateway support: %s is incompatible with unsettled transaction list API (no date/time range allowed)", unknownFlagName(err))
	}
	if isUnknownFlagError(err) {
		return newExitingUsageError("%s", err.Error())
	}
	return err
}

func isUnknownFlagError(err error) bool {
	message := err.Error()
	return strings.HasPrefix(message, "unknown flag:") || strings.HasPrefix(message, "unknown shorthand flag:")
}

func isUnsupportedUnsettledDateRangeFlag(cmd *cobra.Command, err error) bool {
	if cmd == nil || cmd.CommandPath() != "authnet transaction unsettled list" || !isUnknownFlagError(err) {
		return false
	}
	switch unknownFlagName(err) {
	case "--last", "--from", "--to":
		return true
	default:
		return false
	}
}

func unknownFlagName(err error) string {
	fields := strings.Fields(err.Error())
	if len(fields) == 0 {
		return "flag"
	}
	return fields[len(fields)-1]
}

func validateGlobalOptions(cmd *cobra.Command, config *cliConfig, options *globalOptions) error {
	if commandUsesPreferences(cmd) {
		if err := config.applyPreferences(); err != nil {
			return err
		}
	}
	config.applyGlobalOptions(options)
	if options.Automation {
		options.JSON = true
	}
	if options.Environment != "" {
		if err := validateEnvironment(options.Environment); err != nil {
			return err
		}
	}
	colorFlagChanged := cmd.Root().PersistentFlags().Lookup(configKeyColor).Changed
	if options.NoColor && !colorFlagChanged {
		options.Color = "never"
	}
	switch options.Color {
	case "auto", "always", "never":
	default:
		if cmd.CommandPath() == "authnet config validate" && !colorFlagChanged && !isColorEnvironmentOverrideSet() {
			break
		}
		return invalidColorValueError(cmd, config, options.Color)
	}
	if options.Automation {
		options.Color = "never"
		options.NoColor = true
	}
	if options.NoColor {
		options.Color = "never"
	}
	if err := resolveProfileEnvironment(options); err != nil {
		return err
	}
	if options.RawResponse && options.Environment == environmentProduction {
		return newSafetyDeniedError("raw response mode requires a sandbox-classified profile or AUTHNET_ENVIRONMENT=sandbox")
	}
	if options.RawResponse && !commandSupportsRawResponse(cmd) {
		return newUsageError("raw response mode is not supported for %s", cmd.CommandPath())
	}
	if options.RawResponse && options.Environment != environmentSandbox {
		return newSafetyDeniedError("raw response mode requires a sandbox-classified profile or AUTHNET_ENVIRONMENT=sandbox")
	}
	return nil
}

func commandSupportsRawResponse(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Annotations != nil && current.Annotations[rawResponseSupportAnnotation] == "supported" {
			return true
		}
	}
	return false
}

func commandUsesPreferences(cmd *cobra.Command) bool {
	return cmd.CommandPath() != "authnet paths"
}

func invalidColorValueError(cmd *cobra.Command, config *cliConfig, color string) error {
	if cmd.Root().PersistentFlags().Lookup(configKeyColor).Changed {
		return newUsageError("invalid --color value %q: expected auto, always, or never", color)
	}
	if isColorEnvironmentOverrideSet() {
		return newUsageError("invalid AUTHNET_COLOR value %q: expected auto, always, or never", color)
	}
	if config.colorPreferenceSet {
		return newExitingUsageError("invalid preferences.color value %q in %s: expected auto, always, or never", color, config.colorPreferencePath)
	}
	return newUsageError("invalid --color value %q: expected auto, always, or never", color)
}

func isColorEnvironmentOverrideSet() bool {
	return strings.TrimSpace(os.Getenv("AUTHNET_COLOR")) != ""
}

func optionsFromCommand(cmd *cobra.Command) *globalOptions {
	if value := cmd.Context().Value(optionsContextKey); value != nil {
		if options, ok := value.(*globalOptions); ok {
			return options
		}
	}
	return &globalOptions{}
}

func buildFromCommand(cmd *cobra.Command) BuildInfo {
	if value := cmd.Context().Value(buildContextKey); value != nil {
		if build, ok := value.(BuildInfo); ok {
			return build
		}
	}
	return BuildInfo{}.normalized()
}
