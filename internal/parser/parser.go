package parser

import (
	"os"

	"Orkflow/pkg/types"

	"gopkg.in/yaml.v3"
)

func parseYaml(path string) (*types.WorkflowConfig, error) {
	// load file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Unmarshal file
	LoadedWorkflow := types.WorkflowConfig{}
	err = yaml.Unmarshal(data, &LoadedWorkflow)
	if err != nil {
		return nil, err
	}

	// Validate Yaml
	err = validate(&LoadedWorkflow)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
