package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
	"github.com/spf13/cobra"
)

func TestRepairSourceResolutionIsDeadlineAndCancellationResponsive(t *testing.T) {
	for name, contextFor := range map[string]func() (context.Context, context.CancelFunc){
		"phase deadline": func() (context.Context, context.CancelFunc) {
			return context.Background(), func() {}
		},
		"caller cancellation": func() (context.Context, context.CancelFunc) {
			return context.WithCancel(context.Background())
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := contextFor()
			if name == "caller cancellation" {
				cancel()
			} else {
				defer cancel()
			}
			started := time.Now()
			_, err := resolveRepairSource(ctx, 20*time.Millisecond, func(inner context.Context) (loadedPackage, error) {
				<-inner.Done()
				return loadedPackage{}, inner.Err()
			})
			if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
				t.Fatalf("repair resolution error = %v", err)
			}
			if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
				t.Fatalf("repair resolution cancellation took %s", elapsed)
			}
		})
	}
}

func TestHelpKeepsAutomationConfirmationFlagOutOfUserFlow(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	stdout, _, err := fixture.execute(true, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "--yes") {
		t.Fatalf("user-facing help exposed the automation-only flag: %s", stdout)
	}
	if !strings.Contains(stdout, "codex, chatgpt, cursor") || !strings.Contains(stdout, "claude, gemini") {
		t.Fatalf("user-facing help omitted the distinct ChatGPT target: %s", stdout)
	}
}

func TestSwitchRendererNeverClaimsCompletedAfterApplyFailure(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	result := switchOutput{DryRun: false, Status: string(usecase.GroupPhaseManagedActivationFailed),
		Group: &usecase.GroupResult{Phase: usecase.GroupPhaseManagedActivationFailed}}
	if err := renderSwitchResult(&output, "human", result); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Switch: completed") || !strings.Contains(output.String(), "managed_committed_activation_failed") {
		t.Fatalf("failure rendering = %q", output.String())
	}
}

func TestOperationalAndReadCommandsProduceVersionedJSON(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	stdout, _, err = fixture.execute(false, "list", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "list")
	if strings.Contains(stdout, fixture.root) {
		t.Fatalf("list JSON leaked sandbox path: %s", stdout)
	}
	stdout, _, err = fixture.execute(false, "info", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "info")
	if strings.Contains(stdout, fixture.root) {
		t.Fatalf("info JSON leaked sandbox path: %s", stdout)
	}
	stdout, _, err = fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "doctor")
	if strings.Contains(stdout, fixture.root) {
		t.Fatalf("doctor JSON leaked sandbox path: %s", stdout)
	}
}

func TestValidateLoadsPackageWithoutDetectionOrStateMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "validate", plugin, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "validate")
	var envelope struct {
		Data validationResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.Data.Conformant || envelope.Data.Name != "demo" || envelope.Data.SchemaURI != domain.PluginSchemaV1 {
		t.Fatalf("validation result = %+v", envelope.Data)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("validate mutated state: %+v", state)
	}
	if _, _, err := fixture.execute(false, "validate", "demo"); err == nil || !strings.Contains(err.Error(), "local path or exact") {
		t.Fatalf("short-name validation error = %v", err)
	}
}

func TestLifecycleOutputKeepsPrivatePathsOutOfPublicJSON(t *testing.T) {
	t.Parallel()
	privatePath := "/home/private-user/.codex/plugins/demo"
	addResult := usecase.AddResult{Activation: domain.ActivationOutcome{
		Authentication: domain.AuthenticationPending,
		UserActions:    []string{"open the prepared plugin in the selected client"},
		LocalActions:   []string{"open the client plugin settings for " + privatePath},
	}}
	wantAdd := "open the prepared plugin in the selected client; complete authentication, then rerun add to verify activation and authentication"
	if got := nextLifecycleAction(addResult); got != wantAdd {
		t.Fatalf("add next action = %q, want %q", got, wantAdd)
	}
	if got := nextLocalLifecycleAction(addResult); !strings.Contains(got, privatePath) {
		t.Fatalf("local add next action lost actionable path: %q", got)
	}
	var public bytes.Buffer
	if err := renderAddResult(&public, "json", domain.PackageEnvelope{}, addResult, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(public.String(), privatePath) {
		t.Fatalf("public add JSON leaked provider local action: %s", public.String())
	}
	var structured struct {
		Data addResultData `json:"data"`
	}
	if err := json.Unmarshal(public.Bytes(), &structured); err != nil {
		t.Fatal(err)
	}
	if structured.Data.NextAction != wantAdd {
		t.Fatalf("structured add next action = %q, want %q", structured.Data.NextAction, wantAdd)
	}
	var human bytes.Buffer
	if err := renderAddResult(&human, "human", domain.PackageEnvelope{}, addResult, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human.String(), privatePath) {
		t.Fatalf("human add output lost actionable provider guidance: %q", human.String())
	}
	fallbackResult := usecase.AddResult{
		Plan: domain.DeliveryPlan{
			ActivePath:   privatePath,
			LocalActions: []string{"open the prepared package at " + privatePath},
		},
		Activation: domain.ActivationOutcome{Authentication: domain.AuthenticationPending},
	}
	fallback := newAddResultData(domain.PackageEnvelope{}, fallbackResult, false).NextAction
	if strings.Contains(fallback, privatePath) || fallback != "complete authentication for the prepared plugin in the selected client, then rerun add to verify activation and authentication" {
		t.Fatalf("path-free fallback next action = %q", fallback)
	}
	removeResult := usecase.RemoveResult{Deactivation: domain.DeactivationOutcome{
		LocalActions: []string{"retain data at the owned data path"},
		UserActions:  []string{"restart the client"},
	}}
	if got := nextRemoveAction(removeResult); got != "retain data at the owned data path" {
		t.Fatalf("remove next action = %q", got)
	}
}

func TestLifecycleOutputDoesNotDuplicateAuthenticationGuidance(t *testing.T) {
	t.Parallel()
	action := "verify this plugin's authentication requirements before using it"
	result := usecase.AddResult{
		Plan: domain.DeliveryPlan{UserActions: []string{action}},
		Activation: domain.ActivationOutcome{
			Authentication: domain.AuthenticationNotChecked,
		},
	}
	if got := nextLifecycleAction(result); got != action {
		t.Fatalf("next action = %q, want %q", got, action)
	}
}

func TestAutomatedAddRequiresExplicitTargetButNotYes(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin); err == nil || !strings.Contains(err.Error(), "requires --target") {
		t.Fatalf("missing-target error = %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("missing-target command mutated state: %+v", state)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatalf("explicit-target add without --yes failed: %v", err)
	}
}

func TestInstallAliasProducesTheSamePlanAndStateAsAdd(t *testing.T) {
	t.Parallel()
	plugin := writeCLIPlugin(t)
	addFixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	installFixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})

	addOutput, _, err := addFixture.execute(false, "add", plugin, "--target", "cursor", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	installOutput, _, err := installFixture.execute(false, "install", plugin, "--target", "cursor", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(encoded string) map[string]any {
		var value map[string]any
		if err := json.Unmarshal([]byte(encoded), &value); err != nil {
			t.Fatal(err)
		}
		var strip func(any)
		strip = func(current any) {
			switch item := current.(type) {
			case map[string]any:
				for _, key := range []string{"operation_id", "installation_id", "physical_artifact_id"} {
					delete(item, key)
				}
				for _, child := range item {
					strip(child)
				}
			case []any:
				for _, child := range item {
					strip(child)
				}
			}
		}
		strip(value)
		return value
	}
	if addPlan, installPlan := normalize(addOutput), normalize(installOutput); !reflect.DeepEqual(addPlan, installPlan) {
		t.Fatalf("install alias plan differs from add:\nadd=%s\ninstall=%s", addOutput, installOutput)
	}
	for name, fixture := range map[string]cliFixture{"add": addFixture, "install": installFixture} {
		state, err := fixture.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		if len(state.Installations) != 0 {
			t.Fatalf("%s dry-run mutated state: %+v", name, state)
		}
	}
	if _, _, err := addFixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := installFixture.execute(false, "install", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	addState, err := addFixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installState, err := installFixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(addState.Installations) != 1 || len(installState.Installations) != 1 {
		t.Fatalf("alias installation counts differ: add=%d install=%d", len(addState.Installations), len(installState.Installations))
	}
	addInstallation, installInstallation := addState.Installations[0], installState.Installations[0]
	if addInstallation.DeclaredName != installInstallation.DeclaredName || addInstallation.OriginMode != installInstallation.OriginMode ||
		addInstallation.Source.RequestedSource != installInstallation.Source.RequestedSource ||
		addInstallation.Source.CanonicalSource != installInstallation.Source.CanonicalSource ||
		addInstallation.Source.ResolvedRevision != installInstallation.Source.ResolvedRevision ||
		addInstallation.Source.TreeDigest != installInstallation.Source.TreeDigest ||
		!reflect.DeepEqual(addInstallation.Package, installInstallation.Package) {
		t.Fatalf("install alias persisted different package state:\nadd=%+v\ninstall=%+v", addInstallation, installInstallation)
	}
	if len(addInstallation.Clients) != 1 || len(installInstallation.Clients) != 1 {
		t.Fatalf("alias client counts differ: add=%d install=%d", len(addInstallation.Clients), len(installInstallation.Clients))
	}
	var addClient, installClient domain.ClientBinding
	for _, addClient = range addInstallation.Clients {
	}
	for _, installClient = range installInstallation.Clients {
	}
	if addClient.ClientID != installClient.ClientID || addClient.Scope != installClient.Scope ||
		addClient.Materialization != installClient.Materialization || addClient.Activation != installClient.Activation ||
		addClient.Authentication != installClient.Authentication || addClient.Policy != installClient.Policy ||
		addClient.Verification != installClient.Verification || !reflect.DeepEqual(addClient.PackageRevision, installClient.PackageRevision) {
		t.Fatalf("install alias persisted different client state:\nadd=%+v\ninstall=%+v", addClient, installClient)
	}
}

func TestCommaSeparatedTargetsRunAddUpdateAndRemove(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor),
	})
	plugin := writeCLIPlugin(t)

	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "codex, cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertBatchJSON(t, stdout, "add", 2, 0)
	assertClientBindings(t, fixture, domain.MaterializationMaterialized, 1)

	manifest := filepath.Join(plugin, "plugin.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = fixture.execute(false, "update", "demo", "--target", "codex,cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertBatchJSON(t, stdout, "update", 2, 0)
	assertClientBindings(t, fixture, domain.MaterializationMaterialized, 2)

	stdout, _, err = fixture.execute(false, "remove", "demo", "--target", "codex,cursor", "--external-uninstalled", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertBatchJSON(t, stdout, "remove", 2, 0)
	var removed struct {
		Data struct {
			DataRetained       bool                 `json:"data_retained"`
			RetainedData       []retainedDataResult `json:"retained_data"`
			RetainedDataAction string               `json:"retained_data_action"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &removed); err != nil {
		t.Fatal(err)
	}
	if !removed.Data.DataRetained || len(removed.Data.RetainedData) == 0 || removed.Data.RetainedDataAction == "" {
		t.Fatalf("grouped remove retained-data output = %+v", removed.Data)
	}
	for _, receipt := range removed.Data.RetainedData {
		if receipt.DataReceiptID == "" || receipt.PhysicalBackend == "" || receipt.Scope != string(domain.ScopeUser) || receipt.State != domain.DataReceiptOwned {
			t.Fatalf("path-free retained-data receipt = %+v", receipt)
		}
	}
	if strings.Contains(stdout, fixture.root) || strings.Contains(stdout, string(os.PathSeparator)+"tmp"+string(os.PathSeparator)) {
		t.Fatalf("grouped remove JSON leaked an absolute path: %s", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 || len(state.Installations[0].DataReceipts) == 0 {
		t.Fatalf("normal grouped remove did not retain owned data: %+v", state)
	}
}

func TestMultiTargetRemoveHumanOutputExplainsHowToPurgeRetainedData(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor),
	})
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "remove", "demo", "--target", "codex,cursor", "--external-uninstalled")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PLUGIN_DATA: preserved", "Next: run `agentplugins remove ", " --purge-data` to delete retained plugin data"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("human grouped remove output %q omitted %q", stdout, want)
		}
	}
}

func TestMultiTargetAddResolvesOnePackageAndUsesDeterministicOrder(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	counter := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = counter
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,codex", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 {
		t.Fatalf("source resolution calls = %d, want 1", counter.calls)
	}
	var output struct {
		Result string `json:"result"`
		Data   struct {
			OperationID string            `json:"operation_id"`
			Targets     []addTargetResult `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if output.Result != "success" || len(output.Data.Targets) != 2 || output.Data.Targets[0].Target != "codex" || output.Data.Targets[1].Target != "cursor" {
		t.Fatalf("deterministic multi-target output = %+v", output)
	}
	if output.Data.OperationID == "" || output.Data.Targets[0].Output.OperationID == "" || output.Data.Targets[1].Output.OperationID == "" {
		t.Fatalf("multi-target add operation group = %+v", output.Data.Targets)
	}
}

func TestCodexCursorKiroUseOneAcquisitionAndGroupedCommit(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientKiro), fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	plugin := writeCLIPlugin(t)
	revision := strings.Repeat("a", 40)
	acquirer := &localBackedSourceAcquirer{delegate: fixture.app.SourceAcquirer, root: plugin}
	fixture.app.SourceAcquirer = acquirer
	stdout, _, err := fixture.execute(false, "add", "owner/repo@"+revision+"//plugins/demo", "--target", "kiro,codex,cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Data struct {
			OperationID    string                    `json:"operation_id"`
			Acquisition    addAcquisitionProof       `json:"acquisition"`
			TargetOutcomes map[string]addTargetProof `json:"target_outcomes"`
			Targets        []addTargetResult         `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if acquirer.directGitCalls != 1 || len(output.Data.Targets) != 3 || output.Data.Targets[0].Target != "codex" || output.Data.Targets[1].Target != "cursor" || output.Data.Targets[2].Target != "kiro" {
		t.Fatalf("three-target combined plan = %+v; source resolutions = %d", output.Data.Targets, acquirer.directGitCalls)
	}
	proof := output.Data.Acquisition
	if proof.AcquisitionID == "" || proof.AcquisitionCount != 1 || !proof.Fetched || !proof.Validated || proof.SourceKind != "github" {
		t.Fatalf("group acquisition proof = %+v", proof)
	}
	installationID := output.Data.Targets[0].Output.Result.InstallationID
	if proof.AcquisitionID == output.Data.OperationID || proof.AcquisitionID == installationID || len(output.Data.TargetOutcomes) != 3 {
		t.Fatalf("acquisition identity or target cardinality = acquisition:%q operation:%q installation:%q targets:%+v", proof.AcquisitionID, output.Data.OperationID, installationID, output.Data.TargetOutcomes)
	}
	for _, target := range []string{"codex", "cursor", "kiro"} {
		binding, ok := output.Data.TargetOutcomes[target]
		if !ok || binding.Outcome != "passed" || binding.AcquisitionID != proof.AcquisitionID || binding.TreeDigest != proof.TreeDigest || binding.ManifestDigest != proof.ManifestDigest || binding.ClosureDigest != proof.ClosureDigest {
			t.Fatalf("target %s acquisition binding = %+v", target, binding)
		}
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 3 || state.Installations[0].OperationGroupID == "" {
		t.Fatalf("three-target grouped state = %+v", state)
	}
	groupID := state.Installations[0].OperationGroupID
	for _, binding := range state.Installations[0].Clients {
		if len(binding.Receipts) != 1 || binding.Receipts[0].OperationGroupID != groupID {
			t.Fatalf("three-target grouped receipt = %+v", binding)
		}
	}
}

func TestGroupedAcquisitionClosureDigestGoldenVectorAndFieldBindings(t *testing.T) {
	t.Parallel()
	source := domain.SourceIdentity{Repository: "owner/repo", PackageSubpath: "plugins/demo", ResolvedRevision: strings.Repeat("b", 40)}
	treeDigest := "sha256:" + strings.Repeat("1", 64)
	manifestDigest := "sha256:" + strings.Repeat("2", 64)
	const expected = "sha256:7cb4354b53d4154f6c93c2c2963bdc38aab5b3d998893493914d24dd4ac899f4"
	baseline := groupedAcquisitionClosureDigest("github", source, treeDigest, manifestDigest)
	if baseline != expected {
		t.Fatalf("closure digest golden vector = %q, want %q", baseline, expected)
	}

	mutations := []struct {
		name           string
		sourceKind     string
		source         domain.SourceIdentity
		treeDigest     string
		manifestDigest string
	}{
		{name: "source kind", sourceKind: "directory", source: source, treeDigest: treeDigest, manifestDigest: manifestDigest},
		{name: "repository", sourceKind: "github", source: domain.SourceIdentity{Repository: "owner/other", PackageSubpath: source.PackageSubpath, ResolvedRevision: source.ResolvedRevision}, treeDigest: treeDigest, manifestDigest: manifestDigest},
		{name: "package subpath", sourceKind: "github", source: domain.SourceIdentity{Repository: source.Repository, PackageSubpath: "plugins/other", ResolvedRevision: source.ResolvedRevision}, treeDigest: treeDigest, manifestDigest: manifestDigest},
		{name: "resolved revision", sourceKind: "github", source: domain.SourceIdentity{Repository: source.Repository, PackageSubpath: source.PackageSubpath, ResolvedRevision: strings.Repeat("c", 40)}, treeDigest: treeDigest, manifestDigest: manifestDigest},
		{name: "tree digest", sourceKind: "github", source: source, treeDigest: "sha256:" + strings.Repeat("3", 64), manifestDigest: manifestDigest},
		{name: "manifest digest", sourceKind: "github", source: source, treeDigest: treeDigest, manifestDigest: "sha256:" + strings.Repeat("4", 64)},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run(mutation.name, func(t *testing.T) {
			t.Parallel()
			got := groupedAcquisitionClosureDigest(mutation.sourceKind, mutation.source, mutation.treeDigest, mutation.manifestDigest)
			if got == baseline {
				t.Fatalf("mutation did not change closure digest %q", got)
			}
		})
	}
}

func TestGroupedAcquisitionClosureDigestSeparatesFramingBoundaries(t *testing.T) {
	t.Parallel()
	treeDigest := "sha256:" + strings.Repeat("1", 64)
	manifestDigest := "sha256:" + strings.Repeat("2", 64)
	left := groupedAcquisitionClosureDigest("github", domain.SourceIdentity{Repository: "ab", PackageSubpath: "c"}, treeDigest, manifestDigest)
	right := groupedAcquisitionClosureDigest("github", domain.SourceIdentity{Repository: "a", PackageSubpath: "bc"}, treeDigest, manifestDigest)
	if left == right {
		t.Fatalf("length-prefix framing collision: (ab, c) and (a, bc) both produced %q", left)
	}
}

func TestGroupedLocalAddProofIsIndependentOfActualSourcePath(t *testing.T) {
	t.Parallel()
	type result struct {
		stdout string
		proof  *addAcquisitionProof
	}
	pluginPaths := []string{writeCLIPlugin(t), writeCLIPlugin(t)}
	if pluginPaths[0] == pluginPaths[1] {
		t.Fatalf("local fixtures unexpectedly shared path %q", pluginPaths[0])
	}
	results := make([]result, 0, len(pluginPaths))
	for _, pluginPath := range pluginPaths {
		fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
		stdout, _, err := fixture.execute(false, "add", pluginPath, "--target", "codex,cursor", "--format", "json")
		if err != nil {
			t.Fatal(err)
		}
		var output struct {
			Data addMultiResult `json:"data"`
		}
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatal(err)
		}
		if output.Data.Acquisition == nil || output.Data.Acquisition.Fetched || output.Data.Acquisition.SourceKind != "local" || !output.Data.Acquisition.Validated {
			t.Fatalf("local acquisition proof for %q = %+v", pluginPath, output.Data.Acquisition)
		}
		results = append(results, result{stdout: stdout, proof: output.Data.Acquisition})
	}
	if results[0].proof.ClosureDigest != results[1].proof.ClosureDigest {
		t.Fatalf("identical local contents at different paths produced closures %q and %q", results[0].proof.ClosureDigest, results[1].proof.ClosureDigest)
	}
	for index, result := range results {
		for _, pluginPath := range pluginPaths {
			if strings.Contains(result.stdout, pluginPath) {
				t.Fatalf("JSON result %d leaked actual plugin source path %q: %s", index, pluginPath, result.stdout)
			}
		}
	}
}

func TestGroupedDirectoryAcquisitionProofBindsSourceIdentity(t *testing.T) {
	t.Parallel()
	source := domain.SourceIdentity{Repository: "owner/directory", PackageSubpath: "plugins/demo", ResolvedRevision: strings.Repeat("d", 40)}
	loaded := loadedPackage{
		origin: domain.OriginModeDirectory,
		envelope: domain.PackageEnvelope{
			Source: source, TreeDigest: "sha256:" + strings.Repeat("5", 64), ManifestDigest: "sha256:" + strings.Repeat("6", 64),
		},
	}
	proof, err := newAddAcquisitionProof(loaded)
	if err != nil {
		t.Fatal(err)
	}
	if proof.SourceKind != "directory" || !proof.Fetched || !proof.Validated {
		t.Fatalf("Directory acquisition proof = %+v", proof)
	}
	const want = "sha256:99db46552e756e40dae4128875f94b5adcd9103bb368bccc3994b2c97db05206"
	if proof.ClosureDigest != want {
		t.Fatalf("Directory closure = %q, want %q", proof.ClosureDigest, want)
	}
	mutated := source
	mutated.Repository = "owner/other-directory"
	if proof.ClosureDigest == groupedAcquisitionClosureDigest("directory", mutated, loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest) {
		t.Fatalf("Directory closure did not bind source identity: %q", proof.ClosureDigest)
	}
}

func TestAddTargetProofOutcomeMappings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		phase usecase.GroupTargetPhase
		want  string
	}{
		{name: "rollback", phase: usecase.GroupTargetManagedRolledBack, want: "rolled_back"},
		{name: "unknown", phase: usecase.GroupTargetManagedUnknown, want: "unknown"},
		{name: "partial", phase: usecase.GroupTargetExternalPartial, want: "partial"},
		{name: "failed", phase: usecase.GroupTargetExternalFailed, want: "failed"},
		{name: "passed", phase: usecase.GroupTargetExternalCompleted, want: "passed"},
		{name: "default not completed", phase: usecase.GroupTargetPhase("unrecognized"), want: "not_completed"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := addTargetProofOutcome(usecase.AddResult{GroupPhase: test.phase}); got != test.want {
				t.Fatalf("addTargetProofOutcome(%q) = %q, want %q", test.phase, got, test.want)
			}
		})
	}
}

func TestGroupedDryRunOmitsCompletedAcquisitionProof(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor)})
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Data addMultiResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if output.Data.Acquisition != nil || output.Data.TargetOutcomes != nil || strings.Contains(stdout, `"acquisition_count"`) {
		t.Fatalf("dry-run exposed completed acquisition proof: %s", stdout)
	}
}

func TestGroupedPartialFailureDoesNotMarkEveryTargetPassed(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientKiro)})
	fixture.app.Lifecycle.Activator = &failSecondCLIGroupActivator{}
	stdout, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "injected grouped activation failure") {
		t.Fatalf("grouped partial failure = %v", err)
	}
	var output struct {
		Data addMultiResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	passed := 0
	for target, targetProof := range output.Data.TargetOutcomes {
		if targetProof.Outcome == "passed" {
			passed++
		}
		if targetProof.AcquisitionID != output.Data.Acquisition.AcquisitionID {
			t.Fatalf("failed target %s lost acquisition binding: %+v", target, targetProof)
		}
	}
	if len(output.Data.TargetOutcomes) != 3 || passed != 1 || output.Data.TargetOutcomes["cursor"].Outcome != "failed" || output.Data.TargetOutcomes["kiro"].Outcome == "passed" {
		t.Fatalf("partial failure target outcomes = %+v", output.Data.TargetOutcomes)
	}
}

func TestMultiTargetAddUsesInstallEngineGroupBoundary(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,codex", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].OperationGroupID == "" || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("grouped add state = %+v", state)
	}
	groupID := state.Installations[0].OperationGroupID
	for _, binding := range state.Installations[0].Clients {
		if len(binding.Receipts) != 1 || binding.Receipts[0].OperationGroupID != groupID {
			t.Fatalf("group receipt = %+v", binding)
		}
	}
}

func TestCopilotAndVSCodeShareOneGroupedPhysicalMutation(t *testing.T) {
	t.Parallel()
	copilot := fixtureClient(t, domain.ClientCopilot)
	copilot.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{copilot, fixtureClient(t, domain.ClientVSCode)})
	counter := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = counter
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "copilot,vscode", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertBatchJSON(t, stdout, "add", 2, 0)
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("shared backend acquisition/state = calls:%d state:%+v", counter.calls, state)
	}
	binding := onlyCLIClient(state.Installations[0])
	if len(binding.Receipts) != 1 || !reflect.DeepEqual(binding.AffectedSurfaces, []string{"copilot", "vscode"}) {
		t.Fatalf("shared backend binding = %+v", binding)
	}
	body, err := os.ReadFile(filepath.Join(plugin, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = fixture.execute(false, "update", "demo", "--target", "copilot,vscode", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertBatchJSON(t, stdout, "update", 2, 0)
	state, err = fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding = onlyCLIClient(state.Installations[0])
	if len(state.Installations[0].Clients) != 1 || len(binding.Receipts) != 2 || binding.PackageRevision.Version != "2.0.0" {
		t.Fatalf("shared backend grouped update = %+v", state.Installations[0])
	}
	for _, targets := range []string{"copilot,vscode", "vscode,copilot"} {
		stdout, _, err = fixture.execute(false, "remove", "demo", "--target", targets, "--external-uninstalled", "--dry-run", "--format", "json")
		if err != nil {
			t.Fatalf("shared backend removal preflight for %s: %v", targets, err)
		}
		assertBatchJSON(t, stdout, "remove", 2, 0)
		if strings.Contains(stdout, `"status":"blocked"`) {
			t.Fatalf("shared backend removal preflight for %s = %s", targets, stdout)
		}
	}
}

func TestMissingManagedStdioRuntimeFailsAutomaticActivationPreflightWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		client     domain.ClientID
		executable string
	}{
		{client: domain.ClientCodex, executable: "/test/bin/codex"},
		{client: domain.ClientCopilot, executable: "/test/bin/copilot"},
		{client: domain.ClientKiro, executable: "/test/bin/kiro-cli"},
	} {
		t.Run(string(test.client), func(t *testing.T) {
			client := fixtureClient(t, test.client)
			client.ExecutablePath = test.executable
			fixture := newCLIFixture(t, []domain.DetectedClient{client})
			runner := &cliCommandRunner{}
			fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
			plugin := writeCLIPlugin(t)
			mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"uap-runtime-that-does-not-exist"}}}`
			if err := os.WriteFile(filepath.Join(plugin, "mcp.json"), []byte(mcp), 0o644); err != nil {
				t.Fatal(err)
			}
			_, _, err := fixture.execute(false, "add", plugin, "--target", string(test.client))
			if err == nil || !strings.Contains(err.Error(), `requires executable "uap-runtime-that-does-not-exist" on PATH`) || !strings.Contains(err.Error(), "install it explicitly") || !strings.Contains(err.Error(), "never installs runtimes") {
				t.Fatalf("missing runtime guidance = %v", err)
			}
			state, loadErr := fixture.store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(state.Installations) != 0 {
				t.Fatalf("missing runtime mutated state: %+v", state)
			}
			if runner.calls != 0 {
				t.Fatalf("missing runtime reached automatic activation runner %d times", runner.calls)
			}
		})
	}
}

func TestKiroMissingDuplexFailsLifecyclePreflightWithoutMutation(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientKiro)
	client.ExecutablePath = "/test/bin/kiro-cli"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	runner := &cliRunOnlyRunner{}
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)

	_, _, err := fixture.execute(false, "add", plugin, "--target", string(domain.ClientKiro))
	if err == nil || !strings.Contains(err.Error(), "requires an ACP duplex process runner") {
		t.Fatalf("duplex preflight error = %v", err)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 || runner.calls != 0 {
		t.Fatalf("duplex preflight mutated state or invoked Kiro: state=%+v calls=%d", state, runner.calls)
	}
	_, _, retryErr := fixture.execute(false, "add", plugin, "--target", string(domain.ClientKiro))
	if retryErr == nil || !strings.Contains(retryErr.Error(), "requires an ACP duplex process runner") {
		t.Fatalf("duplex preflight retry = %v", retryErr)
	}
	state, loadErr = fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 || runner.calls != 0 {
		t.Fatalf("duplex retry entered verify-only or mutated: state=%+v calls=%d", state, runner.calls)
	}
	if _, statErr := os.Stat(client.ConfigRoot); !os.IsNotExist(statErr) {
		t.Fatalf("duplex preflight created Kiro package/config root %q: %v", client.ConfigRoot, statErr)
	}
}

func TestUnavailableSelectedTargetFailsBeforePackageAcquisition(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	counter := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = counter
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "kiro")
	if err == nil || !strings.Contains(err.Error(), `target "kiro" was not detected`) || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("unavailable target error = %v", err)
	}
	if counter.calls != 0 {
		t.Fatalf("unavailable target acquired package %d times", counter.calls)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("unavailable target mutated state: %+v", state)
	}
}

func TestThirdTargetNativeCollisionFailsBeforeAnyGroupedMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientKiro),
	})
	fixture.app.Lifecycle.NativeObserver = selectiveNativeObserver{foreign: domain.ClientKiro}
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "codex,cursor,kiro")
	if err == nil || !strings.Contains(err.Error(), "unmanaged") || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("third-target collision error = %v", err)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("third-target collision mutated state: %+v", state)
	}
	if entries, readErr := os.ReadDir(fixture.app.ManagedRoot); readErr == nil && len(entries) != 0 {
		t.Fatalf("third-target collision staged managed files: %v", entries)
	}
}

func TestInteractiveAddDefaultsDetectedMultiselectToAll(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.executeInput(true, "\n", "add", plugin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "all selected by default") {
		t.Fatalf("multiselect output = %q", stdout)
	}
	assertClientBindings(t, fixture, domain.MaterializationMaterialized, 1)
}

func TestInteractiveAddSkipsDetectedClientThatPackageCannotServe(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientChatGPT), fixtureClient(t, domain.ClientCursor),
	})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, _, err := fixture.executeInput(true, "\n", "add", plugin, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Skipped installed clients that this package cannot install together: chatgpt") {
		t.Fatalf("package-aware interactive output = %q", stdout)
	}
	if strings.Contains(stdout, "Detected supported clients (all selected by default)") {
		t.Fatalf("single compatible target unexpectedly prompted for multiple clients: %q", stdout)
	}
}

func TestInteractiveAddSkipsDetectedClientThatCannotPassActivationPreflight(t *testing.T) {
	t.Parallel()
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/test/bin/kiro-cli"
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), kiro,
	})
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: &cliRunOnlyRunner{}}
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)

	stdout, _, err := fixture.executeInput(true, "\n", "add", plugin, "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Skipped installed clients that this package cannot install together: kiro") {
		t.Fatalf("activation-aware interactive output = %q", stdout)
	}
	if strings.Contains(stdout, "Detected supported clients (all selected by default)") {
		t.Fatalf("single preflight-capable target unexpectedly prompted for multiple clients: %q", stdout)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("interactive dry-run mutated state: %+v", state)
	}
}

func TestInteractiveAddProbesOnlyTargetsSelectedAfterReadOnlyDetection(t *testing.T) {
	clientCodex := fixtureClient(t, domain.ClientCodex)
	clientCursor := fixtureClient(t, domain.ClientCursor)
	detector := &observedProbingDetector{clients: []domain.DetectedClient{clientCursor, clientCodex}}
	fixture := newCLIFixture(t, nil)
	fixture.app.Detector = detector
	command := &cobra.Command{}
	command.SetIn(strings.NewReader("1\n"))
	command.SetOut(io.Discard)
	selection, clients, loaded, err := promptCompatibleDetectedTargets(context.Background(), command, fixture.app, writeCLIPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("direct package was not retained across interactive target selection")
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	targets, err := parseTargetOption(selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := detectSelectedTargetsForLifecycleResolution(context.Background(), detector, targets, clients, true); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 1 || detector.targetedCalls != 1 || detector.probeCalls != 0 {
		t.Fatalf("interactive detection calls: read-only=%d targeted=%d ambient=%d", detector.readOnlyCalls, detector.targetedCalls, detector.probeCalls)
	}
	if !reflect.DeepEqual(detector.targets, []domain.ClientID{domain.ClientCodex}) {
		t.Fatalf("version-probed targets = %#v", detector.targets)
	}
}

func TestInteractiveAddIgnoresConfigOnlyClaudeButExplicitClaudeFailsClosed(t *testing.T) {
	claude := fixtureClient(t, domain.ClientClaude)
	claude.Status = domain.DetectionNotDetected
	claude.ExecutablePath = ""
	claude.Surfaces = []domain.ClientSurface{{ID: "claude_config", Detected: true, Evidence: "configuration_directory"}}
	cursor := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{claude, cursor})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.executeInput(true, "", "add", plugin, "--dry-run"); err != nil {
		t.Fatalf("actionable default target failed because of stale Claude config: %v", err)
	}
	_, _, err := fixture.execute(false, "add", plugin, "--target", "claude", "--dry-run")
	if err == nil || !strings.Contains(err.Error(), `target "claude" was not detected`) || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("explicit config-only Claude error = %v", err)
	}
}

func TestCommaSeparatedTargetsRejectUnsafeValues(t *testing.T) {
	t.Parallel()
	for name, value := range map[string]string{
		"empty":        "codex,,cursor",
		"duplicate":    "codex,cursor,codex",
		"all":          "all",
		"ambiguous":    "openai,cursor",
		"unsupported":  "unknown-agent,cursor",
		"legacy_mixed": "legacy-all,cursor",
	} {
		name, value := name, value
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseTargetOption(value); err == nil {
				t.Fatalf("parseTargetOption(%q) succeeded", value)
			}
		})
	}
}

func TestCommaSeparatedTargetsFailCompletePreflightBeforeMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex)})
	plugin := writeCLIPlugin(t)
	_, _, err := fixture.execute(false, "add", plugin, "--target", "cursor,codex,kiro", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "kiro") {
		t.Fatalf("partial failure = %v", err)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("failed complete preflight mutated state = %+v", state)
	}
}

func TestYesFlagIsNotPartOfThePublicCLI(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(true, "add", plugin, "--target", "cursor", "--yes"); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("--yes error = %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("rejected --yes mutated state: %+v", state)
	}
}

func TestProjectScopeIsRejectedBeforeSourceResolution(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	counter := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = counter
	plugin := writeCLIPlugin(t)
	_, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--scope", "project")
	if err == nil || !strings.Contains(err.Error(), "supports user scope only") {
		t.Fatalf("project-scope error = %v", err)
	}
	if counter.calls != 0 {
		t.Fatalf("project scope resolved a source %d time(s)", counter.calls)
	}
}

func TestSwitchUsesTheCompleteInstallationEngineBoundaryWithoutHiddenConfirmation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(true, "switch", "demo", "--to", plugin, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "switch")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].OperationGroupID == "" {
		t.Fatalf("switch state = %+v", state)
	}
}

func TestSwitchByInstalledAliasNormalizesRetainedInstallationIdentity(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	installed := writeCLIPlugin(t)
	destination := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", installed, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("expected retained installation without bindings: %+v", state)
	}
	installationID := state.Installations[0].InstallationID
	state.Installations[0].Source.RequestedSource = "installed-alias"
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(true, "switch", "installed-alias", "--to", destination, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "switch")
	updated, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Installations) != 1 || updated.Installations[0].InstallationID != installationID ||
		updated.Installations[0].Source.RequestedSource != destination || !updated.Installations[0].DataRetained {
		t.Fatalf("alias switch did not preserve installation identity and retained data: %+v", updated)
	}
}

func TestPurgeDataUsesOwnershipCheckedEngineBoundaryWithoutHiddenConfirmation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	retained, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(retained.Installations) != 1 || !retained.Installations[0].DataRetained || len(retained.Installations[0].DataReceipts) != 1 {
		t.Fatalf("normal removal did not retain owned data: %+v", retained)
	}
	var dataPath string
	for _, receipt := range retained.Installations[0].DataReceipts {
		dataPath = receipt.Locator
	}
	marker := filepath.Join(dataPath, "marker")
	if err := os.WriteFile(marker, []byte("preserve me"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(true, "remove", "demo", "--purge-data", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "remove")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("purge retained state = %+v", state)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("explicit purge retained marker: %v", err)
	}
}

func TestDryRunAndDoctorAreStrictlyReadOnly(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	if _, err := os.Lstat(fixture.store.Path); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state: %v", err)
	}
	stdout, _, err = fixture.execute(false, "doctor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "doctor")
	if _, err := os.Lstat(fixture.store.Path); !os.IsNotExist(err) {
		t.Fatalf("doctor created state: %v", err)
	}
	if _, err := os.Lstat(fixture.operations); !os.IsNotExist(err) {
		t.Fatalf("doctor created operation journal: %v", err)
	}
}

func TestDoctorNeverUsesExplicitLifecycleVersionProbe(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	detector := &observedProbingDetector{}
	fixture.app.Detector = detector
	if _, _, err := fixture.execute(false, "doctor", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 1 || detector.probeCalls != 0 {
		t.Fatalf("doctor detection calls: read-only=%d probe=%d", detector.readOnlyCalls, detector.probeCalls)
	}
}

func TestLifecycleResolutionExplicitlyRequestsVersionProbe(t *testing.T) {
	detector := &observedProbingDetector{}
	if _, err := detectClientsForLifecycleResolution(context.Background(), detector, true); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 0 || detector.probeCalls != 1 {
		t.Fatalf("lifecycle detection calls: read-only=%d probe=%d", detector.readOnlyCalls, detector.probeCalls)
	}
}

func TestDryRunLifecycleResolutionNeverUsesVersionProbe(t *testing.T) {
	client := fixtureClient(t, domain.ClientCursor)
	detector := &observedProbingDetector{clients: []domain.DetectedClient{client}}
	fixture := newCLIFixture(t, nil)
	fixture.app.Detector = detector
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--dry-run", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 1 || detector.probeCalls != 0 {
		t.Fatalf("dry-run detection calls: read-only=%d probe=%d", detector.readOnlyCalls, detector.probeCalls)
	}
}

func TestDirectLifecycleResolutionNeverUsesVersionProbe(t *testing.T) {
	client := fixtureClient(t, domain.ClientCursor)
	detector := &observedProbingDetector{clients: []domain.DetectedClient{client}}
	fixture := newCLIFixture(t, nil)
	fixture.app.Detector = detector
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if detector.readOnlyCalls != 1 || detector.probeCalls != 0 {
		t.Fatalf("direct-source detection calls: read-only=%d probe=%d", detector.readOnlyCalls, detector.probeCalls)
	}
}

func TestDoctorDiagnosesManagedDirectoryChangesWithRecovery(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if err := os.WriteFile(filepath.Join(binding.TargetLocator, "unexpected.txt"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var output struct {
		Data struct {
			Findings []doctorFinding `json:"findings"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range output.Data.Findings {
		if finding.Status == "degraded" && finding.RecoveryAction == "" {
			t.Fatalf("degraded finding has no recovery: %+v", finding)
		}
		if finding.Code == "managed_directory_changed" {
			found = true
			if finding.InstallationName != "demo" || finding.InstallationID == "" || finding.ClientID != "cursor" {
				t.Fatalf("managed finding scope = %+v", finding)
			}
			if finding.RecoveryAction != "run `agentplugins repair "+finding.InstallationID+" --target cursor`" {
				t.Fatalf("managed finding recovery = %q", finding.RecoveryAction)
			}
		}
	}
	if !found {
		t.Fatalf("managed integrity finding missing: %+v", output.Data.Findings)
	}
}

func TestDoctorBlocksRepairForExcludedOwnershipMarker(t *testing.T) {
	t.Parallel()
	for _, marker := range []string{".git", ".plugin-kit-ai.lock"} {
		t.Run(marker, func(t *testing.T) {
			fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
			plugin := writeCLIPlugin(t)
			if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
				t.Fatal(err)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyCLIClient(state.Installations[0])
			markerPath := filepath.Join(binding.TargetLocator, marker)
			if marker == ".git" {
				if err := os.Mkdir(markerPath, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(markerPath, []byte("excluded"), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, _, err := fixture.execute(false, "doctor", state.Installations[0].InstallationID, "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			var output struct {
				Data struct {
					Findings []doctorFinding `json:"findings"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatal(err)
			}
			for _, finding := range output.Data.Findings {
				if finding.Code != "excluded_ownership_marker" {
					continue
				}
				if finding.Status != "unknown" || finding.InstallationID != state.Installations[0].InstallationID ||
					!strings.Contains(finding.RecoveryAction, "manually review and remove") || strings.Contains(finding.RecoveryAction, "agentplugins repair") {
					t.Fatalf("excluded marker finding = %+v", finding)
				}
				return
			}
			t.Fatalf("excluded ownership marker finding missing: %+v", output.Data.Findings)
		})
	}
}

func TestDoctorRecoveryIncludesProjectScopeAndExactTarget(t *testing.T) {
	t.Parallel()
	installation := domain.Installation{InstallationID: "00000000-0000-4000-8000-000000000001", DeclaredName: "demo"}
	binding := domain.ClientBinding{ClientID: "cursor", Scope: string(domain.ScopeProject)}
	if got := repairAction(installation, binding); got != "run `agentplugins repair 00000000-0000-4000-8000-000000000001 --target cursor --scope project`" {
		t.Fatalf("project repair action = %q", got)
	}
}

func TestRepairSourcePinsSelectedClientRevision(t *testing.T) {
	t.Parallel()
	installation := domain.Installation{Source: domain.SourceBinding{
		Repository: "acme/plugins", PackageSubpath: "plugins/demo", ResolvedRevision: "moving-head",
	}}
	binding := domain.ClientBinding{PackageRevision: &domain.ClientPackageRevision{ResolvedRevision: "client-commit"}}
	got, err := repairSource(installation, binding)
	if err != nil {
		t.Fatal(err)
	}
	if got != "github:acme/plugins@client-commit//plugins/demo" {
		t.Fatalf("repair source = %q", got)
	}
	binding.PackageRevision.ResolvedRevision = ""
	if _, err := repairSource(installation, binding); err == nil || !strings.Contains(err.Error(), "moving source") {
		t.Fatalf("unpinned repair error = %v", err)
	}
}

func TestDoctorScopesPendingRecoveryAndRendersIdentity(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationPending
		state.Installations[0].Clients[key] = binding
	}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "doctor", "demo")
	if err != nil {
		t.Fatal(err)
	}
	installationID := state.Installations[0].InstallationID
	for _, expected := range []string{
		"Installation: demo (" + installationID + ")",
		"Client: cursor",
		"complete the displayed external activation step, then rerun the same `agentplugins add ... --target cursor` command",
		"complete the displayed external authentication step, then rerun the same `agentplugins add ... --target cursor` command",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("doctor output omitted %q:\n%s", expected, stdout)
		}
	}
	if strings.Contains(stdout, "then rerun doctor") {
		t.Fatalf("pending lifecycle recovery incorrectly recommends doctor:\n%s", stdout)
	}
}

func TestDoctorBlocksAutomaticMutationForMissingPhysicalIdentity(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.PhysicalArtifact = ""
		state.Installations[0].Clients[key] = binding
	}
	findings := doctorFindings(context.Background(), fixture.app, fixture.app.Detector.(staticDetector).clients, state, nil, &state.Installations[0])
	var found doctorFinding
	for _, finding := range findings {
		if finding.Code == "managed_target_unverifiable" {
			found = finding
		}
	}
	if found.Code == "" || !strings.Contains(found.RecoveryAction, "automatic mutation is intentionally blocked") {
		t.Fatalf("missing identity recovery = %+v", findings)
	}
	if strings.Contains(found.RecoveryAction, "update") || strings.Contains(found.RecoveryAction, "remove") {
		t.Fatalf("blocked identity recovery invented a refusing mutation: %+v", found)
	}
}

func TestDoctorDoesNotRequireCopilotCLIForVSCodeBinding(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientVSCode)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "vscode"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "copilot_cli_missing") {
		t.Fatalf("VS Code binding incorrectly required Copilot CLI: %s", stdout)
	}
}

func TestDoctorDoesNotReportMissingCopilotCLIWhenCopilotIsNotVisible(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCopilot)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	fixture.app.Detector = staticDetector{}
	var stdout, stderr bytes.Buffer
	app := fixture.app
	app.Output = &stdout
	app.ErrorOutput = &stderr
	command := NewRoot(app)
	command.SetArgs([]string{"doctor", "demo", "--format", "json"})
	if err := command.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "client_not_visible") || strings.Contains(stdout.String(), "copilot_cli_missing") {
		t.Fatalf("invisible Copilot findings = %s", stdout.String())
	}
}

func TestDoctorDoesNotCallVerifierFailureChangedContent(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	fixture.app.Lifecycle.Stager = failingVerifyStager{err: errors.New("temporary verifier unavailable")}
	stdout, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "managed_integrity_check_failed") || strings.Contains(stdout, "managed_directory_changed") {
		t.Fatalf("verification infrastructure error was mislabeled: %s", stdout)
	}
}

func TestDoctorSkipsClientWarningsForAbsentBinding(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Materialization = domain.MaterializationAbsent
		state.Installations[0].Clients[key] = binding
	}
	findings := doctorFindings(context.Background(), fixture.app, nil, state, nil, &state.Installations[0])
	for _, finding := range findings {
		if finding.ClientID != "" {
			t.Fatalf("absent binding produced client finding: %+v", finding)
		}
	}
}

func TestHumanCodexFlowNeverClaimsPreparedPackageIsInstalled(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.execute(true, "add", plugin, "--target", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Package prepared") || strings.Contains(stdout, "Installed and verified") {
		t.Fatalf("human output overclaimed activation: %s", stdout)
	}
	if strings.Count(stdout, "Next:") != 1 || !strings.Contains(stdout, fixture.root) || !strings.Contains(stdout, "verify") {
		t.Fatalf("human output must contain one path-bearing verification step: %s", stdout)
	}
}

func TestRepeatedAddResumesManualLifecycleWithoutAnotherReceipt(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stdout, "Next:") != 1 || !strings.Contains(stdout, "verify") || !strings.Contains(stdout, "plugins/local") {
		t.Fatalf("resume output = %q", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if receipts := onlyCLIClient(state.Installations[0]).Receipts; len(receipts) != 1 {
		t.Fatalf("resume created another materialization receipt: %+v", receipts)
	}
}

func TestExplicitInteractiveAddTreatsCommandAsConsentBeforeLifecycle(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.executeInput(true, "n\n", "add", plugin, "--target", "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "Apply this plan? [y/N]") || !strings.Contains(stdout, "Have you completed activation") {
		t.Fatalf("initial prompt order = %q", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || onlyCLIClient(state.Installations[0]).Materialization != domain.MaterializationMaterialized {
		t.Fatalf("explicit command did not materialize before declined manual activation: %+v", state)
	}
}

func TestInteractiveActivationYesAuthNoKeepsIndependentResumableState(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.executeInput(true, "y\nn\n", "add", plugin, "--target", "cursor")
	if err != nil {
		t.Fatal(err)
	}
	activation := strings.Index(stdout, "Have you completed activation")
	auth := strings.Index(stdout, "Have you completed required authentication")
	if strings.Contains(stdout, "Apply this plan? [y/N]") || activation < 0 || auth <= activation {
		t.Fatalf("interactive prompt order = %q", stdout)
	}
	if strings.Count(stdout, "Next:") != 1 || !strings.Contains(stdout, "Activation is user-attested") {
		t.Fatalf("interactive next action or attestation output = %q", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled || binding.Authentication != domain.AuthenticationNotChecked {
		t.Fatalf("declined auth state = %+v", binding)
	}
}

func TestCLICompletionFlagsRequirePriorMaterialization(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	_, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--activation-complete", "--auth-complete")
	if err == nil || !strings.Contains(err.Error(), "already materialized") {
		t.Fatalf("fresh completion error = %v", err)
	}
}

func TestCLIAddAndUpdatePersistConvergedNegativeObservationBeforeConfirmation(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	notDetectedVSCode := fixtureClient(t, domain.ClientVSCode)
	notDetectedVSCode.Status = domain.DetectionNotDetected
	fixture := newCLIFixture(t, []domain.DetectedClient{client, notDetectedVSCode})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	receiptCount := len(onlyCLIClient(state.Installations[0]).Receipts)

	for _, command := range []string{"add", "update"} {
		state, err = fixture.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		for key, binding := range state.Installations[0].Clients {
			binding.Activation = domain.ActivationActive
			binding.Authentication = domain.AuthenticationNotRequired
			binding.Verification = domain.VerificationInstalled
			state.Installations[0].Clients[key] = binding
		}
		if err := fixture.store.Save(state); err != nil {
			t.Fatal(err)
		}
		observer := &cliObservedActivator{outcome: domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: domain.AuthenticationNotRequired,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed, AuthoritativeObservation: true,
		}, err: errors.New("recognized negative client evidence")}
		fixture.app.Lifecycle.Activator = observer
		selector := plugin
		if command == "update" {
			selector = "demo"
		}
		args := []string{command, selector, "--target", "copilot"}
		stdout, _, commandErr := fixture.execute(true, args...)
		if commandErr == nil || !strings.Contains(commandErr.Error(), "recognized negative client evidence") {
			t.Fatalf("%s error=%v output=%q", command, commandErr, stdout)
		}
		if observer.calls != 1 || strings.Contains(stdout, "Apply this") {
			t.Fatalf("%s verifier calls=%d output=%q", command, observer.calls, stdout)
		}
		state, err = fixture.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		binding := onlyCLIClient(state.Installations[0])
		if binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed || len(binding.Receipts) != receiptCount {
			t.Fatalf("%s state=%+v", command, binding)
		}
	}
}

func TestSharedBackendUpdateUsesSingleCopilotDetector(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	receipts := len(onlyCLIClient(before.Installations[0]).Receipts)
	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"2.0.0","description":"updated shared backend"}`
	if err := os.WriteFile(filepath.Join(plugin, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "update", "demo", "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations) != 1 || len(after.Installations[0].Clients) != 1 {
		t.Fatalf("shared update created another physical binding: %+v", after)
	}
	binding := onlyCLIClient(after.Installations[0])
	if !reflect.DeepEqual(binding.AffectedSurfaces, []string{"copilot", "vscode"}) || len(binding.Receipts) != receipts+1 {
		t.Fatalf("shared update binding = %+v", binding)
	}
}

func TestSharedBackendRepairUsesSingleCopilotDetector(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	notDetectedVSCode := fixtureClient(t, domain.ClientVSCode)
	notDetectedVSCode.Status = domain.DetectionNotDetected
	fixture := newCLIFixture(t, []domain.DetectedClient{client, notDetectedVSCode})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(before.Installations[0])
	receipts := len(binding.Receipts)
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "repair", "demo", "--target", "vscode"); err != nil {
		t.Fatal(err)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations) != 1 || len(after.Installations[0].Clients) != 1 {
		t.Fatalf("shared repair created another physical binding: %+v", after)
	}
	binding = onlyCLIClient(after.Installations[0])
	if !reflect.DeepEqual(binding.AffectedSurfaces, []string{"copilot", "vscode"}) || len(binding.Receipts) != receipts+1 {
		t.Fatalf("shared repair binding = %+v", binding)
	}
	if _, err := os.Stat(binding.TargetLocator); err != nil {
		t.Fatalf("shared repair did not restore physical package: %v", err)
	}
}

func TestSharedBackendUpdateRejectsWhenNeitherSurfaceIsDetected(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "copilot"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	receipts := len(onlyCLIClient(before.Installations[0]).Receipts)
	fixture.app.Detector = staticDetector{}
	if _, _, err := fixture.execute(false, "update", "demo", "--target", "copilot"); err == nil || !strings.Contains(err.Error(), "was not detected") {
		t.Fatalf("missing shared backend error = %v", err)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Installations) != 1 || len(after.Installations[0].Clients) != 1 || len(onlyCLIClient(after.Installations[0]).Receipts) != receipts {
		t.Fatalf("missing shared backend mutated state: %+v", after)
	}
}

func TestInteractiveUnknownCopilotOutputReverifiesBeforeAttestation(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	runner := &cliCommandRunner{}
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.executeInput(true, "y\nn\n", "add", plugin, "--target", "copilot")
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls == 0 || !strings.Contains(stdout, "output contract was not recognized") {
		t.Fatalf("unknown-output path did not execute verifier or show recovery: calls=%d output=%q", runner.calls, stdout)
	}
	if !strings.Contains(stdout, "Have you completed activation") || !strings.Contains(stdout, "Activation is user-attested") {
		t.Fatalf("unknown observable output did not complete usable attestation flow: %q", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("attested output state = %+v", binding)
	}
	if runner.calls != 4 {
		t.Fatalf("runner calls = %d, want 4 (activation commands plus retry verifier)", runner.calls)
	}
}

func TestInteractiveUnknownRetryRecognizedNegativeFailsClosed(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientCopilot)
	client.ExecutablePath = "/test/bin/copilot"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	runner := &cliCommandRunner{run: func(call int, command legacyports.Command) legacyports.CommandResult {
		if strings.HasSuffix(strings.Join(command.Argv, " "), "plugin list") && call >= 4 {
			return legacyports.CommandResult{Stdout: []byte("Installed plugins:\n  No plugins installed.")}
		}
		return legacyports.CommandResult{}
	}}
	fixture.app.Lifecycle.Activator = providers.Activator{Runner: runner}
	plugin := writeCLIPlugin(t)
	stdout, _, err := fixture.executeInput(true, "y\n", "add", plugin, "--target", "copilot")
	if err == nil || !strings.Contains(err.Error(), "recognized negative client evidence") {
		t.Fatalf("retry err=%v output=%q", err, stdout)
	}
	if runner.calls != 4 {
		t.Fatalf("runner calls = %d, want 4", runner.calls)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed {
		t.Fatalf("recognized negative retry state = %+v", binding)
	}
}

func TestRepairExplicitlyRestoresMissingManagedDirectory(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	target := onlyCLIClient(state.Installations[0]).TargetLocator
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "repair", "demo", "--target", "cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "repaired from the exact installed revision") {
		t.Fatalf("repair output = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(target, "plugin.json")); err != nil {
		t.Fatalf("repaired package: %v", err)
	}
}

func TestRepairUnavailableSelectedTargetFailsBeforePackageAcquisition(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	if _, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	counter := &countingSourceAcquirer{delegate: fixture.app.SourceAcquirer}
	fixture.app.SourceAcquirer = counter
	fixture.app.Detector = staticDetector{}
	_, _, err = fixture.execute(false, "repair", "demo", "--target", "cursor")
	if err == nil || !strings.Contains(err.Error(), `target "cursor" was not detected`) || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("unavailable repair target error = %v", err)
	}
	if counter.calls != 0 {
		t.Fatalf("unavailable repair target acquired package %d times", counter.calls)
	}
	after, loadErr := fixture.store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("unavailable repair target mutated state: before=%+v after=%+v", before, after)
	}
}

func TestRepairWithoutTargetsChecksAllInstalledBindings(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex),
		fixtureClient(t, domain.ClientCursor),
	})
	plugin := writeCLIPlugin(t)
	for _, target := range []string{"codex", "cursor"} {
		if _, _, err := fixture.execute(false, "add", plugin, "--target", target); err != nil {
			t.Fatal(err)
		}
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var codexTarget string
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(domain.ClientCodex) {
			codexTarget = binding.TargetLocator
		}
	}
	if err := os.RemoveAll(codexTarget); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(true, "repair", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Repair demo: completed") {
		t.Fatalf("repair summary = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(codexTarget, "plugin.json")); err != nil {
		t.Fatalf("selected target was not repaired: %v", err)
	}
}

func TestInteractiveMultiTargetRepairUsesOneCombinedPlan(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex),
		fixtureClient(t, domain.ClientCursor),
	})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "codex,cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	targets := make(map[string]string, len(state.Installations[0].Clients))
	for _, binding := range state.Installations[0].Clients {
		targets[binding.ClientID] = binding.TargetLocator
		if err := os.RemoveAll(binding.TargetLocator); err != nil {
			t.Fatal(err)
		}
	}

	stdout, _, err := fixture.execute(true, "repair", "demo", "--target", "codex,cursor")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Repair demo: completed") {
		t.Fatalf("repair output = %q", stdout)
	}
	for client, target := range targets {
		if _, err := os.Stat(filepath.Join(target, "plugin.json")); err != nil {
			t.Fatalf("%s target was not repaired: %v", client, err)
		}
	}
}

func TestJSONRepairAutoConfirmsWithoutYes(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	target := onlyCLIClient(state.Installations[0]).TargetLocator
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(true, "repair", "demo", "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "repair")
	if _, err := os.Stat(filepath.Join(target, "plugin.json")); err != nil {
		t.Fatalf("JSON repair without --yes did not restore target: %v", err)
	}
}

func TestRepairReturnsOutputErrors(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(onlyCLIClient(state.Installations[0]).TargetLocator); err != nil {
		t.Fatal(err)
	}
	app := fixture.app
	app.Input = strings.NewReader("n\n")
	app.Output = alwaysErrorWriter{}
	app.ErrorOutput = io.Discard
	app.Terminal = true
	command := NewRoot(app)
	command.SetArgs([]string{"repair", "demo", "--target", "cursor"})
	if err := command.ExecuteContext(context.Background()); err == nil || !strings.Contains(err.Error(), "synthetic output failure") {
		t.Fatalf("repair output error = %v", err)
	}
}

func TestChatGPTTargetIsDistinctFromCodex(t *testing.T) {
	t.Parallel()
	if got := normalizeTarget("chatgpt"); got != domain.ClientChatGPT {
		t.Fatalf("ChatGPT target = %s", got)
	}
}

func TestChatGPTDoctorDoesNotApplyAnotherTargetsNewerInventory(t *testing.T) {
	t.Parallel()
	installation := domain.Installation{
		Source:  domain.SourceBinding{TreeDigest: "sha256:new-tree"},
		Package: domain.PackageBinding{ManifestDigest: "sha256:new-manifest", Inventory: domain.ComponentInventory{MCPPresent: true}},
	}
	binding := domain.ClientBinding{PackageRevision: &domain.ClientPackageRevision{TreeDigest: "sha256:chatgpt-tree", ManifestDigest: "sha256:chatgpt-manifest"}}
	if packageInventoryAppliesToBinding(installation, binding) {
		t.Fatal("newer package inventory was applied to an older ChatGPT client revision")
	}
}

func TestOpenAITargetFailsAsAmbiguousWithoutMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "openai"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("openai target error = %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("ambiguous target mutated state: %+v, %v", state, err)
	}
}

func TestChatGPTMCPWithoutAppBindingFailsBeforeMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt")
	if err == nil || !strings.Contains(err.Error(), "Developer Mode") || !strings.Contains(err.Error(), ".app.json") {
		t.Fatalf("missing app error = %v", err)
	}
	if !strings.Contains(stdout, "Developer Mode") || !strings.Contains(stdout, ".app.json") || !strings.Contains(stdout, "demo") {
		t.Fatalf("human unsupported plan omitted recovery guidance: %s", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("missing app mutated state: %+v, %v", state, err)
	}
	if _, err := os.Stat(fixture.app.ManagedRoot); !os.IsNotExist(err) {
		t.Fatalf("unsupported plan mutated managed filesystem: %v", err)
	}
}

func TestChatGPTUnsupportedPlanRendersStructuredJSONGuidance(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt", "--dry-run", "--format", "json")
	if err == nil {
		t.Fatal("unsupported ChatGPT dry-run succeeded")
	}
	if !strings.Contains(stdout, `"status":"unsupported"`) || !strings.Contains(stdout, `"user_actions"`) || !strings.Contains(stdout, "Developer Mode") || !strings.Contains(stdout, ".app.json") {
		t.Fatalf("JSON unsupported plan omitted structured guidance: %s", stdout)
	}
	state, stateErr := fixture.store.Load()
	if stateErr != nil || len(state.Installations) != 0 {
		t.Fatalf("unsupported JSON plan mutated state: %+v, %v", state, stateErr)
	}
}

func TestChatGPTUnsupportedUpdateRendersRecoveryWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		format string
	}{
		{name: "human", format: "human"},
		{name: "json", format: "json"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newCLIFixture(t, nil)
			plugin := writeCLIOfficialAppPlugin(t)
			if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
				t.Fatal(err)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyCLIClient(state.Installations[0])
			stateBefore, err := os.ReadFile(fixture.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(plugin, ".app.json")); err != nil {
				t.Fatal(err)
			}
			args := []string{"update", "demo", "--target", "chatgpt"}
			if test.format == "json" {
				args = append(args, "--format", "json")
			}
			stdout, _, updateErr := fixture.execute(false, args...)
			if updateErr == nil {
				t.Fatal("unsupported ChatGPT update succeeded")
			}
			if test.format == "json" {
				var failure map[string]any
				if err := json.Unmarshal([]byte(stdout), &failure); err != nil || failure["schema_version"] != float64(1) || failure["command"] != "update" || failure["result"] != "failure" {
					t.Fatalf("update failure envelope = %+v, %v", failure, err)
				}
				if !strings.Contains(stdout, `"user_actions"`) {
					t.Fatalf("JSON update omitted structured user actions: %s", stdout)
				}
			}
			for _, expected := range []string{"Developer Mode", ".app.json", "demo"} {
				if !strings.Contains(stdout, expected) || !strings.Contains(updateErr.Error(), expected) {
					t.Fatalf("%s update omitted %q recovery guidance: stdout=%q error=%v", test.format, expected, stdout, updateErr)
				}
			}
			stateAfter, err := os.ReadFile(fixture.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stateBefore, stateAfter) {
				t.Fatal("unsupported ChatGPT update mutated state")
			}
			if _, err := os.Stat(filepath.Join(binding.TargetLocator, ".app.json")); err != nil {
				t.Fatalf("unsupported ChatGPT update mutated the managed package: %v", err)
			}
		})
	}
}

func TestChatGPTUnsupportedRepairRendersStructuredRecoveryWithoutMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIOfficialAppPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if err := os.Remove(filepath.Join(plugin, ".app.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, repairErr := fixture.execute(false, "repair", "demo", "--target", "chatgpt", "--format", "json")
	if repairErr == nil {
		t.Fatal("unsupported ChatGPT repair succeeded")
	}
	var failureEnvelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &failureEnvelope); err != nil || failureEnvelope["schema_version"] != float64(1) || failureEnvelope["command"] != "repair" || failureEnvelope["result"] != "failure" {
		t.Fatalf("repair failure envelope = %+v, %v", failureEnvelope, err)
	}
	if !strings.Contains(repairErr.Error(), "exact applied package revision") {
		t.Fatalf("changed direct repair source was not rejected as non-reproducible: %v", repairErr)
	}
	stateAfter, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stateBefore, stateAfter) {
		t.Fatal("unsupported ChatGPT repair mutated state")
	}
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("unsupported ChatGPT repair mutated the managed package: %v", err)
	}
}

func TestNoDetectedClientSuggestsExplicitChatGPTTarget(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIPlugin(t)
	_, _, err := fixture.execute(true, "add", plugin)
	if err == nil || !strings.Contains(err.Error(), "--target chatgpt") || !strings.Contains(err.Error(), "install/detect another client") {
		t.Fatalf("zero-client guidance = %v", err)
	}
	state, stateErr := fixture.store.Load()
	if stateErr != nil || len(state.Installations) != 0 {
		t.Fatalf("zero-client add mutated state: %+v, %v", state, stateErr)
	}
}

func TestCatalogChatGPTAppBindingVerifiesMCPAndSynthesizesApp(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	envelope, err := fixture.app.PackageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: plugin})
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadedPackage{envelope: envelope, hints: domain.CompatibilityHints{Compatibility: map[string]domain.CatalogCompatibility{
		"chatgpt": {
			Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired,
			AppBinding: &domain.CatalogAppBinding{AppKey: "demo", ID: "asdk_app_demo_123", MCPServer: "demo", MCPURL: "https://example.test/mcp", RuntimeEvidence: "tests/e2e/results/chatgpt-demo.json", RuntimeEvidenceRevision: strings.Repeat("e", 40)},
		},
	}}}
	if err := prepareLoadedPackageForClient(&loaded, domain.ClientChatGPT); err != nil {
		t.Fatal(err)
	}
	if !loaded.envelope.App.Enabled || loaded.envelope.App.Bindings["demo"].ID != "asdk_app_demo_123" || !strings.Contains(string(loaded.envelope.App.Raw), "asdk_app_demo_123") {
		t.Fatalf("catalog app synthesis = %+v", loaded.envelope.App)
	}

	mismatch := loaded
	mismatch.envelope = envelope
	binding := *mismatch.hints.Compatibility["chatgpt"].AppBinding
	binding.MCPURL = "https://other.example.test/mcp"
	compatibility := mismatch.hints.Compatibility["chatgpt"]
	compatibility.AppBinding = &binding
	mismatch.hints.Compatibility = map[string]domain.CatalogCompatibility{"chatgpt": compatibility}
	if err := prepareLoadedPackageForClient(&mismatch, domain.ClientChatGPT); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("catalog URL mismatch = %v", err)
	}

	existingMismatch := loaded
	existingMismatch.envelope = envelope
	existingMismatch.envelope.App = domain.AppComponent{
		Present: true, Declared: true, Enabled: true,
		Bindings: map[string]domain.AppBinding{"demo": {Alias: "demo", ID: "connector_wrong"}},
	}
	if err := prepareLoadedPackageForClient(&existingMismatch, domain.ClientChatGPT); err == nil || !strings.Contains(err.Error(), "does not exactly match") {
		t.Fatalf("existing app mismatch = %v", err)
	}

	restored := loadedPackage{envelope: envelope}
	revisionBinding := domain.ClientBinding{PackageRevision: &domain.ClientPackageRevision{CatalogEvidence: &domain.CatalogEvidence{
		SchemaVersion: 2, Compatibility: cloneCatalogCompatibility(loaded.hints.Compatibility),
	}}}
	restoreCatalogEvidence(&restored, revisionBinding)
	if err := prepareLoadedPackageForClient(&restored, domain.ClientChatGPT); err != nil || !restored.envelope.App.Enabled {
		t.Fatalf("persisted catalog evidence did not restore ChatGPT repair binding: %+v, %v", restored.envelope.App, err)
	}
}

func TestRepairRestoresCatalogEvidenceFromSelectedClientRevision(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	envelope, err := fixture.app.PackageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: plugin})
	if err != nil {
		t.Fatal(err)
	}
	compatibility := func(id string) map[string]domain.CatalogCompatibility {
		return map[string]domain.CatalogCompatibility{"chatgpt": {
			Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired,
			AppBinding: &domain.CatalogAppBinding{AppKey: "demo", ID: id, MCPServer: "demo", MCPURL: "https://example.test/mcp", RuntimeEvidence: "tests/e2e/results/chatgpt-demo.json", RuntimeEvidenceRevision: strings.Repeat("e", 40)},
		}}
	}
	installation := domain.Installation{Clients: map[string]domain.ClientBinding{
		"chatgpt-old": {ClientID: "chatgpt", PackageRevision: &domain.ClientPackageRevision{
			Version: "1.0.0", TreeDigest: "sha256:tree-a", ManifestDigest: "sha256:manifest-a",
			CatalogEvidence: &domain.CatalogEvidence{SchemaVersion: 2, Digest: "sha256:catalog-a", Compatibility: compatibility("connector_app_a")},
		}},
		"codex-new": {ClientID: "codex", PackageRevision: &domain.ClientPackageRevision{
			Version: "2.0.0", TreeDigest: "sha256:tree-b", ManifestDigest: "sha256:manifest-b",
			CatalogEvidence: &domain.CatalogEvidence{SchemaVersion: 2, Digest: "sha256:catalog-b", Compatibility: compatibility("connector_app_b")},
		}},
	}}
	restored := loadedPackage{envelope: envelope}
	restoreCatalogEvidence(&restored, installation.Clients["chatgpt-old"])
	if err := prepareLoadedPackageForClient(&restored, domain.ClientChatGPT); err != nil {
		t.Fatal(err)
	}
	if got := restored.envelope.App.Bindings["demo"].ID; got != "connector_app_a" {
		t.Fatalf("ChatGPT repair used evidence from a different client revision: got %q", got)
	}
}

func TestChatGPTAppPackagePreparesManualInstallWithoutDesktopDetection(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)})
	plugin := writeCLIOfficialAppPlugin(t)
	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Package prepared") || !strings.Contains(stdout, "ChatGPT Plugins") || !strings.Contains(stdout, ".app.json") {
		t.Fatalf("ChatGPT output = %q", stdout)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.ClientID != string(domain.ClientChatGPT) || binding.Activation != domain.ActivationManual {
		t.Fatalf("ChatGPT binding = %+v", binding)
	}
	manifest := readCLIObject(t, filepath.Join(binding.TargetLocator, ".codex-plugin", "plugin.json"))
	if manifest["apps"] != "./.app.json" || manifest["mcpServers"] != "./.mcp.json" {
		t.Fatalf("ChatGPT projection = %+v", manifest)
	}
	if mcp := readCLIObject(t, filepath.Join(binding.TargetLocator, ".mcp.json")); mcp["mcpServers"] == nil {
		t.Fatalf("ChatGPT projection lost bundled MCP parity: %+v", mcp)
	}
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("portable manifest shadows official ChatGPT projection: %v", err)
	}
	reloaded, err := fixture.app.NativePackageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: binding.TargetLocator})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FormatID != domain.FormatIDOpenAIPlugin || !reloaded.App.Enabled || !reloaded.MCP.Enabled {
		t.Fatalf("ChatGPT staged artifact is not a runnable official package: %+v", reloaded)
	}
	doctor, _, err := fixture.execute(false, "doctor", "demo", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(doctor, "client_not_visible") || !strings.Contains(doctor, "chatgpt_registration_unverified") {
		t.Fatalf("ChatGPT doctor findings = %s", doctor)
	}
}

func TestOfficialOpenAIPackagePreparesForChatGPT(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(plugin, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"official-demo","apps":"./.app.json","mcpServers":"./.mcp.json","interface":{"displayName":"Official Demo"},"future":{"preserved":true}}`
	if err := os.WriteFile(filepath.Join(plugin, ".codex-plugin", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, ".app.json"), []byte(`{"apps":{"demo":{"id":"plugin_asdk_app_demo_123"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, ".mcp.json"), []byte(`{"mcpServers":{"demo":{"type":"http","url":"https://example.test/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	installation := state.Installations[0]
	if installation.Package.FormatID != domain.FormatIDOpenAIPlugin || installation.Package.SchemaURI != "" {
		t.Fatalf("official binding = %+v", installation.Package)
	}
	projected := readCLIObject(t, filepath.Join(onlyCLIClient(installation).TargetLocator, ".codex-plugin", "plugin.json"))
	if projected["apps"] != "./.app.json" || projected["mcpServers"] != "./.mcp.json" || projected["interface"] == nil || projected["future"] == nil {
		t.Fatalf("official projection lost fields = %+v", projected)
	}
	if mcp := readCLIObject(t, filepath.Join(onlyCLIClient(installation).TargetLocator, ".mcp.json")); mcp["mcpServers"] == nil {
		t.Fatalf("official ChatGPT projection lost bundled MCP parity: %+v", mcp)
	}
}

func TestOfficialHooksFailClosedBeforeDryRunOrMutation(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(plugin, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"hooked","hooks":{"PreToolUse":[{"command":"./hooks/run"}]}}`
	if err := os.WriteFile(filepath.Join(plugin, ".codex-plugin", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt", "--dry-run", "--format", "json"); err == nil {
		t.Fatal("hook dry-run succeeded")
	} else {
		var loadErr *domain.LoadError
		if !errors.As(err, &loadErr) || loadErr.Diagnostic.Code != "official_hooks_unsupported" {
			t.Fatalf("hook dry-run error = %v", err)
		}
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("hook dry-run mutated state: %+v, %v", state, err)
	}
	if _, err := os.Stat(fixture.app.ManagedRoot); !os.IsNotExist(err) {
		t.Fatalf("hook dry-run mutated managed filesystem: %v", err)
	}
}

func TestImplicitPortableHooksAreIgnoredAndNotStaged(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := writeCLIPlugin(t)
	if err := os.MkdirAll(filepath.Join(plugin, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "hooks", "hooks.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt", "--dry-run", "--format", "json"); err != nil {
		t.Fatalf("portable package with unsupported root directory failed validation: %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("dry-run mutated state: %+v, %v", state, err)
	}
	if _, err := os.Stat(fixture.app.ManagedRoot); !os.IsNotExist(err) {
		t.Fatalf("dry-run mutated managed filesystem: %v", err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
		t.Fatalf("install portable package with ignored hooks: %v", err)
	}
	state, err = fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("installed state = %+v, %v", state, err)
	}
	installed := onlyCLIClient(state.Installations[0]).TargetLocator
	if _, err := os.Lstat(filepath.Join(installed, "hooks")); !os.IsNotExist(err) {
		t.Fatalf("unsupported hooks survived staging: %v", err)
	}
	if _, err := os.Stat(filepath.Join(plugin, "hooks", "hooks.json")); err != nil {
		t.Fatalf("source package was mutated: %v", err)
	}
}

func TestOfficialOpenAIPackageKeepsBundledMCPAndDropsAppForCodex(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCodex)})
	plugin := t.TempDir()
	if err := os.MkdirAll(filepath.Join(plugin, ".codex-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"official-codex","mcpServers":"./.mcp.json","apps":"./.app.json","interface":{"displayName":"Official Codex"}}`
	if err := os.WriteFile(filepath.Join(plugin, ".codex-plugin", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, ".mcp.json"), []byte(`{"mcpServers":{"docs":{"url":"https://example.test/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, ".app.json"), []byte(`{"apps":{"docs":{"id":"plugin_asdk_app_docs_123"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "codex"); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	target := onlyCLIClient(state.Installations[0]).TargetLocator
	projected := readCLIObject(t, filepath.Join(target, ".codex-plugin", "plugin.json"))
	if projected["mcpServers"] != "./.mcp.json" || projected["apps"] != nil || projected["interface"] == nil {
		t.Fatalf("Codex projection = %+v", projected)
	}
	mcp := readCLIObject(t, filepath.Join(target, ".mcp.json"))
	if mcp["mcpServers"] == nil {
		t.Fatalf("Codex MCP projection = %+v", mcp)
	}
	if _, err := os.Stat(filepath.Join(target, ".app.json")); !os.IsNotExist(err) {
		t.Fatalf("Codex projection retained ChatGPT app binding: %v", err)
	}
}

func TestChatGPTBoundLifecycleWorksWithoutLocalDesktopDetection(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}})
	plugin := writeCLIPlugin(t)
	if err := os.WriteFile(filepath.Join(plugin, ".app.json"), []byte(`{"apps":{"demo":{"id":"plugin_asdk_app_demo_123"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "chatgpt"); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(plugin, "plugin.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "update", "demo", "--target", "chatgpt"); err != nil {
		t.Fatalf("undetected ChatGPT update: %v", err)
	}
	updated, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(onlyCLIClient(updated.Installations[0]).TargetLocator); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "repair", "demo", "--target", "chatgpt"); err != nil {
		t.Fatalf("undetected ChatGPT repair: %v", err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "chatgpt", "--external-uninstalled"); err != nil {
		t.Fatalf("undetected ChatGPT remove: %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].Package.Version != "2.0.0" || !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 || len(state.Installations[0].DataReceipts) == 0 {
		t.Fatalf("ChatGPT lifecycle state = %+v", state.Installations[0])
	}
}

func TestCodexAndChatGPTUseIndependentBindings(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex),
		{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected},
	})
	plugin := writeCLIOfficialAppPlugin(t)
	for _, target := range []string{"codex", "chatgpt"} {
		if _, _, err := fixture.execute(false, "add", plugin, "--target", target); err != nil {
			t.Fatalf("add %s: %v", target, err)
		}
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("state = %+v, %v", state, err)
	}
	seen := map[string]domain.ClientBinding{}
	for _, binding := range state.Installations[0].Clients {
		seen[binding.ClientID] = binding
	}
	if seen["codex"].ClientBindingID == seen["chatgpt"].ClientBindingID || seen["codex"].TargetLocator == seen["chatgpt"].TargetLocator {
		t.Fatalf("bindings were conflated: %+v", seen)
	}
}

func TestHumanOutputNeverCallsAuthPendingPackageInstalled(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := renderAddResult(&output, "human", domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "oauth-demo"}}, usecase.AddResult{
		Mutated: true,
		Activation: domain.ActivationOutcome{
			Activation: domain.ActivationActive, Authentication: domain.AuthenticationPending,
			Verification: domain.VerificationInstalled,
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "Installed") || !strings.Contains(output.String(), "Authentication is pending") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestTTYMultipleDetectedClientsRequireAValidInteractiveSelection(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientCodex),
	})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.executeInput(true, "9\n", "add", plugin); err == nil || !strings.Contains(err.Error(), "invalid client multiselect") {
		t.Fatalf("missing-target error = %v", err)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("invalid interactive selection mutated state: %+v", state)
	}
}

func TestUpdateAndRemoveCompleteLifecycleWithVersionedRedactedJSON(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(plugin, "plugin.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "update", "demo", "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "update")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if state.Installations[0].Package.Version != "2.0.0" || len(binding.Receipts) != 2 {
		t.Fatalf("updated state = %+v", state.Installations[0])
	}
	stdout, _, err = fixture.execute(false, "remove", "demo", "--target", "cursor", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "remove")
	state, err = fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 || len(state.Installations[0].DataReceipts) == 0 {
		t.Fatalf("removed state = %+v", state.Installations[0])
	}
}

func TestAutomatedUpdateDefaultsToInstalledBindingsButRemoveRequiresTarget(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "update", "demo"); err != nil {
		t.Fatalf("default installed-binding update failed: %v", err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo"); err == nil {
		t.Fatal("unsafe non-interactive removal succeeded")
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if binding := onlyCLIClient(state.Installations[0]); binding.Materialization != domain.MaterializationMaterialized || len(binding.Receipts) != 1 {
		t.Fatalf("unsafe command mutated state: %+v", binding)
	}
	if _, _, err := fixture.execute(false, "update", "demo", "--target", "cursor"); err != nil {
		t.Fatalf("explicit-target update without --yes failed: %v", err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor"); err != nil {
		t.Fatalf("explicit-target remove without --yes failed: %v", err)
	}
}

func TestHealthyInstallUsesSwitchInsteadOfRebind(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	first := writeCLIPlugin(t)
	second := writeCLIPlugin(t)
	if _, _, err := fixture.execute(false, "add", first, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "rebind", "demo", second, "--dry-run", "--format", "json"); err == nil || !strings.Contains(err.Error(), "use switch") {
		t.Fatalf("healthy rebind error = %v", err)
	}
	if _, _, err := fixture.execute(false, "remove", "demo", "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := fixture.execute(false, "switch", "demo", "--to", second, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "switch")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].Source.CanonicalSource != second || state.Installations[0].Source.RequestedSource != second {
		t.Fatalf("switched source = %+v", state.Installations[0].Source)
	}
}

func TestHumanBindingPlanShowsCompleteReviewedDiff(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	renderHumanBindingPlan(&output, usecase.BindingChangePlan{
		OldName: "legacy-demo", NewName: "standard-demo",
		OldSource: usecase.ProvenanceSummary{
			Kind: "github", Repository: "example/legacy", PackageSubpath: "plugins/demo",
			ResolvedRevision: "old-revision", TreeDigest: "sha256:old-tree",
		},
		NewSource: usecase.ProvenanceSummary{
			Kind: "github", Repository: "example/standard", PackageSubpath: "plugins/demo",
			ResolvedRevision: "new-revision", TreeDigest: "sha256:new-tree",
		},
		OldFormat:     usecase.FormatSummary{FormatID: domain.FormatIDLegacyV1, SchemaURI: "plugin.yaml/v1"},
		NewFormat:     usecase.FormatSummary{FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1},
		OldComponents: domain.ComponentInventory{Skills: []string{"old-skill"}},
		NewComponents: domain.ComponentInventory{
			MCPPresent: true, MCPEnabled: true, MCPServers: []string{"docs"},
			Skills: []string{"new-skill"}, Extensions: []string{"com.example.client"},
		},
		NativeObjectCount: 2,
	})
	for _, expected := range []string{
		"Schema: plugin.yaml/v1 -> " + domain.PluginSchemaV1,
		"example/legacy//plugins/demo@old-revision#sha256:old-tree",
		"example/standard//plugins/demo@new-revision#sha256:new-tree",
		"Components: mcp=absent skills=[old-skill] extensions=[] -> mcp=enabled[docs] skills=[new-skill] extensions=[com.example.client]",
		"Native objects: 2",
		"PLUGIN_DATA: not transferred",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output omitted %q: %s", expected, output.String())
		}
	}
}

func TestMigrateStateIsExplicitPlanFirstAndPathRedacted(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	legacyPath := filepath.Join(fixture.root, "legacy", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"schema_version":1,"installations":[{"integration_id":"legacy-demo","requested_source_ref":{"kind":"github_repo_path","value":"legacy-demo"},"resolved_source_ref":{"kind":"git_commit","value":"https://example.com/demo@abc"},"source_digest":"sha256:tree","manifest_digest":"sha256:manifest","policy":{"scope":"user"},"targets":{}}]}`
	if err := os.WriteFile(legacyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := statemigration.Migrator{LegacyPath: legacyPath, V2Store: fixture.store}
	fixture.app.StateMigrator = &migrator
	stdout, _, err := fixture.execute(false, "migrate-state", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "migrate-state")
	if strings.Contains(stdout, fixture.root) {
		t.Fatalf("migration plan leaked a sandbox path: %s", stdout)
	}
	if _, err := os.Stat(fixture.store.Path); !os.IsNotExist(err) {
		t.Fatalf("dry-run created state v2: %v", err)
	}
	stdout, _, err = fixture.execute(false, "migrate-state", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "migrate-state")
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || state.Installations[0].Package.LoaderKind != domain.LoaderKindLegacy {
		t.Fatalf("migrated state = %+v", state)
	}
}

func TestMigrateStateExplicitlyMigratesAuthoritativeSchemaTwoAndThree(t *testing.T) {
	for _, schema := range []int{domain.LegacyStateSchemaVersion, domain.PreviousStateSchemaVersion} {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			if err := os.MkdirAll(filepath.Dir(fixture.store.Path), 0o700); err != nil {
				t.Fatal(err)
			}
			original := []byte(fmt.Sprintf("{\"schema_version\":%d,\"installations\":[]}\n", schema))
			if err := os.WriteFile(fixture.store.Path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			migrator := statemigration.Migrator{
				LegacyPath: filepath.Join(fixture.root, "missing-legacy-state.json"), V2Store: fixture.store,
				Lock: fixture.app.Lifecycle.Lock, RecoverJournal: fixture.app.Lifecycle.Kernel.Recover,
			}
			fixture.app.StateMigrator = &migrator
			stdout, _, err := fixture.execute(false, "migrate-state", "--dry-run", "--format", "json")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout, fmt.Sprintf(`"source_schema":%d`, schema)) {
				t.Fatalf("authoritative plan omitted source schema: %s", stdout)
			}
			afterPlan, err := os.ReadFile(fixture.store.Path)
			if err != nil || !bytes.Equal(afterPlan, original) {
				t.Fatalf("dry-run rewrote authoritative state: %q, %v", afterPlan, err)
			}
			if _, _, err := fixture.execute(false, "migrate-state", "--format", "json"); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(fixture.store.Path)
			if err != nil || !strings.Contains(string(body), fmt.Sprintf(`"schema_version": %d`, domain.StateSchemaVersion)) {
				t.Fatalf("authoritative state was not fixed forward: %s, %v", body, err)
			}
			backups, err := filepath.Glob(fixture.store.Path + fmt.Sprintf(".schema%d.backup-agentplugins-*", schema))
			if err != nil || len(backups) != 1 {
				t.Fatalf("authoritative backup = %v, %v", backups, err)
			}
			backup, err := os.ReadFile(backups[0])
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatalf("backup does not preserve reviewed bytes: %q, %v", backup, err)
			}
		})
	}
}

func TestReadOnlyCommandsLoadLegacyV2WithoutPersistingMigration(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	installationID := "00000000-0000-4000-8000-000000000001"
	target := filepath.Join(fixture.root, "managed", "demo")
	clientBindingID := domain.ComputeClientBindingID(installationID, "cursor", "user", target)
	legacy := domain.StateFileV2{SchemaVersion: domain.LegacyStateSchemaVersion, Installations: []domain.Installation{{
		InstallationID: installationID, DeclaredName: "demo",
		Source:  domain.SourceBinding{SourceBindingID: "src_demo", RequestedSource: "demo", CanonicalSource: "https://example.test/demo", ResolvedRevision: "abc123", TreeDigest: "sha256:tree"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "demo", ManifestDigest: "sha256:manifest"},
		Clients: map[string]domain.ClientBinding{clientBindingID: {
			ClientBindingID: clientBindingID, ClientID: "cursor", Scope: "user", TargetLocator: target,
			PhysicalArtifact: domain.ComputePhysicalArtifactID("demo", installationID), Materialization: domain.MaterializationMaterialized,
			Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotRequired,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid,
		}},
	}}}
	body, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(fixture.store.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.store.Path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "list", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "doctor", "demo", "--format", "json"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, body) {
		t.Fatalf("read-only commands persisted the v2-to-v3 migration:\n%s", after)
	}
}

func TestLegacyRemovalRequiresExplicitAllTargetAndReconcilesV2(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, nil)
	legacy := &cliLegacyLifecycleStub{exists: true}
	fixture.app.LegacyLifecycle = legacy
	installationID := "00000000-0000-4000-8000-000000000001"
	clientID := domain.ComputeClientBindingID(installationID, "codex", "user", "/legacy/plugin")
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{
		InstallationID: installationID, DeclaredName: "legacy-demo",
		Source:  domain.SourceBinding{SourceBindingID: "src_legacy", RequestedSource: "legacy-demo", CanonicalSource: "legacy-demo"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindLegacy, FormatID: domain.FormatIDLegacyV1, SchemaURI: "plugin.yaml/v1", DeclaredName: "legacy-demo"},
		Clients: map[string]domain.ClientBinding{clientID: {
			ClientBindingID: clientID, ClientID: "codex", Scope: "user", TargetLocator: "/legacy/plugin",
			PhysicalArtifact: domain.ComputePhysicalArtifactID("legacy-demo", installationID), Materialization: domain.MaterializationMaterialized,
			Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotRequired,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationNotRun,
		}},
	}}}
	if err := fixture.store.Save(state); err != nil {
		t.Fatal(err)
	}
	if _, _, err := fixture.execute(false, "remove", "legacy-demo"); err == nil || !strings.Contains(err.Error(), "legacy-all") {
		t.Fatalf("unsafe legacy non-TTY removal error = %v", err)
	}
	stdout, _, err := fixture.execute(false, "remove", "legacy-demo", "--target", "legacy-all", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "remove")
	updated, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range updated.Installations[0].Clients {
		if client.Materialization != domain.MaterializationAbsent {
			t.Fatalf("legacy client was not reconciled: %+v", client)
		}
	}
}

type cliFixture struct {
	root       string
	app        App
	store      statev2.Store
	operations string
}

type alwaysErrorWriter struct{}

func (alwaysErrorWriter) Write([]byte) (int, error) {
	return 0, errors.New("synthetic output failure")
}

type countingSourceAcquirer struct {
	delegate       SourceAcquirer
	calls          int
	discoveryCalls int
}

func (a *countingSourceAcquirer) AcquireLocal(ctx context.Context, path string) (domain.PackageSnapshot, error) {
	a.calls++
	return a.delegate.AcquireLocal(ctx, path)
}
func (a *countingSourceAcquirer) DiscoverGitHubPackages(ctx context.Context, repo, revision string) ([]string, error) {
	a.discoveryCalls++
	return a.delegate.DiscoverGitHubPackages(ctx, repo, revision)
}
func (a *countingSourceAcquirer) AcquireGitHub(ctx context.Context, repo, revision, path string) (domain.PackageSnapshot, error) {
	a.calls++
	return a.delegate.AcquireGitHub(ctx, repo, revision, path)
}
func (a *countingSourceAcquirer) AcquireGitHubVerified(ctx context.Context, repo, revision, path, digest string) (domain.PackageSnapshot, error) {
	a.calls++
	return a.delegate.AcquireGitHubVerified(ctx, repo, revision, path, digest)
}

type selectiveNativeObserver struct{ foreign domain.ClientID }

func (observer selectiveNativeObserver) ObserveNativeIdentity(_ context.Context, client domain.DetectedClient, _ domain.DeliveryPlan, _ *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if client.ClientID == observer.foreign {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
}

type fixtureNativeObserver struct{}

func (fixtureNativeObserver) ObserveNativeIdentity(_ context.Context, _ domain.DetectedClient, _ domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if managed == nil {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
	}
	for _, object := range managed.NativeObjects {
		if object.Kind == "managed_package_directory" && object.ManagedDigest != "" {
			return domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: object.ManagedDigest}, nil
		}
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate}, nil
}

func newCLIFixture(t *testing.T, clients []domain.DetectedClient) cliFixture {
	t.Helper()
	root := t.TempDir()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	store := statev2.Store{Path: filepath.Join(root, "data", "state-v2.json")}
	operations := filepath.Join(root, "data", "operations-v2")
	packageLoader := loader.Loader{Registry: registry}
	managedRoot := filepath.Join(root, "data", "managed")
	stager := providers.Stager{}
	planner := clientplanner.Planner{ManagedRoot: managedRoot, Detected: map[domain.ClientID]domain.DetectedClient{}}
	mutationLock := processlock.Lock{Path: filepath.Join(root, "data", "mutation.lock")}
	directory := dirswap.Manager{JournalDir: operations}
	lifecycle := usecase.Service{StateStore: store, Planner: planner, Targets: planner, Stager: stager, Activator: providers.Activator{}, Lock: mutationLock,
		Kernel: transaction.Kernel{StateStore: store, Directory: directory}, NativeObserver: fixtureNativeObserver{}, PluginData: providers.PluginDataManager{Base: filepath.Join(root, "data", "plugin-data")}}
	return cliFixture{
		root: root, store: store, operations: operations,
		app: App{
			Version: "0.1.0", UserHome: filepath.Join(root, "home"),
			ManagedRoot: managedRoot, StateStore: store, Detector: staticDetector{clients: clients},
			SourceAcquirer: sourceacquisition.Acquirer{TempRoot: root},
			PackageLoader:  packageLoader, NativePackageLoader: loader.OpenAILoader{Loader: packageLoader},
			Lifecycle:       lifecycle,
			LegacyStateLock: locks.FileLock{BaseDir: filepath.Join(root, "legacy-locks")},
		},
	}
}

func (fixture cliFixture) execute(terminal bool, args ...string) (string, string, error) {
	return fixture.executeInput(terminal, "", args...)
}

func (fixture cliFixture) executeInput(terminal bool, input string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	app := fixture.app
	app.Input = strings.NewReader(input)
	app.Output = &stdout
	app.ErrorOutput = &stderr
	app.Terminal = terminal
	command := NewRoot(app)
	command.SetArgs(args)
	err := command.ExecuteContext(context.Background())
	return stdout.String(), stderr.String(), err
}

type staticDetector struct {
	clients []domain.DetectedClient
}

type observedProbingDetector struct {
	readOnlyCalls int
	probeCalls    int
	targetedCalls int
	clients       []domain.DetectedClient
	targets       []domain.ClientID
}

func (detector *observedProbingDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	detector.readOnlyCalls++
	return append([]domain.DetectedClient(nil), detector.clients...), nil
}

func (detector *observedProbingDetector) DetectWithVersionProbe(context.Context) ([]domain.DetectedClient, error) {
	detector.probeCalls++
	return append([]domain.DetectedClient(nil), detector.clients...), nil
}

func (detector *observedProbingDetector) DetectTargetsWithVersionProbe(_ context.Context, targets []domain.ClientID) ([]domain.DetectedClient, error) {
	detector.targetedCalls++
	detector.targets = append([]domain.ClientID(nil), targets...)
	return append([]domain.DetectedClient(nil), detector.clients...), nil
}

type failingVerifyStager struct {
	err error
}

func (failingVerifyStager) Stage(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints) (domain.StagedDelivery, error) {
	return domain.StagedDelivery{}, errors.New("unexpected stage")
}

func (failingVerifyStager) Discard(context.Context, domain.StagedDelivery) error {
	return errors.New("unexpected discard")
}

func (stager failingVerifyStager) Verify(context.Context, string, string) error {
	return stager.err
}

type cliLegacyLifecycleStub struct {
	exists bool
}

type cliObservedActivator struct {
	outcome domain.ActivationOutcome
	err     error
	calls   int
}

type failSecondCLIGroupActivator struct {
	calls int
}

func (activator *failSecondCLIGroupActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	outcome := domain.ActivationOutcome{
		Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired,
		Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled,
	}
	if activator.calls == 2 {
		outcome.Activation = domain.ActivationFailed
		outcome.Verification = domain.VerificationFailed
		return outcome, errors.New("injected grouped activation failure")
	}
	return outcome, nil
}

func (*failSecondCLIGroupActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, errors.New("unexpected deactivation")
}

func (activator *cliObservedActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	activator.calls++
	return activator.outcome, activator.err
}

func (*cliObservedActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return domain.DeactivationOutcome{}, errors.New("unexpected deactivation")
}

type cliCommandRunner struct {
	calls int
	run   func(int, legacyports.Command) legacyports.CommandResult
}

type cliRunOnlyRunner struct {
	calls int
}

type cliDiscardWriteCloser struct{ io.Writer }

func (cliDiscardWriteCloser) Close() error { return nil }

func (*cliCommandRunner) DuplexCapability() error { return nil }

func (runner *cliRunOnlyRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	return legacyports.CommandResult{}, nil
}

func (runner *cliCommandRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	if runner.run != nil {
		return runner.run(runner.calls, command), nil
	}
	return legacyports.CommandResult{}, nil
}

func (runner *cliCommandRunner) RunDuplexWithPlannedShutdown(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.calls++
	output := "{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{\"protocolVersion\":1}}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"sessionId\":\"s\"}}\n" +
		"{\"jsonrpc\":\"2.0\",\"method\":\"_kiro/mcp/status\",\"params\":{\"sessionId\":\"s\",\"serverName\":\"demo\",\"status\":\"connected\",\"tools\":[{\"name\":\"tool\",\"disabled\":false}]}}\n"
	reader, writer := io.Pipe()
	go func() { _, _ = io.WriteString(writer, output) }()
	err := exchange(cliDiscardWriteCloser{Writer: io.Discard}, reader)
	_ = writer.Close()
	_ = reader.Close()
	return err
}

func (stub *cliLegacyLifecycleStub) Exists(context.Context, string) (bool, error) {
	return stub.exists, nil
}

func (stub *cliLegacyLifecycleStub) PlanRemove(context.Context, string) (ports.LegacyRemovalPlan, error) {
	return ports.LegacyRemovalPlan{Summary: "remove", TargetIDs: []string{"codex"}}, nil
}

func (stub *cliLegacyLifecycleStub) Remove(context.Context, string) (ports.LegacyRemovalPlan, error) {
	stub.exists = false
	return ports.LegacyRemovalPlan{Summary: "removed", TargetIDs: []string{"codex"}}, nil
}

func (detector staticDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return append([]domain.DetectedClient(nil), detector.clients...), nil
}

func fixtureClient(t *testing.T, client domain.ClientID) domain.DetectedClient {
	t.Helper()
	return domain.DetectedClient{
		ClientID: client, DisplayName: string(client), Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(t.TempDir(), "home", "."+string(client)),
	}
}

func writeCLIPlugin(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	body := `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "demo",
  "version": "1.0.0",
  "description": "Demo plugin"
}`
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeCLIMCP(t *testing.T, root string) {
	t.Helper()
	body := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeCLIOfficialAppPlugin(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	manifestDirectory := filepath.Join(root, ".codex-plugin")
	if err := os.MkdirAll(manifestDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"demo","version":"1.0.0","apps":"./.app.json","mcpServers":"./.mcp.json"}`
	if err := os.WriteFile(filepath.Join(manifestDirectory, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	mcp := `{"mcpServers":{"demo":{"type":"streamable-http","url":"https://example.test/mcp"}}}`
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(mcp), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"apps":{"demo":{"id":"asdk_app_demo_123","required":true}}}`
	if err := os.WriteFile(filepath.Join(root, ".app.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readCLIObject(t *testing.T, path string) map[string]any {
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

func assertVersionedJSON(t *testing.T, body, command string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("decode JSON output %q: %v", body, err)
	}
	if value["schema_version"] != float64(1) || value["command"] != command || value["result"] != "success" {
		t.Fatalf("JSON envelope = %+v", value)
	}
}

func assertBatchJSON(t *testing.T, body, command string, succeeded, failed int) {
	t.Helper()
	var value struct {
		SchemaVersion int    `json:"schema_version"`
		Command       string `json:"command"`
		Data          struct {
			Batch     bool                `json:"batch"`
			Succeeded int                 `json:"succeeded"`
			Failed    int                 `json:"failed"`
			Targets   []batchTargetResult `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("decode batch JSON output %q: %v", body, err)
	}
	if value.SchemaVersion != outputSchemaVersion || value.Command != command || !value.Data.Batch || value.Data.Succeeded != succeeded || value.Data.Failed != failed || len(value.Data.Targets) != succeeded+failed {
		t.Fatalf("batch JSON envelope = %+v", value)
	}
}

func assertClientBindings(t *testing.T, fixture cliFixture, materialization domain.MaterializationState, receipts int) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("state = %+v", state)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.Materialization != materialization || len(binding.Receipts) != receipts {
			t.Fatalf("binding = %+v", binding)
		}
	}
}

func onlyCLIClient(installation domain.Installation) domain.ClientBinding {
	for _, binding := range installation.Clients {
		return binding
	}
	return domain.ClientBinding{}
}
