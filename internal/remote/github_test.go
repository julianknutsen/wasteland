package remote

import "testing"

func TestGitHubProviderDatabaseURL(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider()

	tests := []struct {
		org, db, want string
	}{
		{"steveyegge", "wl-commons", "https://github.com/steveyegge/wl-commons.git"},
		{"alice", "mydb", "https://github.com/alice/mydb.git"},
	}
	for _, tc := range tests {
		got := p.DatabaseURL(tc.org, tc.db)
		if got != tc.want {
			t.Errorf("DatabaseURL(%q, %q) = %q, want %q", tc.org, tc.db, got, tc.want)
		}
	}
}

func TestGitHubProviderDatabaseURL_Deterministic(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider()
	url1 := p.DatabaseURL("org", "db")
	url2 := p.DatabaseURL("org", "db")
	if url1 != url2 {
		t.Errorf("DatabaseURL not deterministic: %q != %q", url1, url2)
	}
}

func TestGitHubProviderDatabaseURL_DifferentInputs(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider()
	url1 := p.DatabaseURL("org-a", "db")
	url2 := p.DatabaseURL("org-b", "db")
	if url1 == url2 {
		t.Errorf("DatabaseURL should differ for different orgs: both %q", url1)
	}

	url3 := p.DatabaseURL("org", "db-a")
	url4 := p.DatabaseURL("org", "db-b")
	if url3 == url4 {
		t.Errorf("DatabaseURL should differ for different dbs: both %q", url3)
	}
}

func TestGitHubProviderType(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider()
	if got := p.Type(); got != "github" {
		t.Errorf("Type() = %q, want %q", got, "github")
	}
}
