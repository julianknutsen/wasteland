package main

import (
	"fmt"
	"io"

	"github.com/julianknutsen/wasteland/internal/commons"
	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
)

// mutationContext wraps branch checkout/return/push logic so all mutation
// commands don't duplicate it. In wild-west mode it's a no-op passthrough;
// in PR mode it checks out a per-item branch and returns to main afterward.
type mutationContext struct {
	cfg      *federation.Config
	wantedID string
	branch   string // computed branch name, empty in wild-west mode
	noPush   bool
	stdout   io.Writer
}

// newMutationContext creates a mutation context for the given config and wanted ID.
func newMutationContext(cfg *federation.Config, wantedID string, noPush bool, stdout io.Writer) *mutationContext {
	mc := &mutationContext{
		cfg:      cfg,
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

// Setup prepares the mutation context: checks dolt, syncs upstream, and
// (in PR mode) checks out the item branch.
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
// In wild-west mode: PushWithSync (upstream + origin).
// In PR mode: PushBranch (origin only).
func (m *mutationContext) Push() error {
	if m.noPush {
		return nil
	}
	if m.branch != "" {
		return commons.PushBranch(m.cfg.LocalDir, m.branch, m.stdout)
	}
	return commons.PushWithSync(m.cfg.LocalDir, m.stdout)
}
