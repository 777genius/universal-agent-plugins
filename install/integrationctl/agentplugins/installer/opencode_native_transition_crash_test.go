package installer

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type nativeCrashSave struct {
	transaction.StateStore
	after bool
}

// Model a foreign state edit after the recorder advances its native journal,
// before its state publication. Recovery must retain that edit and uncertainty.
type nativeRecoveryDriftStore struct {
	transaction.StateStore
	drifted bool
}

func (s *nativeRecoveryDriftStore) Load() (domain.StateFileV2, error) {
	state, err := s.StateStore.Load()
	if err != nil || s.drifted {
		return state, err
	}
	pending, err := (opencode.NativeTransitions{State: transaction.Kernel{StateStore: s.StateStore}}).Pending()
	if err != nil {
		return state, err
	}
	for _, item := range pending {
		if item.Phase == "native_committed" {
			s.drifted = true
			state.Installations[0].DataRetained = true
			if err := s.StateStore.Save(state); err != nil {
				return state, err
			}
			break
		}
	}
	return state, nil
}

func (s nativeCrashSave) Save(state domain.StateFileV2) error {
	target := false
	for _, in := range state.Installations {
		for _, b := range in.Clients {
			if b.NativeActivationAttempt != "" {
				continue
			}
			for _, o := range b.NativeObjects {
				if o.Kind == opencode.OpenCodeV2MCPObjectKind {
					target = true
				}
			}
		}
	}
	if !target {
		return s.StateStore.Save(state)
	}
	if s.after {
		if err := s.StateStore.Save(state); err != nil {
			return err
		}
	}
	panic("TEST crash at native state publication")
}

// Generic directory recovery alone cannot pass this fresh-facade crash test.
// No caller supplies projection bytes or a host during stored recovery.
func TestOpenCodeNativeFacadeCrashRecovery(t *testing.T) {
	for _, scenario := range []struct {
		name         string
		after, drift bool
	}{{name: "native"}, {name: "state", after: true}, {name: "foreign_state_during_recovery", drift: true}} {
		t.Run(scenario.name, func(t *testing.T) {
			after := scenario.after
			root := openCodeTestRoot(t)
			v1 := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.34", "ok")
			v2 := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
			req := openCodeRequest(t, root, v1)
			req.InstallationID = "TEST-crash-facade"
			cfg := Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: v1, OpenCodeProbeEnvironment: []string{"PATH="}}
			engine := openCodeEngine(t, cfg)
			first, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
			first.Close()
			if err != nil {
				t.Fatal(err)
			}
			req.Operation, req.InstallationID, req.ClientExecutable = OpUpdate, result.InstallationID, v2
			next, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			engine.store = nativeCrashSave{StateStore: engine.store, after: after}
			func() {
				defer func() {
					if value := recover(); value != "TEST crash at native state publication" {
						t.Fatalf("unexpected crash boundary: %v", value)
					}
				}()
				_, _ = engine.Apply(testCtx(t), next, Decision{Confirmed: true})
				t.Fatal("crash boundary not reached")
			}()
			cfg.OpenCodeProbe = func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				t.Fatal("recovery probed host")
				return clientdetect.ProbeEvidence{}, errors.New("forbidden")
			}
			cfg.Runner = forbiddenV2Runner{t}
			cfg.EnableNativeObserver = true
			reopened := openCodeEngine(t, cfg)
			view, err := reopened.Inspect(testCtx(t))
			if err != nil || !view.Recovery.Required {
				t.Fatalf("missing durable recovery evidence %+v %v", view, err)
			}
			if after {
				// The ordinary usecase entry recovers under its already-held operation
				// lock before removal, without reentering/acquiring that lock again.
				service := reopened.lifecycle(nil, BindingFacts{}, nil)
				if _, err := service.Remove(testCtx(t), usecase.RemoveInput{Selector: req.InstallationID, Client: domain.DetectedClient{ClientID: domain.ClientOpenCode, ConfigRoot: req.ClientConfigRoot}, Scope: domain.ScopeUser, Confirmed: true, OperationID: "TEST-remove-after-crash"}); err != nil {
					t.Fatal(err)
				}
				view, err := reopened.Inspect(testCtx(t))
				if err != nil || view.Recovery.Required {
					t.Fatal("normal mutation did not recover first", err)
				}
				present, _, err := nativeconfig.New().Inspect(nativeconfig.Paths{JSON: filepath.Join(req.ClientConfigRoot, "opencode.json"), JSONC: filepath.Join(req.ClientConfigRoot, "opencode.jsonc")}, nativeconfig.CodecOpenCodeV2, "sample-notify", nil)
				if err != nil || present {
					t.Fatal("normal mutation did not remove recovered target", err)
				}
				return
			}
			if scenario.drift {
				drift := &nativeRecoveryDriftStore{StateStore: reopened.store}
				reopened.store = drift
				if _, err := reopened.Recover(testCtx(t), view); !errors.Is(err, ErrPlanChanged) {
					t.Fatalf("recovery accepted foreign state: %v", err)
				}
				state, err := drift.StateStore.Load()
				if err != nil || !drift.drifted || !state.Installations[0].DataRetained {
					t.Fatalf("foreign state was overwritten: %+v %v", state, err)
				}
				for _, binding := range state.Installations[0].Clients {
					if binding.NativeActivationAttempt == "" {
						t.Fatal("foreign state edit cleared unresolved attempt")
					}
				}
				remaining, err := reopened.Inspect(testCtx(t))
				if err != nil || !remaining.Recovery.Required || len(remaining.Recovery.Journals) == 0 {
					t.Fatalf("lost durable uncertainty: %+v %v", remaining.Recovery, err)
				}
				return
			}
			if _, err := reopened.Recover(testCtx(t), view); err != nil {
				t.Fatal(err)
			}
			state, err := reopened.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range state.Installations[0].Clients {
				if b.NativeActivationAttempt != "" {
					t.Fatal("native attempt retained")
				}
				if err := opencode.VerifyOpenCodeNativeObjects(req.ClientConfigRoot, b.TargetLocator, b.NativeObjects, nativeconfig.New(), false); err != nil {
					t.Fatal(err)
				}
				for _, o := range b.NativeObjects {
					codec, mcp, _ := nativeconfig.OpenCodeCodecForKind(o.Kind)
					if mcp && codec != nativeconfig.CodecOpenCodeV2 {
						t.Fatal("lost target receipts")
					}
				}
			}
			view, err = reopened.Inspect(testCtx(t))
			if err != nil || view.Recovery.Required {
				t.Fatal("recovery not settled", err)
			}
		})
	}
}
