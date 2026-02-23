package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

func newVerifyCmd(stdout, stderr io.Writer) *cobra.Command {
	var last int

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Show GPG signature status of recent Dolt commits",
		Long: `Show GPG signature verification for recent commits in the local
commons clone. Runs 'dolt log --show-signature' under the hood.

Use --last to control how many commits to inspect (default 5).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVerify(cmd, stdout, stderr, last)
		},
	}

	cmd.Flags().IntVar(&last, "last", 5, "Number of recent commits to verify")

	return cmd
}

func runVerify(cmd *cobra.Command, stdout, stderr io.Writer, last int) error {
	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	args := []string{"log", "--show-signature", "-n", strconv.Itoa(last)}
	dolt := exec.Command("dolt", args...)
	dolt.Dir = wlCfg.LocalDir
	dolt.Stdout = stdout
	dolt.Stderr = stderr
	dolt.Stdin = os.Stdin

	if err := dolt.Run(); err != nil {
		return fmt.Errorf("dolt log --show-signature: %w", err)
	}
	return nil
}
