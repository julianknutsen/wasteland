package commons

import (
	"context"
	"fmt"
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

// DatabaseExists checks whether a database exists in the data directory.
func DatabaseExists(dataDir, dbName string) bool {
	doltDir := fmt.Sprintf("%s/%s/.dolt", dataDir, dbName)
	_, err := os.Stat(doltDir)
	return err == nil
}

// InitDB initializes a new dolt database in the data directory.
func InitDB(dataDir, dbName string) error {
	dbDir := fmt.Sprintf("%s/%s", dataDir, dbName)

	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return fmt.Errorf("creating database directory: %w", err)
	}

	cmd := exec.Command("dolt", "init")
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("initializing Dolt database: %w\n%s", err, output)
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
