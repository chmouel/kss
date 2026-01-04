package util

import (
	"os/exec"
)

// Runner defines the interface for running commands.
type Runner interface {
	Run(name string, args ...string) ([]byte, error)
}

// RealRunner implementation using os/exec.
type RealRunner struct{}

// Run executes a command and returns its combined output.
func (r *RealRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}
