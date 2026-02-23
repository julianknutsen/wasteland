package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
)

func newReviewCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		jsonOut bool
		mdOut   bool
		statOut bool
		ghPR    bool
	)

	cmd := &cobra.Command{
		Use:   "review [branch]",
		Short: "Review PR-mode branches",
		Long: `List or review PR-mode branches.

Without arguments, lists all wl/* branches.
With a branch name, shows the diff between main and the branch.

Output formats (mutually exclusive):
  (default)  Full diff piped to stdout
  --stat     Summary statistics
  --json     JSON diff output
  --md       Markdown-formatted diff for pasting into PRs
  --gh-pr    Push branch to GitHub fork and open a cross-fork PR shell

Examples:
  wl review                          # list wl/* branches
  wl review wl/my-rig/w-abc123       # terminal diff
  wl review wl/my-rig/w-abc123 --stat
  wl review wl/my-rig/w-abc123 --md
  wl review wl/my-rig/w-abc123 --gh-pr`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var branch string
			if len(args) == 1 {
				branch = args[0]
			}
			return runReview(cmd, stdout, stderr, branch, jsonOut, mdOut, statOut, ghPR)
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output diff as JSON")
	cmd.Flags().BoolVar(&mdOut, "md", false, "Output diff as Markdown")
	cmd.Flags().BoolVar(&statOut, "stat", false, "Output diff statistics")
	cmd.Flags().BoolVar(&ghPR, "gh-pr", false, "Push to GitHub fork and open a PR shell")

	return cmd
}

func runReview(cmd *cobra.Command, stdout, _ io.Writer, branch string, jsonOut, mdOut, statOut, ghPR bool) error {
	// Validate mutually exclusive flags.
	flagCount := 0
	if jsonOut {
		flagCount++
	}
	if mdOut {
		flagCount++
	}
	if statOut {
		flagCount++
	}
	if ghPR {
		flagCount++
	}
	if flagCount > 1 {
		return fmt.Errorf("--json, --md, --stat, and --gh-pr are mutually exclusive")
	}

	if ghPR && branch == "" {
		return fmt.Errorf("--gh-pr requires a branch argument")
	}

	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if branch == "" {
		return listReviewBranches(stdout, cfg.LocalDir)
	}

	doltPath, err := exec.LookPath("dolt")
	if err != nil {
		return fmt.Errorf("dolt not found in PATH — install from https://docs.dolthub.com/introduction/installation")
	}

	if ghPR {
		return runGitHubPR(stdout, cfg, doltPath, branch)
	}

	return showDiff(stdout, cfg.LocalDir, doltPath, branch, jsonOut, mdOut, statOut)
}

func listReviewBranches(stdout io.Writer, dbDir string) error {
	branches, err := commons.ListBranches(dbDir, "wl/")
	if err != nil {
		return fmt.Errorf("listing branches: %w", err)
	}

	if len(branches) == 0 {
		fmt.Fprintln(stdout, "No review branches found.")
		return nil
	}

	fmt.Fprintf(stdout, "%s\n", style.Bold.Render("Review branches:"))
	for _, b := range branches {
		fmt.Fprintf(stdout, "  %s\n", b)
	}
	return nil
}

func showDiff(stdout io.Writer, dbDir, doltPath, branch string, jsonOut, mdOut, statOut bool) error {
	if statOut {
		cmd := exec.Command(doltPath, "diff", "--stat", "main..."+branch)
		cmd.Dir = dbDir
		cmd.Stdout = stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dolt diff --stat: %w", err)
		}
		return nil
	}

	if jsonOut {
		cmd := exec.Command(doltPath, "diff", "-r", "json", "main..."+branch)
		cmd.Dir = dbDir
		cmd.Stdout = stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("dolt diff -r json: %w", err)
		}
		return nil
	}

	if mdOut {
		return renderMarkdownDiff(stdout, dbDir, doltPath, branch)
	}

	// Default: full terminal diff.
	cmd := exec.Command(doltPath, "diff", "main..."+branch)
	cmd.Dir = dbDir
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dolt diff: %w", err)
	}
	return nil
}

func renderMarkdownDiff(stdout io.Writer, dbDir, doltPath, branch string) error {
	fmt.Fprintf(stdout, "## wl review: %s\n\n", branch)

	// Summary (stat).
	fmt.Fprintln(stdout, "### Summary")
	fmt.Fprintln(stdout, "```")

	statCmd := exec.Command(doltPath, "diff", "--stat", "main..."+branch)
	statCmd.Dir = dbDir
	statOut, err := statCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(stdout, "(no changes)\n")
	} else {
		fmt.Fprint(stdout, strings.TrimRight(string(statOut), "\n")+"\n")
	}
	fmt.Fprintln(stdout, "```")
	fmt.Fprintln(stdout)

	// Changes (SQL diff).
	fmt.Fprintln(stdout, "### Changes")
	fmt.Fprintln(stdout, "```sql")

	diffCmd := exec.Command(doltPath, "diff", "-r", "sql", "main..."+branch)
	diffCmd.Dir = dbDir
	diffOut, err := diffCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(stdout, "-- (no SQL changes)\n")
	} else {
		fmt.Fprint(stdout, strings.TrimRight(string(diffOut), "\n")+"\n")
	}
	fmt.Fprintln(stdout, "```")

	return nil
}

// --- GitHub PR shell ---

func runGitHubPR(stdout io.Writer, cfg *federation.Config, doltPath, branch string) error {
	if cfg.GitHubRepo == "" {
		return fmt.Errorf("github-repo not configured (run 'wl config set github-repo owner/repo')")
	}

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return fmt.Errorf("gh not found in PATH — install from https://cli.github.com")
	}

	// Ensure "github" Dolt remote exists.
	fmt.Fprintln(stdout, "  Configuring GitHub remote...")
	if err := commons.EnsureGitHubRemote(cfg.LocalDir, cfg.ForkOrg, cfg.ForkDB); err != nil {
		return fmt.Errorf("setting up GitHub remote: %w", err)
	}

	// Push Dolt branch to fork on GitHub.
	if err := commons.PushBranchToRemote(cfg.LocalDir, "github", branch, stdout); err != nil {
		return fmt.Errorf("pushing to GitHub fork: %w", err)
	}

	// Generate markdown diff.
	var mdBuf bytes.Buffer
	if err := renderMarkdownDiff(&mdBuf, cfg.LocalDir, doltPath, branch); err != nil {
		return fmt.Errorf("generating markdown diff: %w", err)
	}

	// Get wanted title for PR title.
	title := wantedTitleFromBranch(doltPath, cfg.LocalDir, branch)
	prTitle := fmt.Sprintf("[wl] %s", title)

	// Create git-native branch on fork + cross-fork PR to upstream.
	prURL, err := createGitHubPR(ghPath, cfg.GitHubRepo, cfg.ForkOrg, cfg.ForkDB, branch, prTitle, mdBuf.String(), stdout)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "\n%s %s\n", style.Bold.Render("PR:"), prURL)
	return nil
}

func createGitHubPR(ghPath, upstreamRepo, forkOrg, forkDB, wlBranch, title, mdBody string, stdout io.Writer) (string, error) {
	forkRepo := forkOrg + "/" + forkDB
	wantedID := extractWantedID(wlBranch)
	markerPath := ".wasteland/" + wantedID + ".md"

	// 1. Get fork's default branch SHA.
	fmt.Fprintln(stdout, "  Getting fork HEAD...")
	refData, err := ghAPICall(ghPath, "GET", fmt.Sprintf("repos/%s/git/ref/heads/main", forkRepo), "")
	if err != nil {
		return "", fmt.Errorf("getting fork HEAD: %w", err)
	}
	var ref struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(refData, &ref); err != nil {
		return "", fmt.Errorf("parsing fork HEAD: %w", err)
	}
	headSHA := ref.Object.SHA

	// 2. Get base tree SHA from the commit.
	commitData, err := ghAPICall(ghPath, "GET", fmt.Sprintf("repos/%s/git/commits/%s", forkRepo, headSHA), "")
	if err != nil {
		return "", fmt.Errorf("getting base commit: %w", err)
	}
	var commitObj struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	if err := json.Unmarshal(commitData, &commitObj); err != nil {
		return "", fmt.Errorf("parsing base commit: %w", err)
	}
	baseTreeSHA := commitObj.Tree.SHA

	// 3. Create blob with marker file content.
	fmt.Fprintln(stdout, "  Creating marker file...")
	blobBody, _ := json.Marshal(map[string]string{
		"content":  mdBody,
		"encoding": "utf-8",
	})
	blobData, err := ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/git/blobs", forkRepo), string(blobBody))
	if err != nil {
		return "", fmt.Errorf("creating blob: %w", err)
	}
	var blob struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(blobData, &blob); err != nil {
		return "", fmt.Errorf("parsing blob response: %w", err)
	}

	// 4. Create tree with marker file.
	treeBody, _ := json.Marshal(map[string]interface{}{
		"base_tree": baseTreeSHA,
		"tree": []map[string]string{{
			"path": markerPath,
			"mode": "100644",
			"type": "blob",
			"sha":  blob.SHA,
		}},
	})
	treeData, err := ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/git/trees", forkRepo), string(treeBody))
	if err != nil {
		return "", fmt.Errorf("creating tree: %w", err)
	}
	var tree struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(treeData, &tree); err != nil {
		return "", fmt.Errorf("parsing tree response: %w", err)
	}

	// 5. Create commit on fork.
	fmt.Fprintln(stdout, "  Creating commit...")
	newCommitBody, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("wl review: %s", wlBranch),
		"tree":    tree.SHA,
		"parents": []string{headSHA},
	})
	newCommitData, err := ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/git/commits", forkRepo), string(newCommitBody))
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}
	var newCommit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(newCommitData, &newCommit); err != nil {
		return "", fmt.Errorf("parsing commit response: %w", err)
	}

	// 6. Create or update ref on fork.
	fmt.Fprintln(stdout, "  Pushing branch to fork...")
	refBody, _ := json.Marshal(map[string]string{
		"ref": "refs/heads/" + wlBranch,
		"sha": newCommit.SHA,
	})
	_, err = ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/git/refs", forkRepo), string(refBody))
	if err != nil {
		// Ref may already exist — force-update it.
		updateBody, _ := json.Marshal(map[string]interface{}{
			"sha":   newCommit.SHA,
			"force": true,
		})
		_, err = ghAPICall(ghPath, "PATCH", fmt.Sprintf("repos/%s/git/refs/heads/%s", forkRepo, wlBranch), string(updateBody))
		if err != nil {
			return "", fmt.Errorf("creating/updating ref: %w", err)
		}
	}

	// 7. Create cross-fork PR or update existing.
	fmt.Fprintln(stdout, "  Opening PR...")
	head := forkOrg + ":" + wlBranch

	existingURL, existingNumber := findExistingPR(ghPath, upstreamRepo, head)
	if existingNumber != "" {
		// Update existing PR body.
		updateBody, _ := json.Marshal(map[string]string{
			"body": mdBody,
		})
		_, _ = ghAPICall(ghPath, "PATCH", fmt.Sprintf("repos/%s/pulls/%s", upstreamRepo, existingNumber), string(updateBody))
		return existingURL, nil
	}

	prBody, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  mdBody,
		"head":  head,
		"base":  "main",
	})
	prData, err := ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/pulls", upstreamRepo), string(prBody))
	if err != nil {
		return "", fmt.Errorf("creating PR: %w", err)
	}
	var pr struct {
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(prData, &pr); err != nil {
		return "", fmt.Errorf("parsing PR response: %w", err)
	}
	return pr.HTMLURL, nil
}

// findExistingPR checks for an open PR on upstream with the given head ref.
// Returns the PR URL and number, or empty strings if none found.
func findExistingPR(ghPath, upstreamRepo, head string) (url, number string) {
	cmd := exec.Command(ghPath, "pr", "list", "--repo", upstreamRepo, "--head", head, "--state", "open", "--json", "number,url")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", ""
	}
	var prs []struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal(out, &prs); err != nil || len(prs) == 0 {
		return "", ""
	}
	return prs[0].URL, fmt.Sprintf("%d", prs[0].Number)
}

// ghAPICall executes a GitHub API call via the gh CLI.
func ghAPICall(ghPath, method, endpoint, body string) ([]byte, error) {
	args := []string{"api", endpoint}
	if method != "GET" {
		args = append(args, "-X", method)
	}
	if body != "" {
		args = append(args, "--input", "-")
	}
	cmd := exec.Command(ghPath, args...)
	if body != "" {
		cmd.Stdin = strings.NewReader(body)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("gh api %s %s: %w (%s)", method, endpoint, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// extractWantedID extracts the wanted ID from a branch name (wl/<rig>/<id> → <id>).
func extractWantedID(branch string) string {
	parts := strings.SplitN(branch, "/", 3)
	if len(parts) < 3 {
		return branch
	}
	return parts[2]
}

// wantedTitleFromBranch queries the wanted table for the item title.
// Falls back to the branch name if the query fails.
func wantedTitleFromBranch(doltPath, dbDir, branch string) string {
	wantedID := extractWantedID(branch)
	query := fmt.Sprintf(
		"SELECT title FROM wanted AS OF '%s' WHERE id = '%s' LIMIT 1",
		strings.ReplaceAll(branch, "'", "''"),
		strings.ReplaceAll(wantedID, "'", "''"),
	)
	cmd := exec.Command(doltPath, "sql", "-r", "csv", "-q", query)
	cmd.Dir = dbDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return branch
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[1]) == "" {
		return branch
	}
	return strings.TrimSpace(lines[1])
}

// closeGitHubPR finds and closes an open GitHub PR for the given branch.
// Best-effort: failures print warnings but don't block the merge.
func closeGitHubPR(ghPath, upstreamRepo, forkOrg, forkDB, branch string, stdout io.Writer) {
	head := forkOrg + ":" + branch
	prURL, number := findExistingPR(ghPath, upstreamRepo, head)
	if number == "" {
		return
	}

	// Close the PR.
	closeBody, _ := json.Marshal(map[string]string{
		"state": "closed",
	})
	_, err := ghAPICall(ghPath, "PATCH", fmt.Sprintf("repos/%s/pulls/%s", upstreamRepo, number), string(closeBody))
	if err != nil {
		fmt.Fprintf(stdout, "  warning: failed to close PR %s: %v\n", prURL, err)
		return
	}

	// Add a closing comment.
	commentBody, _ := json.Marshal(map[string]string{
		"body": "Merged via `wl merge`.",
	})
	_, _ = ghAPICall(ghPath, "POST", fmt.Sprintf("repos/%s/issues/%s/comments", upstreamRepo, number), string(commentBody))

	// Delete the branch on the fork.
	forkRepo := forkOrg + "/" + forkDB
	_, err = ghAPICall(ghPath, "DELETE", fmt.Sprintf("repos/%s/git/refs/heads/%s", forkRepo, branch), "")
	if err != nil {
		fmt.Fprintf(stdout, "  warning: failed to delete GitHub branch %s: %v\n", branch, err)
	}

	fmt.Fprintf(stdout, "  Closed PR %s\n", prURL)
}
