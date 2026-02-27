package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/style"
	"github.com/spf13/cobra"
)

func newPostCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		title       string
		description string
		project     string
		itemType    string
		priority    int
		effort      string
		tags        string
		noPush      bool
	)

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post a new wanted item to the commons",
		Long: `Post a new wanted item to the Wasteland commons (shared wanted board).

Creates a wanted item with a unique w-<hash> ID and inserts it into the
fork clone of the commons database. In wild-west mode the commit is
auto-pushed to upstream (canonical) and origin (fork).

Use --no-push to skip pushing (offline work).

Examples:
  wl post --title "Fix auth bug" --project gastown --type bug
  wl post --title "Add federation sync" --type feature --priority 1 --effort large
  wl post --title "Update docs" --tags "docs,federation" --effort small
  wl post --title "Offline item" --no-push`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPost(cmd, stdout, stderr, title, description, project, itemType, priority, effort, tags, noPush)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Title of the wanted item (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Detailed description")
	cmd.Flags().StringVar(&project, "project", "", "Project name (e.g., gastown, beads)")
	cmd.Flags().StringVar(&itemType, "type", "", "Item type: feature, bug, design, rfc, docs")
	cmd.Flags().IntVar(&priority, "priority", 2, "Priority: 0=critical, 1=high, 2=medium, 3=low, 4=backlog")
	cmd.Flags().StringVar(&effort, "effort", "medium", "Effort level: trivial, small, medium, large, epic")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags (e.g., 'go,auth,federation')")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")

	_ = cmd.MarkFlagRequired("title")
	_ = cmd.RegisterFlagCompletionFunc("project", completeProjectNames)
	_ = cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"feature", "bug", "design", "rfc", "docs"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.RegisterFlagCompletionFunc("effort", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"trivial", "small", "medium", "large", "epic"}, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

func runPost(cmd *cobra.Command, stdout, _ io.Writer, title, description, project, itemType string, priority int, effort, tags string, noPush bool) error {
	var tagList []string
	if tags != "" {
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagList = append(tagList, t)
			}
		}
	}

	if err := validatePostInputs(itemType, effort, priority); err != nil {
		return err
	}

	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return hintWrap(err)
	}

	item := &commons.WantedItem{
		ID:          commons.GenerateWantedID(title),
		Title:       title,
		Description: description,
		Project:     project,
		Type:        itemType,
		Priority:    priority,
		Tags:        tagList,
		PostedBy:    wlCfg.RigHandle,
		EffortLevel: effort,
	}

	mc := newMutationContext(wlCfg, item.ID, noPush, stdout)
	cleanup, err := mc.Setup()
	if err != nil {
		return err
	}
	defer cleanup()

	store := openStore(wlCfg.LocalDir, wlCfg.Signing, wlCfg.HopURI)

	if err := postWanted(store, item); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Posted wanted item: %s\n", style.Bold.Render("✓"), style.Bold.Render(item.ID))
	fmt.Fprintf(stdout, "  Title:    %s\n", item.Title)
	if item.Project != "" {
		fmt.Fprintf(stdout, "  Project:  %s\n", item.Project)
	}
	if item.Type != "" {
		fmt.Fprintf(stdout, "  Type:     %s\n", item.Type)
	}
	fmt.Fprintf(stdout, "  Priority: %d\n", item.Priority)
	fmt.Fprintf(stdout, "  Effort:   %s\n", item.EffortLevel)
	if len(item.Tags) > 0 {
		fmt.Fprintf(stdout, "  Tags:     %s\n", strings.Join(item.Tags, ", "))
	}
	fmt.Fprintf(stdout, "  Posted by: %s\n", item.PostedBy)
	if mc.BranchName() != "" {
		fmt.Fprintf(stdout, "  Branch:   %s\n", mc.BranchName())
	}

	if err := mc.Push(); err != nil {
		fmt.Fprintf(stdout, "\n  %s %s\n", style.Warning.Render(style.IconWarn),
			"Push failed — changes saved locally. Run 'wl sync' to retry.")
	}

	fmt.Fprintf(stdout, "\n  %s\n", style.Dim.Render("Next: others can claim this. Browse: wl browse"))

	return nil
}

// validatePostInputs validates the type, effort, and priority fields.
func validatePostInputs(itemType, effort string, priority int) error {
	validTypes := map[string]bool{
		"feature": true, "bug": true, "design": true, "rfc": true, "docs": true,
	}
	if itemType != "" && !validTypes[itemType] {
		return fmt.Errorf("invalid type %q: must be one of feature, bug, design, rfc, docs", itemType)
	}

	validEfforts := map[string]bool{
		"trivial": true, "small": true, "medium": true, "large": true, "epic": true,
	}
	if !validEfforts[effort] {
		return fmt.Errorf("invalid effort %q: must be one of trivial, small, medium, large, epic", effort)
	}

	if priority < 0 || priority > 4 {
		return fmt.Errorf("invalid priority %d: must be 0-4", priority)
	}

	return nil
}

// postWanted contains the testable business logic for posting a wanted item.
func postWanted(store commons.WLCommonsStore, item *commons.WantedItem) error {
	if err := store.InsertWanted(item); err != nil {
		return fmt.Errorf("posting wanted item: %w", err)
	}

	return nil
}
