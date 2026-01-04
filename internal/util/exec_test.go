package util

import (
	"strings"
	"testing"
)

func TestRealRunner(t *testing.T) {
	runner := &RealRunner{}

	// We use a command that exists on almost all unix systems
	output, err := runner.Run("echo", "hello world")
	if err != nil {
		t.Fatalf("RealRunner.Run failed: %v", err)
	}

	got := strings.TrimSpace(string(output))
	if got != "hello world" {
		t.Errorf("RealRunner.Run() = %q, want %q", got, "hello world")
	}
}
