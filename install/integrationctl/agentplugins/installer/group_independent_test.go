package installer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type independentListingRunner struct {
	mu      sync.Mutex
	inner   groupHostReconciliationRunner
	armed   atomic.Bool
	release chan struct{}
}

func (r *independentListingRunner) Run(ctx context.Context, command ports.Command) (ports.CommandResult, error) {
	claude := false
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "CLAUDE_CONFIG_DIR=") {
			claude = true
		}
	}
	if claude && r.armed.Load() && containsArgSeq(command.Argv, "plugin", "list") {
		select {
		case <-r.release:
		case <-ctx.Done():
			return ports.CommandResult{CleanupComplete: true}, ctx.Err()
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inner.Run(ctx, command)
}

// This uses only fixture protocols and fresh TEST directories. It exercises
// the SDK seam, real grouped publication, and the public progress callback.
func TestIndependentGroupReportsCodexWhileClaudeProbePending(t *testing.T) {
	sandbox := newBothClientSandbox(t)
	runner := &independentListingRunner{inner: groupHostReconciliationRunner{claude: listingRunner{configRoot: sandbox.claudeConfig}}, release: make(chan struct{})}
	sandbox.engine.cfg.Runner = runner
	sandbox.engine.cfg.IndependentActivations = true
	sandbox.engine.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
		if facts.ClientID == "claude" {
			runner.armed.Store(true)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	events := make(chan ProgressEvent, 8)
	sandbox.engine.cfg.Progress = func(event ProgressEvent) {
		if event.Client != nil {
			events <- event
		}
	}
	targets := bothClientTargets(sandbox.codexConfig, sandbox.claudeConfig, sandbox.probe)
	targets[0], targets[1] = targets[1], targets[0]
	prepared, err := sandbox.engine.Prepare(ctx, Request{Operation: OpInstall, PackageRoot: sandbox.pkg, InstallationID: "00000000-0000-4000-8000-0000000000e9", OperationID: "independent-probe", RequiredComponents: []string{"mcp", "skills"}, ClientExecutable: sandbox.probe, Targets: targets})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		result, err := sandbox.engine.Apply(ctx, prepared, Decision{Confirmed: true})
		if err == nil && result.Outcome != OutcomeCompleted {
			err = fmt.Errorf("outcome=%s", result.Outcome)
		}
		done <- err
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(30 * time.Second):
			t.Error("SDK fixture did not stop after cancellation")
		}
		_ = prepared.Close()
	}()
	started := map[string]bool{}
	for {
		select {
		case event := <-events:
			if event.Client.GroupPhase == "external_started" {
				started[event.Client.ClientID] = true
			}
			if event.Client.ClientID == "codex" && event.Client.GroupPhase == "external_completed" {
				if !started["claude"] || !started["codex"] {
					t.Fatalf("completion preceded starts: %v", started)
				}
				close(runner.release)
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(30 * time.Second):
					t.Fatal("group did not join")
				}
				return
			}
		case err := <-done:
			t.Fatalf("group returned before Codex progress while Claude pending: %v", err)
		case <-time.After(30 * time.Second):
			t.Fatal("Codex was blocked by Claude probe")
		}
	}
}
