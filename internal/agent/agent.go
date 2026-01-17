package agent

import (
	"fmt"
	"time"

	"Orkflow/pkg/types"
)

const maxRetries = 3

type Runner struct {
	Config          *types.WorkflowConfig
	Context         *ContextManager
	Clients         map[string]LLMClient
	SessionHistory  string
	MessageCallback func(agentID, role, content string) // Called when agent completes
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

	prompt := r.buildPrompt(agentDef)
	spinner := getSpinnerForAgent(agentDef.ID)
	fmt.Printf("[%s] Running agent: %s\n", agentDef.ID, agentDef.Role)

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
	r.Context.AddOutput(agentDef.ID, response)

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
