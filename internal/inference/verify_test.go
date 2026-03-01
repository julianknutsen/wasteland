package inference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Tests below mutate the package-level OllamaURL and must not run in parallel.

func TestVerify_Match(t *testing.T) {
	output := "The answer is 2."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Response: output})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	job := &Job{Prompt: "what is 1+1", Model: "llama3.2:1b", Seed: 42}
	result := &Result{
		Output:     output,
		OutputHash: Hash(output),
		Model:      "llama3.2:1b",
		Seed:       42,
	}

	vr, err := Verify(job, result)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if !vr.Match {
		t.Errorf("Match = false, want true (expected=%s, actual=%s)", vr.ExpectedHash, vr.ActualHash)
	}
	if vr.Output != output {
		t.Errorf("Output = %q, want %q", vr.Output, output)
	}
}

func TestVerify_Mismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Response: "different output"})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	job := &Job{Prompt: "what is 1+1", Model: "llama3.2:1b", Seed: 42}
	result := &Result{
		Output:     "The answer is 2.",
		OutputHash: Hash("The answer is 2."),
		Model:      "llama3.2:1b",
		Seed:       42,
	}

	vr, err := Verify(job, result)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if vr.Match {
		t.Error("Match = true, want false")
	}
	if vr.ExpectedHash != result.OutputHash {
		t.Errorf("ExpectedHash = %q, want %q", vr.ExpectedHash, result.OutputHash)
	}
	if vr.ActualHash != Hash("different output") {
		t.Errorf("ActualHash = %q, want %q", vr.ActualHash, Hash("different output"))
	}
}

func TestVerify_OllamaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	job := &Job{Prompt: "test", Model: "m", Seed: 1}
	result := &Result{Output: "x", OutputHash: "sha256:abc", Model: "m", Seed: 1}

	_, err := Verify(job, result)
	if err == nil {
		t.Fatal("Verify() expected error when ollama fails")
	}
}
