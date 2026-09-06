package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestAddDryRunAndUnconfirmedPlanDoNotMutateStateOrClient(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"dry_run", "unconfirmed"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			observer := &countingNativeObserver{}
			service.NativeObserver = observer
			input := addInput(t, client, "https://example.com/one")
			input.DryRun = mode == "dry_run"
			input.Confirmed = false
			result, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unconfirmed" && !result.RequiresConfirmation {
				t.Fatal("unconfirmed mutation did not require confirmation")
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 0 {
				t.Fatalf("state mutated: %+v", state)
			}
			if _, err := os.Lstat(result.Plan.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("client path mutated: %v", err)
			}
			if mode == "dry_run" && observer.calls != 0 {
				t.Fatalf("dry-run observed native client identity %d times", observer.calls)
			}
			if mode == "dry_run" && observer.preparedCalls != 1 {
				t.Fatalf("dry-run prepared observations = %d, want 1", observer.preparedCalls)
			}
		})
	}
}

func TestAddCommitsCursorPackageReceiptAndLeavesDiscoveryManual(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/one")
	input.Envelope.ManifestSchema = domain.SchemaIdentity{URI: domain.PluginSchemaV1, Version: "1.0.0"}
	input.Envelope.Manifest.Raw = json.RawMessage(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","future":{"kept":true}}`)
	input.Envelope.Manifest.Unknown = map[string]json.RawMessage{"future": json.RawMessage(`{"kept":true}`)}
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{SchemaVersion: 1, CatalogVersion: "0.1.0", Compatibility: map[string]domain.CatalogCompatibility{
		"cursor": {Package: "native", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired},
	}}
	input.Envelope.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityWarning, Code: "plugin_unknown_field", Message: "future field preserved"}}
	input.Confirmed = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || result.Activation.Activation != domain.ActivationManual || result.Activation.Verification != domain.VerificationPackageValid {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(result.Plan.ActivePath, "plugin.json")); err != nil {
		t.Fatalf("installed package: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("installations = %+v", state.Installations)
	}
	packageState := state.Installations[0].Package
	clientState := onlyBinding(state.Installations[0])
	if packageState.SchemaURI != domain.PluginSchemaV1 || packageState.ManifestDigest != input.Envelope.ManifestDigest ||
		clientState.PackageRevision == nil || clientState.PackageRevision.CatalogEvidence == nil ||
		clientState.PackageRevision.CatalogEvidence.Compatibility["cursor"].Verification != "tested" ||
		!strings.Contains(string(input.Envelope.Manifest.Raw), `"future"`) || input.Envelope.CatalogEvidence == nil || len(input.Envelope.Diagnostics) != 1 {
		t.Fatalf("in-memory evidence or client revision binding was lost: envelope=%+v package=%+v client=%+v", input.Envelope, packageState, clientState)
	}
	if clientState.Materialization != domain.MaterializationMaterialized || clientState.Activation != domain.ActivationManual || clientState.Verification != domain.VerificationPackageValid {
		t.Fatalf("client state = %+v", clientState)
	}
	if len(clientState.Receipts) != 1 || clientState.Receipts[0].Phase != transaction.ReceiptPhaseCommitted {
		t.Fatalf("receipts = %+v", clientState.Receipts)
	}
	if clientState.PackageRevision == nil || clientState.PackageRevision.TreeDigest != input.Envelope.TreeDigest {
		t.Fatalf("package revision = %+v", clientState.PackageRevision)
	}
	resumed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatalf("resume add: %v", err)
	}
	if resumed.Mutated || resumed.Activation.Activation != domain.ActivationManual {
		t.Fatalf("resume result = %+v", resumed)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if receipts := onlyBinding(state.Installations[0]).Receipts; len(receipts) != 1 {
		t.Fatalf("resume created a second directory transaction: %+v", receipts)
	}
}

func TestAddKeepsCodexProjectionInManualActivationState(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}
	input := addInput(t, client, "https://example.com/openai")
	input.Confirmed = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Activation.Activation != domain.ActivationManual || result.Activation.Verification != domain.VerificationPackageValid {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(result.Plan.ActivePath, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	clientState := onlyBinding(state.Installations[0])
	if clientState.Activation != domain.ActivationManual || clientState.Verification != domain.VerificationPackageValid {
		t.Fatalf("client state = %+v", clientState)
	}
}

func TestAddKeepsOAuthPackageInAuthPendingState(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	input := addInput(t, client, "https://example.com/oauth")
	input.Envelope.MCP = domain.MCPComponent{
		Present: true, Enabled: true,
		Servers: map[string]domain.MCPServer{"notion": {Name: "notion", Type: "streamable-http", Decoded: map[string]any{"url": "https://mcp.example.com"}}},
	}
	input.Hints = domain.CompatibilityHints{OpenAIMCPAuth: map[string]domain.OpenAIMCPAuthHint{
		"notion": {OAuthResource: "https://mcp.example.com"},
	}}
	input.Confirmed = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Activation.Authentication != domain.AuthenticationPending {
		t.Fatalf("activation = %+v", result.Activation)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if onlyBinding(state.Installations[0]).Authentication != domain.AuthenticationPending {
		t.Fatalf("client = %+v", onlyBinding(state.Installations[0]))
	}
}

func TestOpenAIOAuthHintsDoNotOverrideGenericAuthentication(t *testing.T) {
	t.Parallel()
	for _, clientID := range []domain.ClientID{domain.ClientCursor, domain.ClientCodex} {
		service, _, _ := serviceFixture(t)
		client := domain.DetectedClient{ClientID: clientID, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".client")}
		input := addInput(t, client, "https://example.com/generic-auth-"+string(clientID))
		input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{"server": {Name: "server", Type: "stdio", Decoded: map[string]any{"command":"sh"}}}}
		input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{string(clientID): {Package: map[bool]string{true: "projected", false: "native"}[clientID == domain.ClientCodex], Authentication: domain.AuthenticationRequirementNotRequired}}}
		input.Hints.OpenAIMCPAuth = map[string]domain.OpenAIMCPAuthHint{"server": {OAuthResource: "https://example.com/oauth"}}
		result, err := service.Add(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Plan.Authentication != domain.AuthenticationNotRequired {
			t.Fatalf("client %s authentication = %s", clientID, result.Plan.Authentication)
		}
	}
}

func TestResumeCompletesManualActivationAndPendingAuthOnlyByAttestation(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/pending-completion")
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"cursor": {Package: "native", Authentication: domain.AuthenticationRequirementRequired}}}
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.ActivationComplete = true
	input.AuthComplete = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || !result.Activation.ActivationAttested || !result.Activation.AuthenticationAttested || !fullyConvergedOutcome(result.Activation) {
		t.Fatalf("completion result = %+v", result)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationComplete {
		t.Fatalf("binding = %+v", binding)
	}
}

func TestFreshCompletionFlagsFailBeforeMaterialization(t *testing.T) {
	t.Parallel()
	for _, flags := range []struct {
		name       string
		activation bool
		auth       bool
	}{
		{name: "activation", activation: true},
		{name: "authentication", auth: true},
		{name: "both", activation: true, auth: true},
	} {
		flags := flags
		t.Run(flags.name, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/fresh-"+flags.name)
			input.Confirmed = true
			input.ActivationComplete = flags.activation
			input.AuthComplete = flags.auth
			if _, err := service.Add(context.Background(), input); err == nil || !strings.Contains(err.Error(), "already materialized") {
				t.Fatalf("completion error = %v", err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations) != 0 {
				t.Fatalf("fresh completion flags mutated state: %+v", state)
			}
		})
	}
}

func TestActivationAttestationDoesNotCompleteAuthentication(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/independent-attestations")
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"cursor": {Package: "native", Authentication: domain.AuthenticationRequirementRequired}}}
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	input.ActivationComplete = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Activation.ActivationAttested || result.Activation.AuthenticationAttested || result.Activation.Authentication != domain.AuthenticationPending {
		t.Fatalf("activation-only result = %+v", result)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationPending {
		t.Fatalf("activation-only binding = %+v", binding)
	}
}

func TestDirectSourceConvergesOnlyAfterAuthReviewAttestation(t *testing.T) {
	t.Parallel()
	service, _, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/direct-source")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Activation.Authentication != domain.AuthenticationNotChecked {
		t.Fatalf("direct source auth = %+v", installed.Activation)
	}
	input.ActivationComplete = true
	input.AuthComplete = true
	completed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !completed.Mutated || !completed.Activation.ActivationAttested || !completed.Activation.AuthenticationAttested || !fullyConvergedOutcome(completed.Activation) {
		t.Fatalf("direct completion = %+v", completed)
	}
	repeated, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !repeated.NoChange || repeated.Mutated {
		t.Fatalf("converged repeat = %+v", repeated)
	}
}

func TestResumeCompletesOnlyPendingAuthentication(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/auth-only-completion")
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"cursor": {Package: "native", Authentication: domain.AuthenticationRequirementRequired}}}
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	input.AuthComplete = true
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || result.Activation.ActivationAttested || !result.Activation.AuthenticationAttested || result.Activation.Authentication != domain.AuthenticationComplete || result.Activation.Activation != domain.ActivationActive || result.Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("auth-only result = %+v", result)
	}
}

func TestAuthOnlyResumeRunsVerifierAndPersistsNegativeEvidence(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".copilot")}
	input := addInput(t, client, "https://example.com/auth-negative")
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"copilot": {Package: "native", Authentication: domain.AuthenticationRequirementRequired}}}
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Verification = domain.VerificationInstalled
		binding.Authentication = domain.AuthenticationPending
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	observer := &observedActivator{outcome: domain.ActivationOutcome{
		Activation: domain.ActivationFailed, Authentication: domain.AuthenticationNotChecked,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
	}, err: fmt.Errorf("negative Copilot listing evidence")}
	service.Activator = observer
	input.AuthComplete = true
	input.BackendExecutable = "/test/bin/copilot"
	result, err := service.Add(context.Background(), input)
	if err == nil || observer.calls != 1 {
		t.Fatalf("resume result=%+v err=%v verifier calls=%d", result, err, observer.calls)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed {
		t.Fatalf("negative verifier evidence was discarded: %+v", binding)
	}
}

func TestFailedUpdateRestoresPreviousNativeOwnershipForRemoval(t *testing.T) {
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/native-receipt-v1")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	clientKey := binding.ClientBindingID
	previous := append(append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), domain.NativeObjectOwnership{ObjectID: "native:demo", Kind: "test_native", LogicalName: "demo", ManagedDigest: "sha256:v1", ProtectionClass: "managed"})
	desired := append(append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), domain.NativeObjectOwnership{ObjectID: "native:demo", Kind: "test_native", LogicalName: "demo", ManagedDigest: "sha256:v2", ProtectionClass: "managed"})
	binding.NativeObjects = desired
	state.Installations[0].Clients[clientKey] = binding
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationFailed, Authentication: binding.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed}
	if _, err := service.updateActivationResult(installed.InstallationID, clientKey, outcome, errors.New("injected V2 native activation failure"), previous); err != nil {
		t.Fatal(err)
	}
	persisted, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := onlyBinding(persisted.Installations[0]).NativeObjects; !reflect.DeepEqual(got, previous) {
		t.Fatalf("failed-update native ownership = %+v, want prior %+v", got, previous)
	}
	binding = onlyBinding(persisted.Installations[0])
	binding.NativeObjects = desired
	persisted.Installations[0].Clients[clientKey] = binding
	if err := store.Save(persisted); err != nil {
		t.Fatal(err)
	}
	success := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: binding.Authentication, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if _, err := service.updateActivationResult(installed.InstallationID, clientKey, success, nil, previous); err != nil {
		t.Fatal(err)
	}
	persisted, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := onlyBinding(persisted.Installations[0]).NativeObjects; !reflect.DeepEqual(got, desired) {
		t.Fatalf("successful-update native ownership = %+v, want desired %+v", got, desired)
	}
	if _, err := service.updateActivationResult(installed.InstallationID, clientKey, outcome, errors.New("second injected V2 native activation failure"), previous); err != nil {
		t.Fatal(err)
	}

	activator := &capturingRemovalActivator{}
	service.Activator = activator
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true, OperationID: "remove-after-failed-update"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(activator.nativeObjects, previous) {
		t.Fatalf("remove received native ownership = %+v, want prior %+v", activator.nativeObjects, previous)
	}
}

func TestConvergedManualVerifierOutcomeUsesConfirmationAndReplacesStaleState(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".copilot")}
	input := addInput(t, client, "https://example.com/converged-manual")
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Verification = domain.VerificationInstalled
		binding.Authentication = domain.AuthenticationNotRequired
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	observer := &observedActivator{outcome: domain.ActivationOutcome{
		Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotChecked,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid,
	}}
	service.Activator = observer
	input.BackendExecutable = "/test/bin/copilot"
	input.Confirmed = false
	preview, err := service.Add(context.Background(), input)
	if err != nil || !preview.RequiresConfirmation || preview.NoChange || preview.Activation.Activation != domain.ActivationManual {
		t.Fatalf("manual preview = %+v, err=%v", preview, err)
	}
	input.Confirmed = true
	result, err := service.Add(context.Background(), input)
	if err != nil || !result.Mutated || result.NoChange {
		t.Fatalf("manual correction = %+v, err=%v", result, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if binding := onlyBinding(state.Installations[0]); binding.Activation != domain.ActivationManual || binding.Verification != domain.VerificationPackageValid {
		t.Fatalf("manual correction was not persisted: %+v", binding)
	}
}

func TestUnconfirmedInfrastructureFailureIsNotPersistedAsAuthoritativeEvidenceForAddOrUpdate(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"add", "update"} {
		t.Run(operation, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			client := domain.DetectedClient{ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".copilot")}
			input := addInput(t, client, "https://example.com/non-authoritative-failure-"+operation)
			input.Confirmed = true
			if _, err := service.Add(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for key, binding := range state.Installations[0].Clients {
				binding.Activation = domain.ActivationActive
				binding.Authentication = domain.AuthenticationNotRequired
				binding.Verification = domain.VerificationInstalled
				state.Installations[0].Clients[key] = binding
			}
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			service.Activator = providers.Activator{Runner: fixedUsecaseRunner{err: fmt.Errorf("temporary verifier transport failure")}}
			input.Confirmed = false
			input.PersistAuthoritativeObservations = true
			input.BackendExecutable = "/test/bin/copilot"
			var result AddResult
			if operation == "add" {
				result, err = service.Add(context.Background(), input)
			} else {
				result, err = service.Update(context.Background(), input)
			}
			if err == nil || result.Mutated {
				t.Fatalf("temporary verifier result=%+v err=%v", result, err)
			}
			state, err = store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(state.Installations[0])
			if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("non-authoritative failure mutated converged state: %+v", binding)
			}
		})
	}
}

func TestPlanFirstAuthoritativePersistenceIsSurroundedByMutationLock(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	input := addInput(t, client, "https://example.com/authoritative-lock")
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Authentication = domain.AuthenticationNotRequired
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	var held atomic.Bool
	lock := &trackingMutationLock{held: &held}
	guarded := lockAssertingStore{StateStore: store, held: &held}
	service.StateStore = guarded
	service.Lock = lock
	service.Activator = providers.Activator{Runner: fixedUsecaseRunner{result: legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}}}
	input.Confirmed = false
	input.PersistAuthoritativeObservations = true
	input.BackendExecutable = "/test/bin/codex"
	result, err := service.Add(context.Background(), input)
	if err == nil || !result.Mutated {
		t.Fatalf("authoritative result=%+v err=%v", result, err)
	}
	if lock.acquisitions != 1 || held.Load() {
		t.Fatalf("lock acquisitions=%d held-after-return=%t", lock.acquisitions, held.Load())
	}
	persisted, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if binding := onlyBinding(persisted.Installations[0]); binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed {
		t.Fatalf("authoritative observation was not persisted: %+v", binding)
	}
}

func TestValidateDirectoryTransitionRejectsSameReleaseRevisionRewrite(t *testing.T) {
	installation := domain.Installation{
		OriginMode: domain.OriginModeDirectory,
		Directory:  &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo", DesiredReleaseSequence: 1},
		Source:     domain.SourceBinding{ResolvedRevision: strings.Repeat("a", 40), TreeDigest: "sha256:tree"},
	}
	input := AddInput{
		OriginMode:          domain.OriginModeDirectory,
		DirectoryResolution: &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo", DesiredReleaseSequence: 1},
		Envelope:            domain.PackageEnvelope{Source: domain.SourceIdentity{ResolvedRevision: strings.Repeat("b", 40)}, TreeDigest: "sha256:tree"},
	}
	if err := validateDirectoryTransition(installation, input); err == nil || !strings.Contains(err.Error(), "conflicting package-source revision") {
		t.Fatalf("same-release revision rewrite was accepted: %v", err)
	}
}

func TestPlanFirstAuthoritativePersistenceRejectsStaleObservedBinding(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	input := addInput(t, client, "https://example.com/stale-authoritative-observation")
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Authentication = domain.AuthenticationNotRequired
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	var held atomic.Bool
	var newer domain.ClientBinding
	lock := &trackingMutationLock{held: &held, onAcquire: func() error {
		latest, loadErr := store.Load()
		if loadErr != nil {
			return loadErr
		}
		for key, binding := range latest.Installations[0].Clients {
			binding.PackageRevision.ResolvedRevision = "newer-revision"
			binding.UpdatedAt = "2026-08-09T12:00:00Z"
			latest.Installations[0].Clients[key] = binding
			newer = binding
		}
		return store.Save(latest)
	}}
	service.StateStore = lockAssertingStore{StateStore: store, held: &held}
	service.Lock = lock
	service.Activator = providers.Activator{Runner: fixedUsecaseRunner{result: legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}}}
	input.Confirmed = false
	input.PersistAuthoritativeObservations = true
	input.BackendExecutable = "/test/bin/codex"
	result, err := service.Add(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "stale client binding observation") || !strings.Contains(err.Error(), "retry") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Mutated {
		t.Fatalf("stale observation reported a mutation: %+v", result)
	}
	persisted, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got := onlyBinding(persisted.Installations[0]); !reflect.DeepEqual(got, newer) {
		t.Fatalf("newer binding was overwritten\n got: %+v\nwant: %+v", got, newer)
	}
}

func TestNoChangeChecksManagedDigestBeforeReturning(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/no-change-digest")
	input.Envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"cursor": {Package: "native", Authentication: domain.AuthenticationRequirementNotRequired}}}
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Add(context.Background(), input); err == nil || !strings.Contains(err.Error(), "changed or is missing") {
		t.Fatalf("no-change error = %v", err)
	}
}

func TestNoChangeDryRunChecksManagedDigestWithoutNativeObservation(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/no-change-dry-run")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	observer := &countingNativeObserver{}
	service.NativeObserver = observer
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	input.DryRun, input.Confirmed = true, false
	if _, err := service.Add(context.Background(), input); err == nil || !strings.Contains(err.Error(), "changed or is missing") {
		t.Fatalf("dry-run managed digest error = %v", err)
	}
	if observer.calls != 0 {
		t.Fatalf("dry-run observed native client identity %d times", observer.calls)
	}
	if observer.preparedCalls != 0 {
		t.Fatalf("dry-run prepared observations = %d, want 0 (selection integrity fails before observation)", observer.preparedCalls)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("dry-run changed state: %+v", state.Installations)
	}
}

func TestRepairRejectsStaleTargetAndRollsBackFailure(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/repair")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(installed.Plan.ActivePath, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.TargetLocator = filepath.Join(t.TempDir(), "outside")
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	input.OperationID = "repair-stale"
	if _, err := service.Repair(context.Background(), input); err == nil || !strings.Contains(err.Error(), "untrusted persisted target") {
		t.Fatalf("stale repair error = %v", err)
	}

	state, _ = store.Load()
	for key, binding := range state.Installations[0].Clients {
		binding.TargetLocator = installed.Plan.ActivePath
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	service.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.FaultActivationApplied {
			return fmt.Errorf("injected repair failure")
		}
		return nil
	}
	input.OperationID = "repair-rollback"
	if _, err := service.Repair(context.Background(), input); err == nil {
		t.Fatal("repair unexpectedly succeeded")
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "tampered" {
		t.Fatalf("rollback did not restore prior directory: %q", body)
	}
}

func TestRepairPropagatesIndeterminateVerificationFailure(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/repair-indeterminate")
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	service.Stager = verificationFailureStager{PackageStager: service.Stager, err: fmt.Errorf("permission denied")}
	if _, err := service.Repair(context.Background(), input); err == nil || !strings.Contains(err.Error(), "could not be determined") {
		t.Fatalf("indeterminate repair error = %v", err)
	}
	stateAfter, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(stateAfter.Installations[0]).Receipts) != len(onlyBinding(stateBefore.Installations[0]).Receipts) {
		t.Fatalf("indeterminate verification wrote a receipt: %+v", onlyBinding(stateAfter.Installations[0]).Receipts)
	}
}

func TestRepairReceiptRecordsObservedDigestOrAbsence(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"mismatch", "absent"} {
		t.Run(mode, func(t *testing.T) {
			service, _, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/repair-before-"+mode)
			input.Confirmed = true
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "mismatch" {
				if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
					t.Fatal(err)
				}
			} else if err := os.RemoveAll(installed.Plan.ActivePath); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "repair-before-" + mode
			result, err := service.Repair(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "absent" && result.Receipt.BeforeDigest != "" {
				t.Fatalf("absent BeforeDigest = %q", result.Receipt.BeforeDigest)
			}
			if mode == "mismatch" && (result.Receipt.BeforeDigest == "" || result.Receipt.BeforeDigest == result.Receipt.AfterDigest) {
				t.Fatalf("mismatch receipt = %+v", result.Receipt)
			}
		})
	}
}

func TestRepairAbortsWhenNativeObjectChangesBetweenPreflightAndCommit(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/repair-race")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(installed.Plan.ActivePath, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte("reviewed-tamper"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	service.Stager = &changingRepairStager{PackageStager: service.Stager, changePath: manifestPath}
	input.OperationID = "repair-race"
	if _, err := service.Repair(context.Background(), input); err == nil || !strings.Contains(err.Error(), "changed after repair preflight") {
		t.Fatalf("repair race error = %v", err)
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(after.Installations[0]).Receipts) != len(onlyBinding(before.Installations[0]).Receipts) {
		t.Fatalf("repair race committed a receipt: %+v", onlyBinding(after.Installations[0]).Receipts)
	}
}

func TestKiroCapabilityPreflightFailureChangesNoStateOrNativeConfiguration(t *testing.T) {
	service, store, _ := serviceFixture(t)
	activator := &observedActivator{preflightErr: errors.New("manual_activation_required: delegated cgroup unavailable")}
	service.Activator = activator
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	input := addInput(t, kiro, "./kiro-capability-preflight")
	input.BackendExecutable = "/test/bin/kiro-cli"
	input.Confirmed = true
	result, err := service.Add(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "manual_activation_required") {
		t.Fatalf("capability preflight error = %v", err)
	}
	if activator.calls != 0 {
		t.Fatalf("activation ran %d time(s) after failed capability preflight", activator.calls)
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("failed capability preflight changed state: %+v", state.Installations)
	}
	if result.Plan.ActivePath != "" {
		if _, statErr := os.Lstat(result.Plan.ActivePath); !os.IsNotExist(statErr) {
			t.Fatalf("failed capability preflight materialized native path: %v", statErr)
		}
	}
}

func TestRepairCorrectsDegradedStateAndPreservesUnverifiedExternalLifecycle(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	input := addInput(t, client, "https://example.com/repair-state")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Materialization = domain.MaterializationDegraded
		binding.Activation = domain.ActivationFailed
		binding.Authentication = domain.AuthenticationFailed
		binding.Verification = domain.VerificationFailed
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	result, err := service.Repair(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || result.NoChange || result.Receipt.OperationID != "" {
		t.Fatalf("state-only repair result = %+v", result)
	}
	state, _ = store.Load()
	binding := onlyBinding(state.Installations[0])
	if binding.Materialization != domain.MaterializationMaterialized || binding.Verification != domain.VerificationPackageValid ||
		binding.Activation != domain.ActivationFailed || binding.Authentication != domain.AuthenticationFailed {
		t.Fatalf("corrected state = %+v", binding)
	}

	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "plugin.json"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load()
	beforeReceipts := len(onlyBinding(state.Installations[0]).Receipts)
	for key, current := range state.Installations[0].Clients {
		current.Materialization = domain.MaterializationDegraded
		current.Verification = domain.VerificationFailed
		state.Installations[0].Clients[key] = current
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	input.OperationID = "repair-degraded-replacement"
	result, err = service.Repair(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, _ = store.Load()
	binding = onlyBinding(state.Installations[0])
	if !result.Mutated || result.Receipt.OperationID != input.OperationID || len(binding.Receipts) != beforeReceipts+1 {
		t.Fatalf("replacement result/state = %+v / %+v", result, binding)
	}
	if binding.Materialization != domain.MaterializationMaterialized || binding.Verification != domain.VerificationPackageValid ||
		binding.Activation != domain.ActivationFailed || binding.Authentication != domain.AuthenticationFailed {
		t.Fatalf("replacement lifecycle overclaimed = %+v", binding)
	}
}

func fullyConvergedOutcome(outcome domain.ActivationOutcome) bool {
	return outcome.Activation == domain.ActivationActive && outcome.Authentication == domain.AuthenticationComplete && outcome.Verification == domain.VerificationInstalled
}

func TestAddRejectsSameNativePluginNameFromDifferentSources(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	first := addInput(t, client, "https://example.com/one")
	first.Confirmed = true
	if _, err := service.Add(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := addInput(t, client, "https://example.com/two")
	second.Confirmed = true
	second.InstallationID = "00000000-0000-4000-8000-000000000002"
	second.OperationID = "operation-two"
	if _, err := service.Add(context.Background(), second); err == nil || !strings.Contains(err.Error(), "native client name collision") {
		t.Fatalf("collision error = %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("collision mutated state: %+v", state.Installations)
	}
}

func TestUpdateReplacesExistingArtifactAndPreservesReceiptHistory(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	first := addInput(t, client, "https://example.com/one")
	first.Confirmed = true
	if _, err := service.Add(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	second := addInput(t, client, "https://example.com/one")
	second.Confirmed = true
	second.OperationID = "operation-update"
	second.Envelope.Manifest.Version = "2.0.0"
	second.Envelope.TreeDigest = "sha256:source-tree-v2"
	second.Envelope.ManifestDigest = "sha256:manifest-v2"
	manifestPath := filepath.Join(second.Envelope.SnapshotRoot, "plugin.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(strings.Replace(string(body), "1.0.0", "2.0.0", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := service.Update(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].Package.Version != "2.0.0" {
		t.Fatalf("package = %+v", state.Installations[0].Package)
	}
	clientState := onlyBinding(state.Installations[0])
	if len(clientState.Receipts) != 2 || clientState.Receipts[1].BeforeDigest != clientState.Receipts[0].AfterDigest {
		t.Fatalf("receipts = %+v", clientState.Receipts)
	}
	installed, err := os.ReadFile(filepath.Join(result.Plan.ActivePath, "plugin.json"))
	if err != nil || !strings.Contains(string(installed), "2.0.0") {
		t.Fatalf("installed manifest = %q, err=%v", installed, err)
	}
}

func TestUpdateRefusesUserEditedManagedPackage(t *testing.T) {
	t.Parallel()
	service, _, client := serviceFixture(t)
	first := addInput(t, client, "https://example.com/one")
	first.Confirmed = true
	installed, err := service.Add(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "user-note.txt"), []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	update := addInput(t, client, "https://example.com/one")
	update.Confirmed = true
	update.OperationID = "operation-update-edited"
	if _, err := service.Update(context.Background(), update); err == nil || !strings.Contains(err.Error(), "refusing silent update") {
		t.Fatalf("update error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed.Plan.ActivePath, "user-note.txt")); err != nil {
		t.Fatalf("user file was lost: %v", err)
	}
}

func TestUpdateRejectsSameVersionDigestConflictAndDowngrade(t *testing.T) {
	t.Parallel()
	for name, version := range map[string]string{"same_version_conflict": "1.0.0", "downgrade": "0.9.0"} {
		name, version := name, version
		t.Run(name, func(t *testing.T) {
			service, _, client := serviceFixture(t)
			first := addInput(t, client, "https://example.com/one")
			first.Confirmed = true
			if _, err := service.Add(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			update := addInput(t, client, "https://example.com/one")
			update.Confirmed = true
			update.Envelope.Manifest.Version = version
			update.Envelope.TreeDigest = "sha256:different-tree"
			update.Envelope.ManifestDigest = "sha256:different-manifest"
			_, err := service.Update(context.Background(), update)
			if err == nil {
				t.Fatal("unsafe update succeeded")
			}
			want := "supply-chain conflict"
			if name == "downgrade" {
				want = "downgrade"
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("update error = %v", err)
			}
		})
	}
}

func TestUpdateRejectsIncomparableOpaqueVersionTransitions(t *testing.T) {
	t.Parallel()
	for name, versions := range map[string][2]string{
		"stable_to_opaque": {"1.0.0", "release-candidate"},
		"opaque_to_stable": {"release-candidate", "1.0.0"},
	} {
		name, versions := name, versions
		t.Run(name, func(t *testing.T) {
			service, _, client := serviceFixture(t)
			first := addInput(t, client, "https://example.com/opaque")
			first.Envelope.Manifest.Version = versions[0]
			first.Confirmed = true
			if _, err := service.Add(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			update := addInput(t, client, "https://example.com/opaque")
			update.Envelope.Manifest.Version = versions[1]
			update.Envelope.TreeDigest = "sha256:opaque-transition"
			update.Envelope.ManifestDigest = "sha256:opaque-transition-manifest"
			update.Confirmed = true
			if _, err := service.Update(context.Background(), update); err == nil || !strings.Contains(err.Error(), "incomparable package version transition") {
				t.Fatalf("update error = %v", err)
			}
		})
	}
}

func TestExistingSourceRejectsManifestRenameWithoutChangingAnyClient(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	first := addInput(t, cursor, "https://example.com/identity")
	first.Confirmed = true
	cursorResult, err := service.Add(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	codex := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}
	second := addInput(t, codex, "https://example.com/identity")
	second.Confirmed = true
	second.OperationID = "operation-identity-codex"
	codexResult, err := service.Add(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	rename := addInput(t, cursor, "https://example.com/identity")
	rename.Envelope.Manifest.Name = "renamed-demo"
	rename.Envelope.Manifest.Version = "2.0.0"
	rename.Envelope.TreeDigest = "sha256:renamed-tree"
	rename.Envelope.ManifestDigest = "sha256:renamed-manifest"
	rename.Confirmed = true
	if _, err := service.Update(context.Background(), rename); err == nil || !strings.Contains(err.Error(), "refuse package identity change") {
		t.Fatalf("rename update error = %v", err)
	}
	thirdClient := domain.DetectedClient{
		ClientID: domain.ClientKiro, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".kiro"),
	}
	rename.Client = thirdClient
	if _, err := service.Add(context.Background(), rename); err == nil || !strings.Contains(err.Error(), "refuse package identity change") {
		t.Fatalf("rename add-target error = %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 || state.Installations[0].DeclaredName != "demo" || state.Installations[0].Package.Version != "1.0.0" {
		t.Fatalf("state changed after rename attempts: %+v", state)
	}
	for _, path := range []string{cursorResult.Plan.ActivePath, codexResult.Plan.ActivePath} {
		if _, err := os.Stat(filepath.Join(path, "plugin.json")); err != nil {
			t.Fatalf("existing artifact changed: %v", err)
		}
	}
}

func TestUpdateReturnsNoChangeWithoutMutation(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	first := addInput(t, client, "https://example.com/one")
	first.Confirmed = true
	if _, err := service.Add(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationActive
		binding.Authentication = domain.AuthenticationNotRequired
		binding.Verification = domain.VerificationInstalled
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	result, err := service.Update(context.Background(), addInput(t, client, "https://example.com/one"))
	if err != nil {
		t.Fatal(err)
	}
	if !result.NoChange || result.Mutated || result.RequiresConfirmation {
		t.Fatalf("update result = %+v", result)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBinding(state.Installations[0]).Receipts) != 1 {
		t.Fatalf("no-op update wrote a receipt: %+v", onlyBinding(state.Installations[0]).Receipts)
	}
}

func TestLifecycleConvergenceRequiresActivationAuthenticationAndVerificationForEveryClient(t *testing.T) {
	t.Parallel()
	manual := domain.ClientBinding{Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotRequired, Verification: domain.VerificationPackageValid}
	for _, client := range []domain.ClientID{domain.ClientCursor, domain.ClientCodex, domain.ClientKiro, domain.ClientCopilot, domain.ClientVSCode} {
		manual.ClientID = string(client)
		if lifecycleConverged(manual) {
			t.Fatalf("manual lifecycle for %s was reported converged", client)
		}
		manual.Activation = domain.ActivationActive
		manual.Verification = domain.VerificationInstalled
		if !lifecycleConverged(manual) {
			t.Fatalf("installed lifecycle for %s should be converged", client)
		}
		manual.Authentication = domain.AuthenticationPending
		if lifecycleConverged(manual) {
			t.Fatalf("auth-pending lifecycle for %s was reported converged", client)
		}
		manual.Authentication = domain.AuthenticationNotRequired
		manual.Activation = domain.ActivationManual
		manual.Verification = domain.VerificationPackageValid
	}
}

func TestCopilotAndVSCodeNativeBackendFamilyMapping(t *testing.T) {
	t.Parallel()
	if !sameNativeBackend(domain.ClientCopilot, domain.ClientVSCode) || sameNativeBackend(domain.ClientCursor, domain.ClientVSCode) {
		t.Fatal("native backend family mapping is incorrect")
	}
}

func TestMultiClientUpdateConvergesEachClientRevision(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	evidence := func(digest string) *domain.CatalogEvidence {
		return &domain.CatalogEvidence{
			SchemaVersion: 2, CatalogVersion: "0.2.0", Repository: "example/catalog",
			Revision: strings.Repeat("a", 40), Digest: digest, MinimumCLIVersion: "0.1.6",
			Compatibility: map[string]domain.CatalogCompatibility{
				"cursor": {Package: "native"}, "codex": {Package: "projected"},
			},
		}
	}
	codex := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}
	firstCursor := addInput(t, cursor, "https://example.com/shared")
	firstCursor.Envelope.CatalogEvidence = evidence("sha256:catalog-v1")
	firstCursor.Confirmed = true
	firstCursor.OperationID = "operation-cursor-v1"
	if _, err := service.Add(context.Background(), firstCursor); err != nil {
		t.Fatal(err)
	}
	firstCodex := addInput(t, codex, "https://example.com/shared")
	firstCodex.Envelope.CatalogEvidence = evidence("sha256:catalog-v1")
	firstCodex.Confirmed = true
	firstCodex.OperationID = "operation-codex-v1"
	if _, err := service.Add(context.Background(), firstCodex); err != nil {
		t.Fatal(err)
	}

	updateCursor := addInput(t, cursor, "https://example.com/shared")
	setEnvelopeVersion(t, &updateCursor.Envelope, "2.0.0", "sha256:tree-v2", "sha256:manifest-v2")
	updateCursor.Envelope.CatalogEvidence = evidence("sha256:catalog-v2")
	updateCursor.Confirmed = true
	updateCursor.OperationID = "operation-cursor-v2"
	if result, err := service.Update(context.Background(), updateCursor); err != nil || result.NoChange {
		t.Fatalf("cursor update = %+v, err=%v", result, err)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if revision := revisionForClient(t, state.Installations[0], domain.ClientCodex); revision.Version != "1.0.0" ||
		revision.CatalogEvidence == nil || revision.CatalogEvidence.Digest != "sha256:catalog-v1" {
		t.Fatalf("Codex revision advanced before its update: %+v", state.Installations[0].Clients)
	}
	if revision := revisionForClient(t, state.Installations[0], domain.ClientCursor); revision.CatalogEvidence == nil || revision.CatalogEvidence.Digest != "sha256:catalog-v2" {
		t.Fatalf("Cursor catalog evidence did not advance with its revision: %+v", revision)
	}

	updateCodex := addInput(t, codex, "https://example.com/shared")
	setEnvelopeVersion(t, &updateCodex.Envelope, "2.0.0", "sha256:tree-v2", "sha256:manifest-v2")
	updateCodex.Envelope.CatalogEvidence = evidence("sha256:catalog-v2")
	updateCodex.Confirmed = true
	updateCodex.OperationID = "operation-codex-v2"
	if result, err := service.Update(context.Background(), updateCodex); err != nil || result.NoChange || !result.Mutated {
		t.Fatalf("codex update = %+v, err=%v", result, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, clientID := range []domain.ClientID{domain.ClientCursor, domain.ClientCodex} {
		if revision := revisionForClient(t, state.Installations[0], clientID); revision.Version != "2.0.0" || revision.TreeDigest != "sha256:tree-v2" ||
			revision.CatalogEvidence == nil || revision.CatalogEvidence.Digest != "sha256:catalog-v2" {
			t.Fatalf("%s revision = %+v", clientID, revision)
		}
	}
}

func TestPartialMultiClientUpdateFailurePreservesIndependentRevisions(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	codex := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}

	firstCursor := addInput(t, cursor, "https://example.com/partial-update")
	firstCursor.Confirmed = true
	firstCursor.OperationID = "operation-partial-cursor-v1"
	cursorInstalled, err := service.Add(context.Background(), firstCursor)
	if err != nil {
		t.Fatal(err)
	}
	firstCodex := addInput(t, codex, "https://example.com/partial-update")
	firstCodex.Confirmed = true
	firstCodex.OperationID = "operation-partial-codex-v1"
	codexInstalled, err := service.Add(context.Background(), firstCodex)
	if err != nil {
		t.Fatal(err)
	}

	updateCursor := addInput(t, cursor, "https://example.com/partial-update")
	setEnvelopeVersion(t, &updateCursor.Envelope, "2.0.0", "sha256:partial-tree-v2", "sha256:partial-manifest-v2")
	updateCursor.Confirmed = true
	updateCursor.OperationID = "operation-partial-cursor-v2"
	if result, updateErr := service.Update(context.Background(), updateCursor); updateErr != nil || !result.Mutated {
		t.Fatalf("cursor update = %+v, err=%v", result, updateErr)
	}

	userFile := filepath.Join(codexInstalled.Plan.ActivePath, "user-note.txt")
	if err := os.WriteFile(userFile, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}
	updateCodex := addInput(t, codex, "https://example.com/partial-update")
	setEnvelopeVersion(t, &updateCodex.Envelope, "2.0.0", "sha256:partial-tree-v2", "sha256:partial-manifest-v2")
	updateCodex.Confirmed = true
	updateCodex.OperationID = "operation-partial-codex-v2"
	if _, err := service.Update(context.Background(), updateCodex); err == nil || !strings.Contains(err.Error(), "refusing silent update") {
		t.Fatalf("codex update error = %v", err)
	}

	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := state.Installations[0]
	if revision := revisionForClient(t, installation, domain.ClientCursor); revision.Version != "2.0.0" || revision.TreeDigest != "sha256:partial-tree-v2" {
		t.Fatalf("cursor revision = %+v", revision)
	}
	if revision := revisionForClient(t, installation, domain.ClientCodex); revision.Version != "1.0.0" || revision.TreeDigest != "sha256:source-tree" {
		t.Fatalf("codex revision advanced after failed update: %+v", revision)
	}

	cursorManifest, err := os.ReadFile(filepath.Join(cursorInstalled.Plan.ActivePath, "plugin.json"))
	if err != nil || !strings.Contains(string(cursorManifest), "2.0.0") {
		t.Fatalf("cursor manifest = %q, err=%v", cursorManifest, err)
	}
	codexManifest, err := os.ReadFile(filepath.Join(codexInstalled.Plan.ActivePath, "plugin.json"))
	if err != nil || !strings.Contains(string(codexManifest), "1.0.0") {
		t.Fatalf("codex manifest = %q, err=%v", codexManifest, err)
	}
	if body, err := os.ReadFile(userFile); err != nil || string(body) != "preserve me" {
		t.Fatalf("user file = %q, err=%v", body, err)
	}
}

func TestAddNewTargetEnforcesExistingPackageIdentity(t *testing.T) {
	t.Parallel()
	service, _, cursor := serviceFixture(t)
	first := addInput(t, cursor, "https://example.com/shared")
	first.Confirmed = true
	if _, err := service.Add(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	conflict := addInput(t, codex, "https://example.com/shared")
	conflict.Envelope.TreeDigest = "sha256:changed-same-version"
	conflict.Envelope.ManifestDigest = "sha256:changed-manifest"
	conflict.Confirmed = true
	if _, err := service.Add(context.Background(), conflict); err == nil || !strings.Contains(err.Error(), "supply-chain conflict") {
		t.Fatalf("same-version add error = %v", err)
	}
}

func TestDirectorySuspensionAndRevocationOperationBoundaries(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	directory := &domain.DirectoryOrigin{
		ProductID: "demo", DistributionID: "publisher/demo", DistributionKind: domain.DistributionUpstream,
		DesiredReleaseSequence: 7, SnapshotSchema: 1, SnapshotSequence: 42, SnapshotDigest: "sha256:snapshot",
	}
	installedInput := addInput(t, cursor, "publisher/demo")
	installedInput.OriginMode, installedInput.DirectoryResolution = domain.OriginModeDirectory, directory
	installedInput.Envelope.Source.ResolvedRevision = strings.Repeat("d", 40)
	installedInput.Confirmed = true
	installedInput.OperationID = "directory-initial"
	installed, err := service.Add(context.Background(), installedInput)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	if binding.PackageRevision == nil || binding.PackageRevision.DistributionID != directory.DistributionID || binding.PackageRevision.ReleaseSequence != 7 {
		t.Fatalf("Directory applied revision = %+v", binding.PackageRevision)
	}

	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	suspendedExposure := addInput(t, kiro, "publisher/demo")
	suspendedExposure.OriginMode, suspendedExposure.DirectoryResolution = domain.OriginModeDirectory, directory
	suspendedExposure.Envelope.Source.ResolvedRevision = strings.Repeat("d", 40)
	suspendedExposure.DistributionSuspended = true
	suspendedExposure.Confirmed = true
	if _, err := service.Add(context.Background(), suspendedExposure); err == nil || !strings.Contains(err.Error(), "suspended") {
		t.Fatalf("suspended new-target exposure = %v", err)
	}
	if _, err := service.Repair(context.Background(), AddInput{
		Envelope: installedInput.Envelope, Client: cursor, Scope: domain.ScopeUser, InstallationID: installed.InstallationID,
		OriginMode: domain.OriginModeDirectory, DirectoryResolution: directory, DistributionSuspended: true,
	}); err != nil {
		t.Fatalf("suspension blocked exact non-revoked repair: %v", err)
	}
	revokedRepair := installedInput
	revokedRepair.ReleaseRevoked = true
	if _, err := service.Repair(context.Background(), revokedRepair); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked repair = %v", err)
	}
	revokedNew := addInput(t, kiro, "publisher/demo")
	revokedNew.OriginMode, revokedNew.DirectoryResolution, revokedNew.ReleaseRevoked = domain.OriginModeDirectory, directory, true
	revokedNew.Envelope.Source.ResolvedRevision = strings.Repeat("d", 40)
	if _, err := service.Add(context.Background(), revokedNew); err == nil || !strings.Contains(err.Error(), "revoked") {
		t.Fatalf("revoked new exposure = %v", err)
	}

	directService, _, directClient := serviceFixture(t)
	directWarning := addInput(t, directClient, "./known-revoked-bytes")
	directWarning.ReleaseRevoked, directWarning.DryRun = true, true
	result, err := directService.Add(context.Background(), directWarning)
	if err != nil {
		t.Fatal(err)
	}
	foundRevokedWarning := false
	for _, warning := range result.Plan.Warnings {
		if strings.Contains(warning, "known_revoked") {
			foundRevokedWarning = true
		}
	}
	if !foundRevokedWarning {
		t.Fatalf("direct revoked-digest warning = %+v", result.Plan.Warnings)
	}
}

func TestRemoveCommitsReceiptAndLeavesBindingAbsent(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	add := addInput(t, client, "https://example.com/one")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "operation-remove",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated {
		t.Fatalf("remove result = %+v", removed)
	}
	if _, err := os.Lstat(installed.Plan.ActivePath); !os.IsNotExist(err) {
		t.Fatalf("managed package still exists: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained {
		t.Fatalf("last-binding remove did not retain minimal installation: %+v", state.Installations)
	}
	if len(state.Installations[0].DataReceipts) != 1 {
		t.Fatalf("last-binding remove did not establish one owned PLUGIN_DATA receipt: %+v", state.Installations[0].DataReceipts)
	}
	for _, receipt := range state.Installations[0].DataReceipts {
		if receipt.State != domain.DataReceiptOwned {
			t.Fatalf("retained receipt = %+v", receipt)
		}
	}
	if removed.Receipt.MutationType != "directory_remove" || removed.Receipt.Phase != transaction.ReceiptPhaseCommitted {
		t.Fatalf("remove receipt = %+v", removed.Receipt)
	}
}

func TestPluginDataMarkerSurvivesUpdateRepairAndNormalRemovalUntilOwnedPurge(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	codex := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	withStdio := func(input *AddInput) {
		input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"local": {Name: "local", Type: "stdio", Decoded: map[string]any{
				"type": "stdio", "command": "sh", "args": []any{"-c", "echo ${PLUGIN_DATA}"},
			}},
		}}
		input.Envelope.Inventory.MCPServers = []string{"local"}
	}
	add := addInput(t, codex, "./data-plugin")
	withStdio(&add)
	add.Confirmed, add.OperationID = true, "data-add"
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(state.Installations[0])
	receipt := state.Installations[0].DataReceipts[binding.DataReceiptID]
	marker := filepath.Join(receipt.Locator, "persistent-marker")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	projected := readUsecaseObject(t, filepath.Join(installed.Plan.ActivePath, ".mcp.json"))
	projectedEnv := projected["mcpServers"].(map[string]any)["local"].(map[string]any)["env"].(map[string]any)
	if projectedEnv["PLUGIN_DATA"] != receipt.Locator {
		t.Fatalf("projected PLUGIN_DATA %v != receipt %s", projectedEnv["PLUGIN_DATA"], receipt.Locator)
	}

	update := addInput(t, codex, "./data-plugin")
	withStdio(&update)
	setEnvelopeVersion(t, &update.Envelope, "2.0.0", "sha256:data-tree-v2", "sha256:data-manifest-v2")
	update.Confirmed, update.OperationID = true, "data-update"
	if result, err := service.Update(context.Background(), update); err != nil || !result.Mutated {
		t.Fatalf("data-preserving update = %+v, %v", result, err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep" {
		t.Fatalf("update changed PLUGIN_DATA marker: %q %v", body, err)
	}
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "damage"), []byte("repair me"), 0o600); err != nil {
		t.Fatal(err)
	}
	update.OperationID = "data-repair"
	if result, err := service.Repair(context.Background(), update); err != nil || !result.Mutated {
		t.Fatalf("data-preserving repair = %+v, %v", result, err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep" {
		t.Fatalf("repair changed PLUGIN_DATA marker: %q %v", body, err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: installed.InstallationID, Client: codex, Scope: domain.ScopeUser,
		Confirmed: true, ExternalUninstalled: true, OperationID: "data-remove"}); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep" {
		t.Fatalf("normal removal changed PLUGIN_DATA marker: %q %v", body, err)
	}
	if err := service.PurgeRetainedData(context.Background(), installed.InstallationID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(receipt.Locator); !os.IsNotExist(err) {
		t.Fatalf("owned purge retained PLUGIN_DATA: %v", err)
	}
}

func TestNormalRemoveRollbackDoesNotStrandNewPluginDataOwnership(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	add := addInput(t, client, "./rollback-data")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	faulted := false
	service.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivationPending && !faulted {
			faulted = true
			return fmt.Errorf("injected removal failure")
		}
		return nil
	}
	_, err = service.Remove(context.Background(), RemoveInput{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "rollback-data-remove"})
	if err == nil || transaction.FailurePhase(err) != transaction.GroupFailureRolledBack {
		t.Fatalf("removal failure = %v, phase=%q", err, transaction.FailurePhase(err))
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 || len(state.Installations[0].DataReceipts) != 0 {
		t.Fatalf("rolled-back removal stranded state: %+v", state)
	}
	if _, statErr := os.Stat(installed.Plan.ActivePath); statErr != nil {
		t.Fatalf("rolled-back managed package was not restored: %v", statErr)
	}
	dataBase := service.PluginData.(providers.PluginDataManager).Base
	if entries, readErr := os.ReadDir(dataBase); readErr == nil && len(entries) != 0 {
		t.Fatalf("rolled-back removal stranded PLUGIN_DATA: %v", entries)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
}

func TestRetainedDataPurgePreflightsEveryReceiptBeforeRename(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	add := addInput(t, cursor, "./preflight-data")
	add.Confirmed = true
	added, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: added.InstallationID, Client: cursor, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "preflight-data-remove"}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	manager := service.PluginData.(providers.PluginDataManager)
	second, _, err := manager.EnsureData(context.Background(), added.InstallationID, "second-backend", string(domain.ScopeUser))
	if err != nil {
		t.Fatal(err)
	}
	state.Installations[0].DataReceipts[second.DataReceiptID] = second
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].DataReceipts) != 2 {
		t.Fatalf("retained data state = %+v", state)
	}
	paths := make([]string, 0, 2)
	for key, receipt := range state.Installations[0].DataReceipts {
		paths = append(paths, receipt.Locator)
		receipt.OwnershipDigest = "sha256:stale"
		state.Installations[0].DataReceipts[key] = receipt
		break
	}
	for _, receipt := range state.Installations[0].DataReceipts {
		if !slices.Contains(paths, receipt.Locator) {
			paths = append(paths, receipt.Locator)
		}
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := service.PurgeRetainedData(context.Background(), added.InstallationID, true); err == nil {
		t.Fatal("purge accepted a stale ownership receipt")
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations) != 1 {
		t.Fatalf("failed purge changed retained state: %+v", after)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("failed purge renamed data before complete preflight: %s: %v", path, err)
		}
	}
}

func TestRetainedDataPurgeRollsBackAndRetriesAfterRenameFailure(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	add := addInput(t, client, "./retry-purge")
	add.Confirmed = true
	added, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{Selector: added.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "retry-purge-remove"}); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var dataPath string
	for _, receipt := range state.Installations[0].DataReceipts {
		dataPath = receipt.Locator
	}
	faulted := false
	service.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseActivationPending && !faulted {
			faulted = true
			return fmt.Errorf("injected purge failure")
		}
		return nil
	}
	if err := service.PurgeRetainedData(context.Background(), added.InstallationID, true); err == nil || transaction.FailurePhase(err) != transaction.GroupFailureRolledBack {
		t.Fatalf("purge failure = %v, phase=%q", err, transaction.FailurePhase(err))
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 {
		t.Fatalf("rolled-back purge changed state: %+v", state)
	}
	if _, err := os.Stat(dataPath); err != nil {
		t.Fatalf("rolled-back purge did not restore data: %v", err)
	}
	service.Kernel.Directory.Fault = nil
	if err := service.PurgeRetainedData(context.Background(), added.InstallationID, true); err != nil {
		t.Fatal(err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("retried purge retained state: %+v", state)
	}
	if _, err := os.Stat(dataPath); !os.IsNotExist(err) {
		t.Fatalf("retried purge retained data: %v", err)
	}
}

func TestRemovePlanExposesExternalUninstallAndPreservesCodexArtifactUntilAcknowledged(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}
	add := addInput(t, client, "https://example.com/copied")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []RemoveInput{
		{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser, DryRun: true},
		{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true},
	} {
		result, err := service.Remove(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		if result.Mutated || result.Deactivation.ArtifactRemovalAllowed || len(result.Deactivation.UserActions) != 1 {
			t.Fatalf("blocked remove result = %+v", result)
		}
		if _, err := os.Stat(installed.Plan.ActivePath); err != nil {
			t.Fatalf("managed artifact changed before acknowledgement: %v", err)
		}
	}
	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, ExternalUninstalled: true, OperationID: "operation-remove-codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated || !removed.Deactivation.ExternalRemovalComplete {
		t.Fatalf("acknowledged remove result = %+v", removed)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained {
		t.Fatalf("acknowledged external removal did not leave minimal data-retained state: %+v", state.Installations)
	}
}

func TestRemoveCleansNativeCodexMarketplaceBeforeManagedArtifactDeletion(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{
		ClientID: domain.ClientCodex, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), ".codex"),
	}
	runner := &codexCleanupUsecaseRunner{configRoot: client.ConfigRoot}
	if result := runner.writeConfig(false); result.ExitCode != 0 {
		t.Fatalf("seed Codex config: %s", result.Stderr)
	}
	service.Activator = providers.Activator{Runner: runner}
	add := addInput(t, client, "https://example.com/codex-cleanup")
	add.Confirmed = true
	add.BackendExecutable = "/test/bin/codex"
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, ExternalUninstalled: true, BackendExecutable: "/test/bin/codex",
		OperationID: "operation-remove-native-codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated || !removed.Deactivation.ExternalRemovalComplete {
		t.Fatalf("remove result = %+v", removed)
	}
	marketplace := providers.ManagedMarketplaceName(installed.Plan.PhysicalArtifactID)
	wantCleanup := []string{"/test/bin/codex", "plugin", "marketplace", "remove", marketplace, "--json"}
	if got := runner.commands[len(runner.commands)-1].Argv; !reflect.DeepEqual(got, wantCleanup) {
		t.Fatalf("last command = %#v, want cleanup %#v", got, wantCleanup)
	}
	config, err := os.ReadFile(filepath.Join(client.ConfigRoot, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), marketplace) || !strings.Contains(string(config), "user-marketplace") {
		t.Fatalf("Codex config did not preserve only the user marketplace:\n%s", config)
	}
	if _, err := os.Lstat(installed.Plan.ActivePath); !os.IsNotExist(err) {
		t.Fatalf("managed Codex artifact survived successful cleanup: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained {
		t.Fatalf("remove state = %+v", state.Installations)
	}
}

func TestRemoveRefusesUserEditedManagedPackage(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	add := addInput(t, client, "https://example.com/one")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	userFile := filepath.Join(installed.Plan.ActivePath, "user-note.txt")
	if err := os.WriteFile(userFile, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "operation-remove-edited",
	}); err == nil || !strings.Contains(err.Error(), "refusing silent removal") {
		t.Fatalf("remove error = %v", err)
	}
	if _, err := os.Stat(userFile); err != nil {
		t.Fatalf("user file was removed: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if onlyBinding(state.Installations[0]).Materialization != domain.MaterializationMaterialized {
		t.Fatalf("failed remove changed materialization: %+v", onlyBinding(state.Installations[0]))
	}
}

func TestRemoveRetainsManagedPackageContainingExcludedOwnershipMarkers(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{filepath.Join(".git", "config"), filepath.Join("nested", ".plugin-kit-ai.lock")} {
		marker := marker
		t.Run(marker, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			add := addInput(t, client, "https://example.com/marker-"+filepath.Base(marker))
			add.Confirmed = true
			installed, err := service.Add(context.Background(), add)
			if err != nil {
				t.Fatal(err)
			}
			userFile := filepath.Join(installed.Plan.ActivePath, marker)
			if err := os.MkdirAll(filepath.Dir(userFile), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(userFile, []byte("keep me"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := service.Remove(context.Background(), RemoveInput{
				Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
				Confirmed: true, OperationID: "remove-marker-" + strings.ReplaceAll(filepath.Base(marker), ".", "x"),
			}); err == nil || !strings.Contains(err.Error(), "excluded ownership marker") {
				t.Fatalf("remove error = %v", err)
			}
			if _, err := os.Stat(userFile); err != nil {
				t.Fatalf("ownership marker was removed: %v", err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if onlyBinding(state.Installations[0]).Materialization != domain.MaterializationMaterialized {
				t.Fatalf("failed remove changed state: %+v", onlyBinding(state.Installations[0]))
			}
		})
	}
}

func TestRemoveRejectsTamperedPersistedTarget(t *testing.T) {
	t.Parallel()
	service, store, client := serviceFixture(t)
	add := addInput(t, client, "https://example.com/one")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "unrelated")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.TargetLocator = outside
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "operation-remove-tampered",
	}); err == nil || !strings.Contains(err.Error(), "untrusted persisted target") {
		t.Fatalf("remove error = %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("unrelated target was removed: %v", err)
	}
}

func serviceFixture(t *testing.T) (Service, statev2.Store, domain.DetectedClient) {
	t.Helper()
	root := t.TempDir()
	store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
	managed := filepath.Join(root, "managed")
	client := domain.DetectedClient{
		ClientID: domain.ClientCursor, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(root, "home", ".cursor"),
	}
	stager := providers.Stager{}
	targetPlanner := clientplanner.Planner{ManagedRoot: managed}
	return Service{
		StateStore: store,
		Planner:    targetPlanner,
		Targets:    targetPlanner,
		Stager:     stager,
		Activator:  providers.Activator{},
		PluginData: providers.PluginDataManager{Base: filepath.Join(root, "plugin-data")},
		Lock:       processlock.Lock{Path: filepath.Join(root, "state", "mutation.lock")},
		Kernel: transaction.Kernel{
			Directory: dirswap.Manager{JournalDir: filepath.Join(root, "state", "operations-v2")},
		},
		Now: func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) },
	}, store, client
}

type verificationFailureStager struct {
	ports.PackageStager
	err error
}

type changingRepairStager struct {
	ports.PackageStager
	changePath string
	verifies   int
}

func (stager *changingRepairStager) Verify(ctx context.Context, root, expected string) error {
	stager.verifies++
	if stager.verifies == 2 {
		if err := os.WriteFile(stager.changePath, []byte("changed-after-preflight"), 0o644); err != nil {
			return err
		}
	}
	return stager.PackageStager.Verify(ctx, root, expected)
}

type observedActivator struct {
	outcome      domain.ActivationOutcome
	err          error
	preflightErr error
	calls        int
}

type capturingRemovalActivator struct {
	nativeObjects []domain.NativeObjectOwnership
}

func (*capturingRemovalActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return domain.ActivationOutcome{}, nil
}

func (activator *capturingRemovalActivator) Deactivate(_ context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	activator.nativeObjects = append([]domain.NativeObjectOwnership(nil), request.NativeObjects...)
	return domain.DeactivationOutcome{Activation: domain.ActivationNotRequired, ArtifactRemovalAllowed: true, ExternalRemovalComplete: true}, nil
}

func (activator *observedActivator) PreflightActivation(domain.ActivationRequest) error {
	return activator.preflightErr
}

type fixedUsecaseRunner struct {
	result legacyports.CommandResult
	err    error
}

type codexCleanupUsecaseRunner struct {
	commands           []legacyports.Command
	configRoot         string
	managedMarketplace string
	managedPath        string
}

func (runner *codexCleanupUsecaseRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.commands = append(runner.commands, command)
	if len(command.Argv) >= 5 && command.Argv[1] == "plugin" && command.Argv[2] == "marketplace" && command.Argv[3] == "add" {
		runner.managedPath = command.Argv[4]
		runner.managedMarketplace = providers.ManagedMarketplaceName(filepath.Base(command.Argv[4]))
		return runner.writeConfig(true), nil
	}
	if len(command.Argv) >= 5 && command.Argv[1] == "plugin" && command.Argv[2] == "marketplace" && command.Argv[3] == "remove" {
		if command.Argv[4] != runner.managedMarketplace {
			return legacyports.CommandResult{ExitCode: 1, Stderr: []byte("unexpected marketplace")}, nil
		}
		return runner.writeConfig(false), nil
	}
	if len(command.Argv) >= 4 && command.Argv[1] == "plugin" && command.Argv[2] == "list" {
		marketplace := ""
		name := "demo"
		for _, prior := range runner.commands {
			if len(prior.Argv) >= 4 && prior.Argv[1] == "plugin" && prior.Argv[2] == "add" {
				parts := strings.SplitN(prior.Argv[3], "@", 2)
				if len(parts) == 2 {
					name, marketplace = parts[0], parts[1]
				}
			}
		}
		return legacyports.CommandResult{Stdout: []byte(fmt.Sprintf(
			`{"installed":[{"pluginId":%q,"name":%q,"marketplaceName":%q,"installed":true,"enabled":true}]}`,
			name+"@"+marketplace, name, marketplace,
		))}, nil
	}
	return legacyports.CommandResult{}, nil
}

func (runner *codexCleanupUsecaseRunner) writeConfig(includeManaged bool) legacyports.CommandResult {
	if err := os.MkdirAll(runner.configRoot, 0o700); err != nil {
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte(err.Error())}
	}
	body := "[marketplaces.user-marketplace]\nsource_type = \"local\"\nsource = \"/user/marketplace\"\n"
	if includeManaged {
		body += fmt.Sprintf("\n[marketplaces.%s]\nsource_type = \"local\"\nsource = %q\n", runner.managedMarketplace, runner.managedPath)
	}
	if err := os.WriteFile(filepath.Join(runner.configRoot, "config.toml"), []byte(body), 0o600); err != nil {
		return legacyports.CommandResult{ExitCode: 1, Stderr: []byte(err.Error())}
	}
	return legacyports.CommandResult{}
}

func (runner fixedUsecaseRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return runner.result, runner.err
}

type trackingMutationLock struct {
	held         *atomic.Bool
	acquisitions int
	onAcquire    func() error
}

func (lock *trackingMutationLock) Acquire(context.Context) (ports.UnlockFunc, error) {
	lock.acquisitions++
	if !lock.held.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("mutation lock acquired twice")
	}
	if lock.onAcquire != nil {
		if err := lock.onAcquire(); err != nil {
			lock.held.Store(false)
			return nil, err
		}
	}
	return func() error {
		if !lock.held.CompareAndSwap(true, false) {
			return fmt.Errorf("mutation lock released while not held")
		}
		return nil
	}, nil
}

type lockAssertingStore struct {
	transaction.StateStore
	held *atomic.Bool
}

func (store lockAssertingStore) Save(state domain.StateFileV2) error {
	if !store.held.Load() {
		return fmt.Errorf("state save occurred outside mutation lock")
	}
	return store.StateStore.Save(state)
}

func (activator *observedActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	return activator.outcome, activator.err
}

func (*observedActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}

func (stager verificationFailureStager) Verify(context.Context, string, string) error {
	return stager.err
}

func addInput(t *testing.T, client domain.DetectedClient, canonicalSource string) AddInput {
	t.Helper()
	snapshot := filepath.Join(t.TempDir(), "snapshot")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "demo",
  "version": "1.0.0",
  "description": "Demo plugin"
}`
	if err := os.WriteFile(filepath.Join(snapshot, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return AddInput{
		Envelope: domain.PackageEnvelope{
			LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1,
			SchemaURI: domain.PluginSchemaV1, SchemaVersion: "1.0.0",
			Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0", Description: "Demo plugin"},
			Source: domain.SourceIdentity{
				RequestedSource: canonicalSource, CanonicalSource: canonicalSource, ResolvedRevision: "abc123",
			},
			TreeDigest: "sha256:source-tree", ManifestDigest: "sha256:manifest", SnapshotRoot: snapshot,
		},
		Client: client, Scope: domain.ScopeUser,
		InstallationID: "00000000-0000-4000-8000-000000000001",
		OperationID:    "operation-one",
	}
}

func onlyBinding(installation domain.Installation) domain.ClientBinding {
	for _, client := range installation.Clients {
		return client
	}
	return domain.ClientBinding{}
}

func readUsecaseObject(t *testing.T, path string) map[string]any {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func setEnvelopeVersion(t *testing.T, envelope *domain.PackageEnvelope, version, treeDigest, manifestDigest string) {
	t.Helper()
	envelope.Manifest.Version = version
	envelope.TreeDigest = treeDigest
	envelope.ManifestDigest = manifestDigest
	path := filepath.Join(envelope.SnapshotRoot, "plugin.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(body), "1.0.0", version, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func revisionForClient(t *testing.T, installation domain.Installation, clientID domain.ClientID) domain.ClientPackageRevision {
	t.Helper()
	for _, client := range installation.Clients {
		if client.ClientID == string(clientID) {
			if client.PackageRevision == nil {
				t.Fatalf("%s package revision is missing", clientID)
			}
			return *client.PackageRevision
		}
	}
	t.Fatalf("%s binding is missing", clientID)
	return domain.ClientPackageRevision{}
}
