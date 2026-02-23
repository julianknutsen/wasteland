package remote_test

import (
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/remote"
)

func TestFakeGitHubProviderType(t *testing.T) {
	p := remote.NewFakeGitHubProvider("/tmp/test")
	if got := p.Type(); got != "github" {
		t.Errorf("Type() = %q, want %q", got, "github")
	}
}

func TestFakeGitHubProviderDatabaseURL(t *testing.T) {
	p := remote.NewFakeGitHubProvider("/tmp/base")
	url := p.DatabaseURL("myorg", "mydb")
	if !strings.HasPrefix(url, "file://") {
		t.Errorf("URL should start with file://, got %q", url)
	}
	if !strings.HasSuffix(url, ".git") {
		t.Errorf("URL should end with .git, got %q", url)
	}
	if !strings.Contains(url, "myorg") {
		t.Errorf("URL should contain org, got %q", url)
	}
	if !strings.Contains(url, "mydb") {
		t.Errorf("URL should contain db, got %q", url)
	}
}
