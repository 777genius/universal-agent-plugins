package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RED: empty identity/projection incorrectly requests a codec and consults
// ambiguous user config. No desired objects must still reconcile stored ones.
func TestOpenCodeMetadataOnlyNoAuthorityOrConfigSelection(t *testing.T) {
	assertOpenCodeNoEffectPlan(t, domain.PackageEnvelope{}, nil)
}

// RED if the decoder treats every MCP declaration as selected and requests a
// host/config even when the public component plan explicitly excludes it.
func TestOpenCodeUnselectedMCPNoAuthorityOrConfigSelection(t *testing.T) {
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Type: "streamable-http", Decoded: map[string]any{"url": "https://docs.test"}}}}}
	components := []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportUnsupported}}
	assertOpenCodeNoEffectPlan(t, envelope, components)
}

func assertOpenCodeNoEffectPlan(t *testing.T, envelope domain.PackageEnvelope, components []domain.ComponentDecision) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("foreign"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan := domain.DeliveryPlan{NativeRegistryRoot: root, ActivePath: root, Components: components}
	finding, err := InspectOpenCodeRegistry(plan, nil, nativeconfig.New())
	if err != nil || finding != clients.RegistryClear {
		t.Fatalf("empty identity: %v %v", finding, err)
	}
	objects, err := New().Project(context.Background(), clients.ProjectionInput{StagingPath: root, Envelope: envelope, Plan: plan})
	if err != nil || len(objects) != 0 {
		t.Fatalf("empty project: %+v %v", objects, err)
	}
	objects, err = BuildOpenCodeNativeObjects(root, envelope, plan)
	if err != nil || len(objects) != 0 {
		t.Fatalf("empty active objects: %+v %v", objects, err)
	}
	if _, err := os.Stat(filepath.Join(root, OpenCodeProjectionFile)); !os.IsNotExist(err) {
		t.Fatalf("empty project wrote projection: %v", err)
	}
	if err := ApplyOpenCodeNative(root, root, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(body) != "foreign" {
			t.Fatalf("no effects changed %s: %s %v", name, body, err)
		}
	}
}

// RED if desired skills can bypass the missing-authority gate independently
// of MCP codec selection. Dry-run planning must refuse this honestly.
func TestOpenCodeDesiredSkillWithoutAuthorityRefuses(t *testing.T) {
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportProjected}}}
	err := New().RefinePlan(context.Background(), clients.PlanInput{}, &plan)
	if err == nil {
		t.Fatal("desired skill plan accepted missing authority")
	}
}

// Preserve the existing desired-projection fixture's refusal: an omitted native
// component plan cannot be relabeled as a metadata-only package and succeed.
func TestOpenCodeNativePackageWithoutComponentPlanRefuses(t *testing.T) {
	root := t.TempDir()
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": {Type: "streamable-http", Decoded: map[string]any{"url": "https://docs.test"}}}}}
	plan := domain.DeliveryPlan{NativeRegistryRoot: root, ActivePath: root}
	if err := ProjectOpenCodeNative(root, envelope, plan, ""); err == nil {
		t.Fatal("omitted native plan reported metadata-only success")
	}
	if _, err := BuildOpenCodeNativeObjects(root, envelope, plan); err == nil {
		t.Fatal("object reader accepted omitted native plan")
	}
	if _, err := os.Stat(filepath.Join(root, OpenCodeProjectionFile)); !os.IsNotExist(err) {
		t.Fatalf("omitted plan wrote projection: %v", err)
	}
}
