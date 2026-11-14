//go:build linux

package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func enableGrokSDKFixture(t *testing.T) *grokEnableSDKFixture {
	t.Helper()
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	if result, err := f.execute(t, f.selected(t, OpRepair, true), Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("owned enablement setup: %+v %v", result, err)
	}
	return f
}

func TestProjectGrokSDKUpdateRepairRetainsOwnedMemberAtDurablePackageBoundary(t *testing.T) {
	for _, op := range []Operation{OpUpdate, OpRepair} {
		t.Run(string(op), func(t *testing.T) {
			f := enableGrokSDKFixture(t)
			binding := f.binding(t)
			owned, config := binding.NativeObjects[len(binding.NativeObjects)-1], deathRead(t, f.config)
			skill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			original := deathRead(t, skill)
			if op == OpUpdate {
				deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), strings.Replace(original, "Original", "Updated", 1))
			} else {
				deathWrite(t, skill, "immutable drift")
			}
			req := f.selected(t, op, false)
			seen := false
			f.engine.cfg.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
				current := f.binding(t)
				committed := false
				for _, receipt := range current.Receipts {
					committed = committed || receipt.OperationID == req.OperationID
				}
				if !committed {
					return nil
				} // Existing Prepared handoff precedes this operation.
				seen = true
				if facts.TargetPath != binding.TargetLocator || len(current.NativeObjects) != 2 || current.NativeObjects[1] != owned || current.NativeActivationAttempt == "" {
					t.Fatalf("durable package boundary lost prior native ownership: %+v", current)
				}
				want := original
				if op == OpUpdate {
					want = strings.Replace(original, "Original", "Updated", 1)
				}
				if deathRead(t, skill) != want {
					t.Fatal("callback did not observe the actual new package commit")
				}
				return errors.New("inert interruption after durable package commit")
			}
			if result, err := f.execute(t, req, Decision{Confirmed: true}); err == nil {
				t.Fatalf("commit-boundary interruption accepted: %+v", result)
			}
			if !seen || deathRead(t, f.config) != config || f.native.duplex != 2 {
				t.Fatal("ordinary package replacement changed native enablement")
			}
			f.engine = f.reopen(t, f.home)
			current := f.binding(t)
			if len(current.NativeObjects) != 2 || current.NativeObjects[1] != owned {
				t.Fatal("cold state lost owned member at interruption boundary")
			}
		})
	}
}

func TestProjectGrokSDKRetainedMembershipDriftRefusesBeforePackageOrState(t *testing.T) {
	for _, mode := range []string{"missing", "disabled", "changed-id", "blank-home", "foreign-home"} {
		t.Run(mode, func(t *testing.T) {
			f := enableGrokSDKFixture(t)
			request := f.selected(t, OpUpdate, false)
			binding := f.binding(t)
			skill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			beforeSkill := deathRead(t, skill)
			deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), strings.Replace(beforeSkill, "Original", "Updated", 1))
			switch mode {
			case "missing":
				deathWrite(t, f.config, "[plugins]\nenabled=['foreign']\ndisabled=['other']\n")
			case "disabled":
				deathWrite(t, f.config, strings.Replace(deathRead(t, f.config), "disabled=['other']", "disabled=['other','grok-sdk-owned']", 1))
			case "changed-id":
				deathWrite(t, f.config, strings.Replace(deathRead(t, f.config), "/grok-sdk-owned", "/foreign-owned", 1))
			case "blank-home":
				f.engine = f.reopen(t, "")
			case "foreign-home":
				f.engine = f.reopen(t, f.home+"-foreign")
			}
			beforeState, err := f.engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			beforeConfig, calls, duplex := deathRead(t, f.config), f.native.calls, f.native.duplex
			if prepared, err := f.engine.Prepare(t.Context(), request); err == nil {
				_ = prepared.Close()
				t.Fatal("changed retained native authority accepted")
			}
			afterState, err := f.engine.store.Load()
			if err != nil || !reflect.DeepEqual(beforeState, afterState) || deathRead(t, skill) != beforeSkill || deathRead(t, f.config) != beforeConfig || f.native.calls != calls || f.native.duplex != duplex {
				t.Fatal("retained-membership refusal changed native/package/state authority", err)
			}
		})
	}
}

type grokRemovalInterception struct {
	*grok.Adapter
	before func(*domain.DeactivationRequest)
}

func (adapter grokRemovalInterception) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.Confirmed {
		adapter.before(&request)
	}
	return adapter.Adapter.Deactivate(ctx, env, request)
}

func TestProjectGrokSDKRemovalRefusesChangedFrozenAttemptBeforeNativeEffect(t *testing.T) {
	for _, mode := range []string{"replaced-attempt", "same-id-foreign-state", "blank-proof", "wrong-binding"} {
		t.Run(mode, func(t *testing.T) {
			f := enableGrokSDKFixture(t)
			request := f.selected(t, OpRemove, false)
			binding := f.binding(t)
			skill := filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md")
			beforeSkill, beforeConfig := deathRead(t, skill), deathRead(t, f.config)
			var changed domain.StateFileV2
			called := false
			adapter := grokRemovalInterception{Adapter: grok.New(), before: func(req *domain.DeactivationRequest) {
				called = true
				if req.NativeAttempt.OperationID == "" || req.NativeAttempt.StateDigest == "" {
					t.Fatal("core omitted original removal proof")
				}
				switch mode {
				case "blank-proof":
					req.NativeAttempt.StateDigest = ""
				case "wrong-binding":
					req.NativeAttempt.BindingID = "foreign-binding"
				default:
					state, err := f.engine.store.Load()
					if err != nil {
						t.Fatal(err)
					}
					if mode == "replaced-attempt" {
						current := state.Installations[0].Clients[binding.ClientBindingID]
						current.NativeActivationAttempt = "foreign-replaced-attempt"
						state.Installations[0].Clients[binding.ClientBindingID] = current
					} else {
						state.Installations[0].Source.Publisher = "foreign late authority"
					}
					if err := f.engine.store.Save(state); err != nil {
						t.Fatal(err)
					}
					changed, err = f.engine.store.Load()
					if err != nil {
						t.Fatal(err)
					}
				}
			}}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			cfg := f.engine.cfg
			cfg.Registry = registry
			f.engine, err = New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			before, err := f.engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if result, err := f.execute(t, request, Decision{Confirmed: true}); err == nil {
				t.Fatalf("foreign removal authority accepted: %+v", result)
			}
			if !called || f.native.duplex != 2 || deathRead(t, f.config) != beforeConfig || deathRead(t, skill) != beforeSkill {
				t.Fatal("refused removal changed native or managed package")
			}
			want := before
			if len(changed.Installations) != 0 {
				want = changed
			}
			after, err := f.engine.store.Load()
			if err != nil || !reflect.DeepEqual(after, want) {
				t.Fatal("refused removal overwrote changed state or failed exact no-effect acknowledgement", err)
			}
			if _, err := os.Stat(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
		})
	}
}
