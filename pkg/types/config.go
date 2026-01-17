package types

type WorkflowConfig struct {
	Agents   []Agent          `yaml:"agents"`
	Workflow *WorkflowSpec    `yaml:"workflow,omitempty"`
	Models   map[string]Model `yaml:"models,omitempty"`
}
