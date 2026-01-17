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
	"Orkflow/internal/memory"
	"Orkflow/internal/parser"
	"Orkflow/pkg/types"

	"github.com/spf13/cobra"
)

var (
	sessionID      string
	continueLatest bool
	userPrompt     string
	useProvider    string
	useModel       string
)

var runCmd = &cobra.Command{
	Use:   "run <workflow.yaml>",
	Short: "Run a workflow",
	Long: `Run executes a workflow defined in a YAML file.

The workflow file should contain the definition of agents, steps,
and their execution order (sequential or parallel).

Session Options:
  --session <id>    Continue a specific session
  --continue        Continue the most recent session
  --prompt <text>   Provide input prompt for the session

Model Override:
  --use-provider    Override provider for all agents (e.g., ollama, gemini)
  --use-model       Override model name for all agents

Examples:
  orka run workflow.yaml
  orka run workflow.yaml --use-provider ollama --use-model llama3
  orka run workflow.yaml --continue --prompt "Follow up question"`,
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

		// Apply model/provider overrides if specified
		if useProvider != "" || useModel != "" {
			for name, model := range config.Models {
				if useProvider != "" {
					model.Provider = useProvider
					fmt.Printf("⚡ Overriding provider for '%s' → %s\n", name, useProvider)
				}
				if useModel != "" {
					model.Model = useModel
					fmt.Printf("⚡ Overriding model for '%s' → %s\n", name, useModel)
				}
				model.APIKey = "" // Clear API key so it gets re-prompted for new provider
				config.Models[name] = model
			}
		}

		// Check and prompt for missing API keys
		if err := ensureAPIKeys(config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Handle session
		var session *memory.Session
		if sessionID != "" {
			session, err = memory.LoadSession(sessionID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading session %s: %v\n", sessionID, err)
				os.Exit(1)
			}
			fmt.Printf("📚 Continuing session: %s\n", session.ID)
		} else if continueLatest {
			session, err = memory.GetLatestSession()
			if err != nil || session == nil {
				fmt.Println("No previous session found. Starting new session.")
				session = memory.NewSession(workflowFile)
			} else {
				fmt.Printf("📚 Continuing latest session: %s\n", session.ID)
			}
		} else {
			session = memory.NewSession(workflowFile)
			fmt.Printf("📝 New session: %s\n", session.ID)
		}

		// If user provided a prompt, add it to session
		if userPrompt != "" {
			session.AddMessage("user", "input", userPrompt)
			fmt.Printf("💬 User prompt: %s\n", userPrompt)
		}

		executor := engine.NewExecutor(config)

		// Pass session history (including user prompt) to executor
		executor.SetSessionHistory(session.GetHistory())

		// Set callback to save each agent's response to session
		executor.SetMessageCallback(func(agentID, role, content string) {
			session.AddMessage(agentID, role, content)
		})

		output, err := executor.Execute()
		if err != nil {
			// Save partial session progress before exiting
			if saveErr := session.Save(); saveErr != nil {
				fmt.Printf("Warning: Could not save session: %v\n", saveErr)
			} else {
				fmt.Printf("💾 Partial session saved: %s (use --continue to retry)\n", session.ID)
			}
			fmt.Fprintf(os.Stderr, "Error executing workflow: %v\n", err)

			// Show helpful tip if quota exceeded
			if strings.Contains(err.Error(), "QUOTA_EXCEEDED") {
				fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
				fmt.Println("║  💡 QUOTA EXCEEDED - Switch to a different model          ║")
				fmt.Println("╠═══════════════════════════════════════════════════════════╣")
				fmt.Println("║  Try one of these:                                        ║")
				fmt.Println("║  --use-provider ollama --use-model llama3                 ║")
				fmt.Println("║  --use-model gemini-2.0-flash (different quota bucket)    ║")
				fmt.Println("╚═══════════════════════════════════════════════════════════╝")
			}
			os.Exit(1)
		}

		// Save session (all agent messages already added via callback)
		if err := session.Save(); err != nil {
			fmt.Printf("Warning: Could not save session: %v\n", err)
		}

		// Cleanup old sessions
		memory.CleanupOldSessions()

		fmt.Println("\n--- Final Output ---")
		fmt.Println(output)
		fmt.Printf("\n💾 Session saved: %s\n", session.ID)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&sessionID, "session", "", "Continue a specific session by ID")
	runCmd.Flags().BoolVar(&continueLatest, "continue", false, "Continue the most recent session")
	runCmd.Flags().StringVarP(&userPrompt, "prompt", "p", "", "Provide input prompt for the session")
	runCmd.Flags().StringVar(&useProvider, "use-provider", "", "Override provider for all agents (e.g., ollama, gemini)")
	runCmd.Flags().StringVar(&useModel, "use-model", "", "Override model for all agents (e.g., llama3, gemini-2.5-flash)")
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
