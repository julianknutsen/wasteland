//go:build integration

// Package offline contains integration tests that exercise the wl binary
// against real dolt databases using local remotes. No network required.
//
// Every test is parameterized over two backends:
//   - file: dolt remote stores via --remote-base (file:// URLs)
//   - git: bare git repos via --git-remote (file:// URLs to .git dirs)
//
// Every test goes through the front door: "wl join" sets up the fork and
// config, then post/claim/done/sync operate on the result.
package offline

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	wlBinary string // path to compiled wl binary
	doltPath string // path to dolt binary
)

func TestMain(m *testing.M) {
	var err error
	doltPath, err = exec.LookPath("dolt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "dolt not found in PATH — skipping offline integration tests\n")
		os.Exit(1)
	}

	// Build the wl binary once for all tests.
	tmpDir, err := os.MkdirTemp("", "wl-offline-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	wlBinary = filepath.Join(tmpDir, "wl")
	cmd := exec.Command("go", "build", "-o", wlBinary, "./cmd/wl")
	cmd.Dir = findRepoRoot()
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building wl binary: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// findRepoRoot walks up from the test file to find the repository root (containing go.mod).
func findRepoRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Fallback: assume we're 3 levels deep from repo root (test/integration/offline)
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "..")
}

// backendKind identifies the remote provider backend for parameterized tests.
type backendKind string

const (
	fileBackend backendKind = "file"
	gitBackend  backendKind = "git"
)

// backends lists all backends that every test runs against.
var backends = []backendKind{fileBackend, gitBackend}

// testEnv provides an isolated filesystem environment for each test.
type testEnv struct {
	Root       string      // top-level temp dir
	DataHome   string      // XDG_DATA_HOME
	ConfigHome string      // XDG_CONFIG_HOME
	DataDir    string      // XDG_DATA_HOME/wasteland
	ConfigDir  string      // XDG_CONFIG_HOME/wasteland
	Home       string      // HOME override
	RemoteBase string      // base dir for file:// remote stores
	Backend    backendKind // "file" or "git"
}

func newTestEnv(t *testing.T, backend backendKind) *testEnv {
	t.Helper()
	root := t.TempDir()
	dataHome := filepath.Join(root, "data")
	configHome := filepath.Join(root, "config")
	home := filepath.Join(root, "home")
	remoteBase := filepath.Join(root, "remotes")

	for _, d := range []string{dataHome, configHome, home, remoteBase} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("creating dir %s: %v", d, err)
		}
	}

	// Configure dolt globally in the test HOME so dolt init/clone/push works.
	doltCfgDir := filepath.Join(home, ".dolt")
	if err := os.MkdirAll(doltCfgDir, 0755); err != nil {
		t.Fatalf("creating dolt config dir: %v", err)
	}
	globalCfg := `{"user.name":"test-user","user.email":"test@example.com","user.creds":""}` + "\n"
	if err := os.WriteFile(filepath.Join(doltCfgDir, "config_global.json"), []byte(globalCfg), 0644); err != nil {
		t.Fatalf("writing dolt global config: %v", err)
	}

	return &testEnv{
		Root:       root,
		DataHome:   dataHome,
		ConfigHome: configHome,
		DataDir:    filepath.Join(dataHome, "wasteland"),
		ConfigDir:  filepath.Join(configHome, "wasteland"),
		Home:       home,
		RemoteBase: remoteBase,
		Backend:    backend,
	}
}

// envSlice returns the environment variables for subprocess execution.
func (e *testEnv) envSlice() []string {
	return []string{
		"XDG_DATA_HOME=" + e.DataHome,
		"XDG_CONFIG_HOME=" + e.ConfigHome,
		"HOME=" + e.Home,
		"PATH=" + os.Getenv("PATH"),
		"DOLT_ROOT_PATH=" + e.Home,
	}
}

// createUpstreamStore creates an upstream remote store with the wl-commons
// schema. For the file backend, this is a dolt remote store at
// {remoteBase}/{org}/{db}. For the git backend, this is a bare git repo at
// {remoteBase}/{org}/{db}.git. Both simulate an existing upstream that
// "wl join" can fork from.
func (e *testEnv) createUpstreamStore(t *testing.T, org, db string) {
	t.Helper()

	// Step 1: init a temp working directory with the schema.
	workDir := filepath.Join(e.Root, "upstream-work")
	initDoltDB(t, e, workDir)
	doltSQLScript(t, e, workDir, wlCommonsSchema())

	// Step 2: create remote store and push into it.
	storeURL := e.createStoreDir(t, org, db)
	doltCmd(t, e, workDir, "remote", "add", "store", storeURL)
	doltCmd(t, e, workDir, "push", "store", "main")
}

// createUpstreamStoreWithData creates an upstream and adds seed data.
func (e *testEnv) createUpstreamStoreWithData(t *testing.T, org, db string) {
	t.Helper()

	// Step 1: init a temp working directory with the schema + seed data.
	workDir := filepath.Join(e.Root, "upstream-work")
	initDoltDB(t, e, workDir)
	doltSQLScript(t, e, workDir, wlCommonsSchema())

	seedSQL := `INSERT INTO wanted (id, title, status, type, priority, effort_level, created_at, updated_at)
VALUES ('w-seed001', 'Seed item from upstream', 'open', 'feature', 2, 'medium', NOW(), NOW());
CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('-m', 'Seed upstream data');
`
	doltSQLScript(t, e, workDir, seedSQL)

	// Step 2: create remote store and push into it.
	storeURL := e.createStoreDir(t, org, db)
	doltCmd(t, e, workDir, "remote", "add", "store", storeURL)
	doltCmd(t, e, workDir, "push", "store", "main")
}

// pushToUpstreamStore adds data to the upstream by committing in the work dir
// and pushing to the store. The workDir must already exist from createUpstreamStore*.
func (e *testEnv) pushToUpstreamStore(t *testing.T, sql string) {
	t.Helper()
	workDir := filepath.Join(e.Root, "upstream-work")
	doltSQLScript(t, e, workDir, sql)
	doltCmd(t, e, workDir, "push", "store", "main")
}

// createStoreDir creates the remote store directory for the current backend
// and returns the URL that dolt can push to / clone from.
func (e *testEnv) createStoreDir(t *testing.T, org, db string) string {
	t.Helper()
	switch e.Backend {
	case gitBackend:
		gitDir := filepath.Join(e.RemoteBase, org, db+".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("creating upstream git dir: %v", err)
		}
		gitCmd(t, e, "", "init", "--bare", gitDir)
		return fmt.Sprintf("file://%s", gitDir)
	default:
		storeDir := filepath.Join(e.RemoteBase, org, db)
		if err := os.MkdirAll(storeDir, 0755); err != nil {
			t.Fatalf("creating upstream store dir: %v", err)
		}
		return fmt.Sprintf("file://%s", storeDir)
	}
}

// remoteArgs returns the CLI flags to select the right remote provider backend.
func (e *testEnv) remoteArgs() []string {
	switch e.Backend {
	case gitBackend:
		return []string{"--git-remote", e.RemoteBase}
	default:
		return []string{"--remote-base", e.RemoteBase}
	}
}

// joinWasteland runs "wl join" with the appropriate remote provider as the front door.
func (e *testEnv) joinWasteland(t *testing.T, upstream, forkOrg string) {
	t.Helper()
	args := append([]string{"join", upstream}, e.remoteArgs()...)
	args = append(args, "--fork-org", forkOrg)
	stdout, stderr, err := runWL(t, e, args...)
	if err != nil {
		t.Fatalf("wl join failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
}

// loadConfig reads the wasteland config.json that wl join wrote.
func (e *testEnv) loadConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.ConfigDir, "config.json"))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parsing config: %v", err)
	}
	return cfg
}

// runWL executes the wl binary with controlled env and returns stdout, stderr, error.
func runWL(t *testing.T, env *testEnv, args ...string) (string, string, error) {
	t.Helper()
	cmd := exec.Command(wlBinary, args...)
	cmd.Env = env.envSlice()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// doltSQL runs a dolt SQL query against a database directory and returns CSV output.
func doltSQL(t *testing.T, dbDir, query string) string {
	t.Helper()
	cmd := exec.Command(doltPath, "sql", "-r", "csv", "-q", query)
	cmd.Dir = dbDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dolt sql in %s: %v\n%s", dbDir, err, string(out))
	}
	return string(out)
}

// gitCmd runs an arbitrary git command with the test env's environment.
func gitCmd(t *testing.T, env *testEnv, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = env.envSlice()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

// doltCmd runs an arbitrary dolt command with the test env's environment.
func doltCmd(t *testing.T, env *testEnv, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(doltPath, args...)
	cmd.Dir = dir
	cmd.Env = env.envSlice()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dolt %s in %s: %v\n%s", strings.Join(args, " "), dir, err, string(out))
	}
	return string(out)
}

// parseCSV parses CSV output into rows (including header).
func parseCSV(t *testing.T, raw string) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(raw)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parsing CSV: %v\nraw: %s", err, raw)
	}
	return rows
}

// extractWantedID extracts a w-<hash> ID from wl post output.
var wantedIDRe = regexp.MustCompile(`w-[0-9a-f]+`)

func extractWantedID(t *testing.T, stdout string) string {
	t.Helper()
	match := wantedIDRe.FindString(stdout)
	if match == "" {
		t.Fatalf("no wanted ID found in output: %s", stdout)
	}
	return match
}

// initDoltDB initializes a dolt database in a directory.
func initDoltDB(t *testing.T, env *testEnv, dbDir string) {
	t.Helper()
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		t.Fatalf("creating db dir: %v", err)
	}

	cmd := exec.Command(doltPath, "init")
	cmd.Dir = dbDir
	cmd.Env = env.envSlice()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dolt init in %s: %v\n%s", dbDir, err, string(out))
	}
}

// doltSQLScript runs a multi-statement SQL script in a dolt database.
func doltSQLScript(t *testing.T, env *testEnv, dbDir, script string) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "dolt-script-*.sql")
	if err != nil {
		t.Fatalf("creating temp SQL file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(script); err != nil {
		tmpFile.Close()
		t.Fatalf("writing SQL script: %v", err)
	}
	tmpFile.Close()

	cmd := exec.Command(doltPath, "sql", "--file", tmpFile.Name())
	cmd.Dir = dbDir
	cmd.Env = env.envSlice()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dolt sql script in %s: %v\n%s", dbDir, err, string(out))
	}
}

// wlCommonsSchema returns the full SQL schema for wl-commons.
func wlCommonsSchema() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS _meta (
    %s VARCHAR(64) PRIMARY KEY,
    value TEXT
);

INSERT IGNORE INTO _meta (%s, value) VALUES ('schema_version', '1.0');
INSERT IGNORE INTO _meta (%s, value) VALUES ('wasteland_name', 'Gas Town Wasteland');

CREATE TABLE IF NOT EXISTS rigs (
    handle VARCHAR(255) PRIMARY KEY,
    display_name VARCHAR(255),
    dolthub_org VARCHAR(255),
    hop_uri VARCHAR(512),
    owner_email VARCHAR(255),
    gt_version VARCHAR(32),
    trust_level INT DEFAULT 0,
    registered_at TIMESTAMP,
    last_seen TIMESTAMP,
    rig_type VARCHAR(16) DEFAULT 'human',
    parent_rig VARCHAR(255)
);

CREATE TABLE IF NOT EXISTS wanted (
    id VARCHAR(64) PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    project VARCHAR(64),
    type VARCHAR(32),
    priority INT DEFAULT 2,
    tags JSON,
    posted_by VARCHAR(255),
    claimed_by VARCHAR(255),
    status VARCHAR(32) DEFAULT 'open',
    effort_level VARCHAR(16) DEFAULT 'medium',
    evidence_url TEXT,
    sandbox_required TINYINT(1) DEFAULT 0,
    sandbox_scope JSON,
    sandbox_min_tier VARCHAR(32),
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS completions (
    id VARCHAR(64) PRIMARY KEY,
    wanted_id VARCHAR(64),
    completed_by VARCHAR(255),
    evidence TEXT,
    validated_by VARCHAR(255),
    stamp_id VARCHAR(64),
    parent_completion_id VARCHAR(64),
    block_hash VARCHAR(64),
    hop_uri VARCHAR(512),
    completed_at TIMESTAMP,
    validated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stamps (
    id VARCHAR(64) PRIMARY KEY,
    author VARCHAR(255) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    valence JSON NOT NULL,
    confidence FLOAT DEFAULT 1,
    severity VARCHAR(16) DEFAULT 'leaf',
    context_id VARCHAR(64),
    context_type VARCHAR(32),
    skill_tags JSON,
    message TEXT,
    prev_stamp_hash VARCHAR(64),
    block_hash VARCHAR(64),
    hop_uri VARCHAR(512),
    created_at TIMESTAMP,
    CHECK (NOT(author = subject))
);

CREATE TABLE IF NOT EXISTS badges (
    id VARCHAR(64) PRIMARY KEY,
    rig_handle VARCHAR(255),
    badge_type VARCHAR(64),
    awarded_at TIMESTAMP,
    evidence TEXT
);

CREATE TABLE IF NOT EXISTS chain_meta (
    chain_id VARCHAR(64) PRIMARY KEY,
    chain_type VARCHAR(32),
    parent_chain_id VARCHAR(64),
    hop_uri VARCHAR(512),
    dolt_database VARCHAR(255),
    created_at TIMESTAMP
);

CALL DOLT_ADD('-A');
CALL DOLT_COMMIT('--allow-empty', '-m', 'Initialize wl-commons schema v1.0');
`, backtickKey(), backtickKey(), backtickKey())
}

func backtickKey() string {
	return "`key`"
}
