//go:build linux

package main

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestPublicCommandTreeReapedIndependentOfCLIExit(t *testing.T) {
	for _, tc := range []struct {
		name string
		result ports.CommandResult
		want bool
	}{
		{"nonzero-reaped", ports.CommandResult{ExitCode: 1, CleanupComplete: true}, true},
		{"zero-reaped", ports.CommandResult{ExitCode: 0, CleanupComplete: true}, true},
		{"nonzero-unproved", ports.CommandResult{ExitCode: 1}, false},
		{"never-started", ports.CommandResult{CleanupComplete: true, NoChildStarted: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicCommandTreeReaped(tc.result); got != tc.want {
				t.Fatalf("cleanup proof differs: got %t, want %t", got, tc.want)
			}
		})
	}
}
