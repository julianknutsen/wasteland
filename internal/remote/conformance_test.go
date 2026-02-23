//go:build integration

package remote_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steveyegge/wasteland/internal/remote"
)

// providerFactory creates a Provider rooted at a test-specific base directory,
// with an upstream source already populated so Fork has something to copy.
type providerFactory struct {
	name    string
	setup   func(t *testing.T, baseDir string) remote.Provider
	urlTest func(t *testing.T, url string) // validate URL format
}

var doltPath string

func TestMain(m *testing.M) {
	var err error
	doltPath, err = exec.LookPath("dolt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "dolt not found in PATH — skipping conformance tests\n")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// doltHome creates a temp HOME with dolt config so dolt commands work.
func doltHome(t *testing.T) string {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home")
	doltCfg := filepath.Join(home, ".dolt")
	if err := os.MkdirAll(doltCfg, 0o755); err != nil {
		t.Fatalf("creating dolt config: %v", err)
	}
	cfg := `{"user.name":"conformance-test","user.email":"test@example.com","user.creds":""}` + "\n"
	if err := os.WriteFile(filepath.Join(doltCfg, "config_global.json"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("writing dolt config: %v", err)
	}
	return home
}

// doltEnv returns env vars for running dolt commands in a test.
func doltEnv(home string) []string {
	return []string{
		"HOME=" + home,
		"DOLT_ROOT_PATH=" + home,
		"PATH=" + os.Getenv("PATH"),
	}
}

// createDoltSource creates a dolt database with a test table, pushes it to
// {baseDir}/{org}/{db} as a dolt remote store, and returns the env.
func createDoltSource(t *testing.T, baseDir, org, db string) []string {
	t.Helper()
	home := doltHome(t)
	env := doltEnv(home)

	// Init a workspace.
	workDir := filepath.Join(t.TempDir(), "src")
	run(t, env, workDir, true, "dolt", "init")
	run(t, env, workDir, false, "dolt", "sql", "-q",
		"CREATE TABLE conformance_test (id INT PRIMARY KEY, val VARCHAR(255));"+
			"CALL DOLT_ADD('-A');"+
			"CALL DOLT_COMMIT('-m','init');")

	// Push to remote store.
	storeDir := filepath.Join(baseDir, org, db)
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("creating store dir: %v", err)
	}
	storeURL := fmt.Sprintf("file://%s", storeDir)
	run(t, env, workDir, false, "dolt", "remote", "add", "store", storeURL)
	run(t, env, workDir, false, "dolt", "push", "store", "main")

	return env
}

// createGitSource creates a dolt database, pushes it to a bare git repo at
// {baseDir}/{org}/{db}.git, and returns the env.
func createGitSource(t *testing.T, baseDir, org, db string) []string {
	t.Helper()
	home := doltHome(t)
	env := doltEnv(home)

	// Init a workspace.
	workDir := filepath.Join(t.TempDir(), "src")
	run(t, env, workDir, true, "dolt", "init")
	run(t, env, workDir, false, "dolt", "sql", "-q",
		"CREATE TABLE conformance_test (id INT PRIMARY KEY, val VARCHAR(255));"+
			"CALL DOLT_ADD('-A');"+
			"CALL DOLT_COMMIT('-m','init');")

	// Init bare git repo with an initial commit so dolt can push to it.
	gitDir := filepath.Join(baseDir, org, db+".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("creating git dir: %v", err)
	}
	run(t, env, "", false, "git", "init", "--bare", gitDir)
	seedDir := filepath.Join(t.TempDir(), "git-seed")
	run(t, env, seedDir, true, "git", "init", "-b", "main")
	run(t, env, seedDir, false, "git",
		"-c", "user.name=init", "-c", "user.email=init@init",
		"commit", "--allow-empty", "-m", "init")
	run(t, env, seedDir, false, "git", "push", "file://"+gitDir, "main")

	// Push to git repo.
	gitURL := fmt.Sprintf("file://%s", gitDir)
	run(t, env, workDir, false, "dolt", "remote", "add", "store", gitURL)
	run(t, env, workDir, false, "dolt", "push", "store", "main")

	return env
}

// run executes a command. If mkDir is true, creates the working dir first.
func run(t *testing.T, env []string, dir string, mkDir bool, name string, args ...string) string {
	t.Helper()
	if mkDir && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("creating dir %s: %v", dir, err)
		}
	}
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// providers returns the factories for each provider type we can test offline.
func providers() []providerFactory {
	return []providerFactory{
		{
			name: "FileProvider",
			setup: func(t *testing.T, baseDir string) remote.Provider {
				createDoltSource(t, baseDir, "src-org", "testdb")
				return remote.NewFileProvider(baseDir)
			},
			urlTest: func(t *testing.T, url string) {
				if !strings.HasPrefix(url, "file://") {
					t.Errorf("FileProvider URL should start with file://, got %q", url)
				}
				if strings.HasSuffix(url, ".git") {
					t.Errorf("FileProvider URL should not end with .git, got %q", url)
				}
			},
		},
		{
			name: "GitProvider",
			setup: func(t *testing.T, baseDir string) remote.Provider {
				createGitSource(t, baseDir, "src-org", "testdb")
				return remote.NewGitProvider(baseDir)
			},
			urlTest: func(t *testing.T, url string) {
				if !strings.HasPrefix(url, "file://") {
					t.Errorf("GitProvider URL should start with file://, got %q", url)
				}
				if !strings.HasSuffix(url, ".git") {
					t.Errorf("GitProvider URL should end with .git, got %q", url)
				}
			},
		},
	}
}

// --- Conformance tests ---
// Each test runs against every provider factory.

func TestConformance_TypeNotEmpty(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)
			if p.Type() == "" {
				t.Error("Type() must return a non-empty string")
			}
		})
	}
}

func TestConformance_DatabaseURL(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			url := p.DatabaseURL("myorg", "mydb")
			if url == "" {
				t.Fatal("DatabaseURL must return a non-empty string")
			}

			// Must contain org and db somewhere.
			if !strings.Contains(url, "myorg") {
				t.Errorf("DatabaseURL %q should contain org 'myorg'", url)
			}
			if !strings.Contains(url, "mydb") {
				t.Errorf("DatabaseURL %q should contain db 'mydb'", url)
			}

			// Provider-specific URL format checks.
			pf.urlTest(t, url)
		})
	}
}

func TestConformance_DatabaseURL_Deterministic(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			url1 := p.DatabaseURL("org", "db")
			url2 := p.DatabaseURL("org", "db")
			if url1 != url2 {
				t.Errorf("DatabaseURL not deterministic: %q != %q", url1, url2)
			}
		})
	}
}

func TestConformance_DatabaseURL_DifferentForDifferentInputs(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

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
		})
	}
}

func TestConformance_Fork_CreatesCloneableDatabase(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			// Fork from the source we set up.
			err := p.Fork("src-org", "testdb", "fork-org")
			if err != nil {
				t.Fatalf("Fork() error: %v", err)
			}

			// The fork's URL should be cloneable by dolt.
			forkURL := p.DatabaseURL("fork-org", "testdb")
			cloneDir := filepath.Join(t.TempDir(), "clone")
			home := doltHome(t)
			env := doltEnv(home)

			run(t, env, "", false, "dolt", "clone", forkURL, cloneDir)

			// The cloned database should have our test table.
			out := run(t, env, cloneDir, false, "dolt", "sql", "-r", "csv", "-q", "SHOW TABLES")
			if !strings.Contains(out, "conformance_test") {
				t.Errorf("cloned database missing conformance_test table; got:\n%s", out)
			}
		})
	}
}

func TestConformance_Fork_Idempotent(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			// First fork.
			if err := p.Fork("src-org", "testdb", "fork-org"); err != nil {
				t.Fatalf("first Fork() error: %v", err)
			}

			// Second fork — must not error.
			if err := p.Fork("src-org", "testdb", "fork-org"); err != nil {
				t.Fatalf("second Fork() should be idempotent, got error: %v", err)
			}

			// Still cloneable after double fork.
			forkURL := p.DatabaseURL("fork-org", "testdb")
			cloneDir := filepath.Join(t.TempDir(), "clone")
			home := doltHome(t)
			env := doltEnv(home)
			run(t, env, "", false, "dolt", "clone", forkURL, cloneDir)
		})
	}
}

func TestConformance_Fork_MissingSource(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			// Fork from a non-existent source should error.
			err := p.Fork("nonexistent-org", "nope", "fork-org")
			if err == nil {
				t.Error("Fork() with missing source should return an error")
			}
		})
	}
}

func TestConformance_Fork_PreservesData(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			// Add data to the source before forking.
			home := doltHome(t)
			env := doltEnv(home)

			// Clone source, insert data, push back.
			srcURL := p.DatabaseURL("src-org", "testdb")
			workDir := filepath.Join(t.TempDir(), "src-work")
			run(t, env, "", false, "dolt", "clone", srcURL, workDir)
			run(t, env, workDir, false, "dolt", "sql", "-q",
				"INSERT INTO conformance_test VALUES (1, 'hello'), (2, 'world');"+
					"CALL DOLT_ADD('-A');"+
					"CALL DOLT_COMMIT('-m','add data');")
			run(t, env, workDir, false, "dolt", "push", "origin", "main")

			// Fork.
			if err := p.Fork("src-org", "testdb", "data-fork"); err != nil {
				t.Fatalf("Fork() error: %v", err)
			}

			// Clone fork and verify data.
			forkURL := p.DatabaseURL("data-fork", "testdb")
			cloneDir := filepath.Join(t.TempDir(), "fork-clone")
			run(t, env, "", false, "dolt", "clone", forkURL, cloneDir)

			out := run(t, env, cloneDir, false, "dolt", "sql", "-r", "csv", "-q",
				"SELECT COUNT(*) AS cnt FROM conformance_test")
			if !strings.Contains(out, "2") {
				t.Errorf("forked database should have 2 rows; got:\n%s", out)
			}
		})
	}
}

func TestConformance_ForkThenUpstreamRemote(t *testing.T) {
	for _, pf := range providers() {
		t.Run(pf.name, func(t *testing.T) {
			baseDir := filepath.Join(t.TempDir(), "remotes")
			p := pf.setup(t, baseDir)

			// Fork.
			if err := p.Fork("src-org", "testdb", "fork-org"); err != nil {
				t.Fatalf("Fork() error: %v", err)
			}

			// Clone the fork.
			home := doltHome(t)
			env := doltEnv(home)
			forkURL := p.DatabaseURL("fork-org", "testdb")
			cloneDir := filepath.Join(t.TempDir(), "clone")
			run(t, env, "", false, "dolt", "clone", forkURL, cloneDir)

			// Add the source as an upstream remote — simulates wl join's AddUpstreamRemote.
			srcURL := p.DatabaseURL("src-org", "testdb")
			run(t, env, cloneDir, false, "dolt", "remote", "add", "upstream", srcURL)

			// Fetch from upstream should succeed.
			run(t, env, cloneDir, false, "dolt", "fetch", "upstream")
		})
	}
}
