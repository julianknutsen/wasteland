package inference

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHash_Deterministic(t *testing.T) {
	t.Parallel()
	h1 := Hash("hello world")
	h2 := Hash("hello world")
	if h1 != h2 {
		t.Errorf("Hash not deterministic: %q != %q", h1, h2)
	}
}

func TestHash_Format(t *testing.T) {
	t.Parallel()
	h := Hash("test")
	if !strings.HasPrefix(h, "sha256:") {
		t.Errorf("Hash = %q, want sha256: prefix", h)
	}
	hex := strings.TrimPrefix(h, "sha256:")
	if len(hex) != 64 {
		t.Errorf("hex length = %d, want 64", len(hex))
	}
}

func TestHash_DifferentInputs(t *testing.T) {
	t.Parallel()
	h1 := Hash("hello")
	h2 := Hash("world")
	if h1 == h2 {
		t.Error("Hash of different inputs should differ")
	}
}

// Tests below mutate the package-level OllamaURL and must not run in parallel.

func TestRun_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		var req ollamaRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("bad request body: %v", err)
		}
		if req.Model != "llama3.2:1b" {
			t.Errorf("Model = %q, want %q", req.Model, "llama3.2:1b")
		}
		if req.Stream {
			t.Error("Stream should be false")
		}
		if temp, ok := req.Options["temperature"]; !ok || temp != float64(0) {
			t.Errorf("temperature = %v, want 0", temp)
		}
		if seed, ok := req.Options["seed"]; !ok || seed != float64(42) {
			t.Errorf("seed = %v, want 42", seed)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Response: "The answer is 2."})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	result, err := Run(&Job{Prompt: "what is 1+1", Model: "llama3.2:1b", Seed: 42})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.Output != "The answer is 2." {
		t.Errorf("Output = %q, want %q", result.Output, "The answer is 2.")
	}
	if result.OutputHash != Hash("The answer is 2.") {
		t.Errorf("OutputHash = %q, want %q", result.OutputHash, Hash("The answer is 2."))
	}
	if result.Model != "llama3.2:1b" {
		t.Errorf("Model = %q, want %q", result.Model, "llama3.2:1b")
	}
	if result.Seed != 42 {
		t.Errorf("Seed = %d, want 42", result.Seed)
	}
}

func TestRun_MaxTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req ollamaRequest
		_ = json.Unmarshal(body, &req)

		if np, ok := req.Options["num_predict"]; !ok || np != float64(50) {
			t.Errorf("num_predict = %v, want 50", np)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Response: "ok"})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	_, err := Run(&Job{Prompt: "test", Model: "m", Seed: 1, MaxTokens: 50})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestRun_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("model not found"))
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	_, err := Run(&Job{Prompt: "test", Model: "bad", Seed: 1})
	if err == nil {
		t.Fatal("Run() expected error for 500 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want to contain '500'", err.Error())
	}
}

func TestRun_ConnectionError(t *testing.T) {
	old := OllamaURL
	OllamaURL = "http://127.0.0.1:1" // unlikely to be listening
	defer func() { OllamaURL = old }()

	_, err := Run(&Job{Prompt: "test", Model: "m", Seed: 1})
	if err == nil {
		t.Fatal("Run() expected error for connection failure")
	}
}

func TestModelExists_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "llama3.2:1b"},
				{Name: "mistral:7b"},
			},
		})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	found, err := ModelExists("llama3.2:1b")
	if err != nil {
		t.Fatalf("ModelExists() error: %v", err)
	}
	if !found {
		t.Error("ModelExists() = false, want true")
	}
}

func TestModelExists_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "mistral:7b"},
			},
		})
	}))
	defer srv.Close()

	old := OllamaURL
	OllamaURL = srv.URL
	defer func() { OllamaURL = old }()

	found, err := ModelExists("llama3.2:1b")
	if err != nil {
		t.Fatalf("ModelExists() error: %v", err)
	}
	if found {
		t.Error("ModelExists() = true, want false")
	}
}

func TestModelExists_ConnectionError(t *testing.T) {
	old := OllamaURL
	OllamaURL = "http://127.0.0.1:1"
	defer func() { OllamaURL = old }()

	_, err := ModelExists("any")
	if err == nil {
		t.Fatal("ModelExists() expected error for connection failure")
	}
}
