package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell autocompletion scripts for Watchdog commands",
	Long: `To load completions:

Bash:
  $ source <(watchdog completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ watchdog completion bash > /etc/bash_completion.d/watchdog
  # macOS:
  $ watchdog completion bash > $(brew --prefix)/etc/bash_completion.d/watchdog

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ watchdog completion zsh > "${fpath[1]}/_watchdog"

Fish:
  $ watchdog completion fish | source
  # To load completions for each session, execute once:
  $ watchdog completion fish > ~/.config/fish/completions/watchdog.fish

PowerShell:
  PS> watchdog completion powershell | Out-String | Invoke-Expression
  # To load completions for every new PowerShell session:
  PS> watchdog completion powershell > $PROFILE
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Example: `  # Generate Bash completions
  watchdog completion bash

  # Generate Zsh completions
  watchdog completion zsh

  # Generate Fish completions
  watchdog completion fish

  # Generate PowerShell completions
  watchdog completion powershell`,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return RootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return RootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return RootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return RootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return NewExitError(ExitUsageError, "unsupported shell type: %s (use bash, zsh, fish, or powershell)", args[0])
		}
	},
}

func init() {
	RootCmd.AddCommand(completionCmd)
}
