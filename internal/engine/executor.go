package engine

import (
	"fmt"
	"sync"

	"Orkflow/internal/agent"
	"Orkflow/internal/logging"
	"Orkflow/internal/mcp"
	"Orkflow/internal/memory"
	"Orkflow/pkg/types"
)

type Executor struct {
	Config       *types.WorkflowConfig
	Runner       *agent.Runner
	State        *State
	SharedMemory *memory.SharedMemory
	MCPClient    *mcp.Client
	Logger       *logging.Logger
	Stats        *ExecutionStats
}

func NewExecutor(config *types.WorkflowConfig) *Executor {
	totalSteps := 0
	if config.Workflow != nil {
		totalSteps = len(config.Workflow.Steps) + len(config.Workflow.Branches)
		if config.Workflow.Then != nil {
			totalSteps++
		}
	}

	// Create shared memory for this workflow execution
	sharedMem := memory.NewSharedMemory("")

	runner := agent.NewRunner(config)
	runner.SharedMemory = sharedMem // Pass shared memory to runner

	executor := &Executor{
		Config:       config,
		Runner:       runner,
		State:        NewState(totalSteps),
		SharedMemory: sharedMem,
		Stats:        NewExecutionStats(),
	}

	// Connect to MCP servers if defined
	if len(config.MCPServers) > 0 {
		executor.MCPClient = mcp.NewClient()
		for name, serverConfig := range config.MCPServers {
			mcpConfig := mcp.ServerConfig{
				Command: serverConfig.Command,
				Args:    serverConfig.Args,
				Env:     serverConfig.Env,
			}
			if err := executor.MCPClient.Connect(name, mcpConfig); err != nil {
				fmt.Printf("⚠️  Failed to connect to MCP server '%s': %v\n", name, err)
			} else {
				// Register MCP tools with the tool registry
				mcp.RegisterMCPTools(executor.MCPClient, name)
			}
		}
	}

	return executor
}

// SetSessionHistory passes previous session context to the runner
func (e *Executor) SetSessionHistory(history string) {
	e.Runner.SetSessionHistory(history)
}

// SetLogger sets the logger for execution
func (e *Executor) SetLogger(logger *logging.Logger) {
	e.Logger = logger
	e.Runner.Logger = logger // Pass logger to runner
}

// SetMessageCallback sets callback for when agents complete
func (e *Executor) SetMessageCallback(callback func(agentID, role, content string)) {
	e.Runner.MessageCallback = callback
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

	// Check if any agent has listens_to defined (collaborative mode)
	hasCollaborativeAgents := false
	for _, branchID := range e.Config.Workflow.Branches {
		agentDef := e.Runner.GetAgent(branchID)
		if agentDef != nil && len(agentDef.ListensTo) > 0 {
			hasCollaborativeAgents = true
			break
		}
	}

	// If collaborative agents exist, use MessageChannel for real-time messaging
	var channel *memory.MessageChannel
	if hasCollaborativeAgents {
		channel = memory.NewMessageChannel(100)
		defer channel.Close()
		fmt.Printf("🤝 Parallel workflow with real-time messaging enabled\n")
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	results := make(map[string]string)

	// Get max turns from workflow config (default 5)
	maxTurns := e.Config.Workflow.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 5
	}

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

			var response string
			var err error

			// Use collaborative mode if agent has listens_to and channel exists
			if channel != nil && len(agentDef.ListensTo) > 0 {
				// Set max turns if not defined at agent level
				if agentDef.MaxTurns <= 0 {
					agentDef.MaxTurns = maxTurns
				}
				response, err = e.Runner.RunCollaborativeAgent(agentDef, channel)
			} else {
				response, err = e.Runner.RunAgent(agentDef)
			}

			mu.Lock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				// Abort shared memory to wake up waiting agents
				if e.SharedMemory != nil {
					e.SharedMemory.Abort(err.Error())
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
