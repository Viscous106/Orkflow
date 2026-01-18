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
	"Orkflow/internal/logging"
	"Orkflow/internal/memory"
	"Orkflow/internal/parser"
	"Orkflow/internal/vectorstore"
	"Orkflow/pkg/types"

	"github.com/spf13/cobra"
)

var (
	sessionID      string
	continueLatest bool
	userPrompt     string
	useProvider    string
	useModel       string
	smartContext   bool
	enableLogging  bool
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
  --smart-context   Auto-inject relevant context from past sessions (requires Ollama)

Model Override:
  --use-provider    Override provider for all agents (e.g., ollama, gemini)
  --use-provider    Override provider for all agents (e.g., ollama, gemini)
  --use-model       Override model name for all agents

Logging:
  --log             Enable file-based execution logging

Examples:
  orka run workflow.yaml
  orka run workflow.yaml --smart-context
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

		// Handle Smart Context (Vector Search)
		if smartContext {
			fmt.Println("🧠 Smart Context: Searching past sessions...")
			// TODO: Make embedding model configurable
			store, err := vectorstore.NewChromemStoreWithOllama("nomic-embed-text")
			if err == nil {
				defer store.Close()
				query := userPrompt
				if query == "" {
					query = "workflow execution context" // Default query if no prompt
				}

				results, err := store.Search(query, 3)
				if err == nil && len(results) > 0 {
					fmt.Printf("   Found %d relevant past messages. Injecting into context.\n", len(results))
					contextMsg := "=== RELEVANT PAST CONTEXT ===\n"
					for _, r := range results {
						contextMsg += fmt.Sprintf("From Session %s:\n%s\n---\n", r.ID, r.Content)
					}
					session.AddMessage("system", "context", contextMsg)
				} else {
					fmt.Println("   No relevant context found.")
				}
			} else {
				fmt.Printf("   Warning: Smart context failed (is Ollama running?): %v\n", err)
			}
		}

		// Initialize logger if enabled
		var logger *logging.Logger
		if enableLogging {
			var err error
			logger, err = logging.NewLogger(session.ID, "")
			if err != nil {
				fmt.Printf("⚠️  Failed to create logger: %v\n", err)
			} else {
				fmt.Printf("📝 Logging execution to: %s\n", logger.GetFilePath())
				defer logger.Close()
			}
		} else {
			// Use null logger if disabled
			logger = &logging.Logger{} // Will be handled as disabled
		}

		executor := engine.NewExecutor(config)
		if enableLogging && logger != nil {
			executor.SetLogger(logger)
		}

		// Pass session history (including user prompt) to executor
		executor.SetSessionHistory(session.GetHistory())

		// Set callback to save each agent's response to session
		executor.SetMessageCallback(func(agentID, role, content string) {
			session.AddMessage(agentID, role, content)
		})

		// Display workflow start banner with diagram
		fmt.Println("\n" + ColorGreen + "╔═══════════════════════════════════════════════════════════════════════════════╗" + ColorReset)
		fmt.Println(ColorGreen + "║" + ColorReset + ColorBold + "                         🚀 STARTING WORKFLOW 🚀                               " + ColorReset + ColorGreen + "║" + ColorReset)
		fmt.Println(ColorGreen + "╚═══════════════════════════════════════════════════════════════════════════════╝" + ColorReset)

		if config.Workflow != nil {
			fmt.Println()
			if config.Workflow.Type == "sequential" && len(config.Workflow.Steps) > 0 {
				// Sequential diagram
				fmt.Println("                              ┌─────────────────┐")
				for i, step := range config.Workflow.Steps {
					agent := getAgentByID(config.Agents, step.Agent)
					role := step.Agent
					if agent != nil && agent.Role != "" {
						role = agent.Role
					}
					if len(role) > 15 {
						role = role[:12] + "..."
					}
					fmt.Printf("                              │ %-15s │\n", role)
					fmt.Println("                              └────────┬────────┘")
					if i < len(config.Workflow.Steps)-1 {
						fmt.Println("                                       │")
						fmt.Println("                                       ▼")
						fmt.Println("                              ┌─────────────────┐")
					}
				}
			} else if config.Workflow.Type == "parallel" && len(config.Workflow.Branches) > 0 {
				// Parallel diagram
				branchCount := len(config.Workflow.Branches)

				// Top branches
				fmt.Print("           ")
				for i := 0; i < branchCount; i++ {
					fmt.Print("┌─────────────────┐")
					if i < branchCount-1 {
						fmt.Print("     ")
					}
				}
				fmt.Println()

				fmt.Print("           ")
				for i, branchID := range config.Workflow.Branches {
					agent := getAgentByID(config.Agents, branchID)
					role := branchID
					if agent != nil && agent.Role != "" {
						role = agent.Role
					}
					if len(role) > 15 {
						role = role[:12] + "..."
					}
					fmt.Printf("│ %-15s │", role)
					if i < branchCount-1 {
						fmt.Print("     ")
					}
				}
				fmt.Println()

				fmt.Print("           ")
				for i := 0; i < branchCount; i++ {
					fmt.Print("└────────┬────────┘")
					if i < branchCount-1 {
						fmt.Print("     ")
					}
				}
				fmt.Println()

				// Converging arrows
				fmt.Print("                    ")
				for i := 0; i < branchCount; i++ {
					fmt.Print("│")
					if i < branchCount-1 {
						fmt.Print("                        ")
					}
				}
				fmt.Println()

				fmt.Println("                    └────────────┬───────────┘")
				fmt.Println("                                 ▼")

				// Then agent
				if config.Workflow.Then != nil {
					agent := getAgentByID(config.Agents, config.Workflow.Then.Agent)
					role := config.Workflow.Then.Agent
					if agent != nil && agent.Role != "" {
						role = agent.Role
					}
					if len(role) > 15 {
						role = role[:12] + "..."
					}
					fmt.Println("                        ┌─────────────────┐")
					fmt.Printf("                        │ %-15s │\n", role)
					fmt.Println("                        └─────────────────┘")
				}
			}
			fmt.Println()
		}

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
				fmt.Println("║  --use-provider openai --use-model gpt-4o-mini            ║")
				fmt.Println("║  Wait a few minutes and retry with --continue             ║")
				fmt.Println("╚═══════════════════════════════════════════════════════════╝")
			}

			// Show helpful tip for API key errors
			errStr := err.Error()
			if strings.Contains(errStr, "API_KEY") || strings.Contains(errStr, "invalid_api_key") || strings.Contains(errStr, "API key") {
				fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
				fmt.Println("║  🔑 API KEY ERROR - Check your credentials                ║")
				fmt.Println("╠═══════════════════════════════════════════════════════════╣")
				fmt.Println("║  Solutions:                                               ║")
				fmt.Println("║  1. Set env: export GEMINI_API_KEY='...'                  ║")
				fmt.Println("║  2. Set env: export OPENAI_API_KEY='...'                  ║")
				fmt.Println("║  3. Override: --use-provider openai --use-model gpt-4o-mini║")
				fmt.Println("╚═══════════════════════════════════════════════════════════╝")
			}
			os.Exit(1)
		}

		// Save session (all agent messages already added via callback)
		if err := session.Save(); err != nil {
			fmt.Printf("Warning: Could not save session: %v\n", err)
		}

		// Index session in vector store (optional, fails silently if Ollama not running)
		go func() {
			store, err := vectorstore.NewChromemStoreWithOllama("nomic-embed-text")
			if err == nil {
				if err := vectorstore.IndexSession(store, session); err == nil {
					fmt.Println("🧠 Session indexed for smart context.")
				}
				store.Close()
			}
		}()

		// Cleanup old sessions
		memory.CleanupOldSessions()

		// Decorated final output
		fmt.Println("\n" + ColorCyan + "╔═══════════════════════════════════════════════════════════════════════════════╗" + ColorReset)
		fmt.Println(ColorCyan + "║" + ColorReset + ColorBold + "                              ✨ WORKFLOW COMPLETE ✨                           " + ColorReset + ColorCyan + "║" + ColorReset)
		fmt.Println(ColorCyan + "╚═══════════════════════════════════════════════════════════════════════════════╝" + ColorReset)
		fmt.Println()
		fmt.Println(output)
		fmt.Println()

		// Stats summary
		elapsed := executor.Stats.GetElapsedTime()
		cost := executor.Stats.EstimateCost()

		fmt.Println(ColorGreen + "╔═══════════════════════════════════════════════════════════════════════════════╗" + ColorReset)
		fmt.Printf(ColorGreen+"║"+ColorReset+"  💾 Session: "+ColorBold+"%-64s"+ColorReset+ColorGreen+" ║"+ColorReset+"\n", session.ID)
		fmt.Printf(ColorGreen+"║"+ColorReset+"  ⏱️  Time: %-68s"+ColorGreen+" ║"+ColorReset+"\n", FormatDuration(elapsed.Seconds()))
		if cost > 0 {
			fmt.Printf(ColorGreen+"║"+ColorReset+"  💰 Est. Cost: "+ColorYellow+"$%.6f"+ColorReset+"%-56s"+ColorGreen+" ║"+ColorReset+"\n", cost, "")
		}
		fmt.Println(ColorGreen + "╚═══════════════════════════════════════════════════════════════════════════════╝" + ColorReset)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&sessionID, "session", "", "Continue a specific session by ID")
	runCmd.Flags().BoolVar(&continueLatest, "continue", false, "Continue the most recent session")
	runCmd.Flags().StringVarP(&userPrompt, "prompt", "p", "", "Provide input prompt for the session")
	runCmd.Flags().BoolVar(&smartContext, "smart-context", false, "Auto-inject relevant context from past sessions")
	runCmd.Flags().StringVar(&useProvider, "use-provider", "", "Override provider for all agents (e.g., ollama, gemini)")
	runCmd.Flags().StringVar(&useModel, "use-model", "", "Override model for all agents (e.g., llama3, gemini-2.5-flash)")
	runCmd.Flags().BoolVar(&enableLogging, "log", false, "Enable file-based execution logging")
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

// getAgentByID finds an agent by ID from the agents list
func getAgentByID(agents []types.Agent, id string) *types.Agent {
	for i := range agents {
		if agents[i].ID == id {
			return &agents[i]
		}
	}
	return nil
}
