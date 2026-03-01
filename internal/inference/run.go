package inference

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaURL is the base URL for the ollama API. Override in tests.
var OllamaURL = "http://localhost:11434"

// ollamaRequest is the request body for the ollama /api/generate endpoint.
type ollamaRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

// ollamaResponse is the response from the ollama /api/generate endpoint.
type ollamaResponse struct {
	Response string `json:"response"`
}

// ollamaTagsResponse is the response from the ollama /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Run executes an inference job against the local ollama instance and returns the result.
func Run(j *Job) (*Result, error) {
	opts := map[string]any{
		"temperature": 0,
		"seed":        j.Seed,
	}
	if j.MaxTokens > 0 {
		opts["num_predict"] = j.MaxTokens
	}

	reqBody := ollamaRequest{
		Model:   j.Model,
		Prompt:  j.Prompt,
		Stream:  false,
		Options: opts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama request: %w", err)
	}

	resp, err := http.Post(OllamaURL+"/api/generate", "application/json", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decoding ollama response: %w", err)
	}

	output := ollamaResp.Response
	return &Result{
		Output:     output,
		OutputHash: Hash(output),
		Model:      j.Model,
		Seed:       j.Seed,
	}, nil
}

// Hash computes the sha256 hash of the given text and returns it in "sha256:<hex>" format.
func Hash(text string) string {
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("sha256:%x", h)
}

// ModelExists checks whether the given model is available in the local ollama instance.
func ModelExists(model string) (bool, error) {
	resp, err := http.Get(OllamaURL + "/api/tags")
	if err != nil {
		return false, fmt.Errorf("checking ollama models: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("ollama /api/tags returned status %d", resp.StatusCode)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false, fmt.Errorf("decoding ollama tags: %w", err)
	}

	for _, m := range tags.Models {
		if m.Name == model {
			return true, nil
		}
	}
	return false, nil
}
