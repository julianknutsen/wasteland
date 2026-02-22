package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/remote"
	"github.com/steveyegge/wasteland/internal/style"
)

func newJoinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		handle      string
		displayName string
		email       string
		forkOrg     string
		remoteBase  string
		gitRemote   string
	)

	cmd := &cobra.Command{
		Use:   "join <upstream>",
		Short: "Join a wasteland by forking its commons",
		Long: `Join a wasteland community by forking its shared commons database.

This command:
  1. Forks the upstream commons to your org
  2. Clones the fork locally
  3. Registers your rig in the rigs table
  4. Pushes the registration to your fork
  5. Saves wasteland configuration locally

The upstream argument is an org/database path like 'steveyegge/wl-commons'.
You can join multiple wastelands simultaneously.

DoltHub mode (default):
  Requires DOLTHUB_TOKEN and DOLTHUB_ORG (or --fork-org).
  Forks and clones via DoltHub.

Offline file mode (--remote-base):
  Uses file:// dolt remotes. No DoltHub credentials needed.
  Requires --fork-org (or DOLTHUB_ORG).

Git remote mode (--git-remote):
  Uses bare git repos as dolt remotes. No DoltHub credentials needed.
  Requires --fork-org (or DOLTHUB_ORG).

Examples:
  wl join steveyegge/wl-commons
  wl join steveyegge/wl-commons --handle my-rig
  wl join test-org/wl-commons --remote-base /tmp/remotes --fork-org my-fork
  wl join test-org/wl-commons --git-remote /tmp/git-remotes --fork-org my-fork`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runJoin(stdout, stderr, args[0], handle, displayName, email, forkOrg, remoteBase, gitRemote)
		},
	}

	cmd.Flags().StringVar(&handle, "handle", "", "Rig handle for registration (default: fork org)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name for the rig registry")
	cmd.Flags().StringVar(&email, "email", "", "Registration email (default: git config user.email)")
	cmd.Flags().StringVar(&forkOrg, "fork-org", "", "Fork organization (default: DOLTHUB_ORG)")
	cmd.Flags().StringVar(&remoteBase, "remote-base", "", "Base directory for file:// remotes (offline mode)")
	cmd.Flags().StringVar(&gitRemote, "git-remote", "", "Base directory for bare git remotes")
	cmd.MarkFlagsMutuallyExclusive("remote-base", "git-remote")

	return cmd
}

func runJoin(stdout, stderr io.Writer, upstream, handle, displayName, email, forkOrg, remoteBase, gitRemote string) error {
	// Parse upstream path (validate early)
	_, _, err := federation.ParseUpstream(upstream)
	if err != nil {
		return err
	}

	store := federation.NewConfigStore()

	// Fast path: check if already joined to this specific upstream.
	if existing, loadErr := store.Load(upstream); loadErr == nil {
		fmt.Fprintf(stdout, "%s Already joined wasteland: %s\n", style.Bold.Render("⚠"), upstream)
		fmt.Fprintf(stdout, "  Handle: %s\n", existing.RigHandle)
		fmt.Fprintf(stdout, "  Fork: %s/%s\n", existing.ForkOrg, existing.ForkDB)
		fmt.Fprintf(stdout, "  Local: %s\n", existing.LocalDir)
		return nil
	}

	// Resolve fork org: flag > env var
	if forkOrg == "" {
		forkOrg = commons.DoltHubOrg()
	}

	var provider remote.Provider

	switch {
	case remoteBase != "":
		// Offline file mode — file:// dolt remotes, no DoltHub credentials needed.
		if forkOrg == "" {
			return fmt.Errorf("--fork-org is required in offline mode (or set DOLTHUB_ORG)")
		}
		provider = remote.NewFileProvider(remoteBase)

	case gitRemote != "":
		// Git remote mode — bare git repos as dolt remotes, no DoltHub credentials needed.
		if forkOrg == "" {
			return fmt.Errorf("--fork-org is required in git remote mode (or set DOLTHUB_ORG)")
		}
		provider = remote.NewGitProvider(gitRemote)

	default:
		// DoltHub mode — requires token and org.
		token := commons.DoltHubToken()
		if token == "" {
			return fmt.Errorf("DOLTHUB_TOKEN environment variable is required\n\nGet your token from https://www.dolthub.com/settings/tokens")
		}
		if forkOrg == "" {
			return fmt.Errorf("DOLTHUB_ORG environment variable is required\n\nSet this to your DoltHub organization name")
		}
		provider = remote.NewDoltHubProvider(token)
	}

	// Determine handle
	if handle == "" {
		handle = forkOrg
	}

	// Determine display name from flag or git config
	if displayName == "" {
		displayName = gitConfigValue("user.name")
	}

	// Determine email from flag or git config
	if email == "" {
		email = gitConfigValue("user.email")
	}

	wlVersion := "dev"

	svc := federation.NewServiceWith(provider, store)
	svc.OnProgress = func(step string) {
		fmt.Fprintf(stdout, "  %s\n", step)
	}

	dbName := upstream[strings.Index(upstream, "/")+1:]
	fmt.Fprintf(stdout, "Joining wasteland %s (fork to %s/%s)...\n", upstream, forkOrg, dbName)
	cfg, err := svc.Join(upstream, forkOrg, handle, displayName, email, wlVersion)
	if err != nil {
		fmt.Fprintf(stderr, "wl join: %v\n", err)
		return errExit
	}

	fmt.Fprintf(stdout, "\n%s Joined wasteland: %s\n", style.Bold.Render("✓"), upstream)
	fmt.Fprintf(stdout, "  Handle: %s\n", cfg.RigHandle)
	fmt.Fprintf(stdout, "  Fork: %s/%s\n", cfg.ForkOrg, cfg.ForkDB)
	fmt.Fprintf(stdout, "  Local: %s\n", cfg.LocalDir)
	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: wl browse  — browse the wanted board"))
	return nil
}

// gitConfigValue retrieves a value from git config. Returns empty string on error.
func gitConfigValue(key string) string {
	cmd := exec.Command("git", "config", key)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
