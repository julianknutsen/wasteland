package main

import "github.com/steveyegge/wasteland/internal/commons"

// openStore creates a WLCommonsStore for the given local database directory.
// Package-level variable to allow test overrides.
var openStore = func(localDir string) commons.WLCommonsStore {
	return commons.NewWLCommons(localDir)
}
