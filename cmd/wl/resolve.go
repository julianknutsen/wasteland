package main

import (
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
)

// resolveWantedArg resolves a wanted ID or prefix to a full ID using the local database.
// Package-level variable to allow test overrides.
var resolveWantedArg = func(cfg *federation.Config, idOrPrefix string) (string, error) {
	return commons.ResolveWantedID(cfg.LocalDir, idOrPrefix)
}
