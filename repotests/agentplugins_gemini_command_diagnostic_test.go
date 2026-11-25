//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Only fixed classifications and primitive result facts may leave this seam.
// Never inspect or retain the native error text, arguments, paths or streams.
func geminiCommandFailureFacts(result ports.CommandResult, err, contextErr error) map[string]any {
	deadline := errors.Is(err, context.DeadlineExceeded) || errors.Is(contextErr, context.DeadlineExceeded)
	canceled := errors.Is(err, context.Canceled) || errors.Is(contextErr, context.Canceled)
	unclosed := domain.IsUnclosedClientProbe(err) || (err != nil || contextErr != nil) && !result.NoChildStarted && !result.CleanupComplete
	class := "capture-bound"
	switch {
	case unclosed:
		class = "cleanup-unconfirmed"
	case deadline:
		class = "deadline"
	case canceled:
		class = "canceled"
	case result.NoChildStarted:
		class = "not-started"
	case result.ExitCode != 0:
		class = "native-nonzero"
	case err != nil:
		class = "closed-command-error"
	}
	return map[string]any{"class": class, "exit_code": result.ExitCode, "no_child_started": result.NoChildStarted, "cleanup_complete": result.CleanupComplete, "deadline": deadline, "canceled": canceled, "error": err != nil, "context_error": contextErr != nil}
}

func TestGeminiCommandFailureFactsRemainClosedAndPrioritizeCleanup(t *testing.T) {
	canary := "private-native-error-and-path-canary"
	failure := errors.New(canary)
	for _, test := range []struct {
		name            string
		result          ports.CommandResult
		err, contextErr error
		class           string
	}{
		{"native-exit", ports.CommandResult{ExitCode: 7, CleanupComplete: true}, nil, nil, "native-nonzero"},
		{"closed-error", ports.CommandResult{CleanupComplete: true}, failure, nil, "closed-command-error"},
		{"deadline", ports.CommandResult{CleanupComplete: true}, context.DeadlineExceeded, nil, "deadline"},
		{"context-deadline", ports.CommandResult{CleanupComplete: true}, nil, context.DeadlineExceeded, "deadline"},
		{"canceled", ports.CommandResult{CleanupComplete: true}, context.Canceled, nil, "canceled"},
		{"context-canceled", ports.CommandResult{CleanupComplete: true}, nil, context.Canceled, "canceled"},
		{"no-child", ports.CommandResult{NoChildStarted: true}, failure, nil, "not-started"},
		{"cleanup-before-deadline", ports.CommandResult{}, failure, context.DeadlineExceeded, "cleanup-unconfirmed"},
		{"context-only-unclosed", ports.CommandResult{}, nil, context.DeadlineExceeded, "cleanup-unconfirmed"},
		{"typed-cleanup", ports.CommandResult{NoChildStarted: true}, &ports.ProcessClosureError{Cause: failure}, nil, "cleanup-unconfirmed"},
		{"capture-bound", ports.CommandResult{CleanupComplete: true}, nil, nil, "capture-bound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := geminiCommandFailureFacts(test.result, test.err, test.contextErr)
			if len(facts) != 8 || facts["class"] != test.class ||
				facts["exit_code"] != test.result.ExitCode ||
				facts["no_child_started"] != test.result.NoChildStarted ||
				facts["cleanup_complete"] != test.result.CleanupComplete ||
				facts["error"] != (test.err != nil) || facts["context_error"] != (test.contextErr != nil) {
				t.Fatalf("fixed diagnostic facts differ: %+v", facts)
			}
			deadline := errors.Is(test.err, context.DeadlineExceeded) || errors.Is(test.contextErr, context.DeadlineExceeded)
			canceled := errors.Is(test.err, context.Canceled) || errors.Is(test.contextErr, context.Canceled)
			if facts["deadline"] != deadline || facts["canceled"] != canceled {
				t.Fatal("diagnostic lost deadline or cancellation")
			}
			body, err := json.Marshal(facts)
			if err != nil || len(body) > 256 || strings.Contains(string(body), canary) {
				t.Fatal("diagnostic contains native error text or exceeds its bound")
			}
		})
	}
}
