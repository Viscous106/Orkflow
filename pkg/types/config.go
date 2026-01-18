package types

// MCPServerConfig defines an MCP server configuration
type MCPServerConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env,omitempty"`
}

type WorkflowConfig struct {
	Agents     []Agent                    `yaml:"agents"`
	Workflow   *WorkflowSpec              `yaml:"workflow,omitempty"`
	Models     map[string]Model           `yaml:"models,omitempty"`
	MCPServers map[string]MCPServerConfig `yaml:"mcp_servers,omitempty"`
	Memory     *MemoryConfig              `yaml:"memory,omitempty"` // Vector memory configuration
}
