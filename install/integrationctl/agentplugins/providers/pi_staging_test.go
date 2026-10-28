package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

func TestPiStageDiscardUsesOutsideDiscoveryAnchor(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "source")
	profile := filepath.Join(root, "profile")
	if err := os.MkdirAll(filepath.Join(source, "native"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"plugin.json": `{"name":"test-pi","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`, "native/index.ts": "export default function(pi) {}\n"} {
		if err := os.WriteFile(filepath.Join(source, filepath.FromSlash(name)), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	digest, err := (packagesnapshot.Builder{}).Digest(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{SnapshotRoot: source, TreeDigest: digest, Manifest: domain.PluginManifest{Name: "test-pi", Extensions: map[string]json.RawMessage{pi.Namespace: json.RawMessage(`{"version":1,"source":"native"}`)}}}
	plan := domain.DeliveryPlan{ClientID: domain.ClientPi, Scope: domain.ScopeUser, Status: domain.PlanReady, PackageMode: domain.PackageNative, NativeRegistryRoot: profile, TargetAnchor: profile, TargetRoot: filepath.Join(profile, "extensions"), PhysicalArtifactID: "test-pi-123456789abc", ActivePath: filepath.Join(profile, "extensions", "test-pi-123456789abc"), Components: []domain.ComponentDecision{{Kind: domain.ComponentExtension, Name: pi.Namespace, Support: domain.SupportNative}}}
	registry, _ := clients.NewRegistry(pi.New())
	stager := Stager{Registry: registry, Paths: pathpolicy.Policy{}, SnapshotBuilder: packagesnapshot.Builder{}}
	delivery, err := stager.StageWithPluginData(context.Background(), envelope, plan, "TEST-pi-discard", domain.CompatibilityHints{}, filepath.Join(root, "data"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(delivery.StagingPath) != profile {
		t.Fatal("staged under discovery root")
	}
	if err := stager.Discard(context.Background(), delivery); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(delivery.StagingPath); !os.IsNotExist(err) {
		t.Fatalf("discard left staging: %v", err)
	}
}
