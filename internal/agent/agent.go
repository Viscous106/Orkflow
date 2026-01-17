package agent

import (
	"fmt"
	"time"

	"Orkflow/pkg/types"
)

const maxRetries = 3

type Runner struct {
	Config  *types.WorkflowConfig
	Context *ContextManager
	Clients map[string]LLMClient
}

func NewRunner(config *types.WorkflowConfig) *Runner {
	runner := &Runner{
		Config:  config,
		Context: NewContextManager(),
		Clients: make(map[string]LLMClient),
	}

	for name, model := range config.Models {
		runner.Clients[name] = NewLLMClient(
			model.Provider,
			model.Model,
			model.APIKey,
			model.Endpoint,
		)
	}

	return runner
}

func (r *Runner) RunAgent(agentDef *types.Agent) (string, error) {
	client, ok := r.Clients[agentDef.Model]
	if !ok {
		return "", fmt.Errorf("model not found: %s", agentDef.Model)
	}

	prompt := r.buildPrompt(agentDef)
	fmt.Printf("[%s] Running agent: %s\n", agentDef.ID, agentDef.Role)

	var response string
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err = client.Generate(prompt)
		if err == nil {
			break
		}
		fmt.Printf("[%s] Attempt %d failed: %v\n", agentDef.ID, attempt, err)

		if attempt < maxRetries {
			time.Sleep(time.Second * time.Duration(attempt))
		}
	}

	if err != nil {
		return "", fmt.Errorf("agent %s failed after %d attempts: %w", agentDef.ID, maxRetries, err)
	}

	r.Context.AddOutput(agentDef.ID, response)
	return response, nil
}

func (r *Runner) buildPrompt(agentDef *types.Agent) string {
	prompt := agentDef.GetPrompt()
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
