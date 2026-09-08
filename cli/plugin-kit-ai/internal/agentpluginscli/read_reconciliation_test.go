package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestInfoReconcilesExactCopilotIdentityWithoutMutationOrPathLeak(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, DisplayName: "GitHub Copilot CLI", Status: domain.DetectionDetected,
		Version: "1.0.80", ExecutablePath: "/test/bin/copilot", ConfigRoot: filepath.Join(fixture.root, "home", ".copilot")}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{client}}
	fixture.app.Detector = detector
	observer := &capturingNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, ReceiptReconciled: true,
		NativeDiscoveryReconciled: true, NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true}}
	fixture.app.Lifecycle.NativeObserver = observer

	installationID := "00000000-0000-4000-8000-000000000001"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
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
		NativeObjects:   []domain.NativeObjectOwnership{{ObjectID: "package:copilot:" + physical, Kind: "managed_package_directory", LogicalName: "demo", ManagedDigest: digest}},
		Receipts:        []domain.MutationReceipt{{OperationID: "op-0000000000000001", Sequence: 1, ClientBindingID: bindingID, AfterDigest: digest, Phase: "committed"}},
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{
		InstallationID: installationID, DeclaredName: "demo",
		Source:  domain.SourceBinding{SourceBindingID: "src_demo", RequestedSource: "demo", CanonicalSource: "https://example.test/demo", ResolvedRevision: strings.Repeat("a", 40), TreeDigest: "sha256:tree"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "demo", Version: "9.9.9", ManifestDigest: "sha256:manifest"},
		Clients: map[string]domain.ClientBinding{bindingID: binding},
	}}}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "info", "demo", "--target", "copilot", "--format", "json")
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
		!evidence.DiscoveryOperation.Discovered || !strings.HasPrefix(evidence.DiscoveryOperation.ProductID, "demo@agentplugins-") ||
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

func TestInfoInvalidOwnershipReceiptIsInconclusive(t *testing.T) {
	binding := domain.ClientBinding{ClientBindingID: "binding", ClientID: "copilot", PhysicalArtifact: "demo-0123456789ab"}
	if hasReconciledOwnershipReceipt(binding, binding.PhysicalArtifact, "demo") {
		t.Fatal("missing ownership receipt reconciled")
	}
}

func TestInfoLegacyBindingFallsBackToInstallationVersion(t *testing.T) {
	installationID := "00000000-0000-4000-8000-000000000002"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
	target := filepath.Join(t.TempDir(), "copilot", physical)
	binding := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeUser, target, physical)
	binding.PackageRevision = nil
	installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo",
		Package: domain.PackageBinding{Version: "0.9.0"}, Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, Version: "1.0.82", ExecutablePath: "/test/bin/copilot",
	}}}
	observer := &capturingNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged}}
	app := App{Detector: detector, Lifecycle: usecase.Service{
		Targets: scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: target}}, NativeObserver: observer,
	}}
	public := publicInstallationView(installation, true)
	if err := reconcileInstalledInfo(context.Background(), app, installation, "copilot", &public); err != nil {
		t.Fatal(err)
	}
	if len(observer.plans) != 1 || observer.plans[0].DeclaredVersion != "0.9.0" {
		t.Fatalf("legacy observer plan = %+v", observer.plans)
	}
}

func TestInfoReconcilesEachSameClientBindingByExactBindingIdentity(t *testing.T) {
	installationID := "00000000-0000-4000-8000-000000000010"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
	userTarget := filepath.Join(t.TempDir(), "user", physical)
	projectTarget := filepath.Join(t.TempDir(), "project", physical)
	user := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeUser, userTarget, physical)
	project := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeProject, projectTarget, physical)
	project.Receipts = nil
	installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{
		user.ClientBindingID: user, project.ClientBindingID: project,
	}}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, Version: "1.0.80", ExecutablePath: "/test/bin/copilot",
	}}}
	app := App{Detector: detector, Lifecycle: usecase.Service{
		Targets:        scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: userTarget, domain.ScopeProject: projectTarget}},
		NativeObserver: reconciledNativeObserver{},
	}}
	public := publicInstallationView(installation, true)
	if err := reconcileInstalledInfo(context.Background(), app, installation, "copilot", &public); err != nil {
		t.Fatal(err)
	}
	if len(public.Clients) != 2 {
		t.Fatalf("clients = %+v", public.Clients)
	}
	byScope := make(map[string]publicClient, len(public.Clients))
	for _, client := range public.Clients {
		byScope[client.Scope] = client
	}
	if byScope[string(domain.ScopeUser)].ReceiptReconciled == nil || !*byScope[string(domain.ScopeUser)].ReceiptReconciled {
		t.Fatalf("user binding was not reconciled: %+v", byScope[string(domain.ScopeUser)])
	}
	if byScope[string(domain.ScopeProject)].ReceiptReconciled == nil || *byScope[string(domain.ScopeProject)].ReceiptReconciled {
		t.Fatalf("invalid project binding did not fail closed: %+v", byScope[string(domain.ScopeProject)])
	}
	if byScope[string(domain.ScopeProject)].NativeDiscoveryEvidence != nil {
		t.Fatalf("invalid project binding exposed unvalidated evidence: %+v", byScope[string(domain.ScopeProject)].NativeDiscoveryEvidence)
	}
}

func TestVSCodeInfoUsesCopilotAsAuthoritativeNativeBackend(t *testing.T) {
	tests := []struct {
		name        string
		observation domain.NativeIdentityObservation
		wantState   domain.NativeIdentityState
		wantNative  bool
	}{
		{name: "exact", observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, ReceiptReconciled: true, NativeDiscoveryReconciled: true, NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true}, wantState: domain.NativeIdentityManaged, wantNative: true},
		{name: "absent", observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, ReceiptReconciled: true, NativeDiscoveryState: domain.NativeIdentityAbsent, NativeDiscoveryAttempted: true}, wantState: domain.NativeIdentityAbsent},
		{name: "unknown", observation: domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, NativeDiscoveryState: domain.NativeIdentityIndeterminate, NativeDiscoveryAttempted: true}, wantState: domain.NativeIdentityIndeterminate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installationID := "00000000-0000-4000-8000-000000000020"
			physical := domain.ComputePhysicalArtifactID("demo", installationID)
			target := filepath.Join(t.TempDir(), "vscode", physical)
			binding := validReconciliationBinding(installationID, domain.ClientVSCode, domain.ScopeUser, target, physical)
			installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
			detector := &observedProbingDetector{clients: []domain.DetectedClient{
				{ClientID: domain.ClientVSCode, Status: domain.DetectionDetected, Version: "9.9.9", ExecutablePath: "/test/bin/code", ConfigRoot: "/test/config/code"},
				{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, Version: "1.0.80", ExecutablePath: "/test/bin/copilot", ConfigRoot: "/test/config/copilot"},
			}}
			observer := &capturingNativeObserver{observation: test.observation}
			app := App{Detector: detector, Lifecycle: usecase.Service{
				Targets: scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: target}}, NativeObserver: observer,
			}}
			public := publicInstallationView(installation, true)
			if err := reconcileInstalledInfo(context.Background(), app, installation, "vscode", &public); err != nil {
				t.Fatal(err)
			}
			if detector.targetedCalls != 1 || len(detector.targets) != 1 || detector.targets[0] != domain.ClientCopilot {
				t.Fatalf("targeted probes = %v", detector.targets)
			}
			if len(observer.clients) != 1 || observer.clients[0].ClientID != domain.ClientVSCode || observer.clients[0].ExecutablePath != "/test/bin/copilot" || observer.clients[0].ConfigRoot != "/test/config/copilot" {
				t.Fatalf("observer clients = %+v", observer.clients)
			}
			got := public.Clients[0]
			if got.ClientVersion != "1.0.80" || got.NativeIdentityState != test.wantState || got.NativeDiscoveryReconciled == nil || *got.NativeDiscoveryReconciled != test.wantNative {
				t.Fatalf("VS Code reconciliation = %+v", got)
			}
			if got.NativeDiscoveryEvidence == nil || got.NativeDiscoveryEvidence.DiscoveryOperation.Discovered != test.wantNative {
				t.Fatalf("VS Code evidence = %+v", got.NativeDiscoveryEvidence)
			}
		})
	}
}

func TestInfoFindsSharedBindingThroughEitherAffectedSurface(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		owner     domain.ClientID
		requested domain.ClientID
	}{
		{name: "copilot owner requested as vscode", owner: domain.ClientCopilot, requested: domain.ClientVSCode},
		{name: "vscode owner requested as copilot", owner: domain.ClientVSCode, requested: domain.ClientCopilot},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			installationID := "00000000-0000-4000-8000-000000000025"
			physical := domain.ComputePhysicalArtifactID("demo", installationID)
			target := filepath.Join(t.TempDir(), string(test.owner), physical)
			binding := validReconciliationBinding(installationID, test.owner, domain.ScopeUser, target, physical)
			binding.AffectedSurfaces = []string{"copilot", "vscode"}
			installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
			detector := &observedProbingDetector{clients: []domain.DetectedClient{{
				ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, Version: "1.0.80",
				ExecutablePath: "/test/bin/copilot", ConfigRoot: "/test/config/copilot",
			}}}
			observer := &capturingNativeObserver{observation: domain.NativeIdentityObservation{
				State: domain.NativeIdentityManaged, ReceiptReconciled: true, NativeDiscoveryReconciled: true,
				NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true,
			}}
			app := App{Detector: detector, Lifecycle: usecase.Service{
				Targets: scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: target}}, NativeObserver: observer,
			}}
			public := publicInstallationView(installation, true)
			if err := reconcileInstalledInfo(context.Background(), app, installation, string(test.requested), &public); err != nil {
				t.Fatal(err)
			}
			if len(public.Clients) != 1 || public.Clients[0].ClientID != string(test.owner) ||
				public.Clients[0].ReceiptReconciled == nil || !*public.Clients[0].ReceiptReconciled ||
				public.Clients[0].NativeDiscoveryReconciled == nil || !*public.Clients[0].NativeDiscoveryReconciled ||
				!reflect.DeepEqual(public.Clients[0].AffectedSurfaces, []string{"copilot", "vscode"}) {
				t.Fatalf("shared info = %+v", public.Clients)
			}
			if detector.targetedCalls != 1 || !reflect.DeepEqual(detector.targets, []domain.ClientID{domain.ClientCopilot}) {
				t.Fatalf("shared info probes = %+v", detector.targets)
			}
			if len(observer.clients) != 1 || observer.clients[0].ClientID != test.owner ||
				observer.clients[0].ExecutablePath != "/test/bin/copilot" || observer.clients[0].Version != "1.0.80" {
				t.Fatalf("shared observer client = %+v", observer.clients)
			}
		})
	}
}

func TestInfoSharedBindingDoesNotMatchUnrelatedExplicitTarget(t *testing.T) {
	installationID := "00000000-0000-4000-8000-000000000026"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
	target := filepath.Join(t.TempDir(), "copilot", physical)
	binding := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeUser, target, physical)
	binding.AffectedSurfaces = []string{"copilot", "vscode"}
	installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{{ClientID: domain.ClientCursor, Status: domain.DetectionDetected, Version: "1.0.0"}}}
	observer := &capturingNativeObserver{}
	public := publicInstallationView(installation, true)
	if err := reconcileInstalledInfo(context.Background(), App{Detector: detector, Lifecycle: usecase.Service{NativeObserver: observer}}, installation, "cursor", &public); err != nil {
		t.Fatal(err)
	}
	if len(public.Clients) != 0 || len(observer.clients) != 0 {
		t.Fatalf("unrelated explicit target matched shared binding: clients=%+v observations=%+v", public.Clients, observer.clients)
	}
}

func TestInfoWithoutTargetDoesNotDetectOrProbeClients(t *testing.T) {
	detector := &observedProbingDetector{}
	public := publicInstallation{}
	if err := reconcileInstalledInfo(context.Background(), App{Detector: detector}, domain.Installation{}, "", &public); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 0 || detector.probeCalls != 0 || detector.targetedCalls != 0 {
		t.Fatalf("unexpected detector calls: %+v", detector)
	}
}

func TestInfoFailsClosedBeforeNativeDiscoveryWithoutAuthoritativeClientVersion(t *testing.T) {
	installationID := "00000000-0000-4000-8000-000000000030"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
	target := filepath.Join(t.TempDir(), "copilot", physical)
	binding := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeUser, target, physical)
	installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ExecutablePath: "/test/bin/copilot",
	}}}
	observer := &capturingNativeObserver{observation: domain.NativeIdentityObservation{
		State: domain.NativeIdentityManaged, ReceiptReconciled: true, NativeDiscoveryReconciled: true,
		NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true,
	}}
	app := App{Detector: detector, Lifecycle: usecase.Service{
		Targets: scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: target}}, NativeObserver: observer,
	}}
	public := publicInstallationView(installation, true)
	if err := reconcileInstalledInfo(context.Background(), app, installation, "copilot", &public); err != nil {
		t.Fatal(err)
	}
	got := public.Clients[0]
	if got.ClientVersion != "" || got.NativeIdentityState != domain.NativeIdentityIndeterminate ||
		got.ReceiptReconciled == nil || *got.ReceiptReconciled || got.NativeDiscoveryReconciled == nil || *got.NativeDiscoveryReconciled ||
		got.NativeDiscoveryEvidence != nil {
		t.Fatalf("missing-version reconciliation did not fail closed: %+v", got)
	}
	if len(observer.clients) != 0 {
		t.Fatalf("native discovery ran without authoritative version: %+v", observer.clients)
	}
}

func TestInfoOmitsNativeEvidenceWhenDiscoveryTimesOut(t *testing.T) {
	installationID := "00000000-0000-4000-8000-000000000040"
	physical := domain.ComputePhysicalArtifactID("demo", installationID)
	target := filepath.Join(t.TempDir(), "copilot", physical)
	binding := validReconciliationBinding(installationID, domain.ClientCopilot, domain.ScopeUser, target, physical)
	installation := domain.Installation{InstallationID: installationID, DeclaredName: "demo", Clients: map[string]domain.ClientBinding{binding.ClientBindingID: binding}}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, Version: "1.0.80", ExecutablePath: "/test/bin/copilot",
	}}}
	observer := &capturingNativeObserver{
		observation: domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, NativeDiscoveryState: domain.NativeIdentityIndeterminate, NativeDiscoveryAttempted: true},
		err:         context.DeadlineExceeded,
	}
	app := App{Detector: detector, Lifecycle: usecase.Service{
		Targets: scopeTargetResolver{targets: map[domain.InstallScope]string{domain.ScopeUser: target}}, NativeObserver: observer,
	}}
	public := publicInstallationView(installation, true)
	if err := reconcileInstalledInfo(context.Background(), app, installation, "copilot", &public); err != nil {
		t.Fatal(err)
	}
	got := public.Clients[0]
	if got.NativeIdentityState != domain.NativeIdentityIndeterminate || got.NativeDiscoveryEvidence != nil {
		t.Fatalf("timed-out reconciliation exposed false evidence: %+v", got)
	}
	if !errors.Is(observer.err, context.DeadlineExceeded) {
		t.Fatalf("observer err = %v", observer.err)
	}
}

func validReconciliationBinding(installationID string, client domain.ClientID, scope domain.InstallScope, target, physical string) domain.ClientBinding {
	bindingID := domain.ComputeClientBindingID(installationID, string(client), string(scope), target)
	digest := "sha256:owned"
	return domain.ClientBinding{
		ClientBindingID: bindingID, ClientID: string(client), Scope: string(scope), TargetLocator: target, PhysicalArtifact: physical,
		Materialization: domain.MaterializationMaterialized,
		PackageRevision: &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest"},
		NativeObjects:   []domain.NativeObjectOwnership{{ObjectID: "package:" + string(client) + ":" + physical, Kind: "managed_package_directory", LogicalName: "demo", ManagedDigest: digest}},
		Receipts:        []domain.MutationReceipt{{OperationID: "op-" + bindingID[:12], Sequence: 1, ClientBindingID: bindingID, AfterDigest: digest, Phase: "committed"}},
	}
}

type scopeTargetResolver struct {
	targets map[domain.InstallScope]string
}

func (resolver scopeTargetResolver) ResolveTarget(_ context.Context, client domain.DetectedClient, scope domain.InstallScope, _ string) (domain.DeliveryTarget, error) {
	active := resolver.targets[scope]
	return domain.DeliveryTarget{TargetAnchor: filepath.Dir(filepath.Dir(active)), TargetRoot: filepath.Dir(active), ActivePath: active}, nil
}

type capturingNativeObserver struct {
	observation domain.NativeIdentityObservation
	clients     []domain.DetectedClient
	plans       []domain.DeliveryPlan
	err         error
}

func (observer *capturingNativeObserver) ObserveNativeIdentity(_ context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, _ *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observer.clients = append(observer.clients, client)
	observer.plans = append(observer.plans, plan)
	return observer.observation, observer.err
}

type reconciledNativeObserver struct{}

func (reconciledNativeObserver) ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, ReceiptReconciled: true, NativeDiscoveryReconciled: true,
		NativeDiscoveryState: domain.NativeIdentityManaged, NativeDiscoveryAttempted: true}, nil
}
