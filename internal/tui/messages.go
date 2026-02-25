package tui

import "github.com/julianknutsen/wasteland/internal/commons"

// activeView identifies which view is currently displayed.
type activeView int

const (
	viewBrowse activeView = iota
	viewDetail
	viewMe
	viewSettings
)

// navigateMsg requests a view switch.
type navigateMsg struct {
	view     activeView
	wantedID string // non-empty when navigating to detail
}

// browseDataMsg carries browse query results.
type browseDataMsg struct {
	items     []commons.WantedSummary
	branchIDs map[string]bool // wanted IDs with active branches
	err       error
}

// detailDataMsg carries detail query results.
type detailDataMsg struct {
	item       *commons.WantedItem
	completion *commons.CompletionRecord
	stamp      *commons.Stamp
	err        error
	branch     string // non-empty when detail was read from a PR branch
	mainStatus string // status on main when detail was read from a branch
}

// meDataMsg carries dashboard query results.
type meDataMsg struct {
	data *commons.DashboardData
	err  error
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

// branchDeltaAction identifies a delta resolution action.
type branchDeltaAction int

const (
	deltaApply   branchDeltaAction = iota // merge branch into main
	deltaDiscard                          // abandon branch, revert to main
)

// deltaRequestMsg is sent when the user presses apply or discard.
type deltaRequestMsg struct {
	action branchDeltaAction
	label  string // computed confirmation text
}

// deltaConfirmedMsg is sent when the user confirms a delta action.
type deltaConfirmedMsg struct {
	action branchDeltaAction
}

// deltaResultMsg carries the result of a delta resolution.
type deltaResultMsg struct {
	err  error
	hint string
}

// settingsSavedMsg carries the result of saving settings.
type settingsSavedMsg struct {
	mode    string
	signing bool
	err     error
}
