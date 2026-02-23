package remote

// FakeGitHubProvider embeds GitProvider and overrides Type() to return "github".
// This enables offline integration tests to exercise GitHub-specific code paths
// (provider gates, config fields) without requiring a real GitHub account or
// the gh CLI.
type FakeGitHubProvider struct {
	*GitProvider
}

// NewFakeGitHubProvider creates a FakeGitHubProvider rooted at baseDir.
func NewFakeGitHubProvider(baseDir string) *FakeGitHubProvider {
	return &FakeGitHubProvider{GitProvider: NewGitProvider(baseDir)}
}

// Type returns "github" so cfg.IsGitHub() returns true after join.
func (f *FakeGitHubProvider) Type() string { return "github" }
