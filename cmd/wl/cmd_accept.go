package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/style"
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
		return fmt.Errorf("loading wasteland config: %w", err)
	}
	rigHandle := wlCfg.RigHandle

	store := commons.NewWLCommons(wlCfg.LocalDir)

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

	if !noPush {
		_ = commons.PushWithSync(wlCfg.LocalDir, stdout)
	}

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

	if item.Status != "in_review" {
		return nil, fmt.Errorf("wanted item %s is not in_review (status: %s)", wantedID, item.Status)
	}

	completion, err := store.QueryCompletion(wantedID)
	if err != nil {
		return nil, fmt.Errorf("querying completion: %w", err)
	}

	if completion.CompletedBy == rigHandle {
		return nil, fmt.Errorf("cannot accept your own completion")
	}

	stampID := generateStampID(wantedID, rigHandle)
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

func generateStampID(wantedID, rigHandle string) string {
	now := time.Now().UTC().Format(time.RFC3339)
	h := sha256.Sum256([]byte(wantedID + "|" + rigHandle + "|" + now))
	return fmt.Sprintf("s-%x", h[:8])
}
