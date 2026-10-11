package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Exercise the public engine result with committed records and actual client
// protocol calls. Persisted prepared/active lifecycle alone cannot express a
// current attempt that failed or a sibling whose activation was never started.
func TestGroupApplyProjectsAttemptFacts(t *testing.T) {
	for _, test := range []struct {
		name       string
		codexFirst bool
		stop       bool
		cancel     bool
	}{
		{name: "local_timeout_stops_sibling", stop: true},
		{name: "local_timeout_keeps_completed_sibling", codexFirst: true, stop: true},
		{name: "parent_cancel_stops_sibling", stop: true, cancel: true},
		{name: "success"},
	} {
		t.Run(test.name, func(t *testing.T) {
			sandbox := newBothClientSandbox(t)
			ctx, cancel := context.WithCancel(sandbox.ctx)
			defer cancel()
			runner := &activationStopListingRunner{inner: groupHostReconciliationRunner{claude: listingRunner{configRoot: sandbox.claudeConfig}}}
			sandbox.engine.cfg.Runner = runner
			// Arm only after the host reports the binding committed, so preflight
			// registry reads remain available and the failure is post-commit.
			sandbox.engine.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
				if facts.ClientID == "claude" && test.stop {
					runner.stop = func() error {
						if test.cancel {
							cancel()
							return ctx.Err()
						}
						return context.DeadlineExceeded
					}
				}
				return nil
			}
			targets := bothClientTargets(sandbox.codexConfig, sandbox.claudeConfig, sandbox.probe)
			if !test.codexFirst {
				targets[0], targets[1] = targets[1], targets[0]
			}
			prepared, err := sandbox.engine.Prepare(ctx, Request{Operation: OpInstall, PackageRoot: sandbox.pkg,
				InstallationID: "00000000-0000-4000-8000-0000000000e1", OperationID: "attempt-facts",
				RequiredComponents: []string{"mcp", "skills"}, ClientExecutable: sandbox.probe, Targets: targets})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = prepared.Close() }()
			result, err := sandbox.engine.Apply(ctx, prepared, Decision{Confirmed: true})
			if test.stop {
				cause := context.DeadlineExceeded
				if test.cancel {
					cause = context.Canceled
				}
				if !errors.Is(err, cause) || result.Outcome != OutcomeIncomplete {
					t.Fatalf("stopped result = %+v %v", result, err)
				}
				if !test.codexFirst && !strings.Contains(err.Error(), "1 of 2 client activations failed; 1 not attempted") {
					t.Fatalf("incorrect stop summary: %v", err)
				}
			} else if err != nil || result.Outcome != OutcomeCompleted {
				t.Fatalf("successful result = %+v %v", result, err)
			}
			if len(result.Targets) != 2 {
				t.Fatalf("missing committed targets: %+v", result)
			}
			byID := map[string]ClientResult{}
			view, inspectErr := sandbox.engine.Inspect(context.Background())
			if inspectErr != nil || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 2 {
				t.Fatalf("missing owned registrations: %+v %v", view, inspectErr)
			}
			for _, target := range result.Targets {
				byID[target.ClientID] = target
				binding := inspectedBinding(t, view, target.ClientID)
				if target.BindingID == "" || target.BindingID != binding.BindingID || target.Materialization != binding.Materialization || target.Activation != binding.Activation {
					t.Fatalf("attempt projection changed saved lifecycle: %+v vs %+v", target, binding)
				}
			}
			claude, codex := byID["claude"], byID["codex"]
			if test.stop {
				stage := "timeout"
				if test.cancel {
					stage = "canceled"
				}
				if claude.GroupPhase != "external_failed" || claude.Failure == nil || claude.Failure.Stage != stage {
					t.Fatalf("missing failed attempt: %+v", claude)
				}
				if !test.codexFirst {
					if codex.GroupPhase != "external_not_attempted" || codex.Failure == nil || codex.Failure.Stage != stage || runner.inner.codex.installed {
						t.Fatalf("unattempted sibling fabricated progress: %+v calls=%+v", codex, runner.inner.codex.calls)
					}
				}
			} else if claude.GroupPhase != "external_completed" || claude.Failure != nil {
				t.Fatalf("missing Claude success: %+v", claude)
			}
			if test.codexFirst || !test.stop {
				if codex.GroupPhase != "external_completed" || codex.Failure != nil || codex.Activation != string(domain.ActivationActive) || !runner.inner.codex.installed {
					t.Fatalf("completed sibling lost: %+v calls=%+v", codex, runner.inner.codex.calls)
				}
			}
			primary := byID[result.Client.ClientID]
			if result.Client.GroupPhase != primary.GroupPhase || (result.Client.Failure == nil) != (primary.Failure == nil) {
				t.Fatalf("primary result dropped attempt: %+v vs %+v", result.Client, primary)
			}
			if primary.Failure != nil && *result.Client.Failure != *primary.Failure {
				t.Fatalf("primary result changed reason: %+v vs %+v", result.Client, primary)
			}
		})
	}
}

func TestSingleClientUpdateProjectsGroupAttemptOnFailure(t *testing.T) {
	sandbox := newBothClientSandbox(t)
	runner := &activationStopListingRunner{inner: groupHostReconciliationRunner{claude: listingRunner{configRoot: sandbox.claudeConfig}}}
	sandbox.engine.cfg.Runner = runner
	id := "00000000-0000-4000-8000-0000000000e2"
	installBothClients(sandbox.ctx, t, sandbox.engine, sandbox.pkg, sandbox.probe, id, "before-update", bothClientTargets(sandbox.codexConfig, sandbox.claudeConfig, sandbox.probe))
	before, err := sandbox.engine.Inspect(sandbox.ctx)
	if err != nil {
		t.Fatal(err)
	}
	codex := inspectedBinding(t, before, "codex")
	if err := os.WriteFile(filepath.Join(sandbox.pkg, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	sandbox.engine.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
		if facts.ClientID == "claude" {
			runner.stop = func() error { return context.DeadlineExceeded }
		}
		return nil
	}
	prepared, err := sandbox.engine.Prepare(sandbox.ctx, Request{Operation: OpUpdate, PackageRoot: sandbox.pkg,
		ClientID: "claude", ClientConfigRoot: sandbox.claudeConfig, ClientExecutable: sandbox.probe,
		InstallationID: id, OperationID: "single-group-update", RequiredComponents: []string{"mcp", "skills"},
		KnownTargets: []TargetFacts{{ClientID: "codex", BindingID: codex.BindingID, ConfigRoot: sandbox.codexConfig, Executable: sandbox.probe}}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	result, err := sandbox.engine.Apply(sandbox.ctx, prepared, Decision{Confirmed: true})
	if !errors.Is(err, context.DeadlineExceeded) || result.Outcome != OutcomeIncomplete || result.Client.GroupPhase != "external_failed" || result.Client.Failure == nil || result.Client.Failure.Stage != "timeout" {
		t.Fatalf("single group update dropped failure facts: %+v %v", result, err)
	}
	after, err := sandbox.engine.Inspect(sandbox.ctx)
	if err != nil {
		t.Fatal(err)
	}
	claude := inspectedBinding(t, after, "claude")
	if result.Client.Materialization != claude.Materialization || result.Client.Activation != claude.Activation || inspectedBinding(t, after, "codex").TreeDigest != codex.TreeDigest {
		t.Fatalf("update changed unrelated sibling or dropped saved target: %+v", after)
	}
}

type activationStopListingRunner struct {
	inner groupHostReconciliationRunner
	stop  func() error
}

func (r *activationStopListingRunner) Run(ctx context.Context, cmd ports.Command) (ports.CommandResult, error) {
	if r.stop != nil && containsArgSeq(cmd.Argv, "plugin", "list") {
		for _, entry := range cmd.Env {
			if strings.HasPrefix(entry, "CLAUDE_CONFIG_DIR=") {
				return ports.CommandResult{}, r.stop()
			}
		}
	}
	return r.inner.Run(ctx, cmd)
}
