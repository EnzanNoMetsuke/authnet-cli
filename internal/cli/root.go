package cli

import (
	"context"
	"errors"
	"fmt"
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

const (
	rawResponseProductionSafetyMessage = "raw response mode is unavailable for production-classified profiles"
	rawResponseSandboxRequiredMessage  = "raw response mode requires a sandbox-classified profile or AUTHNET_ENVIRONMENT=sandbox"
)

type globalOptions struct {
	JSON               bool
	Automation         bool
	Yes                bool
	Profile            string
	Environment        string
	RawResponse        bool
	Color              string
	NoColor            bool
	PreferenceWarnings []warning
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
	requireSubcommandFor(root)
	root.SetContext(ctx)
	root.SetUsageFunc(writeUsage)
	root.CompletionOptions.DisableDefaultCmd = true

	root.SetVersionTemplate("authnet {{.Version}}\n")
	root.PersistentFlags().BoolVar(&options.JSON, "json", false, "emit the stable JSON contract")
	root.PersistentFlags().BoolVar(&options.Automation, "automation", false, "enable deterministic non-interactive automation mode")
	root.PersistentFlags().BoolVar(&options.Yes, "yes", false, "approve non-interactive confirmations where supported")
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

const requiresSubcommandAnnotation = "authnet.requires_subcommand"

func requireSubcommandFor(cmd *cobra.Command) {
	cmd.RunE = requireSubcommand
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[requiresSubcommandAnnotation] = "true"
}

func requireSubcommand(cmd *cobra.Command, _ []string) error {
	message := cmd.CommandPath() + " requires a subcommand"
	if optionsFromCommand(cmd).JSON {
		return newUsageError("%s", message)
	}
	if err := cmd.Help(); err != nil {
		return err
	}
	return renderedError{exitCode: exitUsageOrConfig, message: message}
}

func requireExactArgs(count int, placeholders ...string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == count {
			return nil
		}
		message := missingArgumentMessage(cmd, count, placeholders)
		if optionsFromCommand(cmd).JSON {
			return newUsageError("%s", message)
		}
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), message); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(cmd.ErrOrStderr()); err != nil {
			return err
		}
		if err := cmd.Usage(); err != nil {
			return err
		}
		return renderedError{exitCode: exitUsageOrConfig, message: message}
	}
}

func missingArgumentMessage(cmd *cobra.Command, count int, placeholders []string) string {
	commandPath := strings.TrimPrefix(cmd.CommandPath(), "authnet ")
	if len(placeholders) == 0 {
		return fmt.Sprintf("%s requires %d argument(s)", commandPath, count)
	}
	return fmt.Sprintf("%s requires %s", commandPath, strings.Join(placeholders, " "))
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
		return exitCodeForError(err)
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
	if err := validateOutputModeEnvironmentOverrides(); err != nil {
		return err
	}
	if err := validateOutputModePreferences(cmd, config); err != nil {
		return err
	}
	applyOutputModeExclusivity(cmd, options)
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
	if options.RawResponse {
		options.PreferenceWarnings = append(options.PreferenceWarnings, config.rawResponsePreferenceWarnings()...)
		if options.Environment == environmentProduction {
			return newSafetyDeniedError(rawResponseProductionSafetyMessage)
		}
		if !commandSupportsRawResponse(cmd) {
			return newUsageError("raw response mode is not supported for %s", cmd.CommandPath())
		}
		if options.Environment != environmentSandbox {
			return newSafetyDeniedError(rawResponseSandboxRequiredMessage)
		}
	}
	return nil
}

func applyOutputModeExclusivity(cmd *cobra.Command, options *globalOptions) {
	flags := cmd.Root().PersistentFlags()
	if flags.Lookup(configKeyAutomation).Changed {
		if options.Automation {
			options.JSON = true
		}
		return
	}
	if flags.Lookup(configKeyJSON).Changed && options.JSON {
		options.Automation = false
		return
	}

	automationEnvSet, automationEnvValue := boolEnvironmentOverride("AUTHNET_AUTOMATION")
	if automationEnvSet {
		if automationEnvValue {
			options.Automation = true
			options.JSON = true
			return
		}
	} else if jsonEnvSet, jsonEnvValue := boolEnvironmentOverride("AUTHNET_JSON"); jsonEnvSet && jsonEnvValue {
		options.Automation = false
		return
	}

	if options.Automation {
		options.JSON = true
	}
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
	switch cmd.CommandPath() {
	case "authnet paths", "authnet config migrate":
		return false
	default:
		return true
	}
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

func validateOutputModeEnvironmentOverrides() error {
	for _, envName := range []string{"AUTHNET_JSON", "AUTHNET_AUTOMATION"} {
		value := strings.TrimSpace(os.Getenv(envName))
		if value == "" {
			continue
		}
		if _, err := strconv.ParseBool(value); err != nil {
			return newUsageError("invalid %s value %q: expected true or false", envName, value)
		}
	}
	return nil
}

func boolEnvironmentOverride(name string) (bool, bool) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return true, false
	}
	return true, parsed
}

func validateOutputModePreferences(cmd *cobra.Command, config *cliConfig) error {
	if cmd.CommandPath() == "authnet config validate" {
		return nil
	}
	if config.jsonPreferenceSet && !validOutputModePreference(config.jsonPreferenceValue) {
		return newExitingUsageError("invalid preferences.json value %q in %s: expected always or never", config.jsonPreferenceValue, config.configPath)
	}
	if config.automationPreferenceSet && !validOutputModePreference(config.automationPreferenceValue) {
		return newExitingUsageError("invalid preferences.automation value %q in %s: expected always or never", config.automationPreferenceValue, config.configPath)
	}
	return nil
}

func validOutputModePreference(value string) bool {
	return value == preferenceModeAlways || value == preferenceModeNever
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
