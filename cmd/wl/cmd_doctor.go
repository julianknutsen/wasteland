package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newDoctorCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check your wasteland setup for common issues",
		Long: `Run diagnostic checks on your wasteland setup.

Verifies dolt installation, credentials, environment variables,
and per-wasteland configuration.

Examples:
  wl doctor`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDoctor(stdout, stderr, exec.LookPath, os.Getenv, federation.NewConfigStore())
		},
	}
}

// doctorDeps holds injectable dependencies for testing.
type doctorDeps struct {
	lookPath func(string) (string, error)
	getenv   func(string) string
	store    federation.ConfigStore
}

func runDoctor(stdout, _ io.Writer, lookPath func(string) (string, error), getenv func(string) string, store federation.ConfigStore) error {
	deps := &doctorDeps{lookPath: lookPath, getenv: getenv, store: store}
	runDoctorChecks(stdout, deps)
	return nil
}

func runDoctorChecks(stdout io.Writer, deps *doctorDeps) {
	// 1. dolt installed
	checkDolt(stdout, deps)

	// 2. dolt credentials
	checkDoltCreds(stdout)

	// 3. DOLTHUB_TOKEN
	checkEnvVar(stdout, deps, "DOLTHUB_TOKEN")

	// 4. DOLTHUB_ORG
	checkEnvVar(stdout, deps, "DOLTHUB_ORG")

	// 5. Wastelands joined
	checkWastelands(stdout, deps)
}

func checkDolt(stdout io.Writer, deps *doctorDeps) {
	doltPath, err := deps.lookPath("dolt")
	if err != nil {
		fmt.Fprintf(stdout, "  %s dolt: not found in PATH\n", style.Error.Render(style.IconFail))
		return
	}

	cmd := exec.Command(doltPath, "version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(stdout, "  %s dolt: found but 'dolt version' failed: %v\n", style.Warning.Render(style.IconWarn), err)
		return
	}
	ver := strings.TrimSpace(string(output))
	fmt.Fprintf(stdout, "  %s dolt: %s\n", style.Success.Render(style.IconPass), ver)
}

func checkDoltCreds(stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stdout, "  %s dolt credentials: cannot determine home directory\n", style.Warning.Render(style.IconWarn))
		return
	}
	credsDir := filepath.Join(home, ".dolt", "creds")
	entries, err := os.ReadDir(credsDir)
	if err != nil {
		fmt.Fprintf(stdout, "  %s dolt credentials: no credentials directory found (%s)\n", style.Warning.Render(style.IconWarn), credsDir)
		return
	}
	var keyCount int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jwk") {
			keyCount++
		}
	}
	if keyCount == 0 {
		fmt.Fprintf(stdout, "  %s dolt credentials: no key files found in %s\n", style.Warning.Render(style.IconWarn), credsDir)
	} else {
		fmt.Fprintf(stdout, "  %s dolt credentials: %d key(s) found\n", style.Success.Render(style.IconPass), keyCount)
	}
}

func checkEnvVar(stdout io.Writer, deps *doctorDeps, name string) {
	val := deps.getenv(name)
	if val == "" {
		fmt.Fprintf(stdout, "  %s %s: not set\n", style.Warning.Render(style.IconWarn), name)
	} else {
		// Show partial value for ORG, hide TOKEN
		if name == "DOLTHUB_ORG" {
			fmt.Fprintf(stdout, "  %s %s: set (%s)\n", style.Success.Render(style.IconPass), name, val)
		} else {
			fmt.Fprintf(stdout, "  %s %s: set\n", style.Success.Render(style.IconPass), name)
		}
	}
}

func checkWastelands(stdout io.Writer, deps *doctorDeps) {
	upstreams, err := deps.store.List()
	if err != nil {
		fmt.Fprintf(stdout, "  %s wastelands: error listing: %v\n", style.Error.Render(style.IconFail), err)
		return
	}
	if len(upstreams) == 0 {
		fmt.Fprintf(stdout, "  %s wastelands: none joined (run 'wl join <upstream>')\n", style.Warning.Render(style.IconWarn))
		return
	}
	fmt.Fprintf(stdout, "  %s %d wasteland(s) joined\n", style.Success.Render(style.IconPass), len(upstreams))

	for _, upstream := range upstreams {
		cfg, err := deps.store.Load(upstream)
		if err != nil {
			fmt.Fprintf(stdout, "\n  %s:\n", upstream)
			fmt.Fprintf(stdout, "    %s config: failed to load: %v\n", style.Error.Render(style.IconFail), err)
			continue
		}
		fmt.Fprintf(stdout, "\n  %s:\n", upstream)

		// Local clone exists
		if _, err := os.Stat(cfg.LocalDir); err != nil {
			fmt.Fprintf(stdout, "    %s Local clone: missing (%s)\n", style.Error.Render(style.IconFail), cfg.LocalDir)
		} else {
			fmt.Fprintf(stdout, "    %s Local clone: %s\n", style.Success.Render(style.IconPass), cfg.LocalDir)
		}

		// Mode
		fmt.Fprintf(stdout, "    %s Mode: %s\n", style.Success.Render(style.IconPass), cfg.ResolveMode())

		// GPG signing
		checkGPGSigning(stdout, cfg, deps)
	}
}

func checkGPGSigning(stdout io.Writer, cfg *federation.Config, deps *doctorDeps) {
	if !cfg.Signing {
		fmt.Fprintf(stdout, "    %s GPG signing: disabled\n", style.Warning.Render(style.IconWarn))
		return
	}

	// Check that dolt has a signing key configured.
	doltPath, err := deps.lookPath("dolt")
	if err != nil {
		fmt.Fprintf(stdout, "    %s GPG signing: enabled but dolt not found\n", style.Warning.Render(style.IconWarn))
		return
	}

	cmd := exec.Command(doltPath, "config", "--global", "--get", "sqlserver.global.signingkey")
	keyOut, err := cmd.Output()
	keyID := strings.TrimSpace(string(keyOut))
	if err != nil || keyID == "" {
		fmt.Fprintf(stdout, "    %s GPG signing: enabled but no signing key configured in dolt\n", style.Error.Render(style.IconFail))
		fmt.Fprintf(stdout, "      Run: dolt config --global --add sqlserver.global.signingkey <your-gpg-key-id>\n")
		return
	}

	// Check that the GPG key actually exists locally.
	gpgCmd := exec.Command("gpg", "--list-secret-keys", keyID)
	if err := gpgCmd.Run(); err != nil {
		fmt.Fprintf(stdout, "    %s GPG signing: key %s not found in GPG keyring\n", style.Error.Render(style.IconFail), keyID)
		fmt.Fprintf(stdout, "      Run: gpg --list-secret-keys --keyid-format long\n")
		return
	}

	fmt.Fprintf(stdout, "    %s GPG signing: enabled (key %s)\n", style.Success.Render(style.IconPass), keyID)
}
