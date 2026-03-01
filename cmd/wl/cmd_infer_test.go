package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/inference"
)

// --- executeInferRun tests ---

func TestExecuteInferRun_Success(t *testing.T) {
	t.Parallel()

	output := "The answer is 2."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			Response string `json:"response"`
		}{Response: output})
	}))
	defer srv.Close()

	old := inference.OllamaURL
	inference.OllamaURL = srv.URL
	defer func() { inference.OllamaURL = old }()

	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "what is 1+1", Model: "llama3.2:1b", Seed: 42}
	desc, _ := inference.EncodeJob(job)
	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-infer1",
		Title:       "infer: what is 1+1",
		Description: desc,
		Type:        "inference",
		PostedBy:    "bob",
	})

	completionID, err := executeInferRun(store, "w-infer1", "alice")
	if err != nil {
		t.Fatalf("executeInferRun() error: %v", err)
	}
	if completionID == "" {
		t.Fatal("expected non-empty completion ID")
	}

	// Verify item is now in_review.
	item, _ := store.QueryWanted("w-infer1")
	if item.Status != "in_review" {
		t.Errorf("status = %q, want %q", item.Status, "in_review")
	}

	// Verify completion has evidence with hash.
	completion, err := store.QueryCompletion("w-infer1")
	if err != nil {
		t.Fatalf("QueryCompletion() error: %v", err)
	}
	result, err := inference.DecodeResult(completion.Evidence)
	if err != nil {
		t.Fatalf("DecodeResult() error: %v", err)
	}
	if result.Output != output {
		t.Errorf("result.Output = %q, want %q", result.Output, output)
	}
	if result.OutputHash != inference.Hash(output) {
		t.Errorf("result.OutputHash = %q, want %q", result.OutputHash, inference.Hash(output))
	}
}

func TestExecuteInferRun_WrongType(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-bug1",
		Title: "A bug",
		Type:  "bug",
	})

	_, err := executeInferRun(store, "w-bug1", "alice")
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	if !strings.Contains(err.Error(), "inference") {
		t.Errorf("error = %q, want to mention 'inference'", err.Error())
	}
}

func TestExecuteInferRun_NotOpen(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "test", Model: "m", Seed: 1}
	desc, _ := inference.EncodeJob(job)
	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-claimed1",
		Title:       "Already claimed",
		Description: desc,
		Type:        "inference",
	})
	_ = store.ClaimWanted("w-claimed1", "bob")

	_, err := executeInferRun(store, "w-claimed1", "alice")
	if err == nil {
		t.Fatal("expected error for non-open item")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error = %q, want to mention 'open'", err.Error())
	}
}

func TestExecuteInferRun_BadDescription(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-badjson",
		Title:       "Bad JSON",
		Description: "not valid json",
		Type:        "inference",
	})

	_, err := executeInferRun(store, "w-badjson", "alice")
	if err == nil {
		t.Fatal("expected error for bad description JSON")
	}
}

func TestExecuteInferRun_OllamaFailure(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("model not found"))
	}))
	defer srv.Close()

	old := inference.OllamaURL
	inference.OllamaURL = srv.URL
	defer func() { inference.OllamaURL = old }()

	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "test", Model: "bad", Seed: 1}
	desc, _ := inference.EncodeJob(job)
	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-fail1",
		Title:       "Will fail",
		Description: desc,
		Type:        "inference",
	})

	_, err := executeInferRun(store, "w-fail1", "alice")
	if err == nil {
		t.Fatal("expected error for ollama failure")
	}

	// Verify item was unclaimed after failure.
	item, _ := store.QueryWanted("w-fail1")
	if item.Status != "open" {
		t.Errorf("status = %q, want %q (should unclaim on failure)", item.Status, "open")
	}
}

// --- executeInferVerify tests ---

func TestExecuteInferVerify_Match(t *testing.T) {
	t.Parallel()

	output := "The answer is 2."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			Response string `json:"response"`
		}{Response: output})
	}))
	defer srv.Close()

	old := inference.OllamaURL
	inference.OllamaURL = srv.URL
	defer func() { inference.OllamaURL = old }()

	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "what is 1+1", Model: "llama3.2:1b", Seed: 42}
	desc, _ := inference.EncodeJob(job)
	result := &inference.Result{
		Output:     output,
		OutputHash: inference.Hash(output),
		Model:      "llama3.2:1b",
		Seed:       42,
	}
	evidence, _ := inference.EncodeResult(result)

	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-verify1",
		Title:       "infer: test",
		Description: desc,
		Type:        "inference",
	})
	_ = store.ClaimWanted("w-verify1", "bob")
	_ = store.SubmitCompletion("c-verify1", "w-verify1", "bob", evidence)

	vr, err := executeInferVerify(store, "w-verify1")
	if err != nil {
		t.Fatalf("executeInferVerify() error: %v", err)
	}
	if !vr.Match {
		t.Errorf("Match = false, want true")
	}
}

func TestExecuteInferVerify_Mismatch(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(struct {
			Response string `json:"response"`
		}{Response: "different output"})
	}))
	defer srv.Close()

	old := inference.OllamaURL
	inference.OllamaURL = srv.URL
	defer func() { inference.OllamaURL = old }()

	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "test", Model: "m", Seed: 1}
	desc, _ := inference.EncodeJob(job)
	result := &inference.Result{
		Output:     "original output",
		OutputHash: inference.Hash("original output"),
		Model:      "m",
		Seed:       1,
	}
	evidence, _ := inference.EncodeResult(result)

	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-verify2",
		Title:       "infer: test",
		Description: desc,
		Type:        "inference",
	})
	_ = store.ClaimWanted("w-verify2", "bob")
	_ = store.SubmitCompletion("c-verify2", "w-verify2", "bob", evidence)

	vr, err := executeInferVerify(store, "w-verify2")
	if err != nil {
		t.Fatalf("executeInferVerify() error: %v", err)
	}
	if vr.Match {
		t.Error("Match = true, want false")
	}
}

func TestExecuteInferVerify_WrongType(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	_ = store.InsertWanted(&commons.WantedItem{
		ID:    "w-wrongtype",
		Title: "Not inference",
		Type:  "bug",
	})

	_, err := executeInferVerify(store, "w-wrongtype")
	if err == nil {
		t.Fatal("expected error for wrong type")
	}
	if !strings.Contains(err.Error(), "inference") {
		t.Errorf("error = %q, want to mention 'inference'", err.Error())
	}
}

func TestExecuteInferVerify_NoCompletion(t *testing.T) {
	t.Parallel()
	store := newFakeWLCommonsStore()
	job := &inference.Job{Prompt: "test", Model: "m", Seed: 1}
	desc, _ := inference.EncodeJob(job)
	_ = store.InsertWanted(&commons.WantedItem{
		ID:          "w-nocomp",
		Title:       "No completion",
		Description: desc,
		Type:        "inference",
	})

	_, err := executeInferVerify(store, "w-nocomp")
	if err == nil {
		t.Fatal("expected error for missing completion")
	}
}
