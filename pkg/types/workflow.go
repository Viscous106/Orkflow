package types

// MemoryConfig configures the memory backend for the workflow
type MemoryConfig struct {
	Type        string `yaml:"type"`         // "simple" (default) or "vector"
	PersistPath string `yaml:"persist_path"` // Path for ChromaDB storage
	Embedder    string `yaml:"embedder"`     // "local" (default), "gemini", or "openai"
}

type WorkflowSpec struct {
	Type     string   `yaml:"type"`              // "sequential", "parallel", or "collaborative"
	Steps    []Step   `yaml:"steps,omitempty"`
	Branches []string `yaml:"branches,omitempty"`
	Then     *Step    `yaml:"then,omitempty"`

	// Collaborative workflow fields
	Collaborators []string `yaml:"collaborators,omitempty"` // Agents that can communicate
	MaxTurns      int      `yaml:"max_turns,omitempty"`     // Global max turns (default: 10)
}

type Step struct {
	Agent string `yaml:"agent"`
}

