package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestLockedNPMCommandOutcomePreservesCancellationAndCleanupCauses(t *testing.T) {
	cleanup := errors.New("descendant cleanup failed")
	for _, tc := range []struct {
		name  string
		cause error
		label string
	}{
		{"cancellation", context.Canceled, "canceled"},
		{"deadline", context.DeadlineExceeded, "timed out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := lockedNPMCommandError(ports.CommandResult{}, errors.Join(tc.cause, cleanup))
			if !errors.Is(err, tc.cause) || !errors.Is(err, cleanup) || !strings.Contains(err.Error(), tc.label) {
				t.Fatalf("npm error = %v, want typed %v and cleanup failure", err, tc.cause)
			}
			if errors.Is(tc.cause, context.Canceled) && strings.Contains(err.Error(), "timed out") {
				t.Fatalf("cancellation relabeled as timeout: %v", err)
			}
		})
	}
}

func TestLockedNPMCommandOutcomeRejectsNonzeroExitAndBoundsDiagnosticTail(t *testing.T) {
	diagnostic := strings.Repeat("prefix", 10000) + strings.Repeat("tail", 300)
	err := lockedNPMCommandError(ports.CommandResult{ExitCode: 23, Stderr: []byte(diagnostic)}, nil)
	if err == nil || !strings.Contains(err.Error(), "exited with code 23") ||
		!strings.HasSuffix(err.Error(), strings.Repeat("tail", 300)) || strings.Contains(err.Error(), "prefix") {
		t.Fatalf("npm failure did not retain its bounded diagnostic tail: %v", err)
	}
	if err := lockedNPMCommandError(ports.CommandResult{ExitCode: 0, Stderr: []byte("warning")}, nil); err != nil {
		t.Fatalf("successful command with diagnostic failed: %v", err)
	}
}

func TestLockedNPMCommandOutcomeBoundsEmbeddedRunnerDiagnostic(t *testing.T) {
	cleanup := errors.New("descendant cleanup failed")
	diagnostic := strings.Repeat("verbose npm output", 4000) + "final npm diagnostic"
	cause := fmt.Errorf("%w (client diagnostic: %s)", errors.Join(context.Canceled, cleanup), diagnostic)
	err := lockedNPMCommandError(ports.CommandResult{Stderr: []byte(diagnostic)}, cause)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, cleanup) {
		t.Fatalf("bounded error discarded runner causes: %v", err)
	}
	if len(err.Error()) > 1600 || !strings.HasSuffix(err.Error(), "final npm diagnostic") ||
		!strings.Contains(err.Error(), "descendant cleanup failed") {
		t.Fatalf("runner diagnostic was not bounded with cleanup summary and npm tail: %v", err)
	}
}

func TestLockedRuntimeCanceledSuccessfulRunnerCannotCommitTarget(t *testing.T) {
	manager, envelope, plan, dataPath, _ := lockedRuntimeFixture(t)
	runner := manager.RunNPM
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	manager.RunNPM = func(ctx context.Context, temporary, dataPath string, omitOptional bool) error {
		if err := runner(ctx, temporary, dataPath, omitOptional); err != nil {
			return err
		}
		cancel()
		return nil
	}
	if err := manager.PrepareRuntime(ctx, envelope, plan, dataPath); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled preparation = %v, want context.Canceled", err)
	}
	assertNoPreparedNPMRuntime(t, dataPath)
}

func assertNoPreparedNPMRuntime(t *testing.T, dataPath string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dataPath, "npm-runtime"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), ".lock") {
			t.Errorf("canceled preparation left runtime or staging entry %q", entry.Name())
		}
	}
}
