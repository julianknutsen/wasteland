// Package federation implements the Wasteland federation protocol.
//
// The Wasteland is a federation of Gas Towns via DoltHub. Each rig has a
// sovereign fork of a shared commons database. Rigs register by writing
// to the commons' rigs table, and contribute wanted work items and
// completions through DoltHub's fork/PR/merge primitives.
package federation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/steveyegge/wasteland/internal/remote"
	"github.com/steveyegge/wasteland/internal/xdg"
)

// ErrNotJoined indicates the rig has not joined a wasteland.
var ErrNotJoined = errors.New("rig has not joined a wasteland")

// Mode constants for the wasteland workflow.
const (
	ModeWildWest = "wild-west"
	ModePR       = "pr"
)

// ErrAmbiguous indicates multiple wastelands are joined and --wasteland is required.
var ErrAmbiguous = errors.New("multiple wastelands joined; use --wasteland to select one")

// Config holds the wasteland configuration for a rig.
type Config struct {
	// Upstream is the DoltHub path of the upstream commons (e.g., "steveyegge/wl-commons").
	Upstream string `json:"upstream"`

	// ProviderType is the upstream provider ("dolthub", "file", "git", "github").
	ProviderType string `json:"provider_type,omitempty"`

	// UpstreamURL is the resolved dolt-compatible remote URL for the upstream.
	// Used by browse for ephemeral clones.
	UpstreamURL string `json:"upstream_url,omitempty"`

	// ForkOrg is the DoltHub org where the fork lives (e.g., "alice-dev").
	ForkOrg string `json:"fork_org"`

	// ForkDB is the database name of the fork (e.g., "wl-commons").
	ForkDB string `json:"fork_db"`

	// LocalDir is the absolute path to the local clone of the fork.
	LocalDir string `json:"local_dir"`

	// RigHandle is the rig's handle in the registry.
	RigHandle string `json:"rig_handle"`

	// JoinedAt is when the rig joined the wasteland.
	JoinedAt time.Time `json:"joined_at"`

	// Mode is the workflow mode: "" or "wild-west" (default) or "pr".
	Mode string `json:"mode,omitempty"`

	// Signing enables GPG-signed Dolt commits when true.
	Signing bool `json:"signing,omitempty"`

	// GitHubRepo is the upstream GitHub repo for PR shells (e.g., "steveyegge/wl-commons").
	//
	// Deprecated: use ProviderType == "github" instead.
	GitHubRepo string `json:"github_repo,omitempty"`
}

// ResolveMode returns the effective mode, defaulting to wild-west.
func (c *Config) ResolveMode() string {
	if c.Mode == "" || c.Mode == ModeWildWest {
		return ModeWildWest
	}
	return c.Mode
}

// ResolveProviderType returns the effective provider type.
// Falls back to "dolthub" for backward compatibility with old configs.
func (c *Config) ResolveProviderType() string {
	if c.ProviderType != "" {
		return c.ProviderType
	}
	return "dolthub"
}

// IsGitHub returns true if the provider type is "github".
func (c *Config) IsGitHub() bool {
	return c.ResolveProviderType() == "github"
}

// ParseUpstream parses an upstream path like "steveyegge/wl-commons" into org and db.
func ParseUpstream(upstream string) (org, db string, err error) {
	parts := strings.SplitN(upstream, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid upstream path %q: expected format 'org/database'", upstream)
	}
	return parts[0], parts[1], nil
}

// LocalCloneDir returns the local clone directory for a specific wasteland commons.
func LocalCloneDir(upstreamOrg, upstreamDB string) string {
	return filepath.Join(xdg.DataDir(), upstreamOrg, upstreamDB)
}

// escapeSQLString escapes backslashes and single quotes for SQL string literals.
func escapeSQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "'", "''")
}

// DoltCLI abstracts dolt CLI subprocess operations.
type DoltCLI interface {
	Clone(remoteURL, targetDir string) error
	RegisterRig(localDir, handle, dolthubOrg, displayName, ownerEmail, version string, signed bool) error
	Push(localDir string) error
	AddUpstreamRemote(localDir, remoteURL string) error
}

// ConfigStore abstracts wasteland config persistence.
type ConfigStore interface {
	Load(upstream string) (*Config, error)
	Save(cfg *Config) error
	Delete(upstream string) error
	List() ([]string, error)
}

// ResolveConfig resolves the active wasteland config.
// If explicit is non-empty, loads that specific upstream config.
// If exactly one wasteland is joined, returns it.
// If zero are joined, returns ErrNotJoined.
// If multiple are joined, returns ErrAmbiguous.
func ResolveConfig(store ConfigStore, explicit string) (*Config, error) {
	if explicit != "" {
		cfg, err := store.Load(explicit)
		if err != nil {
			return nil, fmt.Errorf("loading config for %s: %w", explicit, err)
		}
		return cfg, nil
	}

	upstreams, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("listing wastelands: %w", err)
	}

	switch len(upstreams) {
	case 0:
		return nil, fmt.Errorf("%w (run 'wl join <upstream>')", ErrNotJoined)
	case 1:
		cfg, err := store.Load(upstreams[0])
		if err != nil {
			return nil, fmt.Errorf("loading config for %s: %w", upstreams[0], err)
		}
		return cfg, nil
	default:
		var list strings.Builder
		for _, u := range upstreams {
			fmt.Fprintf(&list, "  - %s\n", u)
		}
		return nil, fmt.Errorf("%w:\n%s", ErrAmbiguous, list.String())
	}
}

// Service coordinates wasteland operations with injectable dependencies.
type Service struct {
	Remote     remote.Provider
	CLI        DoltCLI
	Config     ConfigStore
	OnProgress func(step string) // optional callback for progress reporting
}

// Join orchestrates the wasteland join workflow: fork -> clone -> add upstream -> register -> push -> save config.
func (s *Service) Join(upstream, forkOrg, handle, displayName, ownerEmail, version string, signed bool) (*Config, error) {
	upstreamOrg, upstreamDB, err := ParseUpstream(upstream)
	if err != nil {
		return nil, err
	}

	// Check if already joined to this specific upstream (idempotent).
	if existing, err := s.Config.Load(upstream); err == nil {
		return existing, nil
	}

	localDir := LocalCloneDir(upstreamOrg, upstreamDB)
	progress := s.OnProgress
	if progress == nil {
		progress = func(string) {}
	}

	progress("Forking commons...")
	if err := s.Remote.Fork(upstreamOrg, upstreamDB, forkOrg); err != nil {
		return nil, fmt.Errorf("forking commons: %w", err)
	}

	progress("Cloning fork locally...")
	forkURL := s.Remote.DatabaseURL(forkOrg, upstreamDB)
	if err := s.CLI.Clone(forkURL, localDir); err != nil {
		return nil, fmt.Errorf("cloning fork: %w", err)
	}

	progress("Adding upstream remote...")
	upstreamURL := s.Remote.DatabaseURL(upstreamOrg, upstreamDB)
	if err := s.CLI.AddUpstreamRemote(localDir, upstreamURL); err != nil {
		return nil, fmt.Errorf("adding upstream remote: %w", err)
	}

	progress("Registering rig...")
	if err := s.CLI.RegisterRig(localDir, handle, forkOrg, displayName, ownerEmail, version, signed); err != nil {
		return nil, fmt.Errorf("registering rig: %w", err)
	}

	progress("Pushing to fork...")
	if err := s.CLI.Push(localDir); err != nil {
		return nil, fmt.Errorf("pushing to fork: %w", err)
	}

	cfg := &Config{
		Upstream:     upstream,
		ProviderType: s.Remote.Type(),
		UpstreamURL:  upstreamURL,
		ForkOrg:      forkOrg,
		ForkDB:       upstreamDB,
		LocalDir:     localDir,
		RigHandle:    handle,
		JoinedAt:     time.Now(),
	}
	if err := s.Config.Save(cfg); err != nil {
		return nil, fmt.Errorf("saving wasteland config: %w", err)
	}

	return cfg, nil
}

// execDoltCLI implements DoltCLI using real dolt subprocess calls.
type execDoltCLI struct{}

func (e *execDoltCLI) Clone(remoteURL, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, ".dolt")); err == nil {
		return nil
	}

	cmd := exec.Command("dolt", "clone", remoteURL, targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt clone %s: %w (%s)", remoteURL, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (e *execDoltCLI) RegisterRig(localDir, handle, dolthubOrg, displayName, ownerEmail, version string, signed bool) error {
	sql := fmt.Sprintf(
		`INSERT INTO rigs (handle, display_name, dolthub_org, owner_email, gt_version, trust_level, registered_at, last_seen) `+
			`VALUES ('%s', '%s', '%s', '%s', '%s', 1, NOW(), NOW()) `+
			`ON DUPLICATE KEY UPDATE last_seen = NOW(), gt_version = '%s'`,
		escapeSQLString(handle),
		escapeSQLString(displayName),
		escapeSQLString(dolthubOrg),
		escapeSQLString(ownerEmail),
		escapeSQLString(version),
		escapeSQLString(version),
	)

	cmd := exec.Command("dolt", "sql", "-q", sql)
	cmd.Dir = localDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("inserting rig registration: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	addCmd := exec.Command("dolt", "add", ".")
	addCmd.Dir = localDir
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dolt add: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	commitArgs := []string{"commit"}
	if signed {
		commitArgs = append(commitArgs, "-S")
	}
	commitArgs = append(commitArgs, "-m", fmt.Sprintf("Register rig: %s", handle))
	commitCmd := exec.Command("dolt", commitArgs...)
	commitCmd.Dir = localDir
	output, err = commitCmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "nothing to commit") || strings.Contains(lower, "no changes added") {
			return nil
		}
		return fmt.Errorf("dolt commit: %w (%s)", err, msg)
	}

	return nil
}

func (e *execDoltCLI) Push(localDir string) error {
	cmd := exec.Command("dolt", "push", "origin", "main")
	cmd.Dir = localDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt push: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (e *execDoltCLI) AddUpstreamRemote(localDir, remoteURL string) error {
	checkCmd := exec.Command("dolt", "remote", "-v")
	checkCmd.Dir = localDir
	output, err := checkCmd.CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "upstream") {
				return nil
			}
		}
	}

	cmd := exec.Command("dolt", "remote", "add", "upstream", remoteURL)
	cmd.Dir = localDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if strings.Contains(strings.ToLower(msg), "already exists") {
			return nil
		}
		return fmt.Errorf("dolt remote add upstream: %w (%s)", err, msg)
	}
	return nil
}

// fileConfigStore implements ConfigStore using a directory of per-wasteland JSON files.
// Config files live at {configDir}/wastelands/{org}/{db}.json.
type fileConfigStore struct{}

func (f *fileConfigStore) configPath(upstream string) (string, error) {
	org, db, err := ParseUpstream(upstream)
	if err != nil {
		return "", err
	}
	return filepath.Join(xdg.ConfigDir(), "wastelands", org, db+".json"), nil
}

func (f *fileConfigStore) Load(upstream string) (*Config, error) {
	path, err := f.configPath(upstream)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotJoined
		}
		return nil, fmt.Errorf("reading wasteland config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing wasteland config: %w", err)
	}
	return &cfg, nil
}

func (f *fileConfigStore) Save(cfg *Config) error {
	path, err := f.configPath(cfg.Upstream)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling wasteland config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (f *fileConfigStore) Delete(upstream string) error {
	path, err := f.configPath(upstream)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrNotJoined, upstream)
		}
		return fmt.Errorf("deleting wasteland config: %w", err)
	}
	// Clean up empty parent directory (org dir).
	orgDir := filepath.Dir(path)
	entries, err := os.ReadDir(orgDir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(orgDir)
	}
	return nil
}

func (f *fileConfigStore) List() ([]string, error) {
	wasteDir := filepath.Join(xdg.ConfigDir(), "wastelands")
	if _, err := os.Stat(wasteDir); os.IsNotExist(err) {
		return nil, nil
	}

	var upstreams []string
	orgEntries, err := os.ReadDir(wasteDir)
	if err != nil {
		return nil, fmt.Errorf("reading wastelands directory: %w", err)
	}
	for _, orgEntry := range orgEntries {
		if !orgEntry.IsDir() {
			continue
		}
		orgDir := filepath.Join(wasteDir, orgEntry.Name())
		dbEntries, err := os.ReadDir(orgDir)
		if err != nil {
			continue
		}
		for _, dbEntry := range dbEntries {
			name := dbEntry.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			db := strings.TrimSuffix(name, ".json")
			upstreams = append(upstreams, orgEntry.Name()+"/"+db)
		}
	}
	return upstreams, nil
}

// NewConfigStore creates a ConfigStore backed by the filesystem.
func NewConfigStore() ConfigStore {
	return &fileConfigStore{}
}

// NewService creates a Service with real (production) dependencies.
func NewService(provider remote.Provider) *Service {
	return &Service{
		Remote: provider,
		CLI:    &execDoltCLI{},
		Config: &fileConfigStore{},
	}
}

// NewServiceWith creates a Service with an explicit ConfigStore.
func NewServiceWith(provider remote.Provider, store ConfigStore) *Service {
	return &Service{
		Remote: provider,
		CLI:    &execDoltCLI{},
		Config: store,
	}
}
