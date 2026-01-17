/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run <workflow.yaml>",
	Short: "Run a workflow",
	Long: `Run executes a workflow defined in a YAML file.

The workflow file should contain the definition of agents, steps,
and their execution order (sequential or parallel).

Examples:
  orka run workflow.yaml
  orka run examples/sequential.yaml --verbose`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowFile := args[0]
		fmt.Printf("Running workflow: %s\n", workflowFile)

		if verbose {
			fmt.Println("Verbose mode enabled")
		}

		// TODO: Implement workflow execution
		fmt.Println("Workflow execution not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
