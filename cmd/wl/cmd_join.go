package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/federation"
	"github.com/steveyegge/wasteland/internal/style"
)

func newJoinCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		handle      string
		displayName string
		email       string
	)

	cmd := &cobra.Command{
		Use:   "join <upstream>",
		Short: "Join a wasteland by forking its commons",
		Long: `Join a wasteland community by forking its shared commons database.

This command:
  1. Forks the upstream commons to your DoltHub org
  2. Clones the fork locally
  3. Registers your rig in the rigs table
  4. Pushes the registration to your fork
  5. Saves wasteland configuration locally

The upstream argument is a DoltHub path like 'steveyegge/wl-commons'.

Required environment variables:
  DOLTHUB_TOKEN  - Your DoltHub API token
  DOLTHUB_ORG    - Your DoltHub organization name

Examples:
  wl join steveyegge/wl-commons
  wl join steveyegge/wl-commons --handle my-rig
  wl join steveyegge/wl-commons --display-name "Alice's Workshop"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJoin(stdout, stderr, args[0], handle, displayName, email)
		},
	}

	cmd.Flags().StringVar(&handle, "handle", "", "Rig handle for registration (default: DoltHub org)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name for the rig registry")
	cmd.Flags().StringVar(&email, "email", "", "Registration email (default: git config user.email)")

	return cmd
}

func runJoin(stdout, stderr io.Writer, upstream, handle, displayName, email string) error {
	// Parse upstream path (validate early)
	_, _, err := federation.ParseUpstream(upstream)
	if err != nil {
		return err
	}

	// Require DoltHub credentials
	token := commons.DoltHubToken()
	if token == "" {
		return fmt.Errorf("DOLTHUB_TOKEN environment variable is required\n\nGet your token from https://www.dolthub.com/settings/tokens")
	}

	forkOrg := commons.DoltHubOrg()
	if forkOrg == "" {
		return fmt.Errorf("DOLTHUB_ORG environment variable is required\n\nSet this to your DoltHub organization name")
	}

	// Fast path: check if already joined
	if existing, loadErr := federation.LoadConfig(); loadErr == nil {
		if existing.Upstream == upstream {
			fmt.Fprintf(stdout, "%s Already joined wasteland: %s\n", style.Bold.Render("⚠"), upstream)
			fmt.Fprintf(stdout, "  Handle: %s\n", existing.RigHandle)
			fmt.Fprintf(stdout, "  Fork: %s/%s\n", existing.ForkOrg, existing.ForkDB)
			fmt.Fprintf(stdout, "  Local: %s\n", existing.LocalDir)
			return nil
		}
		return fmt.Errorf("already joined to %s; run wl leave first", existing.Upstream)
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

	svc := federation.NewService()
	svc.OnProgress = func(step string) {
		fmt.Fprintf(stdout, "  %s\n", step)
	}

	fmt.Fprintf(stdout, "Joining wasteland %s (fork to %s/%s)...\n", upstream, forkOrg, upstream[strings.Index(upstream, "/")+1:])
	cfg, err := svc.Join(upstream, forkOrg, token, handle, displayName, email, wlVersion)
	if err != nil {
		return err
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
