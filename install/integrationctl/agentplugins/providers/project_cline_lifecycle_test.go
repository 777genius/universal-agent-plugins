package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real loader/planner/stager/kernel; the client has no runner at all.
func TestProjectClineComposedFileLifecycleIgnoresAmbientOverrides(t *testing.T) {
	f := newClineProjectFixture(t, false)
	outside := filepath.Join(f.base, "ambient.json")
	projectGeminiWrite(t, outside, clineForeignConfig)
	t.Setenv("CLINE_MCP_SETTINGS_PATH", outside)
	t.Setenv("CLINE_DATA_DIR", filepath.Join(f.base, "ambient-data"))
	user, other := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
	actor := f.activator(nativeconfig.New())
	out, err := actor.Activate(t.Context(), f.request)
	clinePrepared(t, out, err)
	projectGeminiAssert(t, filepath.Join(f.project, ".cline", "skills", "docs", "SKILL.md"), clineSkill)
	body, err := os.ReadFile(f.config)
	if err != nil || !strings.Contains(string(body), "managed.test/one") || !strings.Contains(string(body), "foreign.test") || !strings.Contains(string(body), `"disabled":true`) {
		t.Fatalf("prepared Cline file: %s %v", body, err)
	}
	action := strings.Join(out.UserActions, " ")
	for _, want := range []string{"CLINE_MCP_SETTINGS_PATH", "CLINE_NO_AUTO_UPDATE", "--auto-approve", "whole MCP source", "not merged", "IDE behavior"} {
		if !strings.Contains(action, want) {
			t.Fatalf("missing truthful action %q: %s", want, action)
		}
	}
	f.request.PreviousNativeObjects, f.request.VerifyOnly = out.NativeObjects, true
	replay, err := actor.Activate(t.Context(), f.request)
	clinePrepared(t, replay, err)
	if replay.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("verify-only changed native files")
	}
	for range 2 {
		removed, err := actor.Deactivate(t.Context(), f.remove(out.NativeObjects))
		if err != nil || !removed.ExternalRemovalComplete {
			t.Fatalf("remove: %+v %v", removed, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.project, ".cline", "skills", "docs")); !os.IsNotExist(err) {
		t.Fatal("owned skill survives", err)
	}
	body, err = os.ReadFile(f.config)
	if err != nil || strings.Contains(string(body), "managed.test") || !strings.Contains(string(body), "foreign.test") {
		t.Fatalf("foreign Cline member lost: %s %v", body, err)
	}
	projectGeminiAssert(t, outside, clineForeignConfig)
	if !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
		t.Fatal("project lifecycle changed user or B")
	}
}

func TestProjectClineRejectsAttestationMalformedContextAndIncompleteDelivery(t *testing.T) {
	for _, bad := range []string{"attestation", "both-user", "other-client", "other-profile", "other-artifact", "other-active", "empty-delivery", "empty-selection", "user-object"} {
		t.Run(bad, func(t *testing.T) {
			f := newClineProjectFixture(t, false)
			before := projectGeminiInventory(t, f.base)
			switch bad {
			case "attestation":
				f.request.ActivationComplete = true
			case "both-user":
				f.request.Plan.Scope, f.request.Plan.TargetContext.Scope = domain.ScopeUser, domain.ScopeUser
			case "other-client":
				f.request.Client.ClientID = domain.ClientCodex
			case "other-profile":
				f.request.Plan.TargetContext.UserConfigRoot = f.other
			case "other-artifact":
				f.request.Plan.PhysicalArtifactID = "foreign"
			case "other-active":
				f.request.Delivery.ActivePath = f.other
			case "empty-delivery":
				f.request.Delivery = domain.StagedDelivery{}
			case "empty-selection":
				f.request.Delivery.NativeObjects = nil
			case "user-object":
				f.request.Delivery.NativeObjects[0].Kind = cline.ClineSkillObjectKind
			}
			if _, err := f.activator(nativeconfig.New()).Activate(t.Context(), f.request); err == nil {
				t.Fatal("malformed/attested project accepted")
			}
			if _, err := cline.New().Activate(t.Context(), clients.Env{NativeConfig: nativeconfig.New()}, f.request); err == nil {
				t.Fatal("direct adapter bypass")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("refused request changed files")
			}
		})
	}
	f := newClineProjectFixture(t, false)
	early := f.request
	early.Delivery = domain.StagedDelivery{}
	if err := f.activator(nativeconfig.New()).PreflightActivation(early); err != nil {
		t.Fatal("legitimate unstaged preflight refused", err)
	}
}

func TestProjectClinePreservesForeignCollisionAndChangedOwnedFiles(t *testing.T) {
	for _, tamper := range []bool{false, true} {
		t.Run(map[bool]string{false: "foreign", true: "owned-tamper"}[tamper], func(t *testing.T) {
			f := newClineProjectFixture(t, false)
			actor := f.activator(nativeconfig.New())
			if tamper {
				out, err := actor.Activate(t.Context(), f.request)
				clinePrepared(t, out, err)
				f.request.PreviousNativeObjects = out.NativeObjects
			}
			path := filepath.Join(f.project, ".cline", "skills", "docs", "SKILL.md")
			projectGeminiWrite(t, path, "outside edit")
			before := projectGeminiInventory(t, f.base)
			if _, err := actor.Activate(context.Background(), f.request); err == nil {
				t.Fatal("foreign/tampered skill accepted")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("collision changed files")
			}
		})
	}
}

func TestProjectClineStdioDataAndCurrentProjection(t *testing.T) {
	f := newClineProjectFixture(t, true)
	out, err := f.activator(nativeconfig.New()).Activate(t.Context(), f.request)
	clinePrepared(t, out, err)
	body, err := os.ReadFile(f.config)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{f.request.Plan.ActivePath, f.data} {
		if !strings.Contains(string(body), strings.ReplaceAll(want, `\`, `\\`)) {
			t.Fatalf("stdio contract missing %q: %s", want, body)
		}
	}
	for _, token := range []string{"${PLUGIN_ROOT}", "${PLUGIN_DATA}", "${package.root}"} {
		if strings.Contains(string(body), token) {
			t.Fatal("unresolved runtime token", token)
		}
	}
}

func TestProjectClineSupportsIndependentSkillAndMCPSelections(t *testing.T) {
	for _, selected := range []domain.ComponentKind{domain.ComponentSkill, domain.ComponentMCPServer} {
		t.Run(string(selected), func(t *testing.T) {
			f := newClineProjectFixture(t, false)
			for i := range f.request.Plan.Components {
				if f.request.Plan.Components[i].Kind != selected {
					f.request.Plan.Components[i].Support = domain.SupportUnsupported
				}
			}
			objects := []domain.NativeObjectOwnership{}
			for _, object := range f.request.Delivery.NativeObjects {
				skill := strings.Contains(object.Kind, "skill_directory")
				if skill == (selected == domain.ComponentSkill) {
					objects = append(objects, object)
				}
			}
			f.request.Delivery.NativeObjects = objects
			before := projectGeminiInventory(t, f.user)
			out, err := f.activator(nativeconfig.New()).Activate(t.Context(), f.request)
			clinePrepared(t, out, err)
			if selected == domain.ComponentSkill {
				projectGeminiAssert(t, f.config, clineForeignConfig)
			} else if _, err := os.Stat(filepath.Join(f.project, ".cline", "skills", "docs")); !os.IsNotExist(err) {
				t.Fatal("MCP-only delivery wrote a skill", err)
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.user)) {
				t.Fatal("single selection changed user profile")
			}
		})
	}
}
