package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func TestGroupedAddUpdateAndRemoveUseOneCommitDecision(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}

	cursorAdd := addInput(t, cursor, "https://example.com/grouped")
	kiroAdd := addInput(t, kiro, "https://example.com/grouped")
	added, err := service.AddGroup(context.Background(), GroupInput{
		Targets: []AddInput{cursorAdd, kiroAdd}, OperationGroupID: "group-add", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !added.Mutated || len(added.Receipts) != 2 {
		t.Fatalf("grouped add = %+v", added)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 || state.Installations[0].OperationGroupID != "group-add" {
		t.Fatalf("grouped add state = %+v", state.Installations)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != "sha256:source-tree" || len(binding.Receipts) != 1 || binding.Receipts[0].OperationGroupID != "group-add" {
			t.Fatalf("grouped add binding = %+v", binding)
		}
	}

	cursorUpdate := addInput(t, cursor, "https://example.com/grouped")
	kiroUpdate := addInput(t, kiro, "https://example.com/grouped")
	setEnvelopeVersion(t, &cursorUpdate.Envelope, "2.0.0", "sha256:group-tree-v2", "sha256:group-manifest-v2")
	setEnvelopeVersion(t, &kiroUpdate.Envelope, "2.0.0", "sha256:group-tree-v2", "sha256:group-manifest-v2")
	updated, err := service.UpdateGroup(context.Background(), GroupInput{
		Targets: []AddInput{cursorUpdate, kiroUpdate}, CompatibilityChecks: []AddInput{cursorUpdate, kiroUpdate},
		OperationGroupID: "group-update", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Mutated || len(updated.Receipts) != 2 {
		t.Fatalf("grouped update = %+v", updated)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.PackageRevision == nil || binding.PackageRevision.Version != "2.0.0" || binding.PackageRevision.TreeDigest != "sha256:group-tree-v2" || len(binding.Receipts) != 2 {
			t.Fatalf("grouped update binding = %+v", binding)
		}
	}

	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector: added.InstallationID,
		Targets: []RemoveInput{
			{Client: cursor, Scope: domain.ScopeUser, ExternalUninstalled: true},
			{Client: kiro, Scope: domain.ScopeUser, ExternalUninstalled: true},
		},
		OperationGroupID: "group-remove", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated || len(removed.Receipts) != 2 {
		t.Fatalf("grouped remove = %+v", removed)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained {
		t.Fatalf("grouped remove state = %+v", state.Installations)
	}
	for _, target := range added.Targets {
		if _, err := os.Lstat(target.Plan.ActivePath); !os.IsNotExist(err) {
			t.Fatalf("grouped remove retained %s: %v", target.Plan.ActivePath, err)
		}
	}
}

func TestGroupedPreflightFailureMutatesNoTarget(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	first := addInput(t, cursor, "https://example.com/no-partial")
	unsupportedScope := addInput(t, kiro, "https://example.com/no-partial")
	unsupportedScope.Scope = domain.ScopeProject
	result, err := service.AddGroup(context.Background(), GroupInput{
		Targets: []AddInput{first, unsupportedScope}, OperationGroupID: "group-preflight", Confirmed: true,
	})
	if err == nil || !strings.Contains(err.Error(), "scope is not proven") {
		t.Fatalf("group preflight error = %v", err)
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("failed group preflight changed state: %+v", state.Installations)
	}
	if len(result.Targets) > 0 && result.Targets[0].Plan.ActivePath != "" {
		if _, statErr := os.Lstat(result.Targets[0].Plan.ActivePath); !os.IsNotExist(statErr) {
			t.Fatalf("failed group preflight changed first target: %v", statErr)
		}
	}
}

func TestGroupedDryRunDoesNotObserveNativeClientIdentity(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	observer := &countingNativeObserver{}
	service.NativeObserver = observer
	targets := []AddInput{
		addInput(t, cursor, "https://example.com/dry-run"),
		addInput(t, kiro, "https://example.com/dry-run"),
	}

	result, err := service.AddGroup(context.Background(), GroupInput{
		Targets:          targets,
		OperationGroupID: "group-dry-run",
		DryRun:           true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observer.calls != 0 {
		t.Fatalf("group dry-run observed native client identity %d times", observer.calls)
	}
	if observer.preparedCalls != 2 {
		t.Fatalf("group dry-run prepared observations = %d, want 2", observer.preparedCalls)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if result.Mutated || len(state.Installations) != 0 {
		t.Fatalf("group dry-run mutated state: result=%+v state=%+v", result, state)
	}

	installed, err := service.AddGroup(context.Background(), GroupInput{
		Targets: targets, OperationGroupID: "group-install", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed.Targets[0].Plan.ActivePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	observer.calls, observer.preparedCalls = 0, 0
	if _, err := service.AddGroup(context.Background(), GroupInput{
		Targets: targets, OperationGroupID: "group-tampered-dry-run", DryRun: true,
	}); err == nil || !strings.Contains(err.Error(), "changed or is missing") {
		t.Fatalf("tampered group dry-run error = %v", err)
	}
	if observer.calls != 0 || observer.preparedCalls != 0 {
		t.Fatalf("tampered group dry-run observations: native=%d prepared=%d", observer.calls, observer.preparedCalls)
	}
}

func TestSwitchGroupPreservesOwnedPluginDataAcrossDistributionSwitchReverseAndRollback(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	upstream := &domain.DirectoryOrigin{
		ProductID: "demo", DistributionID: "publisher/demo", DistributionKind: domain.DistributionUpstream,
		DesiredReleaseSequence: 1, SnapshotSchema: 1, SnapshotSequence: 1, SnapshotDigest: "sha256:upstream-snapshot",
	}
	bridge := &domain.DirectoryOrigin{
		ProductID: "demo", DistributionID: "community/demo-bridge", DistributionKind: domain.DistributionCommunityBridge,
		DesiredReleaseSequence: 2, SnapshotSchema: 1, SnapshotSequence: 2, SnapshotDigest: "sha256:bridge-snapshot",
	}
	stateful := func(source string, directory *domain.DirectoryOrigin, version, tree, manifest string) AddInput {
		input := addInput(t, cursor, source)
		input.OriginMode, input.DirectoryResolution = domain.OriginModeDirectory, directory
		input.Envelope.Source.ResolvedRevision = strings.Repeat("d", 40)
		setEnvelopeVersion(t, &input.Envelope, version, tree, manifest)
		input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"local": {Name: "local", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "sh", "args": []any{"-c", "echo ${PLUGIN_DATA}"}}},
		}}
		input.Envelope.Inventory.MCPServers = []string{"local"}
		return input
	}
	initial := stateful("publisher/demo", upstream, "1.0.0", "sha256:upstream-tree", "sha256:upstream-manifest")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{initial}, OperationGroupID: "upstream-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	before := onlyBinding(state.Installations[0])
	receipt := state.Installations[0].DataReceipts[before.DataReceiptID]
	marker := filepath.Join(receipt.Locator, "switch-marker")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}

	toBridge := addInput(t, cursor, "community/demo-bridge")
	toBridge.OriginMode, toBridge.DirectoryResolution = domain.OriginModeDirectory, bridge
	toBridge.Envelope.Source.ResolvedRevision = strings.Repeat("d", 40)
	setEnvelopeVersion(t, &toBridge.Envelope, "2.0.0", "sha256:bridge-tree", "sha256:bridge-manifest")
	preview, err := service.SwitchGroup(context.Background(), GroupInput{Targets: []AddInput{toBridge}, OperationGroupID: "bridge-preview", DryRun: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PluginData.Disposition != domain.PluginDataRetained || preview.PluginData.Ownership != domain.PluginDataOwnershipOwned || preview.PluginData.Compatibility != domain.PluginDataCompatibilityNotProven || preview.PluginData.Warning != domain.PluginDataCompatibilityWarning {
		t.Fatalf("switch preview data decision = %+v", preview.PluginData)
	}

	originalStager := service.Stager
	service.Stager = &failNthVerificationStager{
		verificationFailureStager: verificationFailureStager{PackageStager: originalStager, err: errors.New("injected switch verification failure")},
		failAt:                    3,
	}
	rolledBack, err := service.SwitchGroup(context.Background(), GroupInput{Targets: []AddInput{toBridge}, OperationGroupID: "bridge-rollback", Confirmed: true, Switch: true})
	if err == nil || rolledBack.Phase != GroupPhaseManagedRolledBack {
		t.Fatalf("switch rollback = %+v, %v", rolledBack, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	afterRollback := onlyBinding(state.Installations[0])
	if afterRollback.DataReceiptID != receipt.DataReceiptID || state.Installations[0].Directory.DistributionID != upstream.DistributionID {
		t.Fatalf("rollback changed data ownership or distribution: %+v", state.Installations[0])
	}
	if body, readErr := os.ReadFile(marker); readErr != nil || string(body) != "preserve" {
		t.Fatalf("rollback changed PLUGIN_DATA marker: %q %v", body, readErr)
	}

	service.Stager = originalStager
	applied, err := service.SwitchGroup(context.Background(), GroupInput{Targets: []AddInput{toBridge}, OperationGroupID: "bridge-apply", Confirmed: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if applied.PluginData.Disposition != domain.PluginDataRetained || onlyBinding(state.Installations[0]).DataReceiptID != receipt.DataReceiptID || state.Installations[0].Directory.DistributionID != bridge.DistributionID {
		t.Fatalf("bridge switch lost retained data: result=%+v state=%+v", applied, state.Installations[0])
	}

	reverse := stateful("publisher/demo", upstream, "3.0.0", "sha256:upstream-return-tree", "sha256:upstream-return-manifest")
	reversed, err := service.SwitchGroup(context.Background(), GroupInput{Targets: []AddInput{reverse}, OperationGroupID: "upstream-return", Confirmed: true, Switch: true})
	if err != nil {
		t.Fatal(err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reversed.InstallationID != added.InstallationID || reversed.PluginData.Disposition != domain.PluginDataRetained || onlyBinding(state.Installations[0]).DataReceiptID != receipt.DataReceiptID || len(state.Installations[0].DataReceipts) != 1 {
		t.Fatalf("reverse switch lost data ownership: result=%+v state=%+v", reversed, state.Installations[0])
	}
	if body, readErr := os.ReadFile(marker); readErr != nil || string(body) != "preserve" {
		t.Fatalf("reverse switch changed PLUGIN_DATA marker: %q %v", body, readErr)
	}
}

func TestGroupedRepairAllowsRecordedObjectToBeAbsentButNeverAdoptsForeignIdentity(t *testing.T) {
	t.Parallel()
	managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
	client := domain.DetectedClient{ClientID: domain.ClientCursor}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCursor}

	service := Service{NativeObserver: fixedNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}}}
	if err := service.observeGroupNativeIdentity(context.Background(), client, plan, managed, true); err != nil {
		t.Fatalf("absent recorded repair target was rejected: %v", err)
	}

	service.NativeObserver = fixedNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}}
	if err := service.observeGroupNativeIdentity(context.Background(), client, plan, managed, true); err == nil || !strings.Contains(err.Error(), "automatic adoption is disabled") {
		t.Fatalf("foreign repair target was accepted: %v", err)
	}

	service.NativeObserver = fixedNativeObserver{observation: domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: "sha256:different"}}
	if err := service.observeGroupNativeIdentity(context.Background(), client, plan, managed, true); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("stale managed repair target was accepted: %v", err)
	}
}

// acceptingVerifier treats any active path as matching any expected digest.
// It stands in for a real content check in tests that only exercise the
// native-identity control flow, not staged-byte verification.
type acceptingVerifier struct{}

func (acceptingVerifier) Verify(context.Context, string, string) error { return nil }

// recoveryProbeNativeObserver simulates Codex's own failure mode from run05:
// the CLI-inclusive registry command fails outright whenever the managed
// directory it would report on is absent, regardless of cause. Filesystem-only
// prepared observation never touches that failing command, matching the real
// provider split between ObservePreparedIdentity and ObserveNativeIdentity.
type recoveryProbeNativeObserver struct {
	stager interface {
		Verify(context.Context, string, string) error
	}
	preparedCalls int
	nativeCalls   int
	// postRestorationState, when set, overrides what ObserveNativeIdentity
	// reports once the target file exists, so a test can simulate the native
	// registry finding something other than confirmed ownership after
	// restoration (for example, a foreign namespace collision).
	postRestorationState domain.NativeIdentityState
	// injectAfterPreparedCall, when nonzero, runs injectFunc immediately after
	// the given 1-based ObservePreparedIdentity call number returns its
	// (truthful, at-that-instant) answer -- simulating a concurrent write that
	// lands after the last absence check but before the kernel actually swaps.
	injectAfterPreparedCall int
	injectFunc              func()
	// watchPaths, when set, simulates Codex's own registry command being
	// global across every installation's marketplace entries: ObserveNativeIdentity
	// fails whenever ANY of these paths is absent, not only plan.ActivePath.
	watchPaths []string
}

func (observer *recoveryProbeNativeObserver) ObservePreparedIdentity(_ context.Context, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observer.preparedCalls++
	if observer.injectAfterPreparedCall != 0 && observer.preparedCalls == observer.injectAfterPreparedCall {
		defer observer.injectFunc()
	}
	if _, err := os.Lstat(plan.ActivePath); os.IsNotExist(err) {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
	} else if err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, err
	}
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: managedDigest(*managed)}, nil
}

func (observer *recoveryProbeNativeObserver) ObserveNativeIdentity(ctx context.Context, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observer.nativeCalls++
	watched := observer.watchPaths
	if len(watched) == 0 {
		watched = []string{plan.ActivePath}
	}
	for _, path := range watched {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			// The real bug: Codex's `plugin list` fails outright while any of
			// its configured local marketplace sources is absent. The exit
			// code must never become proof of absence.
			return domain.NativeIdentityObservation{}, errors.New("Codex plugin registry command failed with exit code 1")
		}
	}
	if observer.postRestorationState != "" {
		return domain.NativeIdentityObservation{State: observer.postRestorationState}, nil
	}
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	expected := managedDigest(*managed)
	if expected == "" || observer.stager == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}
	if err := observer.stager.Verify(ctx, plan.ActivePath, expected); err != nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
	}
	return domain.NativeIdentityObservation{
		State: domain.NativeIdentityManaged, Digest: expected, ReceiptReconciled: true,
		NativeDiscoveryAttempted: true, NativeDiscoveryReconciled: true, NativeDiscoveryState: domain.NativeIdentityManaged,
	}, nil
}

func TestGroupedRepairRecoversAbsentManagedDirectoryWithoutTreatingFailedNativeDiscoveryAsAbsence(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/absent-repair")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "absent-repair-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	activePath := added.Targets[0].Plan.ActivePath
	if _, statErr := os.Stat(activePath); statErr != nil {
		t.Fatalf("installed target missing: %v", statErr)
	}

	// Simulate run05: the whole intact managed directory disappears from under
	// UAP (renamed or removed outside its lifecycle) while state and receipts
	// still record it as owned with a nonempty digest.
	if err := os.RemoveAll(activePath); err != nil {
		t.Fatal(err)
	}

	observer := &recoveryProbeNativeObserver{stager: service.Stager}
	service.NativeObserver = observer

	repairInput := install
	repairInput.InstallationID = added.InstallationID

	if _, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairInput}, OperationGroupID: "absent-repair-dry-run", DryRun: true, Repair: true}); err != nil {
		t.Fatalf("dry-run repair of an absent recorded object was refused: %v", err)
	}
	if observer.nativeCalls != 0 {
		t.Fatalf("dry-run invoked native client discovery %d times, want 0", observer.nativeCalls)
	}

	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairInput}, OperationGroupID: "absent-repair-confirmed", Confirmed: true, Repair: true})
	if err != nil {
		t.Fatalf("confirmed repair of an absent recorded object failed: %v", err)
	}
	if repaired.Phase != GroupPhaseCompleted {
		t.Fatalf("repair phase = %s, want completed", repaired.Phase)
	}
	if observer.nativeCalls == 0 {
		t.Fatal("post-restoration native identity verification was never invoked")
	}
	if _, err := os.Stat(filepath.Join(activePath, "plugin.json")); err != nil {
		t.Fatalf("repair did not restore the managed directory: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).Receipts) != 2 {
		t.Fatalf("repair did not persist a receipt: %+v", onlyBinding(state.Installations[0]))
	}
}

// TestGroupedRepairPreservesForeignContentThatAppearsAfterTheLastAbsenceRecheck
// closes the race an independent review identified: the absence recheck just
// before the kernel commit ("compare absent targets again before publishing")
// can itself go stale before dirswap actually renames the staged directory
// into place. RequireAbsent (dirswap.Input/DirectoryMutation) forces a fresh
// Lstat immediately before that rename and fails closed, without touching
// anything, if the path is no longer absent.
func TestGroupedRepairPreservesForeignContentThatAppearsAfterTheLastAbsenceRecheck(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/absent-repair-race")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "race-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	activePath := added.Targets[0].Plan.ActivePath
	if err := os.RemoveAll(activePath); err != nil {
		t.Fatal(err)
	}

	observer := &recoveryProbeNativeObserver{
		stager: service.Stager,
		// Call 1 is the initial eligibility check (still absent); call 2 is the
		// pre-commit recheck. Inject the foreign write right after call 2
		// truthfully reports absence, simulating it landing in the remaining
		// gap before dirswap's own rename.
		injectAfterPreparedCall: 2,
		injectFunc: func() {
			if err := os.MkdirAll(activePath, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(activePath, "FOREIGN"), []byte("not ours"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	service.NativeObserver = observer

	repairInput := install
	repairInput.InstallationID = added.InstallationID
	result, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairInput}, OperationGroupID: "race-confirmed", Confirmed: true, Repair: true})
	if err == nil {
		t.Fatal("concurrent foreign content at the recovery target was silently adopted or discarded")
	}
	if result.Mutated {
		t.Fatalf("failed race-guarded repair still mutated: %+v", result)
	}
	body, readErr := os.ReadFile(filepath.Join(activePath, "FOREIGN"))
	if readErr != nil || string(body) != "not ours" {
		t.Fatalf("foreign content was not preserved: body=%q err=%v", body, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(activePath, "plugin.json")); statErr == nil {
		t.Fatal("recovery content was written over the foreign directory")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).Receipts) != 1 {
		t.Fatalf("race-guarded failure changed the persisted receipt count: %+v", onlyBinding(state.Installations[0]))
	}
}

func TestGroupedRepairStillRefusesChangedExistingContentWithRecoveryAwareObserver(t *testing.T) {
	t.Parallel()
	service, _, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/changed-repair")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "changed-repair-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	activePath := added.Targets[0].Plan.ActivePath
	if err := os.WriteFile(filepath.Join(activePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	observer := &recoveryProbeNativeObserver{stager: service.Stager}
	service.NativeObserver = observer

	repairInput := install
	repairInput.InstallationID = added.InstallationID
	if _, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairInput}, OperationGroupID: "changed-repair-confirmed", Confirmed: true, Repair: true}); err == nil {
		t.Fatal("repair silently adopted changed existing content")
	}
	if observer.nativeCalls == 0 {
		t.Fatal("changed existing content bypassed the ordinary native identity check")
	}
}

func TestGroupedRepairRollsBackAbsentRecoveryWhenPostRestorationNativeVerificationFails(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	install := addInput(t, codex, "https://example.com/absent-repair-collision")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{install}, OperationGroupID: "absent-collision-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	activePath := added.Targets[0].Plan.ActivePath
	if err := os.RemoveAll(activePath); err != nil {
		t.Fatal(err)
	}

	// Once the directory is restored, the native registry reports the identity
	// as unmanaged (for example, a foreign namespace now claims it) instead of
	// confirming ownership: the whole recovery must roll back, not commit.
	observer := &recoveryProbeNativeObserver{stager: service.Stager, postRestorationState: domain.NativeIdentityUnmanaged}
	service.NativeObserver = observer

	repairInput := install
	repairInput.InstallationID = added.InstallationID
	result, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairInput}, OperationGroupID: "absent-collision-confirmed", Confirmed: true, Repair: true})
	if err == nil {
		t.Fatal("post-restoration native verification failure was ignored")
	}
	if result.Phase != GroupPhaseManagedRolledBack {
		t.Fatalf("repair phase = %s, want rolled back", result.Phase)
	}
	if _, statErr := os.Stat(activePath); !os.IsNotExist(statErr) {
		t.Fatalf("failed recovery left a restored directory behind: %v", statErr)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).Receipts) != 1 {
		t.Fatalf("failed recovery changed the persisted receipt count: %+v", onlyBinding(state.Installations[0]))
	}
}

// TestGroupRecoveryPostApplyVerifyChecksEveryRecoveringTargetOnce covers
// multiple recovering targets in one group at the mechanism level.
// Recovery is deliberately restricted to Codex (see
// observeGroupRecoveryEligibility), and one installation has at most one
// Codex binding, so a real RepairGroup call can never carry two recovering
// targets in production; groupRecoveryPostApplyVerify's own loop over
// multiple recovering entries is exercised directly instead.
// TestGroupedRepairFailsClosedWhenAnotherInstallationsCodexDirectoryIsAlsoAbsent
// covers the cross-installation limitation documented in
// groupRecoveryPostApplyVerify: Codex's own registry command is global across
// every installation's marketplace entries, not scoped to one installation.
// If a wiped managed root leaves two installations each with their own absent
// Codex-owned directory, repairing one restores its bytes locally but the
// registry call still fails because of the other's missing entry, so that
// repair rolls back too -- fail-closed, never a silent adoption, and no worse
// than the pre-fix preflight refusal.
func TestGroupedRepairFailsClosedWhenAnotherInstallationsCodexDirectoryIsAlsoAbsent(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codexA := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	codexB := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	installA := addInput(t, codexA, "https://example.com/multi-install-a")
	installB := addInput(t, codexB, "https://example.com/multi-install-b")
	installB.InstallationID = ""
	installB.Envelope.Manifest.Name = "demo-b"
	if err := os.WriteFile(filepath.Join(installB.Envelope.SnapshotRoot, "plugin.json"), []byte(`{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "demo-b",
  "version": "1.0.0",
  "description": "Demo plugin"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	addedA, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{installA}, OperationGroupID: "multi-install-add-a", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	addedB, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{installB}, OperationGroupID: "multi-install-add-b", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	pathA, pathB := addedA.Targets[0].Plan.ActivePath, addedB.Targets[0].Plan.ActivePath
	if err := os.RemoveAll(pathA); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(pathB); err != nil {
		t.Fatal(err)
	}

	observer := &recoveryProbeNativeObserver{stager: service.Stager, watchPaths: []string{pathA, pathB}}
	service.NativeObserver = observer

	repairA := installA
	repairA.InstallationID = addedA.InstallationID
	result, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairA}, OperationGroupID: "multi-install-repair-a", Confirmed: true, Repair: true})
	if err == nil {
		t.Fatal("repair succeeded despite the sibling installation's Codex directory still being absent")
	}
	if result.Phase != GroupPhaseManagedRolledBack {
		t.Fatalf("phase = %s, want rolled back", result.Phase)
	}
	if _, statErr := os.Stat(pathA); !os.IsNotExist(statErr) {
		t.Fatalf("failed cross-installation repair left A restored instead of rolling back: %v", statErr)
	}
	if _, statErr := os.Stat(pathB); !os.IsNotExist(statErr) {
		t.Fatalf("failed cross-installation repair touched B: %v", statErr)
	}
	stateA, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range stateA.Installations {
		if installation.InstallationID == addedA.InstallationID && len(onlyBinding(installation).Receipts) != 1 {
			t.Fatalf("failed cross-installation repair changed A's receipt count: %+v", onlyBinding(installation))
		}
	}

	// Restoring B too must let A's repair succeed on retry.
	if err := os.MkdirAll(pathB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pathB, "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{repairA}, OperationGroupID: "multi-install-repair-a-retry", Confirmed: true, Repair: true})
	if err != nil {
		t.Fatalf("repair of A still failed once B was restored: %v", err)
	}
	if repaired.Phase != GroupPhaseCompleted {
		t.Fatalf("retried repair phase = %s, want completed", repaired.Phase)
	}
}

func TestGroupRecoveryPostApplyVerifyChecksEveryRecoveringTargetOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	makeTarget := func(name string) plannedGroupTarget {
		active := filepath.Join(root, name)
		if err := os.MkdirAll(active, 0o755); err != nil {
			t.Fatal(err)
		}
		managed := &domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:" + name}}}
		return plannedGroupTarget{
			input:      AddInput{Client: domain.DetectedClient{ClientID: domain.ClientCodex}},
			plan:       domain.DeliveryPlan{ActivePath: active},
			managed:    managed,
			recovering: true,
		}
	}
	planned := []plannedGroupTarget{makeTarget("first"), makeTarget("second"), {noChange: true}}
	observer := &recoveryProbeNativeObserver{stager: acceptingVerifier{}}
	service := Service{NativeObserver: observer}
	verify := service.groupRecoveryPostApplyVerify(planned)
	if verify == nil {
		t.Fatal("expected a non-nil PostApplyVerify hook")
	}
	if err := verify(context.Background()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if observer.nativeCalls != 2 {
		t.Fatalf("native identity verification calls = %d, want 2 (one per recovering target)", observer.nativeCalls)
	}
}

func TestGroupedAddReportsCommittedActivationAndExternalPartialFailuresFromReceipts(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		failCall  int
		wantPhase GroupPhase
		wantFirst GroupTargetPhase
	}{
		{name: "first activation", failCall: 1, wantPhase: GroupPhaseManagedActivationFailed, wantFirst: GroupTargetExternalFailed},
		{name: "second activation", failCall: 2, wantPhase: GroupPhaseExternalPartialFailure, wantFirst: GroupTargetExternalCompleted},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			service.Activator = &failNthGroupActivator{failCall: test.failCall}
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{
				addInput(t, cursor, "https://example.com/phase-aware"),
				addInput(t, kiro, "https://example.com/phase-aware"),
			}, OperationGroupID: "phase-aware-" + strings.ReplaceAll(test.name, " ", "-"), Confirmed: true})
			if err == nil {
				t.Fatal("activation failure was ignored")
			}
			if result.Phase != test.wantPhase || len(result.Receipts) != 2 || result.Targets[0].GroupPhase != test.wantFirst {
				t.Fatalf("phase-aware result = %+v", result)
			}
			if result.Targets[test.failCall-1].GroupPhase != GroupTargetExternalFailed {
				t.Fatalf("failed target outcome = %+v", result.Targets[test.failCall-1])
			}
			state, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
				t.Fatalf("managed commit was not authoritative: %+v", state)
			}
		})
	}
}

func TestGroupedRepairAtomicallyRestoresHeterogeneousRecordedRevisions(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	cursorV1 := addInput(t, cursor, "https://example.com/mixed-repair")
	kiroV1 := addInput(t, kiro, "https://example.com/mixed-repair")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{cursorV1, kiroV1}, OperationGroupID: "mixed-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	cursorV2 := addInput(t, cursor, "https://example.com/mixed-repair")
	kiroV2 := addInput(t, kiro, "https://example.com/mixed-repair")
	setEnvelopeVersion(t, &cursorV2.Envelope, "2.0.0", "sha256:mixed-v2", "sha256:mixed-manifest-v2")
	setEnvelopeVersion(t, &kiroV2.Envelope, "2.0.0", "sha256:mixed-v2", "sha256:mixed-manifest-v2")
	if _, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{cursorV2}, CompatibilityChecks: []AddInput{cursorV2, kiroV2}, OperationGroupID: "mixed-update", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationComplete
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	cursorV2.InstallationID = added.InstallationID
	kiroV1.InstallationID = added.InstallationID
	repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{cursorV2, kiroV1}, OperationGroupID: "mixed-repair", Confirmed: true, Repair: true})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Phase != GroupPhaseCompleted || len(repaired.Receipts) != 2 {
		t.Fatalf("heterogeneous repair result = %+v", repaired)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var versions = map[string]string{}
	for _, binding := range state.Installations[0].Clients {
		versions[binding.ClientID] = binding.PackageRevision.Version
		if binding.Authentication != domain.AuthenticationComplete {
			t.Fatalf("repair reset completed authentication for %s: %+v", binding.ClientID, binding)
		}
	}
	if versions[string(domain.ClientCursor)] != "2.0.0" || versions[string(domain.ClientKiro)] == "2.0.0" {
		t.Fatalf("repair did not preserve exact per-target revisions: %+v", versions)
	}
}

func TestGroupedRepairPreservesCompletedAuthenticationWhenFinalizationFails(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	cursorInput := addInput(t, cursor, "https://example.com/group-repair-auth")
	kiroInput := addInput(t, kiro, "https://example.com/group-repair-auth")
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{cursorInput, kiroInput}, OperationGroupID: "repair-auth-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationComplete
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	cursorInput.InstallationID = added.InstallationID
	kiroInput.InstallationID = added.InstallationID
	faulted := false
	service.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseCommitPending && !faulted {
			faulted = true
			return errors.New("injected post-commit finalization failure")
		}
		return nil
	}
	result, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{cursorInput, kiroInput}, OperationGroupID: "repair-auth-late-failure", Confirmed: true})
	if err == nil || transaction.FailurePhase(err) != transaction.GroupFailureCommitted || result.Phase != GroupPhaseManagedCommitted {
		t.Fatalf("late repair failure = result=%+v err=%v phase=%s", result, err, transaction.FailurePhase(err))
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("committed repair state = %+v", state.Installations)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.Authentication != domain.AuthenticationComplete {
			t.Fatalf("late repair failure reset authentication for %s: %+v", binding.ClientID, binding)
		}
	}
	service.Kernel.Directory.Fault = nil
	kernel := service.Kernel
	kernel.StateStore = store
	if err := kernel.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.Authentication != domain.AuthenticationComplete {
			t.Fatalf("recovery reset authentication for %s: %+v", binding.ClientID, binding)
		}
	}
}

func TestGroupedOpenAIOAuthPromotionMatchesSingleTargetSemantics(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	inputs := func(version, tree, manifest string, explicitCodexCompatibility bool) (AddInput, AddInput) {
		codexInput := addInput(t, codex, "https://example.com/group-oauth")
		cursorInput := addInput(t, cursor, "https://example.com/group-oauth")
		for _, input := range []*AddInput{&codexInput, &cursorInput} {
			setEnvelopeVersion(t, &input.Envelope, version, tree, manifest)
			input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
				"remote": {Name: "remote", Type: "streamable-http", Decoded: map[string]any{"url": "https://mcp.example.com"}},
			}}
			input.Envelope.Inventory.MCPServers = []string{"remote"}
			input.Hints.OpenAIMCPAuth = map[string]domain.OpenAIMCPAuthHint{"remote": {OAuthResource: "https://mcp.example.com"}}
		}
		if explicitCodexCompatibility {
			compatibility := map[string]domain.CatalogCompatibility{
				string(domain.ClientCodex):  {Package: "projected", Authentication: domain.AuthenticationRequirementNotRequired},
				string(domain.ClientCursor): {Package: "native", Authentication: domain.AuthenticationRequirementNotRequired},
			}
			codexInput.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: compatibility}
			cursorInput.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: compatibility}
		}
		return codexInput, cursorInput
	}

	codexAdd, cursorAdd := inputs("1.0.0", "sha256:source-tree", "sha256:manifest", false)
	added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{codexAdd, cursorAdd}, OperationGroupID: "group-oauth-add", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if added.Targets[0].Plan.Authentication != domain.AuthenticationPending || added.Targets[1].Plan.Authentication == domain.AuthenticationPending {
		t.Fatalf("group add authentication plans = codex:%s cursor:%s", added.Targets[0].Plan.Authentication, added.Targets[1].Plan.Authentication)
	}
	assertGroupedAuthentication(t, store, domain.ClientCodex, domain.AuthenticationPending)
	assertGroupedAuthenticationNot(t, store, domain.ClientCursor, domain.AuthenticationPending)

	codexUpdate, cursorUpdate := inputs("2.0.0", "sha256:group-oauth-v2", "sha256:group-oauth-manifest-v2", false)
	updated, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{codexUpdate, cursorUpdate}, CompatibilityChecks: []AddInput{codexUpdate, cursorUpdate}, OperationGroupID: "group-oauth-update", Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Targets[0].Plan.Authentication != domain.AuthenticationPending || updated.Targets[1].Plan.Authentication == domain.AuthenticationPending {
		t.Fatalf("group update authentication plans = codex:%s cursor:%s", updated.Targets[0].Plan.Authentication, updated.Targets[1].Plan.Authentication)
	}
	assertGroupedAuthentication(t, store, domain.ClientCodex, domain.AuthenticationPending)
	assertGroupedAuthenticationNot(t, store, domain.ClientCursor, domain.AuthenticationPending)

	explicitCodex, explicitCursor := inputs("3.0.0", "sha256:group-oauth-v3", "sha256:group-oauth-manifest-v3", true)
	preview, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{explicitCodex, explicitCursor}, CompatibilityChecks: []AddInput{explicitCodex, explicitCursor}, OperationGroupID: "group-oauth-explicit", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Targets[0].Plan.Authentication == domain.AuthenticationPending || preview.Targets[1].Plan.Authentication == domain.AuthenticationPending {
		t.Fatalf("explicit compatibility was falsely promoted: %+v", preview.Targets)
	}
}

func assertGroupedAuthentication(t *testing.T, store interface {
	Load() (domain.StateFileV2, error)
}, clientID domain.ClientID, want domain.AuthenticationState) {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(clientID) {
			if binding.Authentication != want {
				t.Fatalf("%s authentication = %s, want %s", clientID, binding.Authentication, want)
			}
			return
		}
	}
	t.Fatalf("client %s was not persisted", clientID)
}

func assertGroupedAuthenticationNot(t *testing.T, store interface {
	Load() (domain.StateFileV2, error)
}, clientID domain.ClientID, unwanted domain.AuthenticationState) {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(clientID) {
			if binding.Authentication == unwanted {
				t.Fatalf("%s authentication was falsely promoted to %s", clientID, unwanted)
			}
			return
		}
	}
	t.Fatalf("client %s was not persisted", clientID)
}

type fixedNativeObserver struct {
	observation domain.NativeIdentityObservation
	err         error
}

type countingNativeObserver struct {
	calls         int
	preparedCalls int
}

type failNthVerificationStager struct {
	verificationFailureStager
	calls  int
	failAt int
}

func (stager *failNthVerificationStager) Verify(ctx context.Context, root, expected string) error {
	stager.calls++
	if stager.calls == stager.failAt {
		return stager.err
	}
	return stager.PackageStager.Verify(ctx, root, expected)
}

type failNthGroupActivator struct {
	calls    int
	failCall int
}

func (activator *failNthGroupActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if activator.calls == activator.failCall {
		outcome.Activation = domain.ActivationFailed
		outcome.Verification = domain.VerificationFailed
		return outcome, errors.New("injected activation failure")
	}
	return outcome, nil
}

func (*failNthGroupActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{Activation: domain.ActivationNotRequired, ArtifactRemovalAllowed: true, ExternalRemovalComplete: true}, nil
}

func (observer fixedNativeObserver) ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return observer.observation, observer.err
}

func (observer *countingNativeObserver) ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observer.calls++
	return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
}

func (observer *countingNativeObserver) ObservePreparedIdentity(_ context.Context, _ domain.DetectedClient, _ domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	observer.preparedCalls++
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: managedDigest(*managed)}, nil
}
