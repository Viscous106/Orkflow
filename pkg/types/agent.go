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
