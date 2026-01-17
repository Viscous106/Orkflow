package types

type Model struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	Endpoint  string `yaml:"endpoint,omitempty"`
	MaxTokens int    `yaml:"max_tokens,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`
}
