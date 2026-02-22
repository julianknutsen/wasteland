package xdg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigHome_Default(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	got := ConfigHome()
	if !strings.HasSuffix(got, ".config") {
		t.Errorf("ConfigHome() = %q, want suffix %q", got, ".config")
	}
}

func TestConfigHome_Env(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")
	got := ConfigHome()
	if got != "/custom/config" {
		t.Errorf("ConfigHome() = %q, want %q", got, "/custom/config")
	}
}

func TestDataHome_Default(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	got := DataHome()
	if !strings.HasSuffix(got, filepath.Join(".local", "share")) {
		t.Errorf("DataHome() = %q, want suffix %q", got, filepath.Join(".local", "share"))
	}
}

func TestDataHome_Env(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/custom/data")
	got := DataHome()
	if got != "/custom/data" {
		t.Errorf("DataHome() = %q, want %q", got, "/custom/data")
	}
}

func TestConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	got := ConfigDir()
	want := "/tmp/xdg-test/wasteland"
	if got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestDataDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-test")
	got := DataDir()
	want := "/tmp/xdg-test/wasteland"
	if got != want {
		t.Errorf("DataDir() = %q, want %q", got, want)
	}
}

func TestConfigHome_UsesHomeDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := ConfigHome()
	want := filepath.Join(home, ".config")
	if got != want {
		t.Errorf("ConfigHome() = %q, want %q", got, want)
	}
}
