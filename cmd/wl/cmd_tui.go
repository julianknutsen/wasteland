package main

import (
	"fmt"
	"io"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/julianknutsen/wasteland/internal/tui"
	"github.com/spf13/cobra"
)

func newTUICmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI for browsing the wanted board",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTUI(cmd, stdout, stderr)
		},
	}
	return cmd
}

func runTUI(cmd *cobra.Command, _, stderr io.Writer) error {
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	if err := requireDolt(); err != nil {
		return err
	}

	// Sync before launching the TUI.
	sp := style.StartSpinner(stderr, "Syncing with upstream...")
	err = commons.PullUpstream(cfg.LocalDir)
	sp.Stop()
	if err != nil {
		return fmt.Errorf("pulling upstream: %w", err)
	}

	// PR mode: force-push main to origin so it matches upstream.
	// Only the per-item mutation branches should differ.
	if cfg.ResolveMode() == federation.ModePR {
		if err := commons.PushOriginMain(cfg.LocalDir, io.Discard); err != nil {
			fmt.Fprintf(stderr, "  warning: could not sync origin/main: %v\n", err)
		}
	}

	upstream := cfg.Upstream

	m := tui.New(tui.Config{
		DBDir:     cfg.LocalDir,
		RigHandle: cfg.RigHandle,
		Upstream:  upstream,
		Mode:      cfg.ResolveMode(),
		Signing:   cfg.Signing,
	})

	p := bubbletea.NewProgram(m, bubbletea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
