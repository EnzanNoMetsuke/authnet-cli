package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	completion := &cobra.Command{
		Use:   "completion",
		Short: "Generate static shell completion scripts",
	}
	requireSubcommandFor(completion)

	completion.AddCommand(&cobra.Command{
		Use:   "bash",
		Short: "Generate bash completion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), staticBashCompletion(root))
			return err
		},
	})
	completion.AddCommand(&cobra.Command{
		Use:   "zsh",
		Short: "Generate zsh completion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), staticZshCompletion(root))
			return err
		},
	})
	completion.AddCommand(&cobra.Command{
		Use:   "fish",
		Short: "Generate fish completion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), staticFishCompletion(root))
			return err
		},
	})
	completion.AddCommand(&cobra.Command{
		Use:   "powershell",
		Short: "Generate PowerShell completion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), staticPowerShellCompletion(root))
			return err
		},
	})

	return completion
}

func staticBashCompletion(root *cobra.Command) string {
	commands := strings.Join(staticCommandWords(root), " ")
	flags := strings.Join(staticFlagWords(root), " ")
	return fmt.Sprintf(`# bash completion for authnet
_authnet_completions()
{
  local cur
  cur="${COMP_WORDS[COMP_CWORD]}"
  local commands="%s"
  local flags="%s"

  if [[ "${cur}" == -* ]]; then
    COMPREPLY=( $(compgen -W "${flags}" -- "${cur}") )
  else
    COMPREPLY=( $(compgen -W "${commands} ${flags}" -- "${cur}") )
  fi
}
complete -F _authnet_completions authnet
`, commands, flags)
}

func staticZshCompletion(root *cobra.Command) string {
	return fmt.Sprintf("#compdef authnet\n_arguments '*:: :->cmds' && return 0\ncase $state in\n  cmds) _values 'authnet commands' %s ;;\nesac\n", strings.Join(staticCommandWords(root), " "))
}

func staticFishCompletion(root *cobra.Command) string {
	var builder strings.Builder
	for _, command := range staticCommandWords(root) {
		fmt.Fprintf(&builder, "complete -c authnet -f -a %q\n", command)
	}
	for _, flag := range staticFlagWords(root) {
		fmt.Fprintf(&builder, "complete -c authnet -f -l %q\n", strings.TrimPrefix(flag, "--"))
	}
	return builder.String()
}

func staticPowerShellCompletion(root *cobra.Command) string {
	words := append(staticCommandWords(root), staticFlagWords(root)...)
	return fmt.Sprintf(`Register-ArgumentCompleter -Native -CommandName authnet -ScriptBlock {
  param($wordToComplete)
  @(%s) | Where-Object { $_ -like "$wordToComplete*" }
}
`, quotePowerShellWords(words))
}

func staticCommandWords(root *cobra.Command) []string {
	seen := map[string]struct{}{}
	var walk func(*cobra.Command)
	walk = func(command *cobra.Command) {
		for _, child := range command.Commands() {
			if child.Hidden {
				continue
			}
			name := child.Name()
			if name != "" {
				seen[name] = struct{}{}
			}
			walk(child)
		}
	}
	walk(root)
	return sortedKeys(seen)
}

func staticFlagWords(root *cobra.Command) []string {
	seen := map[string]struct{}{}
	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		seen["--"+flag.Name] = struct{}{}
	})
	root.Flags().VisitAll(func(flag *pflag.Flag) {
		seen["--"+flag.Name] = struct{}{}
	})
	seen["--help"] = struct{}{}
	seen["--version"] = struct{}{}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func quotePowerShellWords(words []string) string {
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ReplaceAll(word, "'", "''")))
	}
	return strings.Join(quoted, ", ")
}
