package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestNativeProbeFailurePreservesProcessClosureEvidence(t *testing.T) {
	cause := errors.New("process cleanup observation failed")
	for _, tc := range []struct {
		name     string
		result   legacyports.CommandResult
		err      error
		unclosed bool
	}{
		{"unknown closure", legacyports.CommandResult{}, cause, true},
		{"no process started", legacyports.CommandResult{NoChildStarted: true}, cause, false},
		{"closed timeout", legacyports.CommandResult{CleanupComplete: true}, context.DeadlineExceeded, false},
		{"successful custom runner", legacyports.CommandResult{}, nil, false},
		{"typed exchange failure", legacyports.CommandResult{}, &legacyports.ProcessClosureError{Cause: cause}, true},
		{"typed uncertainty cannot be cleared", legacyports.CommandResult{CleanupComplete: true}, errors.Join(context.DeadlineExceeded, &legacyports.ProcessClosureError{Cause: cause}), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := NativeProbeFailure(tc.result, tc.err)
			if domain.IsUnclosedClientProbe(got) != tc.unclosed || !errors.Is(got, tc.err) {
				t.Fatalf("closure=%v err=%v", domain.IsUnclosedClientProbe(got), got)
			}
		})
	}
}
