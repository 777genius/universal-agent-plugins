package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Codex 0.162 prefers a root Agent Plugins manifest over .codex-plugin. When the
// managed artifact kept it, Codex ignored .mcp.json and wrote plugin data into
// its own profile, so `remove --purge-data` reported a purge that left data.
func TestCodexUserProjectionDropsPortableManifests(t *testing.T) {
	staging := tempRoot(t)
	for _, name := range []string{"plugin.json", "mcp.json"} {
		if err := os.WriteFile(filepath.Join(staging, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, Scope: domain.ScopeUser, PackageMode: domain.PackageProjection,
		PhysicalArtifactID: "demo-0123456789ab", DeclaredName: "demo", DeclaredVersion: "1.0.0", ActivePath: staging}
	envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"}}
	if _, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: staging, Envelope: envelope, Plan: plan}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plugin.json", "mcp.json"} {
		if _, err := os.Stat(filepath.Join(staging, name)); !os.IsNotExist(err) {
			t.Fatalf("portable %s shadows the Codex projection: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(staging, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatalf("Codex manifest missing: %v", err)
	}
}
