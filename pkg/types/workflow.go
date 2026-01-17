package types

type WorkflowSpec struct {
	Type     string   `yaml:"type"`
	Steps    []Step   `yaml:"steps,omitempty"`
	Branches []string `yaml:"branches,omitempty"`
	Then     *Step    `yaml:"then,omitempty"`
}

type Step struct {
	Agent string `yaml:"agent"`
}
