// Package inference provides verifiable distributed LLM inference via ollama.
package inference

// Job describes an LLM inference request to be posted to the wanted board.
type Job struct {
	Prompt    string `json:"prompt"`
	Model     string `json:"model"` // ollama tag, e.g. "llama3.2:1b"
	Seed      int    `json:"seed"`
	MaxTokens int    `json:"max_tokens,omitempty"` // 0 = model default
}

// Result holds the output and verification metadata from an inference run.
type Result struct {
	Output     string `json:"output"`
	OutputHash string `json:"output_hash"` // "sha256:<hex>"
	Model      string `json:"model"`
	Seed       int    `json:"seed"`
}
