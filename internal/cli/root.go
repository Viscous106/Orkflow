/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "orka",
	Short: "A workflow orchestration CLI tool",
	Long: `Orka is a powerful workflow orchestration CLI tool that allows you
to define, validate, and execute workflows using YAML configuration files.

Examples:
  orka run workflow.yaml        Run a workflow
  orka validate workflow.yaml   Validate a workflow file
  orka --help                   Show this help message`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags available to all commands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.orka.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
