/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for orka.

To load completions:

Bash:
  $ source <(orka completion bash)
  # To load completions for each session, add to ~/.bashrc:
  $ echo 'source <(orka completion bash)' >> ~/.bashrc

Zsh:
  $ source <(orka completion zsh)
  # To load completions for each session, add to ~/.zshrc:
  $ echo 'source <(orka completion zsh)' >> ~/.zshrc

Fish:
  $ orka completion fish | source
  # To load completions for each session:
  $ orka completion fish > ~/.config/fish/completions/orka.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
