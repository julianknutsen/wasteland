package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/remote"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/julianknutsen/wasteland/schema"
	"github.com/spf13/cobra"
)

func newCreateCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		handle      string
		displayName string
		email       string
		name        string
		remoteBase  string
		gitRemote   string
		github      bool
		githubLocal string
		localOnly   bool
		signed      bool
	)

	cmd := &cobra.Command{
		Use:   "create <org/db-name>",
		Short: "Create a new wasteland commons database",
		Long: `Create a new wasteland commons database initialized with the standard schema.

This command:
  1. Initializes a new dolt database with the commons schema
  2. Registers the creator as a rig
  3. Commits the initial schema
  4. Pushes to remote (unless --local-only)
  5. Saves wasteland configuration locally

Examples:
  wl create myorg/wl-commons                       # create and push to DoltHub
  wl create myorg/wl-commons --name "My Wasteland"  # custom display name
  wl create myorg/wl-commons --local-only            # skip push
  wl create myorg/wl-commons --signed                # GPG-sign initial commit`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runCreate(stdout, stderr, args[0], name, handle, displayName, email,
				remoteBase, gitRemote, github, githubLocal, localOnly, signed)
		},
	}

	cmd.Flags().StringVar(&handle, "handle", "", "Rig handle for registration (default: org)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name for the rig registry")
	cmd.Flags().StringVar(&email, "email", "", "Registration email (default: GPG key email if --signed, else git config user.email)")
	cmd.Flags().StringVar(&name, "name", "", "Display name for the wasteland (stored in _meta)")
	cmd.Flags().StringVar(&remoteBase, "remote-base", "", "Base directory for file:// remotes (offline mode)")
	cmd.Flags().StringVar(&gitRemote, "git-remote", "", "Base directory for bare git remotes")
	cmd.Flags().BoolVar(&github, "github", false, "Use GitHub as the upstream provider")
	cmd.Flags().StringVar(&githubLocal, "github-local", "", "Local base directory for GitHub-compatible testing mode")
	cmd.Flags().BoolVar(&localOnly, "local-only", false, "Skip pushing to remote")
	cmd.Flags().BoolVar(&signed, "signed", false, "GPG-sign the initial commit")
	cmd.MarkFlagsMutuallyExclusive("remote-base", "git-remote", "github", "github-local")

	return cmd
}

func runCreate(stdout, stderr io.Writer, upstream, name, handle, displayName, email,
	remoteBase, gitRemote string, github bool, githubLocal string, localOnly, signed bool,
) error {
	if err := requireDolt(); err != nil {
		return err
	}

	org, db, err := federation.ParseUpstream(upstream)
	if err != nil {
		return err
	}

	localDir := federation.LocalCloneDir(org, db)

	// Check if .dolt already exists for a clear error message.
	if _, err := os.Stat(filepath.Join(localDir, ".dolt")); err == nil {
		return fmt.Errorf("database already exists at %s", localDir)
	}

	store := federation.NewConfigStore()

	// Resolve provider.
	var provider remote.Provider
	switch {
	case localOnly:
		// Local-only mode doesn't need a real provider, but Service needs one for the interface.
		provider = remote.NewDoltHubProvider("")

	case remoteBase != "":
		provider = remote.NewFileProvider(remoteBase)

	case gitRemote != "":
		provider = remote.NewGitProvider(gitRemote)

	case github:
		provider = remote.NewGitHubProvider()

	case githubLocal != "":
		provider = remote.NewFakeGitHubProvider(githubLocal)

	default:
		provider = remote.NewDoltHubProvider("")
	}

	// Resolve handle — defaults to org.
	if handle == "" {
		handle = org
	}

	// Resolve display name from flag or git config.
	if displayName == "" {
		displayName = gitConfigValue("user.name")
	}

	// Resolve email from flag, GPG key (if signed), or git config.
	if email == "" && signed {
		email = gpgKeyEmail()
	}
	if email == "" {
		email = gitConfigValue("user.email")
	}

	svc := federation.NewServiceWith(provider, store)
	svc.OnProgress = func(step string) {
		fmt.Fprintf(stdout, "  %s\n", step)
	}

	fmt.Fprintf(stdout, "Creating wasteland %s...\n", upstream)

	result, err := svc.Create(federation.CreateOptions{
		Upstream:    upstream,
		Handle:      handle,
		DisplayName: displayName,
		OwnerEmail:  email,
		Version:     "dev",
		SchemaSQL:   schema.SQL,
		Name:        name,
		LocalOnly:   localOnly,
		Signed:      signed,
	})
	if err != nil {
		fmt.Fprintf(stderr, "wl create: %v\n", err)
		return errExit
	}

	cfg := result.Config
	fmt.Fprintf(stdout, "\n%s Created wasteland: %s\n", style.Bold.Render("✓"), upstream)
	fmt.Fprintf(stdout, "  Handle: %s\n", cfg.RigHandle)
	fmt.Fprintf(stdout, "  Local: %s\n", cfg.LocalDir)
	if !localOnly {
		fmt.Fprintf(stdout, "  Pushed to %s/%s\n", org, db)
	}
	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Share: wl join "+upstream))

	return nil
}
