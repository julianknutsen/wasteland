package main

import (
	"bytes"
	"testing"
)

func TestMergeRequiresArg(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := newRootCmd(&stdout, &stderr)

	for _, c := range root.Commands() {
		if c.Name() == "merge" {
			if err := c.Args(c, []string{}); err == nil {
				t.Error("merge should require exactly 1 argument")
			}
			if err := c.Args(c, []string{"wl/rig/w-abc"}); err != nil {
				t.Errorf("merge should accept 1 argument: %v", err)
			}
			if err := c.Args(c, []string{"a", "b"}); err == nil {
				t.Error("merge should reject 2 arguments")
			}
			return
		}
	}
	t.Fatal("merge command not found")
}
