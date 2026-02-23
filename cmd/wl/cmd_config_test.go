package main

import (
	"testing"

	"github.com/steveyegge/wasteland/internal/federation"
)

func TestValidateMode_Valid(t *testing.T) {
	for _, mode := range []string{federation.ModeWildWest, federation.ModePR} {
		if err := validateMode(mode); err != nil {
			t.Errorf("validateMode(%q) = %v, want nil", mode, err)
		}
	}
}

func TestValidateMode_Invalid(t *testing.T) {
	for _, mode := range []string{"", "chaos", "merge", "WILD-WEST"} {
		if err := validateMode(mode); err == nil {
			t.Errorf("validateMode(%q) = nil, want error", mode)
		}
	}
}

func TestValidConfigKeys(t *testing.T) {
	if !validConfigKeys["mode"] {
		t.Error("expected 'mode' to be a valid config key")
	}
	if validConfigKeys["nonexistent"] {
		t.Error("'nonexistent' should not be a valid config key")
	}
}
