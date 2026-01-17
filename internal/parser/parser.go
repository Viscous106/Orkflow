package parser

import (
	"os"

	"Orkflow/pkg/types"

	"gopkg.in/yaml.v3"
)

func ParseYAML(path string) (*types.WorkflowConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := types.WorkflowConfig{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	err = validate(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

