package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/federation"
)

func newConfigCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set wasteland configuration",
		Long: `View or modify wasteland configuration settings.

Use 'wl config get <key>' to read a setting.
Use 'wl config set <key> <value>' to change a setting.

Supported keys:
  mode          Workflow mode: wild-west (default) or pr
  github-repo   Upstream GitHub repo for PR shells (owner/repo)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newConfigGetCmd(stdout, stderr),
		newConfigSetCmd(stdout, stderr),
	)

	return cmd
}

func newConfigGetCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigGet(cmd, stdout, stderr, args[0])
		},
	}
}

func newConfigSetCmd(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(cmd, stdout, stderr, args[0], args[1])
		},
	}
}

// validConfigKeys lists the keys that can be read/written via wl config.
var validConfigKeys = map[string]bool{
	"mode":        true,
	"github-repo": true,
}

func runConfigGet(cmd *cobra.Command, stdout, _ io.Writer, key string) error {
	if !validConfigKeys[key] {
		return fmt.Errorf("unknown config key %q (supported: mode, github-repo)", key)
	}

	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	switch key {
	case "mode":
		fmt.Fprintln(stdout, cfg.ResolveMode())
	case "github-repo":
		fmt.Fprintln(stdout, cfg.GitHubRepo)
	}
	return nil
}

func runConfigSet(cmd *cobra.Command, stdout, _ io.Writer, key, value string) error {
	if !validConfigKeys[key] {
		return fmt.Errorf("unknown config key %q (supported: mode, github-repo)", key)
	}

	switch key {
	case "mode":
		if err := validateMode(value); err != nil {
			return err
		}
	case "github-repo":
		if err := validateGitHubRepo(value); err != nil {
			return err
		}
	}

	explicit, _ := cmd.Flags().GetString("wasteland")
	store := federation.NewConfigStore()
	cfg, err := federation.ResolveConfig(store, explicit)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	switch key {
	case "mode":
		cfg.Mode = value
	case "github-repo":
		cfg.GitHubRepo = value
	}

	if err := store.Save(cfg); err != nil {
		return fmt.Errorf("saving wasteland config: %w", err)
	}

	fmt.Fprintf(stdout, "%s = %s\n", key, value)
	return nil
}

func validateGitHubRepo(value string) error {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("invalid github-repo %q: expected format \"owner/repo\"", value)
	}
	return nil
}

func validateMode(value string) error {
	switch value {
	case federation.ModeWildWest, federation.ModePR:
		return nil
	default:
		return fmt.Errorf("invalid mode %q: must be %q or %q", value, federation.ModeWildWest, federation.ModePR)
	}
}
