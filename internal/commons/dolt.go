package commons

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DoltHubToken returns the DoltHub API token from the environment.
func DoltHubToken() string {
	return os.Getenv("DOLTHUB_TOKEN")
}

// DoltHubOrg returns the default DoltHub organization from the environment.
func DoltHubOrg() string {
	return os.Getenv("DOLTHUB_ORG")
}

// PushWithSync pushes the local main branch to both upstream and origin remotes.
// If a push is rejected (stale), it pulls to merge and retries.
// Warnings are printed but do not cause a fatal error — the local commit is safe.
func PushWithSync(dbDir string, stdout io.Writer) error {
	for _, remote := range []string{"upstream", "origin"} {
		if err := pushRemote(dbDir, remote); err != nil {
			fmt.Fprintf(stdout, "  Syncing with %s...\n", remote)
			if pullErr := pullRemote(dbDir, remote); pullErr != nil {
				fmt.Fprintf(stdout, "  warning: sync from %s failed: %v\n", remote, pullErr)
				continue
			}
			if err := pushRemote(dbDir, remote); err != nil {
				fmt.Fprintf(stdout, "  warning: push to %s failed after sync: %v\n", remote, err)
				continue
			}
		}
		fmt.Fprintf(stdout, "  Pushed to %s\n", remote)
	}
	return nil
}

func pushRemote(dbDir, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "push", remote, "main")
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt push %s main: %w (%s)", remote, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func pullRemote(dbDir, remote string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "pull", remote, "main")
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dolt pull %s main: %w (%s)", remote, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// doltSQLScript executes a SQL script against a dolt database directory.
func doltSQLScript(dbDir, script string) error {
	tmpFile, err := os.CreateTemp("", "dolt-script-*.sql")
	if err != nil {
		return fmt.Errorf("creating temp SQL file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing SQL script: %w", err)
	}
	_ = tmpFile.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dolt", "sql", "--file", tmpFile.Name())
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// doltSQLQuery executes a SQL query and returns the raw CSV output.
func doltSQLQuery(dbDir, query string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dolt", "sql", "-r", "csv", "-q", query)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("dolt sql query failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}
