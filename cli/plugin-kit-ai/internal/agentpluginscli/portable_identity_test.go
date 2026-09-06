package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableReservedInfoReconciliation(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, DisplayName: "GitHub Copilot CLI", Status: domain.DetectionDetected,
		Version: "1.0.80", ExecutablePath: "/test/bin/copilot", ConfigRoot: filepath.Join(fixture.root, "home", ".copilot")}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{client}}
	fixture.app.Detector = detector
	observer := &capturingNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, ReceiptReconciled: true,
		NativeDiscoveryReconciled: true, NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true}}
	fixture.app.Lifecycle.NativeObserver = observer

	installationID := "00000000-0000-4000-8000-000000000001"
	physical := domain.ComputePhysicalArtifactID("con.foo", installationID)
	target, err := fixture.app.Lifecycle.Targets.ResolveTarget(context.Background(), client, domain.ScopeUser, physical)
	if err != nil {
		t.Fatal(err)
	}
	bindingID := domain.ComputeClientBindingID(installationID, string(domain.ClientCopilot), string(domain.ScopeUser), target.ActivePath)
	digest := "sha256:owned"
	binding := domain.ClientBinding{
		ClientBindingID: bindingID, ClientID: string(domain.ClientCopilot), Scope: string(domain.ScopeUser), TargetLocator: target.ActivePath,
		PhysicalArtifact: physical, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationActive,
		Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled,
		PackageRevision: &domain.ClientPackageRevision{Version: "1.7.0-uap.1", TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest"},
		NativeObjects:   []domain.NativeObjectOwnership{{ObjectID: "package:copilot:" + physical, Kind: "managed_package_directory", LogicalName: "con.foo", ManagedDigest: digest}},
		Receipts:        []domain.MutationReceipt{{OperationID: "op-0000000000000001", Sequence: 1, ClientBindingID: bindingID, AfterDigest: digest, Phase: "committed"}},
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{
		InstallationID: installationID, DeclaredName: "con.foo",
		Source:  domain.SourceBinding{SourceBindingID: "src_con.foo", RequestedSource: "con.foo", CanonicalSource: "https://example.test/con.foo", ResolvedRevision: strings.Repeat("a", 40), TreeDigest: "sha256:tree"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "con.foo", Version: "9.9.9", ManifestDigest: "sha256:manifest"},
		Clients: map[string]domain.ClientBinding{bindingID: binding},
	}}}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "info", "con.foo", "--target", "copilot", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(fixture.store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("info mutated state: %v", err)
	}
	if strings.Contains(stdout, fixture.root) || strings.Contains(stdout, client.ExecutablePath) || strings.Contains(stdout, client.ConfigRoot) {
		t.Fatalf("info leaked a local path: %s", stdout)
	}
	var output struct {
		SchemaVersion int `json:"schema_version"`
		Data          struct {
			Clients []struct {
				ReceiptReconciled         bool   `json:"receipt_reconciled"`
				NativeDiscoveryReconciled bool   `json:"native_discovery_reconciled"`
				NativeIdentityState       string `json:"native_identity_state"`
				ClientVersion             string `json:"client_version"`
				NativeDiscoveryEvidence   struct {
					Basis            string `json:"basis"`
					VersionOperation struct {
						Argv                  []string `json:"argv"`
						ObservedClientVersion string   `json:"observed_client_version"`
					} `json:"version_operation"`
					DiscoveryOperation struct {
						Argv       []string `json:"argv"`
						Discovered bool     `json:"discovered"`
						ProductID  string   `json:"product_id"`
					} `json:"discovery_operation"`
				} `json:"native_discovery_evidence"`
			} `json:"clients"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if output.SchemaVersion != 1 || len(output.Data.Clients) != 1 {
		t.Fatalf("output = %s", stdout)
	}
	got := output.Data.Clients[0]
	if !got.ReceiptReconciled || !got.NativeDiscoveryReconciled || got.NativeIdentityState != "managed" || got.ClientVersion != "1.0.80" {
		t.Fatalf("reconciliation = %+v", got)
	}
	evidence := got.NativeDiscoveryEvidence
	if evidence.Basis != "native_client_command" || evidence.VersionOperation.ObservedClientVersion != "1.0.80" ||
		!evidence.DiscoveryOperation.Discovered || !strings.HasPrefix(evidence.DiscoveryOperation.ProductID, "con.foo@agentplugins-") ||
		strings.Join(evidence.VersionOperation.Argv, " ") != "copilot --version" || strings.Join(evidence.DiscoveryOperation.Argv, " ") != "copilot plugin list" {
		t.Fatalf("native discovery evidence = %+v", evidence)
	}
	if detector.targetedCalls != 1 || detector.probeCalls != 0 || detector.readOnlyCalls != 0 || len(detector.targets) != 1 || detector.targets[0] != domain.ClientCopilot {
		t.Fatalf("detector calls = read-only:%d probe:%d targeted:%d targets:%v", detector.readOnlyCalls, detector.probeCalls, detector.targetedCalls, detector.targets)
	}
	if len(observer.plans) != 1 || observer.plans[0].DeclaredVersion != "1.7.0-uap.1" {
		t.Fatalf("observer plan did not use binding revision: %+v", observer.plans)
	}
}
