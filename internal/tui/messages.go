package tui

import "github.com/julianknutsen/wasteland/internal/commons"

// activeView identifies which view is currently displayed.
type activeView int

const (
	viewBrowse activeView = iota
	viewDetail
)

// navigateMsg requests a view switch.
type navigateMsg struct {
	view     activeView
	wantedID string // non-empty when navigating to detail
}

// browseDataMsg carries browse query results.
type browseDataMsg struct {
	items []commons.WantedSummary
	err   error
}

// detailDataMsg carries detail query results.
type detailDataMsg struct {
	item       *commons.WantedItem
	completion *commons.CompletionRecord
	stamp      *commons.Stamp
	err        error
	branch     string // non-empty when detail was read from a PR branch
}

// errMsg carries an error to display.
type errMsg struct {
	err error
}

// actionRequestMsg is sent by the detail view when the user presses an action key.
type actionRequestMsg struct {
	transition commons.Transition
	label      string // e.g. "Claim w-abc123?"
}

// actionConfirmedMsg is sent when the user confirms an action in wild-west mode.
type actionConfirmedMsg struct {
	transition commons.Transition
}

// actionResultMsg carries the result of an executed mutation.
type actionResultMsg struct {
	err    error
	hint   string         // non-empty in PR mode: "branch wl/rig/w-abc pushed to origin"
	detail *detailDataMsg // non-nil: refreshed detail read from branch before checkout main
}
