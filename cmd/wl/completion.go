package main

import (
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/spf13/cobra"
)

// completeWantedIDs returns a ValidArgsFunction that completes wanted IDs,
// optionally filtered by status (e.g. "open", "claimed", "in_review").
func completeWantedIDs(statusFilter string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		cfg, err := resolveWasteland(cmd)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ids, err := commons.ListWantedIDs(cfg.LocalDir, statusFilter)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeBranchNames completes wl/* branch names.
func completeBranchNames(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := resolveWasteland(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	branches, _ := commons.ListBranches(cfg.LocalDir, "wl/")
	return branches, cobra.ShellCompDirectiveNoFileComp
}
