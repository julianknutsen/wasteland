package main

import (
	"bytes"
	"testing"
)

func TestReviewRequiresNoMoreThanOneArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "review" {
			if err := c.Args(c, []string{}); err != nil {
				t.Errorf("review should accept 0 arguments: %v", err)
			}
			if err := c.Args(c, []string{"wl/rig/w-abc"}); err != nil {
				t.Errorf("review should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("review should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("review command not found")
}

func TestReviewMutuallyExclusiveFlags(t *testing.T) {
	err := runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", true, true, false)
	if err == nil {
		t.Error("expected error for --json + --md")
	}

	err = runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", true, false, true)
	if err == nil {
		t.Error("expected error for --json + --stat")
	}

	err = runReview(nil, &bytes.Buffer{}, &bytes.Buffer{}, "wl/x/y", false, true, true)
	if err == nil {
		t.Error("expected error for --md + --stat")
	}
}
