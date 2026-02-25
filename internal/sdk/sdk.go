// Package sdk provides a high-level client for the Wasteland wanted board.
//
// It extracts mode-aware mutation orchestration (wild-west vs PR branch workflow)
// from the TUI into a reusable layer that can be consumed by any frontend.
package sdk

import (
	"sync"

	"github.com/julianknutsen/wasteland/internal/commons"
)

// ClientConfig holds the parameters needed to create a Client.
type ClientConfig struct {
	DB        commons.DB // database backend (required)
	RigHandle string     // current rig handle (required)
	Mode      string     // "wild-west" or "pr"
	Signing   bool       // GPG-signed dolt commits
	HopURI    string     // rig's HOP protocol URI

	// Optional callbacks — nil disables the feature.
	CreatePR   func(branch string) (string, error)
	CheckPR    func(branch string) string
	LoadDiff   func(branch string) (string, error)
	SaveConfig func(mode string, signing bool) error
}

// Client provides mode-aware operations against the Wasteland wanted board.
type Client struct {
	db        commons.DB
	rigHandle string
	mode      string
	signing   bool
	hopURI    string
	mu        sync.Mutex // serializes mutations (dolt CLI is single-writer)

	// CreatePR submits a PR for the given branch. Nil disables the feature.
	CreatePR func(branch string) (string, error)
	// CheckPR returns an existing PR URL for the branch, or "".
	CheckPR func(branch string) string
	// LoadDiff returns a diff for the given branch. Nil disables the feature.
	LoadDiff func(branch string) (string, error)
	// SaveConfig persists mode and signing settings. Nil disables the feature.
	SaveConfig func(mode string, signing bool) error
}

// New creates a Client from the given config.
func New(cfg ClientConfig) *Client {
	return &Client{
		db:         cfg.DB,
		rigHandle:  cfg.RigHandle,
		mode:       cfg.Mode,
		signing:    cfg.Signing,
		hopURI:     cfg.HopURI,
		CreatePR:   cfg.CreatePR,
		CheckPR:    cfg.CheckPR,
		LoadDiff:   cfg.LoadDiff,
		SaveConfig: cfg.SaveConfig,
	}
}

// Mode returns the current workflow mode ("wild-west" or "pr").
func (c *Client) Mode() string { return c.mode }

// RigHandle returns the current rig handle.
func (c *Client) RigHandle() string { return c.rigHandle }
