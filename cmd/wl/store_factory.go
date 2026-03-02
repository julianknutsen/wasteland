package main

import (
	"fmt"

	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
)

// openStore creates a WLCommonsStore for the given local database directory.
// Package-level variable to allow test overrides.
var openStore = func(localDir string, signed bool, hopURI string) commons.WLCommonsStore {
	db := backend.NewLocalDB(localDir, "")
	store := commons.NewWLCommons(db)
	store.SetSigning(signed)
	store.SetHopURI(hopURI)
	return store
}

// openDB creates a commons.DB for the given local database directory.
// Package-level variable to allow test overrides.
var openDB = func(localDir string) commons.DB {
	return backend.NewLocalDB(localDir, "")
}

// openStoreFromConfig creates a WLCommonsStore using the resolved backend from config.
// Package-level variable to allow test overrides.
var openStoreFromConfig = func(cfg *federation.Config) (commons.WLCommonsStore, error) {
	db, err := openDBFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	store := commons.NewWLCommons(db)
	store.SetSigning(cfg.Signing)
	store.SetHopURI(cfg.HopURI)
	return store, nil
}

// openDBFromConfig creates a commons.DB using the resolved backend from config.
// Package-level variable to allow test overrides.
var openDBFromConfig = func(cfg *federation.Config) (commons.DB, error) {
	if cfg.ResolveBackend() == federation.BackendLocal {
		return backend.NewLocalDB(cfg.LocalDir, cfg.ResolveMode()), nil
	}
	token := commons.DoltHubToken()
	if token == "" {
		return nil, fmt.Errorf("DOLTHUB_TOKEN required for remote mode\n\nGet your token from https://www.dolthub.com/settings/tokens")
	}
	upOrg, upDB, err := federation.ParseUpstream(cfg.Upstream)
	if err != nil {
		return nil, err
	}
	return backend.NewRemoteDB(token, upOrg, upDB, cfg.ForkOrg, cfg.ForkDB, cfg.ResolveMode()), nil
}
