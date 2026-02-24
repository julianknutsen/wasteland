package main

import (
	"errors"
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

const defaultUpstream = "hop/wl-commons"

func newJoinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		handle      string
		displayName string
		email       string
		forkOrg     string
		remoteBase  string
		gitRemote   string
		github      bool
		githubLocal string
		signed      bool
		direct      bool
	)

	cmd := &cobra.Command{
		Use:   "join [upstream]",
		Short: "Join a wasteland by forking its commons",
		Long: `Join a wasteland community by forking its shared commons database.

This command:
  1. Forks the upstream commons to your org (or checks that your fork exists)
  2. Clones the fork locally
  3. Registers your rig in the rigs table
  4. Pushes the registration to your fork
  5. Saves wasteland configuration locally

The upstream argument defaults to 'hop/wl-commons' (the main wasteland).
You can specify a different org/database path to join other wastelands.

Getting started:
  1. Sign up at https://www.dolthub.com
  2. Fork the commons: https://www.dolthub.com/repositories/hop/wl-commons
  3. Create an API token at https://www.dolthub.com/settings/tokens
  4. Set environment variables:
       export DOLTHUB_TOKEN=<your-api-token>
       export DOLTHUB_ORG=<your-dolthub-username>
  5. Run: wl join

Examples:
  wl join
  wl join hop/wl-commons --handle my-rig`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			upstream := defaultUpstream
			if len(args) > 0 {
				upstream = args[0]
			}
			return runJoin(stdout, stderr, upstream, handle, displayName, email, forkOrg, remoteBase, gitRemote, github, githubLocal, signed, direct)
		},
	}

	cmd.Flags().StringVar(&handle, "handle", "", "Rig handle for registration (default: fork org)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name for the rig registry")
	cmd.Flags().StringVar(&email, "email", "", "Registration email (default: GPG key email if --signed, else git config user.email)")
	cmd.Flags().StringVar(&forkOrg, "fork-org", "", "Fork organization (default: DOLTHUB_ORG)")
	cmd.Flags().StringVar(&remoteBase, "remote-base", "", "Base directory for file:// remotes (offline mode)")
	cmd.Flags().StringVar(&gitRemote, "git-remote", "", "Base directory for bare git remotes")
	cmd.Flags().BoolVar(&github, "github", false, "Use GitHub as the upstream provider")
	cmd.Flags().StringVar(&githubLocal, "github-local", "", "Local base directory for GitHub-compatible testing mode")
	cmd.Flags().BoolVar(&signed, "signed", false, "GPG-sign the rig registration commit")
	cmd.Flags().BoolVar(&direct, "direct", false, "Skip forking — clone and push to upstream directly (for maintainers)")
	cmd.MarkFlagsMutuallyExclusive("remote-base", "git-remote", "github", "github-local")

	return cmd
}

func runJoin(stdout, stderr io.Writer, upstream, handle, displayName, email, forkOrg, remoteBase, gitRemote string, github bool, githubLocal string, signed, direct bool) error {
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

	case github:
		// GitHub mode — uses gh CLI for forking, GitHub HTTPS URLs as dolt remotes.
		if forkOrg == "" {
			return fmt.Errorf("--fork-org is required in GitHub mode (or set DOLTHUB_ORG)")
		}
		provider = remote.NewGitHubProvider()

	case githubLocal != "":
		// GitHub-local mode — bare git repos that report type "github" for testing.
		if forkOrg == "" {
			return fmt.Errorf("--fork-org is required in GitHub-local mode (or set DOLTHUB_ORG)")
		}
		provider = remote.NewFakeGitHubProvider(githubLocal)

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

	// Determine email from flag, GPG key (if signed), or git config
	if email == "" && signed {
		email = gpgKeyEmail()
	}
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
	result, err := svc.Join(upstream, forkOrg, handle, displayName, email, wlVersion, signed, direct)
	if err != nil {
		var forkErr *remote.ForkRequiredError
		if errors.As(err, &forkErr) {
			printForkInstructions(stdout, forkErr)
			return errExit
		}
		fmt.Fprintf(stderr, "wl join: %v\n", err)
		return errExit
	}

	cfg := result.Config
	fmt.Fprintf(stdout, "\n%s Joined wasteland: %s\n", style.Bold.Render("✓"), upstream)
	fmt.Fprintf(stdout, "  Handle: %s\n", cfg.RigHandle)
	fmt.Fprintf(stdout, "  Fork: %s/%s\n", cfg.ForkOrg, cfg.ForkDB)
	fmt.Fprintf(stdout, "  Local: %s\n", cfg.LocalDir)
	if result.PRURL != "" {
		fmt.Fprintf(stdout, "  PR: %s\n", style.Bold.Render(result.PRURL))
	}
	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: wl browse  — browse the wanted board"))
	return nil
}

func printForkInstructions(w io.Writer, err *remote.ForkRequiredError) {
	fmt.Fprintf(w, "\n%s Fork required\n\n", style.Bold.Render("!"))
	fmt.Fprintf(w, "  To join this wasteland, fork the commons on DoltHub:\n\n")
	fmt.Fprintf(w, "  1. Go to %s\n", style.Bold.Render(err.ForkURL()))
	fmt.Fprintf(w, "  2. Click %s (top right)\n", style.Bold.Render("Fork"))
	fmt.Fprintf(w, "  3. Select your organization: %s\n", style.Bold.Render(err.ForkOrg))
	fmt.Fprintf(w, "  4. Rerun: %s\n", style.Bold.Render("wl join"))
}

// gpgKeyEmail extracts the email from the first GPG secret key's uid.
// Returns empty string if GPG is not available or no keys are found.
func gpgKeyEmail() string {
	cmd := exec.Command("gpg", "--list-secret-keys", "--with-colons")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "uid:") {
			// Colon-delimited format: uid:...:...:...:...:...:...:...:...:Name <email>:...
			fields := strings.Split(line, ":")
			if len(fields) > 9 {
				uid := fields[9]
				// Extract email from "Name <email>" format
				if start := strings.Index(uid, "<"); start >= 0 {
					if end := strings.Index(uid[start:], ">"); end >= 0 {
						return uid[start+1 : start+end]
					}
				}
			}
		}
	}
	return ""
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
