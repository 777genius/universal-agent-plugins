package domain

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetContextKeepsOperationalRootsPrivate(t *testing.T) {
	root := t.TempDir()
	target, err := NewTargetContext(ScopeProject, root, filepath.Join(root, "user-profile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{target, DeliveryPlan{Scope: ScopeProject, TargetContext: target},
		PlanRequest{ProjectRoot: root}, DeactivationRequest{TargetContext: target}} {
		body, err := json.Marshal(value)
		if err != nil || strings.Contains(string(body), root) {
			t.Fatalf("operational root escaped private projection: %s, %v", body, err)
		}
	}
}

func TestTargetContextRejectsAmbiguousScopeRoots(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		scope InstallScope
		root  string
	}{{ScopeUser, root}, {ScopeProject, ""}, {ScopeProject, "."}, {ScopeProject, filepath.VolumeName(root) + string(filepath.Separator)}, {"unknown", root}} {
		if _, err := NewTargetContext(test.scope, test.root, ""); err == nil {
			t.Fatalf("accepted scope %q with root %q", test.scope, test.root)
		}
	}
	if target, err := NewTargetContext("", "", ""); err != nil || target.Scope != ScopeUser {
		t.Fatalf("historical empty scope did not retain user default: %+v, %v", target, err)
	}
}

func TestProjectPolicyRequiresQualifiedDeclaration(t *testing.T) {
	selected := SelectedDelivery{}
	if got := selected.EffectiveTraitsForScope(ClientCodex, ScopeUser); got.LifecycleKind != ClientTraitsFor(ClientCodex).LifecycleKind {
		t.Fatal("user lifecycle policy changed")
	}
	if got := selected.EffectiveTraitsForScope(ClientID("unqualified-fixture"), ScopeProject); got.LifecycleKind != "" {
		t.Fatal("unqualified project borrowed user lifecycle policy")
	}
	if SameClientBackendForScope(ClientCopilot, ClientVSCode, ScopeProject) ||
		!SameClientBackendForScope(ClientCopilot, ClientVSCode, ScopeUser) {
		t.Fatal("project sibling isolation or historical user sharing changed")
	}
	definition := withProjectScope(ClientDefinition{ID: "fixture", Capabilities: ClientCapabilities{Scopes: []InstallScope{ScopeUser}}}, LifecycleNativeConfig)
	if !definition.ProjectPolicy.Valid() || definition.ProjectPolicy.BackendFamily != "fixture" || len(definition.Capabilities.Scopes) != 2 {
		t.Fatalf("qualified table helper lost policy: %+v", definition)
	}
}
