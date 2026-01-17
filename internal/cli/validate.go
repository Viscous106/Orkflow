/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"fmt"
	"os"

	"Orkflow/internal/parser"

	"github.com/spf13/cobra"
)

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

		if verbose {
			fmt.Printf("Validating workflow: %s\n", workflowFile)
		}

		config, err := parser.ParseYAML(workflowFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ Validation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Workflow is valid\n")
		fmt.Printf("  Agents: %d\n", len(config.Agents))
		if config.Workflow != nil {
			fmt.Printf("  Type: %s\n", config.Workflow.Type)
			fmt.Printf("  Steps: %d\n", len(config.Workflow.Steps))
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
