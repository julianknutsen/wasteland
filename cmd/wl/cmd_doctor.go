package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newDoctorCmd(stdout, stderr io.Writer) *cobra.Command {
	var fix, check bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check your wasteland setup for common issues",
		Long: `Run diagnostic checks on your wasteland setup.

Verifies dolt installation, credentials, environment variables,
and per-wasteland configuration.

Use --fix to attempt auto-repair of fixable issues.
Use --check to exit non-zero if any warnings or failures (useful for CI).

Examples:
  wl doctor
  wl doctor --fix
  wl doctor --check`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDoctor(stdout, stderr, exec.LookPath, os.Getenv, federation.NewConfigStore(), fix, check)
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to auto-fix issues")
	cmd.Flags().BoolVar(&check, "check", false, "Exit non-zero if any warnings or failures")

	return cmd
}

// diagnostic holds a single check result.
type diagnostic struct {
	name    string
	status  string // "pass", "warn", "fail"
	message string
	fixFunc func() error // nil if no auto-fix available
	fixHint string       // manual fix instructions
}

// doctorDeps holds injectable dependencies for testing.
type doctorDeps struct {
	lookPath func(string) (string, error)
	getenv   func(string) string
	store    federation.ConfigStore
}

func runDoctor(stdout, _ io.Writer, lookPath func(string) (string, error), getenv func(string) string, store federation.ConfigStore, fix, check bool) error {
	deps := &doctorDeps{lookPath: lookPath, getenv: getenv, store: store}
	results := runDoctorChecks(stdout, deps)

	// --fix: attempt auto-repairs.
	if fix {
		for _, d := range results {
			if (d.status == "fail" || d.status == "warn") && d.fixFunc != nil {
				fmt.Fprintf(stdout, "\n  Fixing %s...\n", d.name)
				if err := d.fixFunc(); err != nil {
					fmt.Fprintf(stdout, "    %s fix failed: %v\n", style.Error.Render(style.IconFail), err)
				} else {
					fmt.Fprintf(stdout, "    %s fixed\n", style.Success.Render(style.IconPass))
				}
			}
		}
	}

	// --check: exit non-zero if any issues.
	if check {
		for _, d := range results {
			if d.status == "fail" || d.status == "warn" {
				return errExit
			}
		}
	}

	return nil
}

func runDoctorChecks(stdout io.Writer, deps *doctorDeps) []diagnostic {
	var results []diagnostic

	// 1. dolt installed
	results = append(results, checkDolt(stdout, deps))

	// 2. dolt credentials
	results = append(results, checkDoltCreds(stdout))

	// 3. DOLTHUB_TOKEN
	results = append(results, checkEnvVar(stdout, deps, "DOLTHUB_TOKEN"))

	// 4. DOLTHUB_ORG
	results = append(results, checkEnvVar(stdout, deps, "DOLTHUB_ORG"))

	// 5. Wastelands joined (may add multiple diagnostics)
	results = append(results, checkWastelands(stdout, deps)...)

	return results
}

func checkDolt(stdout io.Writer, deps *doctorDeps) diagnostic {
	doltPath, err := deps.lookPath("dolt")
	if err != nil {
		d := diagnostic{
			name: "dolt", status: "fail", message: "not found in PATH",
			fixHint: "Install dolt: https://docs.dolthub.com/introduction/installation",
		}
		fmt.Fprintf(stdout, "  %s dolt: %s\n", style.Error.Render(style.IconFail), d.message)
		return d
	}

	cmd := exec.Command(doltPath, "version")
	output, err := cmd.Output()
	if err != nil {
		d := diagnostic{name: "dolt", status: "warn", message: fmt.Sprintf("found but 'dolt version' failed: %v", err)}
		fmt.Fprintf(stdout, "  %s dolt: %s\n", style.Warning.Render(style.IconWarn), d.message)
		return d
	}
	ver := strings.TrimSpace(string(output))
	fmt.Fprintf(stdout, "  %s dolt: %s\n", style.Success.Render(style.IconPass), ver)
	return diagnostic{name: "dolt", status: "pass", message: ver}
}

func checkDoltCreds(stdout io.Writer) diagnostic {
	home, err := os.UserHomeDir()
	if err != nil {
		d := diagnostic{name: "dolt credentials", status: "warn", message: "cannot determine home directory"}
		fmt.Fprintf(stdout, "  %s dolt credentials: %s\n", style.Warning.Render(style.IconWarn), d.message)
		return d
	}
	credsDir := filepath.Join(home, ".dolt", "creds")
	entries, err := os.ReadDir(credsDir)
	if err != nil {
		d := diagnostic{
			name: "dolt credentials", status: "warn",
			message: fmt.Sprintf("no credentials directory found (%s)", credsDir),
			fixHint: "Run: dolt login",
		}
		fmt.Fprintf(stdout, "  %s dolt credentials: %s\n", style.Warning.Render(style.IconWarn), d.message)
		return d
	}
	var keyCount int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jwk") {
			keyCount++
		}
	}
	if keyCount == 0 {
		d := diagnostic{
			name: "dolt credentials", status: "warn",
			message: fmt.Sprintf("no key files found in %s", credsDir),
			fixHint: "Run: dolt login",
		}
		fmt.Fprintf(stdout, "  %s dolt credentials: %s\n", style.Warning.Render(style.IconWarn), d.message)
		return d
	}
	fmt.Fprintf(stdout, "  %s dolt credentials: %d key(s) found\n", style.Success.Render(style.IconPass), keyCount)
	return diagnostic{name: "dolt credentials", status: "pass", message: fmt.Sprintf("%d key(s) found", keyCount)}
}

func checkEnvVar(stdout io.Writer, deps *doctorDeps, name string) diagnostic {
	val := deps.getenv(name)
	if val == "" {
		d := diagnostic{name: name, status: "warn", message: "not set"}
		fmt.Fprintf(stdout, "  %s %s: not set\n", style.Warning.Render(style.IconWarn), name)
		return d
	}
	// Show partial value for ORG, hide TOKEN
	if name == "DOLTHUB_ORG" {
		fmt.Fprintf(stdout, "  %s %s: set (%s)\n", style.Success.Render(style.IconPass), name, val)
	} else {
		fmt.Fprintf(stdout, "  %s %s: set\n", style.Success.Render(style.IconPass), name)
	}
	return diagnostic{name: name, status: "pass", message: "set"}
}

func checkWastelands(stdout io.Writer, deps *doctorDeps) []diagnostic {
	var results []diagnostic

	upstreams, err := deps.store.List()
	if err != nil {
		d := diagnostic{name: "wastelands", status: "fail", message: fmt.Sprintf("error listing: %v", err)}
		fmt.Fprintf(stdout, "  %s wastelands: %s\n", style.Error.Render(style.IconFail), d.message)
		return append(results, d)
	}
	if len(upstreams) == 0 {
		d := diagnostic{
			name: "wastelands", status: "warn", message: "none joined",
			fixHint: "Run 'wl join <upstream>'",
		}
		fmt.Fprintf(stdout, "  %s wastelands: none joined (run 'wl join <upstream>')\n", style.Warning.Render(style.IconWarn))
		return append(results, d)
	}
	fmt.Fprintf(stdout, "  %s %d wasteland(s) joined\n", style.Success.Render(style.IconPass), len(upstreams))
	results = append(results, diagnostic{name: "wastelands", status: "pass", message: fmt.Sprintf("%d joined", len(upstreams))})

	for _, upstream := range upstreams {
		cfg, err := deps.store.Load(upstream)
		if err != nil {
			fmt.Fprintf(stdout, "\n  %s:\n", upstream)
			fmt.Fprintf(stdout, "    %s config: failed to load: %v\n", style.Error.Render(style.IconFail), err)
			results = append(results, diagnostic{name: upstream + "/config", status: "fail", message: fmt.Sprintf("failed to load: %v", err)})
			continue
		}
		fmt.Fprintf(stdout, "\n  %s:\n", upstream)

		// Local clone exists
		if _, err := os.Stat(cfg.LocalDir); err != nil {
			d := diagnostic{
				name:    upstream + "/clone",
				status:  "fail",
				message: fmt.Sprintf("missing (%s)", cfg.LocalDir),
				fixHint: "Re-clone from upstream",
			}
			if cfg.UpstreamURL != "" {
				d.fixFunc = func() error {
					doltPath, err := exec.LookPath("dolt")
					if err != nil {
						return fmt.Errorf("dolt not found in PATH")
					}
					cmd := exec.Command(doltPath, "clone", cfg.UpstreamURL, cfg.LocalDir)
					output, err := cmd.CombinedOutput()
					if err != nil {
						return fmt.Errorf("dolt clone: %s", strings.TrimSpace(string(output)))
					}
					return nil
				}
			}
			fmt.Fprintf(stdout, "    %s Local clone: missing (%s)\n", style.Error.Render(style.IconFail), cfg.LocalDir)
			results = append(results, d)
		} else {
			fmt.Fprintf(stdout, "    %s Local clone: %s\n", style.Success.Render(style.IconPass), cfg.LocalDir)
			results = append(results, diagnostic{name: upstream + "/clone", status: "pass"})
		}

		// Mode
		fmt.Fprintf(stdout, "    %s Mode: %s\n", style.Success.Render(style.IconPass), cfg.ResolveMode())

		// Stale sync check
		results = append(results, checkStaleSync(stdout, cfg, upstream)...)

		// GPG signing
		results = append(results, checkGPGSigning(stdout, cfg, deps))
	}

	return results
}

func checkStaleSync(stdout io.Writer, cfg *federation.Config, upstream string) []diagnostic {
	if cfg.LastSyncAt == nil {
		return nil
	}
	age := time.Since(*cfg.LastSyncAt)
	if age < 24*time.Hour {
		fmt.Fprintf(stdout, "    %s sync: last synced %s ago\n", style.Success.Render(style.IconPass), formatDuration(age))
		return []diagnostic{{name: upstream + "/sync", status: "pass", message: fmt.Sprintf("last synced %s ago", formatDuration(age))}}
	}

	d := diagnostic{
		name:    upstream + "/sync",
		status:  "warn",
		message: fmt.Sprintf("last synced %s ago", formatDuration(age)),
		fixHint: "Run 'wl sync'",
	}
	if cfg.LocalDir != "" {
		localDir := cfg.LocalDir
		d.fixFunc = func() error {
			doltPath, err := exec.LookPath("dolt")
			if err != nil {
				return fmt.Errorf("dolt not found in PATH")
			}
			cmd := exec.Command(doltPath, "pull", "upstream", "main")
			cmd.Dir = localDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("dolt pull: %s", strings.TrimSpace(string(output)))
			}
			updateSyncTimestamp(cfg)
			return nil
		}
	}
	fmt.Fprintf(stdout, "    %s sync: last synced %s ago\n", style.Warning.Render(style.IconWarn), formatDuration(age))
	return []diagnostic{d}
}

func checkGPGSigning(stdout io.Writer, cfg *federation.Config, deps *doctorDeps) diagnostic {
	if !cfg.Signing {
		fmt.Fprintf(stdout, "    %s GPG signing: disabled\n", style.Warning.Render(style.IconWarn))
		return diagnostic{name: "gpg-signing", status: "warn", message: "disabled"}
	}

	// Check that dolt has a signing key configured.
	doltPath, err := deps.lookPath("dolt")
	if err != nil {
		fmt.Fprintf(stdout, "    %s GPG signing: enabled but dolt not found\n", style.Warning.Render(style.IconWarn))
		return diagnostic{name: "gpg-signing", status: "warn", message: "enabled but dolt not found"}
	}

	cmd := exec.Command(doltPath, "config", "--global", "--get", "sqlserver.global.signingkey")
	keyOut, err := cmd.Output()
	keyID := strings.TrimSpace(string(keyOut))
	if err != nil || keyID == "" {
		fmt.Fprintf(stdout, "    %s GPG signing: enabled but no signing key configured in dolt\n", style.Error.Render(style.IconFail))
		fmt.Fprintf(stdout, "      Run: dolt config --global --add sqlserver.global.signingkey <your-gpg-key-id>\n")
		return diagnostic{
			name: "gpg-signing", status: "fail", message: "enabled but no signing key configured",
			fixHint: "Run: dolt config --global --add sqlserver.global.signingkey <your-gpg-key-id>",
		}
	}

	// Check that the GPG key actually exists locally.
	gpgCmd := exec.Command("gpg", "--list-secret-keys", keyID)
	if err := gpgCmd.Run(); err != nil {
		fmt.Fprintf(stdout, "    %s GPG signing: key %s not found in GPG keyring\n", style.Error.Render(style.IconFail), keyID)
		fmt.Fprintf(stdout, "      Run: gpg --list-secret-keys --keyid-format long\n")
		return diagnostic{
			name: "gpg-signing", status: "fail", message: fmt.Sprintf("key %s not found in GPG keyring", keyID),
			fixHint: "Run: gpg --list-secret-keys --keyid-format long",
		}
	}

	fmt.Fprintf(stdout, "    %s GPG signing: enabled (key %s)\n", style.Success.Render(style.IconPass), keyID)
	return diagnostic{name: "gpg-signing", status: "pass", message: fmt.Sprintf("enabled (key %s)", keyID)}
}
