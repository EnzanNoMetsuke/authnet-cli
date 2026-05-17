package cli

import (
	"context"

	"github.com/spf13/cobra"
)

type contextKey string

const (
	optionsContextKey contextKey = "options"
	buildContextKey   contextKey = "build"
)

type globalOptions struct {
	JSON       bool
	Automation bool
	Profile    string
	Color      string
	NoColor    bool
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
	root.PersistentFlags().StringVar(&options.Color, "color", "auto", "control color output: auto, always, never")
	root.PersistentFlags().BoolVar(&options.NoColor, "no-color", false, "disable color output")

	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return validateGlobalOptions(options)
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
	executed, err := command.ExecuteC()
	if err == nil {
		return exitSuccess
	}

	target := executed
	if target == nil {
		target = command
	}
	options := optionsFromCommand(target)
	if options.JSON {
		envelope, code := structuredFailure(target, err)
		if writeErr := writeOutputJSON(target.OutOrStdout(), envelope, colorEnabled(options)); writeErr != nil {
			_, _ = target.ErrOrStderr().Write([]byte(writeErr.Error() + "\n"))
			return exitGeneralFailure
		}
		return code
	}

	_, _ = target.ErrOrStderr().Write([]byte(err.Error() + "\n"))
	return exitCodeForError(err)
}

func validateGlobalOptions(options *globalOptions) error {
	switch options.Color {
	case "auto", "always", "never":
	default:
		return newUsageError("invalid --color value %q: expected auto, always, or never", options.Color)
	}
	if options.Automation {
		options.JSON = true
		options.Color = "never"
		options.NoColor = true
	}
	if options.NoColor {
		options.Color = "never"
	}
	return nil
}

func notImplemented(name string) func(*cobra.Command, []string) error {
	return func(*cobra.Command, []string) error {
		return newNotImplementedError(name)
	}
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
