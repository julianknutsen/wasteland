package main

import (
	"bytes"
	"testing"
)

func TestApproveRequiresArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "approve" {
			if err := c.Args(c, []string{}); err == nil {
				t.Error("approve should require exactly 1 argument")
			}
			if err := c.Args(c, []string{"wl/rig/w-abc"}); err != nil {
				t.Errorf("approve should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("approve should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("approve command not found")
}
