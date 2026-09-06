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
