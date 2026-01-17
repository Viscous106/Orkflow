/*
Copyright © 2026 Orkflow Authors
*/
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"Orkflow/internal/engine"
	"Orkflow/internal/parser"
	"Orkflow/pkg/types"

	"github.com/spf13/cobra"
)

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

		if verbose {
			fmt.Printf("Running workflow: %s\n", workflowFile)
		}

		config, err := parser.ParseYAML(workflowFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing workflow: %v\n", err)
			os.Exit(1)
		}

		if verbose {
			fmt.Printf("Loaded %d agents\n", len(config.Agents))
		}

		// Check and prompt for missing API keys
		if err := ensureAPIKeys(config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		executor := engine.NewExecutor(config)
		output, err := executor.Execute()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing workflow: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n--- Final Output ---")
		fmt.Println(output)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func ensureAPIKeys(config *types.WorkflowConfig) error {
	cliConfig := LoadEffectiveConfig()

	for name, model := range config.Models {
		// Ollama doesn't need API key
		if model.Provider == "ollama" {
			continue
		}

		if model.APIKey != "" {
			continue
		}

		// Check environment variable
		envKey := getEnvKeyName(model.Provider)
		if envVal := os.Getenv(envKey); envVal != "" {
			model.APIKey = envVal
			config.Models[name] = model
			continue
		}

		// Check CLI config
		if cliConfig.APIKey != "" && (cliConfig.Provider == model.Provider || cliConfig.Provider == "") {
			model.APIKey = cliConfig.APIKey
			config.Models[name] = model
			continue
		}

		// Prompt user for API key
		fmt.Printf("API key required for %s (%s)\n", name, model.Provider)
		fmt.Printf("Enter API key (or set %s environment variable): ", envKey)

		reader := bufio.NewReader(os.Stdin)
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}

		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			return fmt.Errorf("API key is required for %s", name)
		}

		model.APIKey = apiKey
		config.Models[name] = model

		// Ask if user wants to save
		fmt.Print("Save this API key to config? (y/n): ")
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) == "y" {
			cliConfig.APIKey = apiKey
			cliConfig.Provider = model.Provider
			if err := saveConfig(getGlobalConfigPath(), cliConfig); err != nil {
				fmt.Printf("Warning: Could not save config: %v\n", err)
			} else {
				fmt.Println("✓ API key saved to config")
			}
		}
	}

	return nil
}

func getEnvKeyName(provider string) string {
	switch provider {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	case "gemini", "google":
		return "GEMINI_API_KEY"
	default:
		return strings.ToUpper(provider) + "_API_KEY"
	}
}
