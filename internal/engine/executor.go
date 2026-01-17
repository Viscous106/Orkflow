package engine

import (
	"fmt"
	"sync"

	"Orkflow/internal/agent"
	"Orkflow/pkg/types"
)

type Executor struct {
	Config *types.WorkflowConfig
	Runner *agent.Runner
	State  *State
}

func NewExecutor(config *types.WorkflowConfig) *Executor {
	totalSteps := 0
	if config.Workflow != nil {
		totalSteps = len(config.Workflow.Steps) + len(config.Workflow.Branches)
		if config.Workflow.Then != nil {
			totalSteps++
		}
	}

	return &Executor{
		Config: config,
		Runner: agent.NewRunner(config),
		State:  NewState(totalSteps),
	}
}

func (e *Executor) Execute() (string, error) {
	if e.Config.Workflow == nil {
		return e.executeSupervisor()
	}

	switch e.Config.Workflow.Type {
	case "sequential":
		return e.executeSequential()
	case "parallel":
		return e.executeParallel()
	default:
		return "", fmt.Errorf("unknown workflow type: %s", e.Config.Workflow.Type)
	}
}

func (e *Executor) executeSequential() (string, error) {
	e.State.Start()

	for _, step := range e.Config.Workflow.Steps {
		agentDef := e.Runner.GetAgent(step.Agent)
		if agentDef == nil {
			err := fmt.Errorf("agent not found: %s", step.Agent)
			e.State.Fail(err)
			return "", err
		}

		_, err := e.Runner.RunAgent(agentDef)
		if err != nil {
			e.State.Fail(err)
			return "", err
		}

		e.State.NextStep()
	}

	e.State.Complete()
	return e.Runner.GetFinalOutput(), nil
}

func (e *Executor) executeParallel() (string, error) {
	e.State.Start()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	results := make(map[string]string)

	for _, branchID := range e.Config.Workflow.Branches {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			agentDef := e.Runner.GetAgent(id)
			if agentDef == nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("agent not found: %s", id)
				}
				mu.Unlock()
				return
			}

			response, err := e.Runner.RunAgent(agentDef)
			mu.Lock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			} else {
				results[id] = response
			}
			mu.Unlock()
		}(branchID)
	}

	wg.Wait()

	if firstErr != nil {
		e.State.Fail(firstErr)
		return "", firstErr
	}

	if e.Config.Workflow.Then != nil {
		thenAgent := e.Runner.GetAgent(e.Config.Workflow.Then.Agent)
		if thenAgent == nil {
			err := fmt.Errorf("then agent not found: %s", e.Config.Workflow.Then.Agent)
			e.State.Fail(err)
			return "", err
		}

		_, err := e.Runner.RunAgent(thenAgent)
		if err != nil {
			e.State.Fail(err)
			return "", err
		}
	}

	e.State.Complete()
	return e.Runner.GetFinalOutput(), nil
}

func (e *Executor) executeSupervisor() (string, error) {
	e.State.Start()

	var rootAgent *types.Agent
	for i := range e.Config.Agents {
		if e.Config.Agents[i].IsSupervisor() {
			rootAgent = &e.Config.Agents[i]
			break
		}
	}

	if rootAgent == nil && len(e.Config.Agents) > 0 {
		rootAgent = &e.Config.Agents[0]
	}

	if rootAgent == nil {
		err := fmt.Errorf("no root agent found")
		e.State.Fail(err)
		return "", err
	}

	response, err := e.Runner.RunAgent(rootAgent)
	if err != nil {
		e.State.Fail(err)
		return "", err
	}

	e.State.Complete()
	return response, nil
}

func (e *Executor) GetState() *State {
	return e.State
}
