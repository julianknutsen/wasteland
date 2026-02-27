package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/julianknutsen/wasteland/internal/federation"
)

func TestHintedError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("something failed")
	h := &HintedError{Err: inner, Hint: "try again"}
	if !errors.Is(h, inner) {
		t.Error("HintedError should unwrap to inner error")
	}
}

func TestHintedError_ErrorString(t *testing.T) {
	inner := fmt.Errorf("boom")
	h := &HintedError{Err: inner, Hint: "fix it"}
	if h.Error() != "boom" {
		t.Errorf("Error() = %q, want %q", h.Error(), "boom")
	}
}

func TestHintWrap_Nil(t *testing.T) {
	if got := hintWrap(nil); got != nil {
		t.Errorf("hintWrap(nil) = %v, want nil", got)
	}
}

func TestHintWrap_NotJoined(t *testing.T) {
	err := hintWrap(federation.ErrNotJoined)
	var h *HintedError
	if !errors.As(err, &h) {
		t.Fatal("expected HintedError")
	}
	if h.Hint != "Run 'wl join <org/db>' to join a wasteland, or 'wl create <org/db>' to start one." {
		t.Errorf("unexpected hint: %s", h.Hint)
	}
	if !errors.Is(err, federation.ErrNotJoined) {
		t.Error("should unwrap to ErrNotJoined")
	}
}

func TestHintWrap_Ambiguous(t *testing.T) {
	err := hintWrap(federation.ErrAmbiguous)
	var h *HintedError
	if !errors.As(err, &h) {
		t.Fatal("expected HintedError")
	}
	if h.Hint != "Use --wasteland <org/db> to select which wasteland." {
		t.Errorf("unexpected hint: %s", h.Hint)
	}
}

func TestHintWrap_GenericError(t *testing.T) {
	err := hintWrap(fmt.Errorf("disk full"))
	var h *HintedError
	if !errors.As(err, &h) {
		t.Fatal("expected HintedError")
	}
	if h.Hint != "Run 'wl doctor' to check your setup." {
		t.Errorf("unexpected hint: %s", h.Hint)
	}
}

func TestHintWrap_WrappedNotJoined(t *testing.T) {
	wrapped := fmt.Errorf("oops: %w", federation.ErrNotJoined)
	err := hintWrap(wrapped)
	var h *HintedError
	if !errors.As(err, &h) {
		t.Fatal("expected HintedError")
	}
	if h.Hint != "Run 'wl join <org/db>' to join a wasteland, or 'wl create <org/db>' to start one." {
		t.Errorf("unexpected hint for wrapped ErrNotJoined: %s", h.Hint)
	}
}
