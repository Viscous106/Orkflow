package types

type Agent struct {
	ID          string   `yaml:"id"`
	Model       string   `yaml:"model"`
	Role        string   `yaml:"role,omitempty"`
	Goal        string   `yaml:"goal,omitempty"`
	Tools       []string `yaml:"tools,omitempty"`
	Toolsets    []string `yaml:"toolsets,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Instruction string   `yaml:"instruction,omitempty"`
	SubAgents   []string `yaml:"sub_agents,omitempty"`
	Outputs     []string `yaml:"outputs,omitempty"`  // Keys to publish to shared memory
	Requires    []string `yaml:"requires,omitempty"` // Keys to wait for before running

	// Collaborative workflow fields
	ListensTo    []string `yaml:"listens_to,omitempty"`     // Agent IDs to receive messages from
	MaxTurns     int      `yaml:"max_turns,omitempty"`      // Max conversation turns (default: 5)
	CanBroadcast bool     `yaml:"can_broadcast,omitempty"`  // Can send to all agents

	// Vector memory options
	UseVectorContext bool `yaml:"use_vector_context,omitempty"` // Use semantic retrieval for context
	ContextTopK      int  `yaml:"context_top_k,omitempty"`      // Number of relevant docs to retrieve (default: 5)
}

func (a *Agent) GetPrompt() string {
	if a.Instruction != "" {
		return a.Instruction
	}
	return a.Goal
}

func (a *Agent) IsSupervisor() bool {
	return len(a.SubAgents) > 0
}
