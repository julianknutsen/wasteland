package main

import "github.com/steveyegge/wasteland/internal/commons"

// openStore creates a WLCommonsStore for the given local database directory.
// Package-level variable to allow test overrides.
var openStore = func(localDir string, signed bool, hopURI string) commons.WLCommonsStore {
	store := commons.NewWLCommons(localDir)
	store.SetSigning(signed)
	store.SetHopURI(hopURI)
	return store
}
