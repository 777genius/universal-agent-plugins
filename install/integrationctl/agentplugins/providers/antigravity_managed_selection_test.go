package providers

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAntigravityActualStagedMCPSelection(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			envelope := stagingEnvelope(t)
			plan := stagingPlan(t, domain.ClientAntigravity, domain.PackageNative)
			plan.Scope = scope
			plan.Components = plan.Components[1:2] // The selected delivery is MCP-only.
			base := tempRoot(t)
			plan.TargetContext = domain.TargetContext{Scope: scope, UserConfigRoot: filepath.Join(base, ".gemini", "config")}
			plan.NativeRegistryRoot = plan.TargetContext.UserConfigRoot
			if scope == domain.ScopeProject {
				plan.TargetContext.ProjectRoot = base
				plan.NativeRegistryRoot = filepath.Join(base, ".agents")
			}
			plan.TargetAnchor = plan.NativeRegistryRoot
			plan.TargetRoot = filepath.Join(plan.NativeRegistryRoot, "plugins")
			plan.ActivePath = filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID)
			stager := testStager(Stager{})
			delivery, err := stager.Stage(t.Context(), envelope, plan, "native-selection", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			assertMissing(t, filepath.Join(delivery.StagingPath, "mcp.json"))
			for _, read := range []func() ([]string, error){
				func() ([]string, error) {
					return stager.ManagedMCPNamesForScope(t.Context(), domain.ClientAntigravity, scope, delivery.StagingPath, delivery.ArtifactDigest)
				},
				func() ([]string, error) {
					return stager.ManagedMCPNames(t.Context(), domain.ClientAntigravity, delivery.StagingPath, delivery.ArtifactDigest)
				},
			} {
				names, err := read()
				if err != nil || !reflect.DeepEqual(names, []string{"notion"}) {
					t.Fatalf("actual native delivery lost recorded MCP selection: names=%v err=%v", names, err)
				}
			}
			writeTestFile(t, filepath.Join(delivery.StagingPath, "mcp_config.json"), `{"mcpServers":{"foreign":{"serverUrl":"https://foreign.test/mcp"}}}`)
			if _, err := stager.ManagedMCPNamesForScope(t.Context(), domain.ClientAntigravity, scope, delivery.StagingPath, delivery.ArtifactDigest); err == nil {
				t.Fatal("changed native selection bypassed the recorded artifact digest")
			}
		})
	}
}
