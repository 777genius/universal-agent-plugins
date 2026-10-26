package claude

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type timeoutProbeRunner struct {
	run func(context.Context) (legacyports.CommandResult, error)
}

func (r timeoutProbeRunner) Run(ctx context.Context, _ legacyports.Command) (legacyports.CommandResult, error) {
	return r.run(ctx)
}

func TestClaudeProbeBudgetAndSafeTimeout(t *testing.T) {
	t.Run("slow startup has thirty seconds", func(t *testing.T) {
		runner := timeoutProbeRunner{run: func(ctx context.Context) (legacyports.CommandResult, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) < 29*time.Second || time.Until(deadline) > 30*time.Second {
				t.Fatalf("listing deadline = %v", deadline)
			}
			return legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}, nil
		}}
		if _, err := RunClaudeListCommand(context.Background(), runner, legacyports.Command{}); err != nil {
			t.Fatal(err)
		}
	})
	for _, complete := range []bool{false, true} {
		name := "unknown cleanup"
		if complete {
			name = "confirmed cleanup"
		}
		t.Run(name, func(t *testing.T) {
			runner := timeoutProbeRunner{run: func(ctx context.Context) (legacyports.CommandResult, error) {
				<-ctx.Done()
				return legacyports.CommandResult{CleanupComplete: complete}, ctx.Err()
			}}
			_, err := runClaudeListCommandWithin(context.Background(), runner, legacyports.Command{}, 10*time.Millisecond)
			if !errors.Is(err, context.DeadlineExceeded) || domain.IsIsolatedProbeTimeout(err) != complete {
				t.Fatalf("cleanup=%v err=%v", complete, err)
			}
			if !strings.Contains(err.Error(), "10ms probe budget") || !strings.Contains(err.Error(), "process cleanup") {
				t.Fatalf("missing bounded diagnostic: %v", err)
			}
		})
	}
	t.Run("parent deadline is never isolated", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		runner := timeoutProbeRunner{run: func(ctx context.Context) (legacyports.CommandResult, error) {
			<-ctx.Done()
			return legacyports.CommandResult{CleanupComplete: true}, ctx.Err()
		}}
		_, err := runClaudeListCommandWithin(ctx, runner, legacyports.Command{}, time.Second)
		if !errors.Is(err, context.DeadlineExceeded) || domain.IsIsolatedProbeTimeout(err) {
			t.Fatalf("parent deadline = %v", err)
		}
	})
}

func TestClaudeProbeGenericCleanupFailureRemainsUnsafe(t *testing.T) {
	failure := errors.New("containment close failed")
	runner := timeoutProbeRunner{run: func(context.Context) (legacyports.CommandResult, error) { return legacyports.CommandResult{}, failure }}
	_, err := RunClaudeListCommand(context.Background(), runner, legacyports.Command{})
	if !errors.Is(err, failure) || !domain.IsUnclosedClientProbe(err) || domain.IsIsolatedProbeTimeout(err) {
		t.Fatalf("lost unsafe cleanup cause: %v", err)
	}
}

func TestClaudeProbeKnownPrestartFailureDoesNotClaimUncertainProcess(t *testing.T) {
	failure := errors.New("fixture executable unavailable before start")
	runner := timeoutProbeRunner{run: func(context.Context) (legacyports.CommandResult, error) {
		return legacyports.CommandResult{NoChildStarted: true}, failure
	}}
	_, err := RunClaudeListCommand(context.Background(), runner, legacyports.Command{})
	if !errors.Is(err, failure) || domain.IsUnclosedClientProbe(err) || domain.IsIsolatedProbeTimeout(err) {
		t.Fatalf("known no-child failure was made uncertain: %v", err)
	}
}
