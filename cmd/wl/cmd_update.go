package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/steveyegge/wasteland/internal/commons"
	"github.com/steveyegge/wasteland/internal/style"
)

func newUpdateCmd(stdout, stderr io.Writer) *cobra.Command {
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
		Use:   "update <wanted-id>",
		Short: "Update fields on an open wanted item",
		Long: `Update mutable fields on an open wanted item.

Only items with status 'open' can be updated — once claimed, the contract is locked.

At least one field must be provided. In wild-west mode any joined rig can update.

In wild-west mode the commit is auto-pushed to upstream and origin.
Use --no-push to skip pushing (offline work).

Examples:
  wl update w-abc123 --title "New title"
  wl update w-abc123 --priority 1 --effort large
  wl update w-abc123 --type bug --tags "go,auth"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, stdout, stderr, args[0], title, description, project, itemType, priority, effort, tags, noPush)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New description")
	cmd.Flags().StringVar(&project, "project", "", "New project")
	cmd.Flags().StringVar(&itemType, "type", "", "Item type: feature, bug, design, rfc, docs")
	cmd.Flags().IntVar(&priority, "priority", -1, "Priority: 0=critical, 1=high, 2=medium, 3=low, 4=backlog")
	cmd.Flags().StringVar(&effort, "effort", "", "Effort level: trivial, small, medium, large, epic")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags (replaces existing)")
	cmd.Flags().BoolVar(&noPush, "no-push", false, "Skip pushing to remotes (offline work)")

	return cmd
}

func runUpdate(cmd *cobra.Command, stdout, _ io.Writer, wantedID, title, description, project, itemType string, priority int, effort, tags string, noPush bool) error {
	fields := make(map[string]string)

	if title != "" {
		fields["title"] = fmt.Sprintf("'%s'", commons.EscapeSQL(title))
	}
	if description != "" {
		fields["description"] = fmt.Sprintf("'%s'", commons.EscapeSQL(description))
	}
	if project != "" {
		fields["project"] = fmt.Sprintf("'%s'", commons.EscapeSQL(project))
	}
	if itemType != "" {
		fields["type"] = fmt.Sprintf("'%s'", commons.EscapeSQL(itemType))
	}
	if priority >= 0 {
		fields["priority"] = fmt.Sprintf("%d", priority)
	}
	if effort != "" {
		fields["effort_level"] = fmt.Sprintf("'%s'", commons.EscapeSQL(effort))
	}
	if tags != "" {
		var tagList []string
		for _, t := range strings.Split(tags, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagList = append(tagList, t)
			}
		}
		if len(tagList) > 0 {
			escaped := make([]string, len(tagList))
			for i, t := range tagList {
				t = strings.ReplaceAll(t, `\`, `\\`)
				t = strings.ReplaceAll(t, `"`, `\"`)
				t = strings.ReplaceAll(t, "'", "''")
				escaped[i] = t
			}
			fields["tags"] = fmt.Sprintf("'[\"%s\"]'", strings.Join(escaped, `","`))
		}
	}

	if len(fields) == 0 {
		return fmt.Errorf("at least one field must be provided to update")
	}

	if err := validateUpdateInputs(itemType, effort, priority); err != nil {
		return err
	}

	wlCfg, err := resolveWasteland(cmd)
	if err != nil {
		return fmt.Errorf("loading wasteland config: %w", err)
	}

	store := commons.NewWLCommons(wlCfg.LocalDir)

	if err := updateWanted(store, wantedID, fields); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s Updated %s\n", style.Bold.Render("✓"), wantedID)
	for col := range fields {
		fmt.Fprintf(stdout, "  %s: updated\n", col)
	}

	if !noPush {
		_ = commons.PushWithSync(wlCfg.LocalDir, stdout)
	}

	return nil
}

// validateUpdateInputs validates type, effort, and priority if provided.
func validateUpdateInputs(itemType, effort string, priority int) error {
	validTypes := map[string]bool{
		"feature": true, "bug": true, "design": true, "rfc": true, "docs": true,
	}
	if itemType != "" && !validTypes[itemType] {
		return fmt.Errorf("invalid type %q: must be one of feature, bug, design, rfc, docs", itemType)
	}

	validEfforts := map[string]bool{
		"trivial": true, "small": true, "medium": true, "large": true, "epic": true,
	}
	if effort != "" && !validEfforts[effort] {
		return fmt.Errorf("invalid effort %q: must be one of trivial, small, medium, large, epic", effort)
	}

	if priority >= 0 && (priority > 4) {
		return fmt.Errorf("invalid priority %d: must be 0-4", priority)
	}

	return nil
}

// updateWanted contains the testable business logic for updating a wanted item.
func updateWanted(store commons.WLCommonsStore, wantedID string, fields map[string]string) error {
	item, err := store.QueryWanted(wantedID)
	if err != nil {
		return fmt.Errorf("querying wanted item: %w", err)
	}

	if item.Status != "open" {
		return fmt.Errorf("wanted item %s is not open (status: %s)", wantedID, item.Status)
	}

	if err := store.UpdateWanted(wantedID, fields); err != nil {
		return fmt.Errorf("updating wanted item: %w", err)
	}

	return nil
}
