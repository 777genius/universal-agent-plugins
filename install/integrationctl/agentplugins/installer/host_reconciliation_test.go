package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type observationRunner struct {
	recordingCodexRunner
	observation string
}

func (r *observationRunner) Run(ctx context.Context, cmd ports.Command) (ports.CommandResult, error) {
	if r.observation != "" && containsArgSeq(cmd.Argv, "plugin", "list") {
		r.calls = append(r.calls, append([]string(nil), cmd.Argv...))
		return ports.CommandResult{Stdout: []byte(r.observation)}, nil
	}
	return r.recordingCodexRunner.Run(ctx, cmd)
}

func TestHostReconciliationRepeat(t *testing.T) {
	for _, failFirst := range []bool{false, true} {
		name := "success"
		if failFirst {
			name = "callback_failure"
		}
		t.Run(name, func(t *testing.T) {
			skipWindowsLauncherExecuteBit(t)
			ctx := testCtx(t)
			probe := buildProbe(t)
			base := t.TempDir()
			pkg, config := filepath.Join(base, "package"), filepath.Join(base, "config")
			writePackage(t, pkg, probe)
			if err := os.MkdirAll(config, 0700); err != nil {
				t.Fatal(err)
			}
			runner := &observationRunner{}
			callbacks, effects := 0, 0
			appliedEffects := map[string]bool{}
			var committed BindingFacts
			eng, err := newTestEngine(t, Config{
				StateRoot: filepath.Join(base, "state"), HelperExecutable: probe, Runner: runner, EnableNativeObserver: true,
				OnCommittedBinding: func(_ context.Context, facts BindingFacts) error {
					callbacks++
					if _, err := os.Stat(facts.TargetPath); err != nil {
						return err
					}
					committed = facts
					// Model a host sidecar upsert whose effect may have succeeded
					// before the callback reports failure. Retry uses the stable
					// binding/digest, even when OperationID changes.
					key := facts.BindingID + ":" + facts.TreeDigest
					if !appliedEffects[key] {
						appliedEffects[key] = true
						effects++
					}
					if failFirst && callbacks == 1 {
						return errors.New("host fault")
					}
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			req := Request{Operation: OpInstall, PackageRoot: pkg, ClientID: "codex", ClientConfigRoot: config, ClientExecutable: probe, InstallationID: "00000000-0000-4000-8000-000000000081", OperationID: "first"}
			apply := func() (Result, error) {
				p, err := eng.Prepare(ctx, req)
				if err != nil {
					return Result{}, err
				}
				defer p.Close()
				return eng.Apply(ctx, p, Decision{Confirmed: true})
			}
			first, err := apply()
			if failFirst {
				if err == nil || first.Outcome != OutcomeIncomplete {
					t.Fatalf("fault: %+v %v", first, err)
				}
			} else if err != nil || first.Outcome != OutcomeCompleted {
				t.Fatalf("install: %+v %v", first, err)
			}
			if committed.BindingID != first.Binding.BindingID || committed.TreeDigest != first.Binding.TreeDigest {
				t.Fatalf("callback facts: %+v / %+v", committed, first)
			}
			before, err := eng.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			targetInfo, err := os.Stat(first.Binding.TargetPath)
			if err != nil {
				t.Fatal(err)
			}
			req.OperationID = "retry"
			retried, err := apply()
			if err != nil {
				t.Fatal(err)
			}
			if failFirst && retried.Outcome != OutcomeCompleted {
				t.Fatalf("retry: %+v", retried)
			}
			req.OperationID = "repeat"
			runner.calls = nil
			repeated, err := apply()
			if err != nil || repeated.Outcome != OutcomeUnchanged || !repeated.NoChange {
				t.Fatalf("repeat: %+v %v", repeated, err)
			}
			if repeated.Client.Authentication != string(domain.AuthenticationNotChecked) || repeated.Client.Activation != string(domain.ActivationActive) || repeated.Client.Verification != string(domain.VerificationInstalled) {
				t.Fatalf("observation: %+v", repeated.Client)
			}
			if repeated.Binding.BindingID != first.Binding.BindingID || repeated.Binding.TreeDigest != first.Binding.TreeDigest {
				t.Fatalf("identity changed: %+v / %+v", first.Binding, repeated.Binding)
			}
			after, err := eng.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.TransactionReceipts, after.TransactionReceipts) || !reflect.DeepEqual(before.Installations[0].Clients[first.Binding.BindingID].Receipts, after.Installations[0].Clients[first.Binding.BindingID].Receipts) {
				t.Fatal("retry/repeat created mutation receipt")
			}
			afterInfo, err := os.Stat(first.Binding.TargetPath)
			if err != nil || !os.SameFile(targetInfo, afterInfo) {
				t.Fatal("retry/repeat restaged package")
			}
			wantCallbacks := 1
			if failFirst {
				wantCallbacks = 2
			}
			if callbacks != wantCallbacks || effects != 1 {
				t.Fatalf("callbacks=%d effects=%d", callbacks, effects)
			}
			if !argvHas(runner.calls, "plugin", "list") {
				t.Fatalf("no VerifyOnly listing: %v", runner.calls)
			}
			for _, argv := range runner.calls {
				if !containsArgSeq(argv, "plugin", "list") {
					t.Fatalf("repeat mutated client: %v", argv)
				}
			}
			observation := `{"installed":[{"pluginId":"foreign@other","name":"foreign","marketplaceName":"other","installed":true,"enabled":true}]}`
			if failFirst {
				observation = `{"unexpected":true}`
			}
			runner.observation = observation
			bad, err := apply()
			if bad.NoChange || bad.Outcome == OutcomeUnchanged || bad.Client.Activation == string(domain.ActivationActive) || bad.Client.Verification == string(domain.VerificationInstalled) {
				t.Fatalf("unverified registration accepted: %+v %v", bad, err)
			}
			if failFirst {
				if err != nil || bad.Client.Activation != string(domain.ActivationManual) || len(bad.ManualActions) == 0 {
					t.Fatalf("unknown listing did not request manual verification: %+v %v", bad, err)
				}
			} else if err == nil || bad.Outcome != OutcomeIncomplete || bad.Client.Activation != string(domain.ActivationFailed) {
				t.Fatalf("foreign listing did not fail activation: %+v %v", bad, err)
			}
			if bad.Client.Authentication != string(domain.AuthenticationNotChecked) || callbacks != wantCallbacks || effects != 1 {
				t.Fatalf("negative observation escalated auth or repeated callback: %+v callbacks=%d effects=%d", bad, callbacks, effects)
			}
		})
	}
}
