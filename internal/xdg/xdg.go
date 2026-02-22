// Package xdg provides XDG Base Directory support for wasteland.
package xdg

import (
	"os"
	"path/filepath"
)

const appName = "wasteland"

// ConfigHome returns the XDG config home directory.
// Uses $XDG_CONFIG_HOME if set, otherwise ~/.config.
func ConfigHome() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DataHome returns the XDG data home directory.
// Uses $XDG_DATA_HOME if set, otherwise ~/.local/share.
func DataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// ConfigDir returns the wasteland config directory: ConfigHome()/wasteland.
func ConfigDir() string {
	return filepath.Join(ConfigHome(), appName)
}

// DataDir returns the wasteland data directory: DataHome()/wasteland.
func DataDir() string {
	return filepath.Join(DataHome(), appName)
}
