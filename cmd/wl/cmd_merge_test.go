package main

import (
	"bytes"
	"testing"
)

func TestMergeApprovalWarning(t *testing.T) {
	tests := []struct {
		name                string
		hasApproval         bool
		hasChangesRequested bool
		want                string
	}{
		{
			name:                "changes requested",
			hasChangesRequested: true,
			want:                "PR has outstanding change requests",
		},
		{
			name: "no approvals",
			want: "PR has no approvals",
		},
		{
			name:        "approved",
			hasApproval: true,
			want:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeApprovalWarning(tc.hasApproval, tc.hasChangesRequested)
			if got != tc.want {
				t.Errorf("mergeApprovalWarning(%v, %v) = %q, want %q",
					tc.hasApproval, tc.hasChangesRequested, got, tc.want)
			}
		})
	}
}

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
