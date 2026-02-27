package main

import (
	"fmt"
	"io"
	"time"

	"github.com/julianknutsen/wasteland/internal/federation"
	"github.com/julianknutsen/wasteland/internal/style"
)

// updateSyncTimestamp records a successful sync in the config.
func updateSyncTimestamp(cfg *federation.Config) {
	now := time.Now()
	cfg.LastSyncAt = &now
	store := federation.NewConfigStore()
	_ = store.Save(cfg) // best-effort; errors here are non-fatal
}

// warnIfStale prints a warning if the last sync was more than 1 hour ago.
func warnIfStale(w io.Writer, cfg *federation.Config) {
	if cfg.LastSyncAt == nil {
		return // never synced — don't nag on first use
	}
	age := time.Since(*cfg.LastSyncAt)
	if age < time.Hour {
		return
	}
	fmt.Fprintf(w, "\n  %s Last synced %s ago. Run 'wl sync' for latest data.\n",
		style.Warning.Render(style.IconWarn), formatDuration(age))
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
