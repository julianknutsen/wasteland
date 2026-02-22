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

// Config holds the wasteland configuration for a rig.
type Config struct {
	// Upstream is the DoltHub path of the upstream commons (e.g., "steveyegge/wl-commons").
	Upstream string `json:"upstream"`

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
}

// ConfigPath returns the path to the wasteland config file.
func ConfigPath() string {
	return filepath.Join(xdg.ConfigDir(), "config.json")
}

// LoadConfig loads the wasteland configuration from disk.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w (run 'wl join <upstream>')", ErrNotJoined)
		}
		return nil, fmt.Errorf("reading wasteland config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing wasteland config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the wasteland configuration to disk.
func SaveConfig(cfg *Config) error {
	dir := xdg.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling wasteland config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(ConfigPath(), data, 0644)
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
	RegisterRig(localDir, handle, dolthubOrg, displayName, ownerEmail, version string) error
	Push(localDir string) error
	AddUpstreamRemote(localDir, remoteURL string) error
}

// ConfigStore abstracts wasteland config persistence.
type ConfigStore interface {
	Load() (*Config, error)
	Save(cfg *Config) error
}

// Service coordinates wasteland operations with injectable dependencies.
type Service struct {
	Remote     remote.Provider
	CLI        DoltCLI
	Config     ConfigStore
	OnProgress func(step string) // optional callback for progress reporting
}

// Join orchestrates the wasteland join workflow: fork -> clone -> add upstream -> register -> push -> save config.
func (s *Service) Join(upstream, forkOrg, handle, displayName, ownerEmail, version string) (*Config, error) {
	upstreamOrg, upstreamDB, err := ParseUpstream(upstream)
	if err != nil {
		return nil, err
	}

	// Check if already joined
	if existing, err := s.Config.Load(); err == nil {
		if existing.Upstream != upstream {
			return nil, fmt.Errorf("already joined to %s; run wl leave first", existing.Upstream)
		}
		return existing, nil
	} else if !errors.Is(err, ErrNotJoined) {
		return nil, fmt.Errorf("loading wasteland config: %w", err)
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
	if err := s.CLI.RegisterRig(localDir, handle, forkOrg, displayName, ownerEmail, version); err != nil {
		return nil, fmt.Errorf("registering rig: %w", err)
	}

	progress("Pushing to fork...")
	if err := s.CLI.Push(localDir); err != nil {
		return nil, fmt.Errorf("pushing to fork: %w", err)
	}

	cfg := &Config{
		Upstream:  upstream,
		ForkOrg:   forkOrg,
		ForkDB:    upstreamDB,
		LocalDir:  localDir,
		RigHandle: handle,
		JoinedAt:  time.Now(),
	}
	if err := s.Config.Save(cfg); err != nil {
		return nil, fmt.Errorf("saving wasteland config: %w", err)
	}

	return cfg, nil
}

// execDoltCLI implements DoltCLI using real dolt subprocess calls.
type execDoltCLI struct{}

func (e *execDoltCLI) Clone(remoteURL, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
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

func (e *execDoltCLI) RegisterRig(localDir, handle, dolthubOrg, displayName, ownerEmail, version string) error {
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

	commitCmd := exec.Command("dolt", "commit", "-m", fmt.Sprintf("Register rig: %s", handle))
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

// fileConfigStore implements ConfigStore using filesystem persistence.
type fileConfigStore struct{}

func (f *fileConfigStore) Load() (*Config, error) {
	return LoadConfig()
}

func (f *fileConfigStore) Save(cfg *Config) error {
	return SaveConfig(cfg)
}

// NewService creates a Service with real (production) dependencies.
func NewService(provider remote.Provider) *Service {
	return &Service{
		Remote: provider,
		CLI:    &execDoltCLI{},
		Config: &fileConfigStore{},
	}
}
