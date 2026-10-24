package providers_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestProjectCodexUpdateKeepsNativeOwnershipAndDataBindings(t *testing.T) {
	f := newProjectCodexFixture(t)
	r := f.stage(t, domain.CompatibilityHints{})
	old, err := f.activator.Activate(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	user, b := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.b)
	projectCodexWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Updated inert project fixture\n---\nUAP-CODEX-UPDATED\n")
	mcp := projectCodexRead(t, filepath.Join(f.source, "mcp.json"))
	projectCodexWrite(t, filepath.Join(f.source, "mcp.json"), strings.ReplaceAll(mcp, "https://managed.test/mcp", "https://managed.test/v2"))
	if err := os.Rename(r.Delivery.ActivePath, r.Delivery.ActivePath+"-previous"); err != nil {
		t.Fatal(err)
	}
	next := f.stage(t, domain.CompatibilityHints{})
	next.PreviousNativeObjects, next.Replacing = old.NativeObjects, true
	updated, err := f.activator.Activate(t.Context(), next)
	if err != nil || updated.Activation != domain.ActivationPrepared || updated.Verification != domain.VerificationInstalled {
		t.Fatalf("project Codex update = %+v, %v", updated, err)
	}
	if !strings.Contains(projectCodexRead(t, filepath.Join(f.a, ".agents", "skills", "docs", "SKILL.md")), "UAP-CODEX-UPDATED") || !strings.Contains(projectCodexRead(t, filepath.Join(f.a, ".codex", "config.toml")), "managed.test/v2") {
		t.Fatal("native components did not receive the selected updated bytes")
	}
	if !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(b, projectGeminiInventory(t, f.b)) {
		t.Fatal("project update changed another scope")
	}
}

func TestProjectCodexActiveProjectionChecksExactDataLocatorWithoutWrites(t *testing.T) {
	f := newProjectCodexFixture(t)
	r := f.stage(t, domain.CompatibilityHints{})
	stager := providers.Stager{Registry: f.registry, Paths: pathpolicy.Policy{}}
	before := projectGeminiInventory(t, r.Delivery.ActivePath)
	data := filepath.Join(f.a, ".uap", "data", "owned-slot")
	objects, err := stager.ProjectActiveNative(t.Context(), f.load(t), r.Plan, r.Delivery.ArtifactDigest, data)
	if err != nil || len(objects) != 4 {
		t.Fatalf("read-only active native projection = %+v, %v", objects, err)
	}
	if _, err := stager.ProjectActiveNative(t.Context(), f.load(t), r.Plan, r.Delivery.ArtifactDigest, filepath.Join(f.a, "wrong-data")); err == nil {
		t.Fatal("active projection accepted another persistent data locator")
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, r.Delivery.ActivePath)) {
		t.Fatal("read-only active projection changed digest-covered package")
	}
}
