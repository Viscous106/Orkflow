package agent

import (
	"fmt"
	"time"

	"Orkflow/internal/logging"
	"Orkflow/internal/memory"
	"Orkflow/internal/tools"
	"Orkflow/pkg/types"
)

const maxRetries = 3

type Runner struct {
	Config          *types.WorkflowConfig
	Context         *ContextManager
	Clients         map[string]LLMClient
	SessionHistory  string
	MessageCallback func(agentID, role, content string) // Called when agent completes
	SharedMemory    *memory.SharedMemory                // Shared memory for inter-agent communication
	Logger          *logging.Logger                     // Execution logger
}

func NewRunner(config *types.WorkflowConfig) *Runner {
	runner := &Runner{
		Config:  config,
		Context: NewContextManager(),
		Clients: make(map[string]LLMClient),
	}

	for name, model := range config.Models {
		fmt.Printf("DEBUG: Creating client for model '%s' with provider='%s' model='%s'\n", name, model.Provider, model.Model)
		runner.Clients[name] = NewLLMClient(
			model.Provider,
			model.Model,
			model.APIKey,
			model.Endpoint,
		)
	}

	return runner
}

// SetSessionHistory stores previous session context
func (r *Runner) SetSessionHistory(history string) {
	r.SessionHistory = history
}

var spinnerStyles = [][]string{
	{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}, // dots
	{"◐", "◓", "◑", "◒"},                     // circle
	{"▖", "▘", "▝", "▗"},                     // square
	{"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"}, // arrows
	{"◴", "◷", "◶", "◵"},                     // pie
	{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}, // braille
}

func getSpinnerForAgent(agentID string) []string {
	hash := 0
	for _, c := range agentID {
		hash += int(c)
	}
	return spinnerStyles[hash%len(spinnerStyles)]
}

func (r *Runner) RunAgent(agentDef *types.Agent) (string, error) {
	client, ok := r.Clients[agentDef.Model]
	if !ok {
		return "", fmt.Errorf("model not found: %s", agentDef.Model)
	}

	// Wait for required keys from shared memory
	if r.SharedMemory != nil && len(agentDef.Requires) > 0 {
		fmt.Printf("[%s] ⏳ Waiting for required data: %v\n", agentDef.ID, agentDef.Requires)
		if r.Logger != nil {
			r.Logger.LogAgent(agentDef.ID, "WAITING_FOR_REQUIRED", fmt.Sprintf("Keys: %v", agentDef.Requires))
		}
		for _, key := range agentDef.Requires {
			val, err := r.SharedMemory.WaitFor(key, 5*time.Minute) // 5 min timeout for slow models
			if err != nil {
				return "", fmt.Errorf("agent %s: failed to get required key '%s': %w", agentDef.ID, key, err)
			}
			// Inject into context
			r.Context.AddOutput(fmt.Sprintf("shared:%s", key), fmt.Sprintf("%v", val))
			fmt.Printf("[%s] ✓ Received '%s' from shared memory\n", agentDef.ID, key)
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "SHARED_MEMORY_RECEIVED", key)
			}
		}
	}

	prompt := r.buildPrompt(agentDef)
	spinner := getSpinnerForAgent(agentDef.ID)
	fmt.Printf("[%s] Running agent: %s\n", agentDef.ID, agentDef.Role)
	if r.Logger != nil {
		r.Logger.LogAgent(agentDef.ID, "STARTED", fmt.Sprintf("Role: %s", agentDef.Role))
	}

	var response string
	var err error
	startTime := time.Now()

	// Start progress indicator (log-based for parallel compatibility)
	done := make(chan bool)
	go func() {
		i := 0
		lastLog := time.Now()
		for {
			select {
			case <-done:
				return
			default:
				elapsed := time.Since(startTime).Seconds()
				// Log every 5 seconds for parallel agents
				if time.Since(lastLog) >= 5*time.Second {
					fmt.Printf("[%s] %s Still generating... (%.0fs)\n", agentDef.ID, spinner[i%len(spinner)], elapsed)
					lastLog = time.Now()
				}
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err = client.Generate(prompt)
		if err == nil {
			break
		}
		fmt.Printf("[%s] Attempt %d failed: %v\n", agentDef.ID, attempt, err)

		if attempt < maxRetries {
			fmt.Printf("[%s] Retrying in %d seconds...\n", agentDef.ID, attempt)
			time.Sleep(time.Second * time.Duration(attempt))
		}
	}

	close(done)
	elapsed := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("agent %s failed after %d attempts: %w", agentDef.ID, maxRetries, err)
	}

	fmt.Printf("[%s] ✓ Completed in %.1fs (%d chars)\n", agentDef.ID, elapsed.Seconds(), len(response))

	// Handle tool calls if agent has tools
	if len(agentDef.Tools) > 0 && tools.HasToolCalls(response) {
		toolCalls := tools.ParseToolCalls(response)
		if len(toolCalls) > 0 {
			results := tools.ExecuteToolCalls(toolCalls)

			// Log tool execution
			if r.Logger != nil {
				for i, res := range results {
					input := toolCalls[i].Input
					output := res.Output
					if res.Error != nil {
						output = fmt.Sprintf("ERROR: %v", res.Error)
					}
					r.Logger.LogToolCall(res.ToolName, input, output)
				}
			}

			toolOutput := tools.FormatToolResults(results)

			// Make a follow-up call with tool results
			if toolOutput != "" {
				followupPrompt := prompt + "\n\nPrevious response:\n" + response + toolOutput + "\n\nNow provide your final response incorporating the tool results:"
				followupResponse, followupErr := client.Generate(followupPrompt)
				if followupErr == nil {
					response = followupResponse
					fmt.Printf("[%s] ✓ Follow-up completed (%d chars)\n", agentDef.ID, len(response))
				}
			}
		}
	}

	r.Context.AddOutput(agentDef.ID, response)

	// Publish outputs to shared memory
	if r.SharedMemory != nil && len(agentDef.Outputs) > 0 {
		for _, key := range agentDef.Outputs {
			r.SharedMemory.Set(key, response)
			fmt.Printf("[%s] 📤 Published '%s' to shared memory\n", agentDef.ID, key)
			if r.Logger != nil {
				r.Logger.LogAgent(agentDef.ID, "SHARED_MEMORY_PUBLISH", key)
			}
		}
	}

	// Log full output
	if r.Logger != nil {
		r.Logger.LogAgentOutput(agentDef.ID, agentDef.Role, response)
		r.Logger.LogAgent(agentDef.ID, "COMPLETED", fmt.Sprintf("Duration: %.1fs, Chars: %d", elapsed.Seconds(), len(response)))
	}

	// Save to session if callback is set
	if r.MessageCallback != nil {
		r.MessageCallback(agentDef.ID, agentDef.Role, response)
	}

	return response, nil
}

func (r *Runner) buildPrompt(agentDef *types.Agent) string {
	prompt := agentDef.GetPrompt()

	// Add session history from previous runs
	if r.SessionHistory != "" {
		prompt = r.SessionHistory + "\n\n" + prompt
	}

	// Add context from current run
	context := r.Context.GetContext()
	if context != "" {
		prompt = prompt + "\n\n" + context
	}

	// Add tool descriptions if agent has tools or toolsets
	var allTools []tools.Tool

	// 1. Add explicitly listed tools
	if len(agentDef.Tools) > 0 {
		agentTools, err := tools.GetByNames(agentDef.Tools)
		if err == nil {
			allTools = append(allTools, agentTools...)
		}
	}

	// 2. Add tools from toolsets (MCP servers)
	if len(agentDef.Toolsets) > 0 {
		for _, toolset := range agentDef.Toolsets {
			// Get tools starting with "serverName."
			setTools := tools.GetByPrefix(toolset + ".")
			allTools = append(allTools, setTools...)
		}
	}

	if len(allTools) > 0 {
		prompt = prompt + "\n\n" + tools.FormatToolsForPrompt(allTools)
	}

	return prompt
}

func (r *Runner) GetAgent(id string) *types.Agent {
	for i := range r.Config.Agents {
		if r.Config.Agents[i].ID == id {
			return &r.Config.Agents[i]
		}
	}
	return nil
}

func (r *Runner) GetFinalOutput() string {
	return r.Context.GetLastOutput()
}
