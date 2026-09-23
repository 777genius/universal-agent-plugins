package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

// hostArgsStager models a host's ProjectArgs callback after UAP has staged
// the package. It retains the real stager's PLUGIN_DATA projection and digest
// verification while varying only the host-owned arguments.
type hostArgsStager struct {
	ports.PackageStager
	args       []string
	stageCalls int
	dataPaths  []string
}

func (stager *hostArgsStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	aware := stager.PackageStager.(ports.PluginDataAwareStager)
	delivery, err := aware.StageWithPluginData(ctx, envelope, plan, operationID, hints, dataPath)
	if err != nil {
		return delivery, err
	}
	stager.stageCalls++
	stager.dataPaths = append(stager.dataPaths, dataPath)
	projection := filepath.Join(delivery.StagingPath, ".mcp.json")
	body, err := os.ReadFile(projection)
	if err != nil {
		return delivery, err
	}
	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		return delivery, err
	}
	server := config["mcpServers"].(map[string]any)["local"].(map[string]any)
	server["args"] = stager.args
	body, err = json.Marshal(config)
	if err != nil {
		return delivery, err
	}
	if err := os.WriteFile(projection, body, 0o600); err != nil {
		return delivery, err
	}
	snapshot, err := (packagesnapshot.Builder{}).Build(ctx, delivery.StagingPath)
	if err != nil {
		return delivery, err
	}
	delivery.ArtifactDigest = snapshot.Digest
	if err := snapshot.Close(); err != nil {
		return delivery, err
	}
	for i := range delivery.NativeObjects {
		if delivery.NativeObjects[i].Kind == "managed_package_directory" {
			delivery.NativeObjects[i].ManagedDigest = delivery.ArtifactDigest
		}
	}
	return delivery, nil
}

type mutatingRefreshObserver struct{ path string }

type nativeRefreshStager struct {
	ports.PackageStager
	desiredNativeDigest string
}

func (stager nativeRefreshStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, dataPath string) (domain.StagedDelivery, error) {
	delivery, err := stager.PackageStager.(ports.PluginDataAwareStager).StageWithPluginData(ctx, envelope, plan, operationID, hints, dataPath)
	if err != nil {
		return delivery, err
	}
	if err := os.WriteFile(filepath.Join(delivery.StagingPath, "host-projection.txt"), []byte("refreshed"), 0o600); err != nil {
		return delivery, err
	}
	snapshot, err := (packagesnapshot.Builder{}).Build(ctx, delivery.StagingPath)
	if err != nil {
		return delivery, err
	}
	delivery.ArtifactDigest = snapshot.Digest
	if err := snapshot.Close(); err != nil {
		return delivery, err
	}
	for index := range delivery.NativeObjects {
		object := &delivery.NativeObjects[index]
		if object.Kind == "managed_package_directory" {
			object.ManagedDigest = delivery.ArtifactDigest
		} else {
			object.ManagedDigest = stager.desiredNativeDigest
		}
	}
	return delivery, nil
}

type nativeRefreshRetryActivator struct {
	t                 *testing.T
	previousOwnership []domain.NativeObjectOwnership
	calls             int
}

func (activator *nativeRefreshRetryActivator) Activate(_ context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.t.Helper()
	activator.calls++
	previousExternal := make([]domain.NativeObjectOwnership, 0, len(request.PreviousNativeObjects))
	for _, object := range request.PreviousNativeObjects {
		if object.Kind != "managed_package_directory" {
			previousExternal = append(previousExternal, object)
		}
	}
	expectedExternal := make([]domain.NativeObjectOwnership, 0, len(activator.previousOwnership))
	for _, object := range activator.previousOwnership {
		if object.Kind != "managed_package_directory" {
			expectedExternal = append(expectedExternal, object)
		}
	}
	if !reflect.DeepEqual(previousExternal, expectedExternal) || request.VerifyOnly {
		activator.t.Fatalf("refresh attempt %d previous ownership = %+v, verify only = %t", activator.calls, request.PreviousNativeObjects, request.VerifyOnly)
	}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if activator.calls == 1 {
		return outcome, fmt.Errorf("injected native activation failure")
	}
	return outcome, nil
}

func (*nativeRefreshRetryActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, nil
}

func (observer mutatingRefreshObserver) ObserveNativeIdentity(_ context.Context, _ domain.DetectedClient, _ domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if err := os.WriteFile(observer.path, []byte("changed after observation"), 0o600); err != nil {
		return domain.NativeIdentityObservation{}, err
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: managedDigest(*managed)}, nil
}

func hostProjectionFixture(t *testing.T) (Service, AddInput, *hostArgsStager) {
	t.Helper()
	service, _, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".codex")}
	input := addInput(t, client, "./refresh-host-projection")
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"local": {Name: "local", Type: "stdio", Decoded: map[string]any{"type": "stdio", "command": "sh", "args": []any{"-c", "echo ${PLUGIN_DATA}"}}},
	}}
	input.Envelope.Inventory.MCPServers = []string{"local"}
	stager := &hostArgsStager{PackageStager: service.Stager, args: []string{"--global-config", "/old/global.json", "--primary", "/old/primary"}}
	service.Stager = stager
	input.Confirmed = true
	return service, input, stager
}

func projectedHostArgs(t *testing.T, activePath string) []string {
	t.Helper()
	projected := readUsecaseObject(t, filepath.Join(activePath, ".mcp.json"))
	values := projected["mcpServers"].(map[string]any)["local"].(map[string]any)["args"].([]any)
	args := make([]string, len(values))
	for i, value := range values {
		args[i] = value.(string)
	}
	return args
}

func TestRefreshProjectionRestagesIntactPackageWithCurrentHostArgs(t *testing.T) {
	t.Parallel()
	service, input, stager := hostProjectionFixture(t)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	old := onlyBinding(state.Installations[0])
	oldData := state.Installations[0].DataReceipts[old.DataReceiptID].Locator
	if got := projectedHostArgs(t, installed.Plan.ActivePath); !reflect.DeepEqual(got, stager.args) {
		t.Fatalf("initial host args = %v", got)
	}
	stager.args = []string{"--global-config", "/new/global.json", "--primary", "/new/primary"}
	ordinaryRepair, err := service.Repair(context.Background(), input)
	if err != nil || !ordinaryRepair.NoChange || stager.stageCalls != 1 {
		t.Fatalf("ordinary exact repair = %+v, %v; stage calls = %d", ordinaryRepair, err, stager.stageCalls)
	}
	input.OperationID = "refresh-projection"
	result, err := service.RefreshProjection(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || result.NoChange || result.Receipt.BeforeDigest != managedDigest(old) || result.Receipt.AfterDigest == managedDigest(old) {
		t.Fatalf("refresh result = %+v", result)
	}
	if got := projectedHostArgs(t, installed.Plan.ActivePath); !reflect.DeepEqual(got, stager.args) {
		t.Fatalf("refreshed host args = %v", got)
	}
	state, err = service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	current := onlyBinding(state.Installations[0])
	if current.DataReceiptID != old.DataReceiptID || managedDigest(current) != result.Receipt.AfterDigest || len(current.Receipts) != len(old.Receipts)+1 {
		t.Fatalf("refreshed binding = %+v", current)
	}
	if len(stager.dataPaths) != 2 || stager.dataPaths[1] != oldData {
		t.Fatalf("PLUGIN_DATA paths = %v, want retained %s", stager.dataPaths, oldData)
	}
	repeat, err := service.RefreshProjection(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !repeat.NoChange || repeat.Mutated || repeat.Receipt.OperationID != "" {
		t.Fatalf("idempotent refresh = %+v", repeat)
	}
	state, err = service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(onlyBinding(state.Installations[0]).Receipts); got != len(current.Receipts) {
		t.Fatalf("repeated refresh wrote %d receipts, want %d", got, len(current.Receipts))
	}
}

func TestRefreshProjectionNativeFailureRetainsPreviousOwnershipForReceiptFreeRetry(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLINE_MCP_SETTINGS_PATH", filepath.Join(root, "cline-data", "settings", "cline_mcp_settings.json"))
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, ".cline")}
	input := clinePackageInput(t, client, "1.0.0", "sha256:cline-refresh-v1", "sha256:cline-refresh-manifest-v1", "sh")
	input.Confirmed = true
	input.OperationID = "native-refresh-install"
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	previous := append([]domain.NativeObjectOwnership(nil), onlyBinding(state.Installations[0]).NativeObjects...)
	if len(previous) < 2 {
		t.Fatalf("Cline fixture is missing external ownership: %+v", previous)
	}
	service.Stager = nativeRefreshStager{PackageStager: service.Stager, desiredNativeDigest: "sha256:desired-native-receipt"}
	activator := &nativeRefreshRetryActivator{t: t, previousOwnership: previous}
	service.Activator = activator
	input.OperationID = "native-refresh-failed"
	first, err := service.RefreshProjection(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "injected native activation failure") || !first.Mutated {
		t.Fatalf("first refresh = %+v, %v", first, err)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	failed := onlyBinding(state.Installations[0])
	if managedDigest(failed) == managedDigest(domain.ClientBinding{NativeObjects: previous}) || len(failed.Receipts) != 2 || failed.Activation != domain.ActivationFailed {
		t.Fatalf("failed refresh package state = %+v", failed)
	}
	for _, object := range failed.NativeObjects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		var prior domain.NativeObjectOwnership
		for _, candidate := range previous {
			if candidate.ObjectID == object.ObjectID {
				prior = candidate
				break
			}
		}
		if !reflect.DeepEqual(object, prior) {
			t.Fatalf("failed refresh replaced previous native ownership: %+v, want %+v", object, prior)
		}
	}
	input.OperationID = "native-refresh-retry"
	retry, err := service.RefreshProjection(context.Background(), input)
	if err != nil || !retry.Mutated || retry.Receipt.OperationID != "" || activator.calls != 2 {
		t.Fatalf("retry refresh = %+v, %v; activation calls = %d", retry, err, activator.calls)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	current := onlyBinding(state.Installations[0])
	if len(current.Receipts) != 2 || current.Activation != domain.ActivationActive || managedDigest(current) != managedDigest(failed) {
		t.Fatalf("retry package state = %+v", current)
	}
	for _, object := range current.NativeObjects {
		if object.Kind != "managed_package_directory" && object.ManagedDigest != "sha256:desired-native-receipt" {
			t.Fatalf("retry did not promote desired native ownership: %+v", object)
		}
	}
	if _, err := os.Stat(filepath.Join(installed.Plan.ActivePath, "host-projection.txt")); err != nil {
		t.Fatalf("refreshed managed package was lost: %v", err)
	}
}

func TestRefreshProjectionRollsBackFailedDirectorySwap(t *testing.T) {
	t.Parallel()
	service, input, stager := hostProjectionFixture(t)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	before := projectedHostArgs(t, installed.Plan.ActivePath)
	stager.args = []string{"--primary", "/new/primary"}
	input.OperationID = "refresh-rollback"
	service.Kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.FaultActivationApplied {
			return fmt.Errorf("injected refresh failure")
		}
		return nil
	}
	if _, err := service.RefreshProjection(context.Background(), input); err == nil || !strings.Contains(err.Error(), "injected refresh failure") {
		t.Fatalf("faulted refresh error = %v", err)
	}
	if got := projectedHostArgs(t, installed.Plan.ActivePath); !reflect.DeepEqual(got, before) {
		t.Fatalf("rollback restored args %v, want %v", got, before)
	}
	state, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(onlyBinding(state.Installations[0]).Receipts); got != 1 {
		t.Fatalf("failed refresh wrote %d receipts", got)
	}
	service.Kernel.Directory.Fault = nil
	input.OperationID = "refresh-retry"
	result, err := service.RefreshProjection(context.Background(), input)
	if err != nil || !result.Mutated {
		t.Fatalf("retry refresh = %+v, %v", result, err)
	}
	if got := projectedHostArgs(t, installed.Plan.ActivePath); !reflect.DeepEqual(got, stager.args) {
		t.Fatalf("retry projected args %v, want %v", got, stager.args)
	}
}

func TestRefreshProjectionRequiresIntactPackageAndRechecksBeforeCommit(t *testing.T) {
	t.Parallel()
	service, input, stager := hostProjectionFixture(t)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	stager.args = []string{"--primary", "/new/primary"}
	input.OperationID = "refresh-precondition"
	projection := filepath.Join(installed.Plan.ActivePath, ".mcp.json")
	original, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projection, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RefreshProjection(context.Background(), input); err == nil || !strings.Contains(err.Error(), "not intact") {
		t.Fatalf("damaged refresh error = %v", err)
	}
	if err := os.WriteFile(projection, original, 0o600); err != nil {
		t.Fatal(err)
	}
	service.NativeObserver = mutatingRefreshObserver{path: projection}
	if _, err := service.RefreshProjection(context.Background(), input); err == nil || !strings.Contains(err.Error(), "changed after refresh preflight") {
		t.Fatalf("racing refresh error = %v", err)
	}
	state, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(onlyBinding(state.Installations[0]).Receipts); got != 1 {
		t.Fatalf("racing refresh wrote %d receipts", got)
	}
}

func TestRefreshProjectionUnconfirmedDoesNotStage(t *testing.T) {
	t.Parallel()
	service, input, stager := hostProjectionFixture(t)
	if _, err := service.Add(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	stager.args = []string{"--primary", "/new/primary"}
	input.Confirmed = false
	result, err := service.RefreshProjection(context.Background(), input)
	if err != nil || !result.RequiresConfirmation || result.Mutated || stager.stageCalls != 1 {
		t.Fatalf("unconfirmed refresh = %+v, %v; stage calls = %d", result, err, stager.stageCalls)
	}
	input.DryRun = true
	result, err = service.RefreshProjection(context.Background(), input)
	if err != nil || result.RequiresConfirmation || result.Mutated || stager.stageCalls != 1 {
		t.Fatalf("dry-run refresh = %+v, %v; stage calls = %d", result, err, stager.stageCalls)
	}
}
