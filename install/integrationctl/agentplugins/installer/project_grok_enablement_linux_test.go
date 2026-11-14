//go:build linux

package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectGrokSDKExplicitRepairOwnsMemberAndRemovesOnlyThatMember(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	original, binding := deathRead(t, f.config), f.binding(t)
	if f.native.duplex != 0 || binding.Activation != domain.ActivationPrepared {
		t.Fatal("ordinary install granted native enablement")
	}
	request := f.selected(t, OpRepair, true)
	prepared, err := f.engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	if plan := prepared.Plan(); plan.NoChange || !plan.RequiresConfirmation || f.native.duplex != 0 {
		t.Fatalf("explicit enable preview: %+v", plan)
	}
	request.ConfirmPluginEnable = false
	request.EnvironmentAuthority.EnvironmentID = "foreign-after-prepare"
	result, err := f.engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("consented public repair: %+v %v", result, err)
	}
	enabled := f.binding(t)
	if enabled.Activation != domain.ActivationPrepared || enabled.NativeActivationAttempt != "" || len(enabled.NativeObjects) != len(binding.NativeObjects)+1 || f.native.duplex != 2 {
		t.Fatalf("durable prepared ownership: %+v ACP=%d", enabled, f.native.duplex)
	}
	owned := enabled.NativeObjects[len(enabled.NativeObjects)-1]
	if owned.Kind != "grok_enabled_plugin" || owned.Path != binding.TargetLocator {
		t.Fatalf("wrong ownership: %+v", owned)
	}
	config := deathRead(t, f.config)
	if config == original || !strings.Contains(config, "'foreign'") || !strings.Contains(config, "disabled=['other']") {
		t.Fatal("native enabled membership lost original settings")
	}
	result, err = f.execute(t, f.selected(t, OpRepair, true), Decision{Confirmed: true})
	if err != nil || f.native.duplex != 3 || deathRead(t, f.config) != config {
		t.Fatalf("repeated public consent: %+v %v ACP=%d", result, err, f.native.duplex)
	}
	// A foreign editor can append another member after enablement. Removal
	// uses the exact owned full ID against this current document.
	foreign := strings.Replace(config, "enabled=[", "enabled=['later',", 1)
	deathWrite(t, f.config, foreign)
	result, err = f.execute(t, f.selected(t, OpRemove, false), Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("owned public removal: %+v %v", result, err)
	}
	if got := deathRead(t, f.config); got != strings.Replace(original, "enabled=[", "enabled=['later',", 1) {
		t.Fatalf("foreign config changed on remove: %s", got)
	}
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("owned package remains after native membership removal", err)
	}
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("completed public lifecycle retained recovery: %+v %v", view, err)
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
	prepared, err = f.engine.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := f.engine.Apply(t.Context(), prepared, Decision{}); !errors.Is(err, ErrCancelled) || result.Outcome != OutcomeCancelled {
		t.Fatalf("unconfirmed repair: %+v %v", result, err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if f.native.duplex != 0 || deathRead(t, f.config) != original {
		t.Fatal("preview or canceled Apply changed native state")
	}
}

func TestProjectGrokSDKColdSavedRecoveryRequiresSameEnvironmentAndHome(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	original := deathRead(t, f.config)
	req := f.selected(t, OpRepair, true)
	f.native.wrongReadback = true
	if result, err := f.execute(t, req, Decision{Confirmed: true}); err == nil {
		t.Fatalf("unproved native readback accepted: %+v", result)
	}
	if f.native.duplex != 2 || f.binding(t).NativeActivationAttempt == "" {
		t.Fatal("missing pending attempt after native failure")
	}
	f.engine = f.reopen(t, f.home)
	view, err := f.engine.Inspect(t.Context())
	if err != nil || !view.Recovery.Required || len(view.Recovery.Journals) != 1 || view.Recovery.Journals[0].EnvironmentID != "project-default" {
		t.Fatalf("cold saved observation: %+v %v", view, err)
	}
	native, state := deathRead(t, f.config), deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json"))
	for _, owner := range []string{"", "foreign-environment"} {
		var err error
		if owner == "" {
			_, err = f.engine.Recover(t.Context(), view)
		} else {
			_, err = f.engine.RecoverEnvironment(t.Context(), view, owner)
		}
		if !errors.Is(err, ErrRecoveryRequired) || deathRead(t, f.config) != native || deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json")) != state {
			t.Fatalf("foreign recovery changed authority: %s %v", owner, err)
		}
	}
	for _, home := range []string{"", f.home + "-different"} {
		fresh := f.reopen(t, home)
		if _, err := fresh.RecoverEnvironment(t.Context(), view, "project-default"); err == nil {
			t.Fatal("different constructor home recovered native ownership")
		}
		if deathRead(t, f.config) != native || deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json")) != state {
			t.Fatal("different constructor home changed pending state")
		}
	}
	result, err := f.engine.RecoverEnvironment(t.Context(), view, "project-default")
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("exact cold saved recovery: %+v %v", result, err)
	}
	if deathRead(t, f.config) != original || f.binding(t).NativeActivationAttempt != "" || f.native.duplex != 2 {
		t.Fatal("cold recovery did not restore exact prior state without native execution")
	}
	after, err := f.engine.Inspect(t.Context())
	if err != nil || after.Recovery.Required {
		t.Fatalf("cold recovery incomplete: %+v %v", after, err)
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
