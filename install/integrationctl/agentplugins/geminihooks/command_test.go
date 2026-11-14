package geminihooks_test

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// Bound fixture and toolchain processes; never run a native agent.
func testCommand(t *testing.T, executable string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, executable, args...)
}
