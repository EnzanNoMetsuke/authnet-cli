package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func writeUsage(cmd *cobra.Command) error {
	var builder strings.Builder

	appendString(&builder, "Usage:")
	writeUsageLines(&builder, cmd)
	writeAliases(&builder, cmd)
	writeExamples(&builder, cmd)
	writeAvailableCommands(&builder, cmd)
	writeFlags(&builder, cmd)
	writeAdditionalHelpTopics(&builder, cmd)
	writeSubcommandHelpHint(&builder, cmd)
	appendString(&builder, "\n")

	_, err := fmt.Fprint(cmd.OutOrStdout(), builder.String())
	return err
}

func writeUsageLines(builder *strings.Builder, cmd *cobra.Command) {
	switch {
	case requiresSubcommand(cmd):
		appendf(builder, "\n  %s <command>%s", cmd.CommandPath(), flagsSuffix(cmd))
	case cmd.Runnable():
		appendf(builder, "\n  %s", cmd.UseLine())
	case cmd.HasAvailableSubCommands():
		appendf(builder, "\n  %s <command>", cmd.CommandPath())
	}
}

func writeAliases(builder *strings.Builder, cmd *cobra.Command) {
	if len(cmd.Aliases) == 0 {
		return
	}
	appendf(builder, "\n\nAliases:\n  %s", cmd.NameAndAliases())
}

func writeExamples(builder *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasExample() {
		return
	}
	appendf(builder, "\n\nExamples:\n%s", cmd.Example)
}

func writeAvailableCommands(builder *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasAvailableSubCommands() {
		return
	}

	commands := cmd.Commands()
	if len(cmd.Groups()) == 0 {
		appendString(builder, "\n\nAvailable Commands:")
		for _, subcommand := range commands {
			if subcommand.IsAvailableCommand() || subcommand.Name() == "help" {
				appendf(builder, "\n  %s %s", rpad(subcommand.Name(), subcommand.NamePadding()), subcommand.Short)
			}
		}
		return
	}

	for _, group := range cmd.Groups() {
		appendf(builder, "\n\n%s", group.Title)
		for _, subcommand := range commands {
			if subcommand.GroupID == group.ID && (subcommand.IsAvailableCommand() || subcommand.Name() == "help") {
				appendf(builder, "\n  %s %s", rpad(subcommand.Name(), subcommand.NamePadding()), subcommand.Short)
			}
		}
	}
	if cmd.AllChildCommandsHaveGroup() {
		return
	}
	appendString(builder, "\n\nAdditional Commands:")
	for _, subcommand := range commands {
		if subcommand.GroupID == "" && (subcommand.IsAvailableCommand() || subcommand.Name() == "help") {
			appendf(builder, "\n  %s %s", rpad(subcommand.Name(), subcommand.NamePadding()), subcommand.Short)
		}
	}
}

func writeFlags(builder *strings.Builder, cmd *cobra.Command) {
	if cmd.HasAvailableLocalFlags() {
		appendf(builder, "\n\nFlags:\n%s", strings.TrimRight(cmd.LocalFlags().FlagUsages(), " \n"))
	}
	if cmd.HasAvailableInheritedFlags() {
		appendf(builder, "\n\nGlobal Flags:\n%s", strings.TrimRight(cmd.InheritedFlags().FlagUsages(), " \n"))
	}
}

func writeAdditionalHelpTopics(builder *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasHelpSubCommands() {
		return
	}
	appendString(builder, "\n\nAdditional help topics:")
	for _, subcommand := range cmd.Commands() {
		if subcommand.IsAdditionalHelpTopicCommand() {
			appendf(builder, "\n  %s %s", rpad(subcommand.CommandPath(), subcommand.CommandPathPadding()), subcommand.Short)
		}
	}
}

func writeSubcommandHelpHint(builder *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasAvailableSubCommands() {
		return
	}
	appendf(builder, "\n\nUse \"%s <command> --help\" for more information about a command.", cmd.CommandPath())
}

func requiresSubcommand(cmd *cobra.Command) bool {
	return cmd.Annotations[requiresSubcommandAnnotation] == "true"
}

func flagsSuffix(cmd *cobra.Command) string {
	if cmd.HasAvailableFlags() {
		return " [flags]"
	}
	return ""
}

func rpad(value string, padding int) string {
	template := fmt.Sprintf("%%-%ds", padding)
	return fmt.Sprintf(template, value)
}

func appendString(builder *strings.Builder, value string) {
	_, _ = builder.WriteString(value)
}

func appendf(builder *strings.Builder, format string, args ...any) {
	_, _ = fmt.Fprintf(builder, format, args...)
}
