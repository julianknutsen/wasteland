package main

import (
	"fmt"
	"os/exec"
)

// requireDolt checks that the dolt CLI is available on PATH.
// Returns a helpful error with install instructions if not found.
func requireDolt() error {
	if _, err := exec.LookPath("dolt"); err != nil {
		return fmt.Errorf("dolt is not installed or not in PATH\n\n" +
			"Install dolt: https://docs.dolthub.com/introduction/installation")
	}
	return nil
}
