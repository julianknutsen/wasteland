package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/julianknutsen/wasteland/internal/remote"
)

func TestGitConfigValue_MissingKey(t *testing.T) {
	t.Parallel()
	got := gitConfigValue("wasteland.nonexistent.key.12345")
	if got != "" {
		t.Errorf("gitConfigValue(missing) = %q, want empty string", got)
	}
}

func TestGitConfigValue_UserName(t *testing.T) {
	t.Parallel()
	// git config user.name may or may not be set in CI; just verify it doesn't panic
	_ = gitConfigValue("user.name")
}

func TestPrintForkInstructions(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	forkErr := &remote.ForkRequiredError{
		UpstreamOrg: "hop",
		UpstreamDB:  "wl-commons",
		ForkOrg:     "alice",
	}

	printForkInstructions(&buf, forkErr)
	got := buf.String()

	if !strings.Contains(got, "Fork required") {
		t.Errorf("output missing 'Fork required': %q", got)
	}
	if !strings.Contains(got, forkErr.ForkURL()) {
		t.Errorf("output missing fork URL %q: %q", forkErr.ForkURL(), got)
	}
	if !strings.Contains(got, "alice") {
		t.Errorf("output missing org name 'alice': %q", got)
	}
	if !strings.Contains(got, "wl join") {
		t.Errorf("output missing 'wl join': %q", got)
	}
}
