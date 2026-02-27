package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/backend"
	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
)

// mutationContext wraps branch checkout/return/push logic so all mutation
// commands don't duplicate it. In wild-west mode it's a no-op passthrough;
// in PR mode it checks out a per-item branch and returns to main afterward.
type mutationContext struct {
	cfg      *federation.Config
	db       commons.DB
	wantedID string
	branch   string // computed branch name, empty in wild-west mode
	noPush   bool
	stdout   io.Writer
	location *commons.ItemLocation // detected item location across remotes
}

// newMutationContext creates a mutation context for the given config and wanted ID.
func newMutationContext(cfg *federation.Config, wantedID string, noPush bool, stdout io.Writer) *mutationContext {
	db := backend.NewLocalDB(cfg.LocalDir, cfg.ResolveMode())
	mc := &mutationContext{
		cfg:      cfg,
		db:       db,
		wantedID: wantedID,
		noPush:   noPush,
		stdout:   stdout,
	}
	if cfg.ResolveMode() == federation.ModePR {
		mc.branch = commons.BranchName(cfg.RigHandle, wantedID)
	}
	return mc
}

// BranchName returns the branch name, or "" in wild-west mode.
func (m *mutationContext) BranchName() string {
	return m.branch
}

// Setup prepares the mutation context: checks dolt, syncs upstream, detects
// item location, and (in PR mode) checks out the item branch.
// The returned cleanup function must be deferred to return to main.
func (m *mutationContext) Setup() (cleanup func(), err error) {
	noop := func() {}
	if err := requireDolt(); err != nil {
		return noop, err
	}
	sp := style.StartSpinner(m.stdout, "Syncing with upstream...")
	syncErr := commons.PullUpstream(m.cfg.LocalDir)
	sp.Stop()
	if syncErr != nil {
		fmt.Fprintf(m.stdout, "  warning: upstream sync failed: %v\n", syncErr)
	} else {
		updateSyncTimestamp(m.cfg)
	}

	// Detect item location across remotes (best-effort).
	if m.wantedID != "" {
		loc, _ := commons.DetectItemLocation(m.cfg.LocalDir, m.db, m.wantedID)
		m.location = loc
	}

	if m.branch == "" {
		return noop, nil
	}
	if err := commons.CheckoutBranch(m.cfg.LocalDir, m.branch); err != nil {
		return noop, err
	}
	return func() {
		_ = commons.CheckoutMain(m.cfg.LocalDir)
	}, nil
}

// Push pushes changes to the appropriate remote(s).
// Uses location detection in PR mode to minimize push operations.
// In wild-west mode: PushWithSync (upstream + origin).
// In PR mode with branches: PushBranch (origin only).
// In PR mode on main: uses ResolvePushTarget for location-aware pushing.
func (m *mutationContext) Push() error {
	if m.noPush {
		return nil
	}
	// PR mode with branch — push branch to origin, then refresh any existing PR.
	if m.branch != "" {
		if err := m.db.PushBranch(m.branch, m.stdout); err != nil {
			return err
		}
		m.refreshPR()
		return nil
	}

	// Wild-west mode without location info — existing behavior.
	if m.location == nil || m.cfg.ResolveMode() != federation.ModePR {
		return m.db.PushWithSync(m.stdout)
	}

	// PR mode on main — use location-aware push.
	// Re-read local status after the mutation has been applied.
	if status, found, err := commons.QueryItemStatus(m.db, m.wantedID, ""); err == nil && found {
		m.location.LocalStatus = status
	}
	target := commons.ResolvePushTarget("pr", m.location)

	if target.PushUpstream {
		return m.db.PushWithSync(m.stdout)
	}
	if target.PushOrigin {
		if err := m.db.PushMain(m.stdout); err != nil {
			return err
		}
	}

	m.printHint(target)
	return nil
}

// printHint shows a next-step hint based on the push target.
func (m *mutationContext) printHint(target commons.PushTarget) {
	if target.Hint != "" {
		fmt.Fprintf(m.stdout, "  %s\n", style.Dim.Render(target.Hint))
	}
}
