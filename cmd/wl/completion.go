package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/spf13/cobra"
)

const completionCacheTTL = 5 * time.Second

// completeWantedIDs returns a ValidArgsFunction that completes wanted IDs,
// optionally filtered by status (e.g. "open", "claimed", "in_review").
func completeWantedIDs(statusFilter string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := resolveWasteland(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cacheKey := "wanted-" + statusFilter
		if cached := readCompletionCache(cacheKey); cached != nil {
			return cached, cobra.ShellCompDirectiveNoFileComp
		}
		ids := listWantedIDsWithTimeout(cfg.LocalDir, statusFilter)
		writeCompletionCache(cacheKey, ids)
		return ids, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeBranchNames completes wl/* branch names.
func completeBranchNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cacheKey := "branches"
	if cached := readCompletionCache(cacheKey); cached != nil {
		return cached, cobra.ShellCompDirectiveNoFileComp
	}
	branches := listBranchesWithTimeout(cfg.LocalDir)
	writeCompletionCache(cacheKey, branches)
	return branches, cobra.ShellCompDirectiveNoFileComp
}

// listWantedIDsWithTimeout queries wanted IDs with a 2-second timeout.
// Returns items in cobra completion format: "id\tPn title" for rich shell hints.
func listWantedIDsWithTimeout(dbDir, statusFilter string) []string {
	query := "SELECT id, title, priority FROM wanted"
	if statusFilter != "" {
		query += " WHERE status = '" + commons.EscapeSQL(statusFilter) + "'"
	}
	query += " ORDER BY created_at DESC LIMIT 50"
	out := doltQueryWithTimeout(dbDir, query, 2*time.Second)
	if out == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil
	}
	var items []string
	for _, line := range lines[1:] {
		fields := strings.SplitN(line, ",", 3)
		if len(fields) < 1 {
			continue
		}
		id := strings.TrimSpace(fields[0])
		if id == "" {
			continue
		}
		if len(fields) >= 2 {
			title := strings.TrimSpace(fields[1])
			if len(title) > 40 {
				title = title[:40] + "..."
			}
			if len(fields) >= 3 {
				pri := strings.TrimSpace(fields[2])
				id += "\t" + "P" + pri + " " + title
			} else {
				id += "\t" + title
			}
		}
		items = append(items, id)
	}
	return items
}

// listBranchesWithTimeout queries wl/* branches with a 2-second timeout.
func listBranchesWithTimeout(dbDir string) []string {
	query := "SELECT name FROM dolt_branches WHERE name LIKE 'wl/%' ORDER BY name"
	out := doltQueryWithTimeout(dbDir, query, 2*time.Second)
	if out == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil
	}
	var branches []string
	for _, line := range lines[1:] {
		name := strings.TrimSpace(line)
		if name != "" {
			branches = append(branches, name)
		}
	}
	return branches
}

// doltQueryWithTimeout runs a dolt SQL query with a strict timeout.
func doltQueryWithTimeout(dbDir, query string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "dolt", "sql", "-r", "csv", "-q", query)
	cmd.Dir = dbDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}

// completionCacheDir returns the directory for completion cache files.
func completionCacheDir() string {
	return filepath.Join(os.TempDir(), "wl-completion-cache")
}

// readCompletionCache returns cached completions if the cache is fresh.
func readCompletionCache(key string) []string {
	path := filepath.Join(completionCacheDir(), key+".json")
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > completionCacheTTL {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

// completeProjectNames provides completion for --project flags.
func completeProjectNames(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cacheKey := "projects"
	if cached := readCompletionCache(cacheKey); cached != nil {
		return cached, cobra.ShellCompDirectiveNoFileComp
	}
	query := "SELECT DISTINCT project FROM wanted WHERE project != '' ORDER BY project LIMIT 50"
	out := doltQueryWithTimeout(cfg.LocalDir, query, 2*time.Second)
	if out == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var projects []string
	for _, line := range lines[1:] {
		p := strings.TrimSpace(line)
		if p != "" {
			projects = append(projects, p)
		}
	}
	writeCompletionCache(cacheKey, projects)
	return projects, cobra.ShellCompDirectiveNoFileComp
}

// completeWastelandNames provides completion for the --wasteland persistent flag.
func completeWastelandNames(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	store := federation.NewConfigStore()
	upstreams, err := store.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return upstreams, cobra.ShellCompDirectiveNoFileComp
}

// writeCompletionCache writes completions to the cache.
func writeCompletionCache(key string, items []string) {
	dir := completionCacheDir()
	_ = os.MkdirAll(dir, 0o755)
	data, err := json.Marshal(items)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, key+".json"), data, 0o644)
}
