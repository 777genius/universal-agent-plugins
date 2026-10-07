package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/hostprep"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// These tests inject only the trusted probe port. Real planners, namespace and
// identity readers, stagers, activation and filesystem/state transactions run.
// Injected evidence is source-contract evidence, never native qualification.
type hostBoundaryTrace struct {
	t                 *testing.T
	host              domain.OpenCodeHostAuthority
	staged, activated int
}

func (trace *hostBoundaryTrace) check(client domain.DetectedClient, plan domain.DeliveryPlan) {
	trace.t.Helper()
	if client.ClientID != domain.ClientOpenCode {
		return
	}
	if trace.host == nil || client.OpenCodeHost != trace.host || plan.OpenCodeHost != trace.host {
		trace.t.Fatal("authority changed across boundary")
	}
}

type hostBoundaryPlanner struct {
	ports.DeliveryPlanner
	trace *hostBoundaryTrace
}

func (p hostBoundaryPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	if request.Client.ClientID == domain.ClientOpenCode {
		if request.Client.OpenCodeHost == nil || request.Detected[domain.ClientOpenCode].OpenCodeHost != request.Client.OpenCodeHost {
			p.trace.t.Fatal("planner/detected lacks prepared snapshot")
		}
		if p.trace.host != nil && p.trace.host != request.Client.OpenCodeHost {
			p.trace.t.Fatal("preview/apply replaced snapshot")
		}
		p.trace.host = request.Client.OpenCodeHost
	}
	plan, err := p.DeliveryPlanner.Plan(ctx, request)
	if err == nil {
		p.trace.check(request.Client, plan)
	}
	return plan, err
}

type hostBoundaryStager struct {
	ports.PackageStager
	trace *hostBoundaryTrace
}

func (s hostBoundaryStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, id string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if plan.ClientID == domain.ClientOpenCode && plan.OpenCodeHost != s.trace.host {
		s.trace.t.Fatal("stager lost authority")
	}
	s.trace.staged++
	return s.PackageStager.Stage(ctx, envelope, plan, id, hints)
}

type hostBoundaryActivator struct {
	ports.ClientActivator
	trace *hostBoundaryTrace
}

func (a hostBoundaryActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.trace.check(request.Client, request.Plan)
	a.trace.activated++
	return a.ClientActivator.Activate(ctx, request)
}
func (a hostBoundaryActivator) PreflightActivation(request domain.ActivationRequest) error {
	a.trace.check(request.Client, request.Plan)
	if guard, ok := a.ClientActivator.(ports.ActivationPreflighter); ok {
		return guard.PreflightActivation(request)
	}
	return nil
}
func hostBridgeInput(t *testing.T, client domain.DetectedClient) AddInput {
	t.Helper()
	input := openCodePluginInput(t, client, "1.0.0", "sha256:bridge", "sha256:bridge-manifest", "unused")
	body := []byte(`{"mcpServers":{"docs":{"type":"streamable-http","url":"https://docs.test/mcp"}}}`)
	input.Envelope.MCP.Raw = body
	input.Envelope.MCP.Servers = map[string]domain.MCPServer{"docs": {Name: "docs", Type: "streamable-http", Decoded: map[string]any{"url": "https://docs.test/mcp"}}}
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	input.Client.OpenCodeHost = nil
	input.Confirmed = true
	return input
}
func attachHostBridge(t *testing.T, service *Service, probe hostprep.Probe) *hostBoundaryTrace {
	t.Helper()
	preparer, err := hostprep.New(clientregistry.Default(), probe, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	service.OpenCodeHosts = preparer
	service.Detected = map[domain.ClientID]domain.DetectedClient{}
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	trace := &hostBoundaryTrace{t: t}
	service.Planner = hostBoundaryPlanner{service.Planner, trace}
	service.Stager = hostBoundaryStager{service.Stager, trace}
	service.Activator = hostBoundaryActivator{service.Activator, trace}
	return trace
}
func bridgeEvidence(identity string) clientdetect.ProbeEvidence {
	return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: identity}}
}

// RED at CF5: production service cannot qualify a desired target, even though
// the trusted explicit-target probe is available. The same authority must reach
// real projection/activation and the actual on-disk skill and MCP receipts.
func TestOpenCodeHostBridgeSingleAndGroup(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "group"}[group], func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "selected-host")}
			calls := 0
			trace := attachHostBridge(t, &service, func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				calls++
				if target.Executable != client.ExecutablePath {
					t.Fatal("ambient target")
				}
				return bridgeEvidence("TEST-injected-identity"), nil
			})
			input := hostBridgeInput(t, client)
			if group {
				cursorInput := input
				cursorInput.Client = cursor
				service.PrepareHostsForPreview = true
				if _, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{cursorInput, input}, DryRun: true}); err != nil {
					t.Fatal(err)
				}
				if trace.staged != 0 || trace.activated != 0 {
					t.Fatal("online preview changed targets")
				}
				result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{cursorInput, input}, Confirmed: true})
				if err != nil || result.Phase != GroupPhaseCompleted {
					t.Fatalf("group: %+v %v", result, err)
				}
			} else {
				result, err := service.Add(context.Background(), input)
				if err != nil || !result.Mutated {
					t.Fatalf("single: %+v %v", result, err)
				}
			}
			if calls < 2 || trace.staged == 0 || trace.activated == 0 {
				t.Fatalf("boundaries skipped: probes=%d %+v", calls, trace)
			}
			body, err := os.ReadFile(filepath.Join(client.ConfigRoot, "opencode.json"))
			if err != nil {
				t.Fatal(err)
			}
			var config map[string]any
			if err := json.Unmarshal(body, &config); err != nil {
				t.Fatal(err)
			}
			if config["mcp"].(map[string]any)["docs"] == nil {
				t.Fatal("no actual MCP effect")
			}
			if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil || len(state.Installations) != 1 {
				t.Fatalf("state: %+v %v", state, err)
			}
		})
	}
}

// RED if a plan-first pass silently reprobes a changed executable/root as a
// newly acceptable target, or one group target stages before the whole fence.
func TestOpenCodeHostBridgePreviewIdentityChangeRefusesEntireGroup(t *testing.T) {
	for _, change := range []string{"executable", "version", "root"} {
		t.Run(change, func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			root := t.TempDir()
			config := filepath.Join(root, "config")
			if err := os.Mkdir(config, 0700); err != nil {
				t.Fatal(err)
			}
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: config, ExecutablePath: filepath.Join(root, "selected-host")}
			evidence := bridgeEvidence("TEST-before")
			trace := attachHostBridge(t, &service, func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
				return evidence, nil
			})
			service.PrepareHostsForPreview = true
			input := hostBridgeInput(t, client)
			cursorInput := input
			cursorInput.Client = cursor
			group := GroupInput{Targets: []AddInput{cursorInput, input}, DryRun: true}
			if _, err := service.AddGroup(context.Background(), group); err != nil {
				t.Fatal(err)
			}
			if trace.staged != 0 || trace.activated != 0 {
				t.Fatal("preview mutated target")
			}
			switch change {
			case "executable":
				evidence.ExecutableIdentity = "TEST-after"
			case "version":
				evidence.Version = "2.0.21"
			case "root":
				other := filepath.Join(root, "other")
				if err := os.Mkdir(other, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(config, config+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, config); err != nil {
					t.Fatal(err)
				}
			}
			group.DryRun = false
			group.Confirmed = true
			result, err := service.AddGroup(context.Background(), group)
			if !errors.Is(err, hostprep.ErrPlanChanged) || result.Mutated || trace.staged != 0 || trace.activated != 0 {
				t.Fatalf("changed host affected group: %+v trace=%+v %v", result, trace, err)
			}
			state, err := store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("changed host wrote state: %+v %v", state, err)
			}
			if _, err := os.Stat(cursor.ConfigRoot); !os.IsNotExist(err) {
				t.Fatalf("cursor changed: %v", err)
			}
		})
	}
}

// RED if dry-run gains a probe, or empty desired effects skip cleanup of owned
// MCP/skills instead of using stored authority without another host lookup.
func TestOpenCodeHostBridgeOfflineAndStoredCleanup(t *testing.T) {
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "unlaunched")}
	calls := 0
	preparer, err := hostprep.New(clientregistry.Default(), func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		return bridgeEvidence("TEST-injected"), nil
	}, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	service.OpenCodeHosts = preparer
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	input := hostBridgeInput(t, client)
	input.DryRun = true
	if _, err := service.Add(context.Background(), input); err == nil || calls != 0 {
		t.Fatalf("offline missing authority: probes=%d %v", calls, err)
	}
	input.DryRun = false
	result, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	callsBefore := calls
	empty := addInput(t, client, input.Envelope.Source.CanonicalSource)
	setEnvelopeVersion(t, &empty.Envelope, "1.0.1", "sha256:empty", "sha256:empty-manifest")
	empty.Confirmed = true
	empty.OperationID = "operation-empty-update"
	empty.InstallationID = result.InstallationID
	update, err := service.Update(context.Background(), empty)
	if err != nil || !update.Mutated || calls != callsBefore {
		t.Fatalf("stored cleanup probes=%d before=%d: %+v %v", calls, callsBefore, update, err)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("old skill retained: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(client.ConfigRoot, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if config["mcp"].(map[string]any)["docs"] != nil {
		t.Fatal("old MCP retained")
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if binding := onlyBinding(state.Installations[0]); len(binding.NativeObjects) != 1 {
		t.Fatalf("stored native receipts retained: %+v", binding.NativeObjects)
	}
}

// RED if the entire group's explicit-target fence is delayed until staging,
// activation or after another target has written its managed package.
func TestOpenCodeHostBridgeGroupFenceBeforeStage(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "selected")}
	calls := 0
	trace := attachHostBridge(t, &service, func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		identity := "TEST-first"
		if calls > 1 {
			identity = "TEST-changed"
		}
		return bridgeEvidence(identity), nil
	})
	input := hostBridgeInput(t, client)
	other := input
	other.Client = cursor
	result, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{other, input}, Confirmed: true})
	if !errors.Is(err, hostprep.ErrPlanChanged) || result.Mutated || trace.staged != 0 || trace.activated != 0 {
		t.Fatalf("group fence: %+v %+v %v", result, trace, err)
	}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 0 {
		t.Fatalf("group fence wrote state: %+v %v", state, err)
	}
}

// RED if stored native removal starts requiring the current executable's
// existence, version or profile instead of exact recorded ownership receipts.
func TestOpenCodeHostBridgeStoredRemoveNoLookup(t *testing.T) {
	service, _, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "selected")}
	calls := 0
	preparer, err := hostprep.New(clientregistry.Default(), func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		return bridgeEvidence("TEST-stored"), nil
	}, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	service.OpenCodeHosts = preparer
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	installed, err := service.Add(context.Background(), hostBridgeInput(t, client))
	if err != nil {
		t.Fatal(err)
	}
	prior := calls
	client.ExecutablePath = ""
	removed, err := service.Remove(context.Background(), RemoveInput{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true, OperationID: "remove-stored-native"})
	if err != nil || !removed.Mutated || calls != prior {
		t.Fatalf("stored remove probes=%d prior=%d: %+v %v", calls, prior, removed, err)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("stored skill remains: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(client.ConfigRoot, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if config["mcp"].(map[string]any)["docs"] != nil {
		t.Fatal("stored MCP remains")
	}
}

// RED if grouped empty desired delivery bypasses stored reconciliation, or
// tries to acquire a new host/codec to remove the last owned native objects.
func TestOpenCodeHostBridgeGroupStoredCleanup(t *testing.T) {
	service, _, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "selected")}
	calls := 0
	preparer, err := hostprep.New(clientregistry.Default(), func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		return bridgeEvidence("TEST-group-stored"), nil
	}, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	service.OpenCodeHosts = preparer
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	input := hostBridgeInput(t, client)
	installed, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	prior := calls
	empty := addInput(t, client, input.Envelope.Source.CanonicalSource)
	setEnvelopeVersion(t, &empty.Envelope, "1.0.1", "sha256:group-empty", "sha256:group-empty-manifest")
	empty.InstallationID = installed.InstallationID
	update, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{empty}, Confirmed: true})
	if err != nil || !update.Mutated || calls != prior {
		t.Fatalf("group stored cleanup: %+v probes=%d prior=%d %v", update, calls, prior, err)
	}
	if _, err := os.Stat(filepath.Join(client.ConfigRoot, "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("group old skill retained: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(client.ConfigRoot, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if config["mcp"].(map[string]any)["docs"] != nil {
		t.Fatal("group old MCP retained")
	}
}

// RED at CF5's direct service seam: single repair with desired native effects
// cannot recover an absent exact-owned global skill without explicit authority.
func TestOpenCodeHostBridgeSingleRepair(t *testing.T) {
	service, _, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode"), ExecutablePath: filepath.Join(t.TempDir(), "selected")}
	calls := 0
	preparer, err := hostprep.New(clientregistry.Default(), func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		calls++
		return bridgeEvidence("TEST-repair"), nil
	}, []string{"PATH="})
	if err != nil {
		t.Fatal(err)
	}
	service.OpenCodeHosts = preparer
	service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
	input := hostBridgeInput(t, client)
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(client.ConfigRoot, "skills", "docs", "SKILL.md")
	original, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Dir(skill)); err != nil {
		t.Fatal(err)
	}
	prior := calls
	input.InstallationID = installed.InstallationID
	input.OperationID = "repair-bridge"
	repaired, err := service.Repair(context.Background(), input)
	if err != nil || repaired.Plan.OpenCodeHost == nil || calls <= prior {
		t.Fatalf("single repair: %+v probes=%d prior=%d %v", repaired, calls, prior, err)
	}
	body, err := os.ReadFile(skill)
	if err != nil || string(body) != string(original) {
		t.Fatalf("exact owned skill not restored: %s %v", body, err)
	}
}
