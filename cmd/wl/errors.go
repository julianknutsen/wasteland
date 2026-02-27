package main

import (
	"errors"
	"fmt"

	"github.com/julianknutsen/wasteland/internal/federation"
)

// HintedError wraps an error with a user-facing recovery hint.
type HintedError struct {
	Err  error
	Hint string
}

func (h *HintedError) Error() string { return h.Err.Error() }
func (h *HintedError) Unwrap() error { return h.Err }

// hintWrap wraps a config-loading error with an appropriate recovery hint.
func hintWrap(err error) error {
	if err == nil {
		return nil
	}
	var hint string
	switch {
	case errors.Is(err, federation.ErrNotJoined):
		hint = "Run 'wl join <org/db>' to join a wasteland, or 'wl create <org/db>' to start one."
	case errors.Is(err, federation.ErrAmbiguous):
		hint = "Use --wasteland <org/db> to select which wasteland."
	default:
		hint = "Run 'wl doctor' to check your setup."
	}
	return &HintedError{Err: fmt.Errorf("loading wasteland config: %w", err), Hint: hint}
}
