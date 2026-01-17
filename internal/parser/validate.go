package parser

import (
	"fmt"

	"Orkflow/pkg/types"
)

func validate(config *types.WorkflowConfig) error {
	if len(config.Agents) == 0 {
		return fmt.Errorf("no agents defined")
	}

	agentIDs := make(map[string]bool)
	for _, agent := range config.Agents {
		if agent.ID == "" {
			return fmt.Errorf("agent missing id")
		}
		if agentIDs[agent.ID] {
			return fmt.Errorf("duplicate agent id: %s", agent.ID)
		}
		agentIDs[agent.ID] = true
	}

	if config.Workflow != nil {
		if err := validateWorkflow(config.Workflow, agentIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateWorkflow(wf *types.WorkflowSpec, agentIDs map[string]bool) error {
	if wf.Type != "sequential" && wf.Type != "parallel" {
		return fmt.Errorf("invalid workflow type: %s", wf.Type)
	}
	for _, step := range wf.Steps {
		if !agentIDs[step.Agent] {
			return fmt.Errorf("unknown agent in steps: %s", step.Agent)
		}
	}
	for _, branch := range wf.Branches {
		if !agentIDs[branch] {
			return fmt.Errorf("unknown agent in branches: %s", branch)
		}
	}
	if wf.Then != nil && !agentIDs[wf.Then.Agent] {
		return fmt.Errorf("unknown agent in then: %s", wf.Then.Agent)
	}
	return nil
}
