//go:build linux

package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectGrokSDKExplicitRepairRefusesUnqualifiedNativeEffects(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	binding := f.binding(t)
	if binding.Activation != domain.ActivationPrepared || binding.NativeActivationAttempt != "" || f.native.duplex != 0 {
		t.Fatal("ordinary install granted native enablement")
	}
	request := f.selected(t, OpRepair, true)
	unchanged := snapshotGrokSDKFiles(t, f, f.config, filepath.Join(f.project, ".uap", "state-v2.json"), filepath.Join(binding.TargetLocator, "skills", "docs", "SKILL.md"))
	if prepared, err := f.engine.Prepare(t.Context(), request); err == nil {
		_ = prepared.Close()
		t.Fatal("unqualified explicit enablement was prepared")
	}
	unchanged()
	if f.binding(t).NativeActivationAttempt != "" {
		t.Fatal("refusal recorded a native attempt")
	}
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("refused preview created recovery work: %+v %v", view, err)
	}
	if result, err := f.execute(t, f.selected(t, OpRepair, false), Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeUnchanged || !result.NoChange {
		t.Fatalf("ordinary Prepared repair refused: %+v %v", result, err)
	}
	original := deathRead(t, f.config)
	if result, err := f.execute(t, f.selected(t, OpRemove, false), Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("ordinary Prepared removal refused: %+v %v", result, err)
	}
	if deathRead(t, f.config) != original || f.native.duplex != 0 {
		t.Fatal("Prepared lifecycle changed native membership")
	}
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("ordinary owned package remains", err)
	}
}

func TestProjectGrokSDKConsentIsFrozenAndPreviewHasNoNativeEffect(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	original := deathRead(t, f.config)
	request := f.selected(t, OpRepair, false)
	prepared, err := f.engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmPluginEnable = true
	if result, err := f.engine.Apply(t.Context(), prepared, Decision{Confirmed: true}); err != nil {
		t.Fatalf("ordinary frozen repair: %+v %v", result, err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if f.native.duplex != 0 || deathRead(t, f.config) != original {
		t.Fatal("caller granted consent after Prepare")
	}
	request = f.selected(t, OpRepair, true)
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic Request
	if err := json.Unmarshal(body, &diagnostic); err != nil {
		t.Fatal(err)
	}
	if diagnostic.ConfirmPluginEnable {
		t.Fatal("diagnostic JSON granted enable consent")
	}
	unchanged := snapshotGrokSDKFiles(t, f, f.config, filepath.Join(f.project, ".uap", "state-v2.json"))
	if prepared, err := f.engine.Prepare(t.Context(), request); err == nil {
		_ = prepared.Close()
		t.Fatal("unsupported explicit consent became a preview")
	}
	unchanged()
	prepared, err = f.engine.Prepare(t.Context(), f.selected(t, OpRepair, false))
	if err != nil {
		t.Fatal(err)
	}
	if result, err := f.engine.Apply(t.Context(), prepared, Decision{}); !errors.Is(err, ErrCancelled) || result.Outcome != OutcomeCancelled {
		t.Fatalf("unconfirmed ordinary repair: %+v %v", result, err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	unchanged()
}

func TestProjectGrokSDKColdSavedPrototypeAttemptIsRetained(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	// Historical prototype state is authored explicitly. No public enablement
	// path or native success is used to manufacture this pending fixture.
	state, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := f.binding(t)
	binding.NativeActivationAttempt = "prototype-unqualified"
	state.Installations[0].Clients[binding.ClientBindingID] = binding
	if err := f.engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	f.engine = f.reopen(t, f.home)
	view, err := f.engine.Inspect(t.Context())
	if err == nil || !view.Recovery.Required || len(view.EnvironmentReceipts) != 1 || view.EnvironmentReceipts[0].EnvironmentID != "project-default" {
		t.Fatalf("cold saved observation: %+v %v", view, err)
	}
	unchanged := snapshotGrokSDKFiles(t, f, f.config, filepath.Join(f.project, ".uap", "state-v2.json"))
	for _, owner := range []string{"", "foreign-environment", "project-default"} {
		var err error
		if owner == "" {
			_, err = f.engine.Recover(t.Context(), view)
		} else {
			_, err = f.engine.RecoverEnvironment(t.Context(), view, owner)
		}
		if err == nil {
			t.Fatalf("prototype attempt was acknowledged for %q", owner)
		}
		unchanged()
	}
	for _, home := range []string{"", f.home + "-different"} {
		fresh := f.reopen(t, home)
		if _, err := fresh.RecoverEnvironment(t.Context(), view, "project-default"); err == nil {
			t.Fatal("different home recovered prototype ownership")
		}
		unchanged()
	}
	if f.binding(t).NativeActivationAttempt != binding.NativeActivationAttempt {
		t.Fatal("prototype attempt lost its original marker")
	}
}

func TestProjectGrokSDKInvalidConsentSelectionRefusesBeforeNativeOrState(t *testing.T) {
	f := newGrokEnableSDKFixture(t, false)
	f.install(t)
	for _, variant := range []string{"install", "update", "remove", "user", "implicit-scope", "missing-install", "group"} {
		t.Run(variant, func(t *testing.T) {
			req := f.selected(t, OpRepair, true)
			switch variant {
			case "install":
				req.Operation = OpInstall
			case "update":
				req.Operation = OpUpdate
			case "remove":
				req.Operation = OpRemove
			case "user":
				req.Scope = domain.ScopeUser
			case "implicit-scope":
				req.Scope = ""
			case "missing-install":
				req.InstallationID = ""
			case "group":
				req.Targets = []ClientTarget{{ClientID: "grok"}, {ClientID: "claude"}}
			}
			before, _ := f.engine.store.Load()
			calls, duplex := f.native.calls, f.native.duplex
			if prepared, err := f.engine.Prepare(t.Context(), req); err == nil {
				_ = prepared.Close()
				t.Fatal("unsupported consent selection accepted")
			}
			after, _ := f.engine.store.Load()
			if !reflect.DeepEqual(before, after) || f.native.calls != calls || f.native.duplex != duplex {
				t.Fatal("early refusal changed state or called native client")
			}
		})
	}
}
