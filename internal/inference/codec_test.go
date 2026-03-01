package inference

import (
	"strings"
	"testing"
)

func TestEncodeDecodeJob_RoundTrip(t *testing.T) {
	t.Parallel()
	job := &Job{
		Prompt:    "what is 1+1",
		Model:     "llama3.2:1b",
		Seed:      42,
		MaxTokens: 100,
	}

	encoded, err := EncodeJob(job)
	if err != nil {
		t.Fatalf("EncodeJob() error: %v", err)
	}

	decoded, err := DecodeJob(encoded)
	if err != nil {
		t.Fatalf("DecodeJob() error: %v", err)
	}

	if decoded.Prompt != job.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, job.Prompt)
	}
	if decoded.Model != job.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, job.Model)
	}
	if decoded.Seed != job.Seed {
		t.Errorf("Seed = %d, want %d", decoded.Seed, job.Seed)
	}
	if decoded.MaxTokens != job.MaxTokens {
		t.Errorf("MaxTokens = %d, want %d", decoded.MaxTokens, job.MaxTokens)
	}
}

func TestEncodeDecodeJob_NoMaxTokens(t *testing.T) {
	t.Parallel()
	job := &Job{Prompt: "hello", Model: "llama3.2:1b", Seed: 1}

	encoded, err := EncodeJob(job)
	if err != nil {
		t.Fatalf("EncodeJob() error: %v", err)
	}
	if strings.Contains(encoded, "max_tokens") {
		t.Errorf("encoded should omit max_tokens when zero: %s", encoded)
	}

	decoded, err := DecodeJob(encoded)
	if err != nil {
		t.Fatalf("DecodeJob() error: %v", err)
	}
	if decoded.MaxTokens != 0 {
		t.Errorf("MaxTokens = %d, want 0", decoded.MaxTokens)
	}
}

func TestEncodeJob_EmptyPrompt(t *testing.T) {
	t.Parallel()
	_, err := EncodeJob(&Job{Model: "llama3.2:1b"})
	if err == nil {
		t.Fatal("EncodeJob() expected error for empty prompt")
	}
}

func TestEncodeJob_EmptyModel(t *testing.T) {
	t.Parallel()
	_, err := EncodeJob(&Job{Prompt: "hello"})
	if err == nil {
		t.Fatal("EncodeJob() expected error for empty model")
	}
}

func TestDecodeJob_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := DecodeJob("not json")
	if err == nil {
		t.Fatal("DecodeJob() expected error for invalid JSON")
	}
}

func TestDecodeJob_MissingPrompt(t *testing.T) {
	t.Parallel()
	_, err := DecodeJob(`{"model":"llama3.2:1b","seed":42}`)
	if err == nil {
		t.Fatal("DecodeJob() expected error for missing prompt")
	}
}

func TestDecodeJob_MissingModel(t *testing.T) {
	t.Parallel()
	_, err := DecodeJob(`{"prompt":"hello","seed":42}`)
	if err == nil {
		t.Fatal("DecodeJob() expected error for missing model")
	}
}

func TestEncodeDecodeResult_RoundTrip(t *testing.T) {
	t.Parallel()
	result := &Result{
		Output:     "The answer is 2.",
		OutputHash: "sha256:abc123",
		Model:      "llama3.2:1b",
		Seed:       42,
	}

	encoded, err := EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult() error: %v", err)
	}

	decoded, err := DecodeResult(encoded)
	if err != nil {
		t.Fatalf("DecodeResult() error: %v", err)
	}

	if decoded.Output != result.Output {
		t.Errorf("Output = %q, want %q", decoded.Output, result.Output)
	}
	if decoded.OutputHash != result.OutputHash {
		t.Errorf("OutputHash = %q, want %q", decoded.OutputHash, result.OutputHash)
	}
	if decoded.Model != result.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, result.Model)
	}
	if decoded.Seed != result.Seed {
		t.Errorf("Seed = %d, want %d", decoded.Seed, result.Seed)
	}
}

func TestEncodeResult_EmptyOutput(t *testing.T) {
	t.Parallel()
	_, err := EncodeResult(&Result{OutputHash: "sha256:abc", Model: "m"})
	if err == nil {
		t.Fatal("EncodeResult() expected error for empty output")
	}
}

func TestEncodeResult_EmptyHash(t *testing.T) {
	t.Parallel()
	_, err := EncodeResult(&Result{Output: "hi", Model: "m"})
	if err == nil {
		t.Fatal("EncodeResult() expected error for empty hash")
	}
}

func TestEncodeResult_EmptyModel(t *testing.T) {
	t.Parallel()
	_, err := EncodeResult(&Result{Output: "hi", OutputHash: "sha256:abc"})
	if err == nil {
		t.Fatal("EncodeResult() expected error for empty model")
	}
}

func TestDecodeResult_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult("not json")
	if err == nil {
		t.Fatal("DecodeResult() expected error for invalid JSON")
	}
}

func TestDecodeResult_MissingOutput(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult(`{"output_hash":"sha256:abc","model":"m","seed":1}`)
	if err == nil {
		t.Fatal("DecodeResult() expected error for missing output")
	}
}

func TestDecodeResult_MissingHash(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult(`{"output":"hi","model":"m","seed":1}`)
	if err == nil {
		t.Fatal("DecodeResult() expected error for missing hash")
	}
}

func TestDecodeResult_MissingModel(t *testing.T) {
	t.Parallel()
	_, err := DecodeResult(`{"output":"hi","output_hash":"sha256:abc","seed":1}`)
	if err == nil {
		t.Fatal("DecodeResult() expected error for missing model")
	}
}
