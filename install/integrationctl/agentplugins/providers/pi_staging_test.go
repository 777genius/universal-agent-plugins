package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

func piGoldenStagingFixture(t *testing.T, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (domain.PackageEnvelope, domain.DeliveryPlan, string) {
	t.Helper()
	plan.PackageMode = domain.PackageNative
	plan.NativeRegistryRoot = plan.TargetAnchor
	plan.TargetRoot = filepath.Join(plan.TargetAnchor, "extensions")
	plan.ActivePath = filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentExtension, Name: pi.Namespace, Support: domain.SupportNative}}
	envelope.Manifest.Extensions = map[string]json.RawMessage{pi.Namespace: json.RawMessage(`{"version":1,"source":"native"}`)}
	writeTestFile(t, filepath.Join(envelope.SnapshotRoot, "native", "index.ts"), "export default function(pi) {}\n")
	envelope.TreeDigest = "sha256:" + strings.Repeat("a", 64)
	return envelope, plan, filepath.Join(plan.TargetAnchor, "TEST-data")
}

func piGoldenActivationRequest(t *testing.T, root string) domain.ActivationRequest {
	t.Helper()
	profile := filepath.Join(root, "pi", "config")
	source := filepath.Join(root, "pi", "source")
	writeTestFile(t, filepath.Join(source, "plugin.json"), `{"name":"demo","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`)
	writeTestFile(t, filepath.Join(source, "native", "index.ts"), "export default function(pi) {}\n")
	plan := domain.DeliveryPlan{ClientID: domain.ClientPi, Scope: domain.ScopeUser, Status: domain.PlanReady, PackageMode: domain.PackageNative, DeclaredName: "demo", DeclaredVersion: "1.0.0", Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, TargetAnchor: profile, NativeRegistryRoot: profile, TargetRoot: filepath.Join(profile, "extensions"), PhysicalArtifactID: "demo-0123456789ab", Components: []domain.ComponentDecision{{Kind: domain.ComponentExtension, Name: pi.Namespace, Support: domain.SupportNative}}}
	plan.ActivePath = filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID)
	envelope := domain.PackageEnvelope{SnapshotRoot: source, TreeDigest: "sha256:" + strings.Repeat("a", 64), Manifest: domain.PluginManifest{Name: "demo", Extensions: map[string]json.RawMessage{pi.Namespace: json.RawMessage(`{"version":1,"source":"native"}`)}}}
	delivery, err := testStager(Stager{}).StageWithPluginData(context.Background(), envelope, plan, "TEST-golden-pi", domain.CompatibilityHints{}, filepath.Join(root, "pi", "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.TargetRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	return domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientPi, ConfigRoot: profile, Status: domain.DetectionDetected}, DeclaredName: "demo", Plan: plan, Delivery: delivery}
}

func piGoldenActivationOutcome(out domain.ActivationOutcome) domain.ActivationOutcome {
	for i := range out.NativeObjects {
		if out.NativeObjects[i].Kind == "managed_package_directory" {
			out.NativeObjects[i].ManagedDigest = "${ARTIFACT}"
		}
	}
	return out
}

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
