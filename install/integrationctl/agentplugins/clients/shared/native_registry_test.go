package shared

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestRunNativeRegistryPreservesCleanupEvidence(t *testing.T) {
	cause := errors.New("process containment close failed")
	marked := &domain.UnclosedClientProbe{Operation: "existing probe", Err: cause}
	cases := []struct {
		name                     string
		err                      error
		cleanupComplete, noChild bool
		wrap                     bool
	}{
		{"unclosed", cause, false, false, true},
		{"wrapped cause", fmt.Errorf("close supervision: %w", cause), false, false, true},
		{"cleanup complete", cause, true, false, false},
		{"no child started", cause, false, true, false},
		{"already marked", marked, false, false, false},
		{"wrapped marker", fmt.Errorf("observe: %w", marked), false, false, false},
		{"no runner error", nil, false, false, false},
	}
	for _, tree := range []bool{false, true} {
		for _, test := range cases {
			t.Run(fmt.Sprintf("%s/tree=%t", test.name, tree), func(t *testing.T) {
				want := legacyports.CommandResult{Stdout: []byte("inventory"), Stderr: []byte("diagnostic"), ExitCode: 7, CleanupComplete: test.cleanupComplete, NoChildStarted: test.noChild}
				command := legacyports.Command{Argv: []string{"native", "inspect"}, Dir: "/fixture"}
				calls := 0
				base := nativeRegistryRunnerFunc(func(_ context.Context, got legacyports.Command) (legacyports.CommandResult, error) {
					calls++
					if !reflect.DeepEqual(got, command) {
						t.Fatalf("command changed: %+v", got)
					}
					return want, test.err
				})
				var runner ports.CommandRunner = base
				treeRunner := &nativeRegistryTreeRunner{CommandRunner: base}
				if tree {
					runner = treeRunner
				}
				got, err := RunNativeRegistry(context.Background(), runner, command)
				if !reflect.DeepEqual(got, want) || calls != 1 || tree && treeRunner.grace != nativeRegistryTreeExitGrace {
					t.Fatalf("runner contract changed: result=%+v calls=%d grace=%s", got, calls, treeRunner.grace)
				}
				if test.wrap {
					var probe *domain.UnclosedClientProbe
					if !errors.As(err, &probe) || !errors.Is(probe.Err, test.err) || !errors.Is(err, cause) {
						t.Fatalf("cleanup evidence or original cause lost: %v", err)
					}
				} else if !errors.Is(err, test.err) || !errors.Is(test.err, err) { // same error, not a wrapper around it
					t.Fatalf("existing result error changed: got %v, want %v", err, test.err)
				}
			})
		}
	}
}

type nativeRegistryRunnerFunc func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (run nativeRegistryRunnerFunc) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return run(ctx, command)
}

type nativeRegistryTreeRunner struct {
	ports.CommandRunner
	grace time.Duration
}

func (runner *nativeRegistryTreeRunner) RunWithTreeExitGrace(ctx context.Context, command legacyports.Command, grace time.Duration) (legacyports.CommandResult, error) {
	runner.grace = grace
	return runner.Run(ctx, command)
}
