package remote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FileProvider implements Provider using file:// dolt remotes.
// The baseDir contains dolt remote stores (the format produced by
// "dolt push" to a file:// URL), not working directories.
//
// Useful for testing and offline development.
type FileProvider struct {
	baseDir string
}

// NewFileProvider creates a FileProvider rooted at baseDir.
func NewFileProvider(baseDir string) *FileProvider {
	return &FileProvider{baseDir: baseDir}
}

// DatabaseURL returns the file:// URL for the given org/db.
func (f *FileProvider) DatabaseURL(org, db string) string {
	return fmt.Sprintf("file://%s", filepath.Join(f.baseDir, org, db))
}

// Fork copies the source dolt remote store to create a fork under toOrg.
func (f *FileProvider) Fork(fromOrg, fromDB, toOrg string) error {
	srcURL := f.DatabaseURL(fromOrg, fromDB)
	destPath := filepath.Join(f.baseDir, toOrg, fromDB)
	destURL := f.DatabaseURL(toOrg, fromDB)

	// Already forked?
	if info, err := os.Stat(destPath); err == nil && info.IsDir() {
		return nil
	}

	// Clone from source remote store into a temp working directory.
	tmpDir, err := os.MkdirTemp("", "dolt-fork-*")
	if err != nil {
		return fmt.Errorf("creating temp dir for fork: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	workDir := filepath.Join(tmpDir, "work")
	cmd := exec.Command("dolt", "clone", srcURL, workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloning source for fork: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	// Create destination remote store directory and push into it.
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("creating fork remote store: %w", err)
	}

	cmd = exec.Command("dolt", "remote", "add", "fork-dest", destURL)
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adding fork dest remote: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	cmd = exec.Command("dolt", "push", "fork-dest", "main")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pushing to fork dest: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Type returns "file".
func (f *FileProvider) Type() string { return "file" }
