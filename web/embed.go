// Package web provides the embedded frontend assets for the Wasteland web UI.
package web

import "embed"

// Assets holds the built web UI files from the dist/ directory.
// When dist/ is empty (web UI not yet built), the embed will contain
// only the .gitkeep file, and the SPA handler will show a fallback message.
//
//go:embed all:dist
var Assets embed.FS
