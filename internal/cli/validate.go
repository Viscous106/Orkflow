/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate <workflow.yaml>",
	Short: "Validate a workflow file",
	Long: `Validate checks a workflow YAML file for syntax errors and
structural issues without executing it.

This is useful for checking your workflow definitions before running them.

Examples:
  orka validate workflow.yaml
  orka validate examples/sequential.yaml`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workflowFile := args[0]
		fmt.Printf("Validating workflow: %s\n", workflowFile)

		// TODO: Implement workflow validation
		fmt.Println("Workflow validation not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
