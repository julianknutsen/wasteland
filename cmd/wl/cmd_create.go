package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/schema"
	"github.com/spf13/cobra"
)

func newCreateCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		name      string
		localOnly bool
		signed    bool
	)

	cmd := &cobra.Command{
		Use:   "create <org/db-name>",
		Short: "Create a new wasteland commons database",
		Long: `Create a new wasteland commons database initialized with the standard schema.

This command:
  1. Initializes a new dolt database with the commons schema
  2. Seeds the _meta table with schema version
  3. Commits the initial schema
  4. Pushes to DoltHub (unless --local-only)

Examples:
  wl create myorg/wl-commons                       # create and push to DoltHub
  wl create myorg/wl-commons --name "My Wasteland"  # custom display name
  wl create myorg/wl-commons --local-only            # skip push
  wl create myorg/wl-commons --signed                # GPG-sign initial commit`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCreate(stdout, stderr, args[0], name, localOnly, signed)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name for the wasteland (stored in _meta)")
	cmd.Flags().BoolVar(&localOnly, "local-only", false, "Skip pushing to remote")
	cmd.Flags().BoolVar(&signed, "signed", false, "GPG-sign the initial commit")

	return cmd
}

func runCreate(stdout, _ io.Writer, upstream, name string, localOnly, signed bool) error {
	if err := requireDolt(); err != nil {
		return err
	}

	org, db, err := federation.ParseUpstream(upstream)
	if err != nil {
		return err
	}

	localDir := federation.LocalCloneDir(org, db)

	// Check if already exists.
	if _, err := os.Stat(filepath.Join(localDir, ".dolt")); err == nil {
		return fmt.Errorf("database already exists at %s", localDir)
	}

	// Create parent directory and init.
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	fmt.Fprintf(stdout, "Creating wasteland %s...\n", upstream)

	// dolt init
	initCmd := exec.Command("dolt", "init")
	initCmd.Dir = localDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dolt init: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(stdout, "  Initialized dolt database\n")

	// Apply schema via dolt sql
	sqlScript := schema.SQL
	if name != "" {
		sqlScript += fmt.Sprintf("\nINSERT IGNORE INTO _meta (`key`, value) VALUES ('wasteland_name', '%s');\n",
			strings.ReplaceAll(name, "'", "''"))
	}

	sqlCmd := exec.Command("dolt", "sql", "-q", sqlScript)
	sqlCmd.Dir = localDir
	if out, err := sqlCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("applying schema: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(stdout, "  Applied commons schema v1.0\n")

	// Stage and commit
	addCmd := exec.Command("dolt", "add", "-A")
	addCmd.Dir = localDir
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dolt add: %w (%s)", err, strings.TrimSpace(string(out)))
	}

	commitArgs := []string{"commit", "-m", "Initialize commons schema v1.0"}
	if signed {
		commitArgs = []string{"commit", "-S", "-m", "Initialize commons schema v1.0"}
	}
	commitCmd := exec.Command("dolt", commitArgs...)
	commitCmd.Dir = localDir
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dolt commit: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	fmt.Fprintf(stdout, "  Committed initial schema\n")

	if !localOnly {
		remoteURL := fmt.Sprintf("https://doltremoteapi.dolthub.com/%s/%s", org, db)
		remoteCmd := exec.Command("dolt", "remote", "add", "origin", remoteURL)
		remoteCmd.Dir = localDir
		if out, err := remoteCmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(strings.ToLower(msg), "already exists") {
				return fmt.Errorf("dolt remote add: %w (%s)", err, msg)
			}
		}

		pushCmd := exec.Command("dolt", "push", "origin", "main")
		pushCmd.Dir = localDir
		if out, err := pushCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("dolt push: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		fmt.Fprintf(stdout, "  Pushed to %s/%s\n", org, db)
	}

	fmt.Fprintf(stdout, "\n✓ Created wasteland: %s\n", upstream)
	fmt.Fprintf(stdout, "  Local: %s\n", localDir)
	fmt.Fprintf(stdout, "\n  Share: wl join %s\n", upstream)

	return nil
}
