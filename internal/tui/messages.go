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
}

// errMsg carries an error to display.
type errMsg struct {
	err error
}
