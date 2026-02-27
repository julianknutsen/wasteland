package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newAcceptCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		quality     int
		reliability int
		severity    string
		skills      string
		message     string
		noPush      bool
	)

	cmd := &cobra.Command{
		Use:   "accept <wanted-id>",
		Short: "Accept a completed wanted item and issue a stamp",
		Long: `Accept a completed wanted item by reviewing the work and issuing a reputation stamp.

The item must be in 'in_review' status. You cannot accept your own completion.

A stamp is created with quality and optional reliability ratings (1-5),
severity (leaf/branch/root), and optional skill tags.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl accept w-abc123 --quality 4
  wl accept w-abc123 --quality 5 --reliability 4 --severity branch
  wl accept w-abc123 --quality 3 --skills "go,federation" --message "solid work"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAccept(cmd, stdout, stderr, args[0], quality, reliability, severity, skills, message, noPush)
		},
	}

	cmd.Flags().IntVar(&quality, "quality", 0, "Quality rating 1-5 (required)")
	cmd.Flags().IntVar(&reliability, "reliability", 0, "Reliability rating 1-5 (defaults to quality)")
	cmd.Flags().StringVar(&severity, "severity", "leaf", "Severity: leaf, branch, root")
	cmd.Flags().StringVar(&skills, "skills", "", "Comma-separated skill tags")
	cmd.Flags().StringVar(&message, "message", "", "Freeform message")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")
	_ = cmd.MarkFlagRequired("quality")
	cmd.ValidArgsFunction = completeWantedIDs("in_review")
	_ = cmd.RegisterFlagCompletionFunc("severity", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"leaf", "branch", "root"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runAccept(cmd *cobra.Command, stdout, _ io.Writer, wantedID string, quality, reliability int, severity, skills, message string, noPush bool) error {
	if reliability == 0 {
		reliability = quality
	}

	if err := validateAcceptInputs(quality, reliability, severity); err != nil {
		return err
	}

	var skillTags []string
	if skills != "" {
		for _, s := range strings.Split(skills, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skillTags = append(skillTags, s)
			}
		}
	}

	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	wantedID, err = resolveWantedArg(wlCfg, wantedID)
	if err != nil {
		return err
	}

	rigHandle := wlCfg.RigHandle

	mc := newMutationContext(wlCfg, wantedID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	stamp, err := acceptCompletion(store, wantedID, rigHandle, quality, reliability, severity, skillTags, message)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Accepted %s\n", style.Bold.Render("✓"), wantedID)
	fmt.Fprintf(stdout, "  Stamp ID: %s\n", stamp.ID)
	fmt.Fprintf(stdout, "  Quality: %d, Reliability: %d\n", stamp.Quality, stamp.Reliability)
	fmt.Fprintf(stdout, "  Severity: %s\n", stamp.Severity)
	if len(stamp.SkillTags) > 0 {
		fmt.Fprintf(stdout, "  Skills: %s\n", strings.Join(stamp.SkillTags, ", "))
	}
	if stamp.Message != "" {
		fmt.Fprintf(stdout, "  Message: %s\n", stamp.Message)
	}
	fmt.Fprintf(stdout, "  Status: completed\n")
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch: %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: stamp issued. View: wl status "+wantedID))

	return nil
}

// validateAcceptInputs validates quality, reliability, and severity values.
func validateAcceptInputs(quality, reliability int, severity string) error {
	if quality < 1 || quality > 5 {
		return fmt.Errorf("invalid quality %d: must be 1-5", quality)
	}
	if reliability < 1 || reliability > 5 {
		return fmt.Errorf("invalid reliability %d: must be 1-5", reliability)
	}
	validSeverities := map[string]bool{
		"leaf": true, "branch": true, "root": true,
	}
	if !validSeverities[severity] {
		return fmt.Errorf("invalid severity %q: must be one of leaf, branch, root", severity)
	}
	return nil
}

// acceptCompletion contains the testable business logic for accepting a completion.
func acceptCompletion(store commons.WLCommonsStore, wantedID, rigHandle string, quality, reliability int, severity string, skillTags []string, message string) (*commons.Stamp, error) {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying wanted item: %w", err)
	}

	if _, err := commons.ValidateTransition(item.Status, commons.TransitionAccept); err != nil {
		return nil, fmt.Errorf("wanted item %s: %w", wantedID, err)
	}

	if item.PostedBy != rigHandle {
		return nil, fmt.Errorf("only the poster can accept (posted by %q)", item.PostedBy)
	}

	completion, err := store.QueryCompletion(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying completion: %w", err)
	}

	if completion.CompletedBy == rigHandle {
		return nil, fmt.Errorf("cannot accept your own completion")
	}

	stampID := commons.GeneratePrefixedID("s", wantedID, rigHandle)
	stamp := &commons.Stamp{
		ID:          stampID,
		Author:      rigHandle,
		Subject:     completion.CompletedBy,
		Quality:     quality,
		Reliability: reliability,
		Severity:    severity,
		ContextID:   completion.ID,
		ContextType: "completion",
		SkillTags:   skillTags,
		Message:     message,
	}

	if err := store.AcceptCompletion(wantedID, completion.ID, rigHandle, stamp); err != nil {
		return nil, fmt.Errorf("accepting completion: %w", err)
	}

	return stamp, nil
}
