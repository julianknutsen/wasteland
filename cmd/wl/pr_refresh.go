package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/remote"
)

// refreshPR updates the description of an existing PR after a successful push.
// Best-effort: failures print a warning but don't block the mutation.
func (m *mutationContext) refreshPR() {
	if m.branch == "" {
		return
	}

	switch m.cfg.ResolveProviderType() {
	case "github":
		m.refreshGitHubPR()
	case "dolthub":
		m.refreshDoltHubPR()
	}
}

func (m *mutationContext) refreshGitHubPR() {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		return
	}

	client := newGHClient(ghPath)
	head := m.cfg.ForkOrg + ":" + m.branch
	_, number := client.FindPR(m.cfg.Upstream, head)
	if number == "" {
		return
	}

	body, err := m.generateDiffMarkdown()
	if err != nil {
		return
	}

	if err := client.UpdatePR(m.cfg.Upstream, number, map[string]string{"body": body}); err != nil {
		fmt.Fprintf(m.stdout, "  warning: could not update PR description: %v\n", err)
		return
	}
	fmt.Fprintf(m.stdout, "  Updated PR description\n")
}

func (m *mutationContext) refreshDoltHubPR() {
	token := os.Getenv("DOLTHUB_TOKEN")
	if token == "" {
		return
	}

	upstreamOrg, db, err := federation.ParseUpstream(m.cfg.Upstream)
	if err != nil {
		return
	}

	provider := remote.NewDoltHubProvider(token)
	_, prID := provider.FindPR(upstreamOrg, db, m.cfg.ForkOrg, m.branch)
	if prID == "" {
		return
	}

	body, err := m.generateDiffMarkdown()
	if err != nil {
		return
	}

	doltPath, _ := exec.LookPath("dolt")
	title := wantedTitleFromBranch(doltPath, m.cfg.LocalDir, m.branch)
	prTitle := fmt.Sprintf("[wl] %s", title)

	if err := provider.UpdatePR(upstreamOrg, db, prID, prTitle, body); err != nil {
		fmt.Fprintf(m.stdout, "  warning: could not update PR description: %v\n", err)
		return
	}
	fmt.Fprintf(m.stdout, "  Updated PR description\n")
}

// generateDiffMarkdown renders a markdown diff between the branch and its base.
// Uses three-dot diff (base...branch) which compares refs directly and does
// not depend on the current checkout.
func (m *mutationContext) generateDiffMarkdown() (string, error) {
	doltPath, err := exec.LookPath("dolt")
	if err != nil {
		return "", err
	}

	base := diffBase(m.cfg.LocalDir, doltPath)

	var buf bytes.Buffer
	if err := renderMarkdownDiff(&buf, m.cfg.LocalDir, doltPath, m.branch, base); err != nil {
		return "", err
	}
	return buf.String(), nil
}
