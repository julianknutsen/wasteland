package main

import (
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
)

// resolveWantedArg resolves a wanted ID or prefix to a full ID using the config-aware database.
// In PR mode, items may only exist on per-item branches (not on main), so we
// fall back to querying the item's branch when the main working copy has no match.
// Package-level variable to allow test overrides.
var resolveWantedArg = func(cfg *federation.Config, idOrPrefix string) (string, error) {
	db, err := openDBFromConfig(cfg)
	if err != nil {
		return "", err
	}
	id, err := commons.ResolveWantedID(db, idOrPrefix)
	if err == nil {
		return id, nil
	}
	// In PR mode, try the item's branch (wl/<rigHandle>/<id>).
	if cfg.ResolveMode() == federation.ModePR && cfg.RigHandle != "" {
		branch := commons.BranchName(cfg.RigHandle, idOrPrefix)
		if _, found, _ := commons.QueryItemStatus(db, idOrPrefix, branch); found {
			return idOrPrefix, nil
		}
	}
	return "", err
}
