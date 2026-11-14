package usecase_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// A real injected process boundary with observation, not canned command results.
// Any accidental CLI fallback executes through the existing runner and is counted.
type localBoundaryRunner struct {
	calls    int
	testHome string
}

func (r *localBoundaryRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	command.Env = []string{"HOME=" + r.testHome, "USERPROFILE=" + r.testHome}
	return (process.OS{}).Run(ctx, command)
}
func localEngineWithRunner(t *testing.T, root string, adapter clients.Adapter, runner *localBoundaryRunner) *installer.Engine {
	t.Helper()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	eng, err := installer.New(installer.Config{StateRoot: filepath.Join(root, "state"), Registry: registry, HelperExecutable: helper, Runner: runner, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return eng
}

// Red: a Local receipt interpreted by the historical adapter can invoke Copilot
// uninstall/list, or a changed tuple/profile can bypass the sealed selection.
// Unknown persisted modes must retain their evidence and refuse the same way.
func TestLocalEngineSelectionConflictsHaveZeroNativeEffects(t *testing.T) {
	root := localProcessRoot(t)
	adapter := &testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}
	eng := localTestEngine(t, filepath.Join(root, "state"), adapter)
	req := installerRequest(root)
	handle, err := eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.Apply(t.Context(), handle, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state", "state-v2.json")
	original, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(root, "profile", "settings.json")
	profileBefore, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"historical-install", "historical-remove", "different-profile", "different-shell", "unknown-record"} {
		t.Run(name, func(t *testing.T) {
			local := &testLocalAdapter{Adapter: vscode.New()}
			var selected clients.Adapter = local
			candidate := req
			expected := "delivery"
			if strings.HasPrefix(name, "historical") {
				selected = vscode.New()
			}
			if name == "historical-remove" {
				candidate.Operation, candidate.InstallationID = installer.OpRemove, result.InstallationID
			}
			if name == "different-profile" {
				candidate.ClientConfigRoot = filepath.Join(root, "other-profile")
			}
			if name == "different-shell" {
				local.targetShell = "TEST-other-qualified-shell"
			}
			before := original
			if name == "unknown-record" {
				before = []byte(strings.Replace(string(original), domain.DeliveryVSCodeLocalV1, "future-mode", 1))
				if err := os.WriteFile(statePath, before, 0600); err != nil {
					t.Fatal(err)
				}
				expected = "unknown"
			}
			runner := &localBoundaryRunner{testHome: filepath.Join(root, "TEST-home")}
			selectedEngine := localEngineWithRunner(t, root, selected, runner)
			prepared, err := selectedEngine.Prepare(t.Context(), candidate)
			if name == "historical-remove" && err == nil {
				_, err = selectedEngine.Apply(t.Context(), prepared, confirmedDecision())
			}
			if prepared != nil {
				defer func() { _ = prepared.Close() }()
			}
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("selection conflict admitted: %v", err)
			}
			after, readErr := os.ReadFile(statePath)
			profileAfter, profileErr := os.ReadFile(profilePath)
			if readErr != nil || profileErr != nil || string(before) != string(after) || string(profileBefore) != string(profileAfter) || runner.calls != 0 {
				t.Fatalf("conflict had native/state effects: calls=%d state=%v profile=%v", runner.calls, readErr, profileErr)
			}
			service := localRecoveryService(t, root)
			state, err := service.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, state)
			facts, _ := localOnlyBinding(t, mustLocalState(t, original)).SelectedDelivery.LocalFacts()
			if err := service.Stager.Verify(t.Context(), binding.TargetLocator, facts.ProjectionDigest); err != nil {
				t.Fatalf("conflict changed installed package: %v", err)
			}
			if err := os.WriteFile(statePath, original, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func mustLocalState(t *testing.T, raw []byte) domain.StateFileV2 {
	t.Helper()
	var state domain.StateFileV2
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

// Red: an explicit refreshed component/tuple choice with identical projected
// bytes can be lost as a no-op, leaving recovery bound to an older selection.
func TestLocalEngineRefreshPersistsEqualByteSelection(t *testing.T) {
	root := localProcessRoot(t)
	originalAdapter := &testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}
	eng := localTestEngine(t, filepath.Join(root, "state"), originalAdapter)
	req := installerRequest(root)
	handle, err := eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.Apply(t.Context(), handle, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	service := localRecoveryService(t, root)
	before, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	oldBinding := localOnlyBinding(t, before)
	stop := false
	refreshed := &testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New(), nativeStop: &stop, targetShell: "TEST-refreshed-qualified-shell"}, root: root}
	eng = localTestEngine(t, filepath.Join(root, "state"), refreshed)
	req.Operation, req.InstallationID = installer.OpRefreshProjection, result.InstallationID
	handle, err = eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Apply(t.Context(), handle, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	after, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := localOnlyBinding(t, after)
	facts, _ := binding.SelectedDelivery.LocalFacts()
	if facts.NativeStop || facts.Tuple.TargetShell != "TEST-refreshed-qualified-shell" || facts.ProjectionDigest == "" || facts.ProjectionDigest != mustFacts(t, oldBinding).ProjectionDigest || len(binding.Receipts) != len(oldBinding.Receipts) {
		t.Fatalf("refresh lost its reviewed equal-byte selection or replaced directory: %+v", binding)
	}
}
func mustFacts(t *testing.T, binding domain.ClientBinding) domain.LocalDeliveryFacts {
	t.Helper()
	facts, ok := binding.SelectedDelivery.LocalFacts()
	if !ok {
		t.Fatal("missing Local facts")
	}
	return facts
}

type testSelectedMCPLocalAdapter struct {
	testEffectLocalAdapter
	selected bool
}

func (a *testSelectedMCPLocalAdapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	plan.NativeRegistryRoot = in.Client.ConfigRoot
	names := []string{}
	for i := range plan.Components {
		c := &plan.Components[i]
		if c.Kind != domain.ComponentMCPServer {
			continue
		}
		c.Support, c.Reason = domain.SupportUnsupported, "TEST-explicitly-unselected"
		if a.selected {
			c.Support, c.Reason = domain.SupportNative, ""
			names = append(names, c.Name)
		}
	}
	enabled := true
	return clients.SelectLocalDelivery(plan, domain.LocalDeliveryFacts{ProfileRoot: in.Client.ConfigRoot, SettingsPath: filepath.Join(in.Client.ConfigRoot, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", TargetShell: "bash", QualificationID: "TEST-process-contract"}, NativeStop: true, MCPServers: names, CanonicalDigest: in.Envelope.TreeDigest, Registration: domain.OwnedProfileEntry{ObjectID: "TEST-profile-entry", Selector: plan.ActivePath, DesiredValue: &enabled}})
}

// Red: later repair preflight overrides a reviewed Local MCP enable, or rejects
// controlled deselection. The public Engine uses the real stager and TEST files;
// the declared remote URL is configuration only and is never contacted.
func TestLocalEngineRefreshMCPSelection(t *testing.T) {
	for _, initiallySelected := range []bool{false, true} {
		name := "enable"
		if initiallySelected {
			name = "disable"
		}
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			source := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notify":{"type":"streamable-http","url":"https://example.invalid/TEST-never-contacted"}}}`)
			if err := os.WriteFile(filepath.Join(root, "package", "mcp.json"), source, 0600); err != nil {
				t.Fatal(err)
			}
			adapter := &testSelectedMCPLocalAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}, selected: initiallySelected}
			eng := localTestEngine(t, filepath.Join(root, "state"), adapter)
			req := installerRequest(root)
			prepared, err := eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			result, err := eng.Apply(t.Context(), prepared, confirmedDecision())
			if err != nil {
				t.Fatal(err)
			}
			service := localRecoveryService(t, root)
			state, err := service.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			previous := localOnlyBinding(t, state)
			adapter.selected = !initiallySelected
			req.Operation, req.InstallationID = installer.OpRepair, result.InstallationID
			if handle, err := eng.Prepare(t.Context(), req); err == nil {
				_ = handle.Close()
				t.Fatal("ordinary Repair accepted changed MCP selection")
			}
			req.Operation = installer.OpRefreshProjection
			prepared, err = eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal("reviewed refresh refused:", err)
			}
			defer func() { _ = prepared.Close() }()
			if initiallySelected && !strings.Contains(strings.Join(prepared.Plan().Delivery.Warnings, ","), "managed_component_removal_required") {
				t.Fatal("deselection omitted controlled removal from review")
			}
			if _, err := eng.Apply(t.Context(), prepared, confirmedDecision()); err != nil {
				t.Fatal(err)
			}
			state, err = service.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, state)
			facts := mustFacts(t, binding)
			if (len(facts.MCPServers) == 1) != adapter.selected || !facts.NativeStop || !binding.SelectedDelivery.SameProfile(previous.SelectedDelivery) || facts.CanonicalDigest != mustFacts(t, previous).CanonicalDigest || binding.PackageRevision.ResolvedRevision != previous.PackageRevision.ResolvedRevision {
				t.Fatal("refresh lost reviewed selection or changed package/profile authority")
			}
			_, err = os.Stat(filepath.Join(binding.TargetLocator, "mcp.json"))
			if (adapter.selected && err != nil) || (!adapter.selected && !os.IsNotExist(err)) {
				t.Fatalf("installed MCP projection disagrees with selection: %v", err)
			}
			if err := service.Stager.Verify(t.Context(), binding.TargetLocator, facts.ProjectionDigest); err != nil {
				t.Fatal("persisted projection digest does not match installed bytes:", err)
			}
		})
	}
}
