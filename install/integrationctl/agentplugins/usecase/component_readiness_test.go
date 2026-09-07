package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

func TestSelectedReadinessIsolatesMissingAndUnsupportedStdio(t *testing.T) {
	root := t.TempDir()
	envelope := domain.PackageEnvelope{SnapshotRoot: root, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"missing":     {Type: "stdio", Decoded: map[string]any{"command": "./absent"}},
		"unsupported": {Type: "stdio", Decoded: map[string]any{"env": map[string]any{"PLUGIN_ROOT": "bad"}}},
		"remote":      {Type: "http"},
	}}}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "skill", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "missing", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "unsupported", Support: domain.SupportUnsupported},
		{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportNative},
	}}
	if err := (Service{}).preflightComponents(envelope, &plan, false); err != nil {
		t.Fatal(err)
	}
	if got := domain.SelectedMCPNames(plan); !reflect.DeepEqual(got, []string{"remote"}) {
		t.Fatal(got)
	}
	if len(plan.Diagnostics) != 1 || plan.Diagnostics[0].Item != "missing" || packageNeedsPluginData(envelope, plan) {
		t.Fatalf("incorrect isolation: %+v", plan)
	}
	if envelope.MCP.Servers["missing"].Type != "stdio" {
		t.Fatal("envelope changed")
	}
}

func TestMixedAddAndGroupKeepHealthySiblings(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(map[bool]string{false: "add", true: "group"}[grouped], func(t *testing.T) {
			service, _, client := serviceFixture(t)
			input := clinePackageInput(t, client, "1.0.0", "sha256:mixed", "sha256:mixed-manifest", "agentplugins-definitely-unavailable-runtime")
			input.Confirmed = true
			var result AddResult
			var err error
			if grouped {
				var g GroupResult
				g, err = service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, Confirmed: true})
				if len(g.Targets) > 0 {
					result = g.Targets[0]
				}
			} else {
				result, err = service.Add(context.Background(), input)
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Plan.Diagnostics) != 1 || len(domain.SelectedMCPNames(result.Plan)) != 0 {
				t.Fatalf("plan: %+v", result.Plan)
			}
			if _, err := os.Stat(filepath.Join(result.Plan.ActivePath, "skills", "docs", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(result.Plan.ActivePath, "mcp.json")); !os.IsNotExist(err) {
				t.Fatalf("unselected MCP survived: %v", err)
			}
		})
	}
}

func TestTransientUpdateAndRepairRetainInstalledBytes(t *testing.T) {
	service, store, client := serviceFixture(t)
	input := clinePackageInput(t, client, "1.0.0", "sha256:good", "sha256:good-manifest", "sh")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	beforeState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	beforeJSON, _ := json.Marshal(beforeState)
	before, err := os.ReadFile(filepath.Join(installed.Plan.ActivePath, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, repair := range []bool{false, true} {
		candidate := input
		candidate.InstallationID = installed.InstallationID
		candidate.Envelope.MCP.Servers = map[string]domain.MCPServer{"docs": {Type: "stdio", Decoded: map[string]any{"command": "agentplugins-definitely-unavailable-runtime"}}}
		var result AddResult
		if repair {
			result, err = service.Repair(context.Background(), candidate)
		} else {
			result, err = service.Update(context.Background(), candidate)
		}
		if err == nil || result.Mutated || len(result.Plan.Diagnostics) != 1 {
			t.Fatalf("refusal repair=%v result=%+v err=%v", repair, result, err)
		}
		afterState, _ := store.Load()
		afterJSON, _ := json.Marshal(afterState)
		after, _ := os.ReadFile(filepath.Join(installed.Plan.ActivePath, "mcp.json"))
		if string(beforeJSON) != string(afterJSON) || string(before) != string(after) {
			t.Fatal("refusal changed installation")
		}
	}
}

func TestDataCWDReadinessDoesNotCreateMissingSuffix(t *testing.T) {
	for _, cwd := range []string{"${PLUGIN_DATA}", "${PLUGIN_DATA}/missing"} {
		t.Run(cwd, func(t *testing.T) {
			service, _, client := serviceFixture(t)
			input := clinePackageInput(t, client, "1.0.0", "sha256:data", "sha256:data-manifest", "sh")
			server := input.Envelope.MCP.Servers["docs"]
			server.Decoded["cwd"] = cwd
			input.Envelope.MCP.Servers["docs"] = server
			input.Confirmed = true
			result, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if cwd == "${PLUGIN_DATA}" {
				want = 1
			}
			if len(domain.SelectedMCPNames(result.Plan)) != want {
				t.Fatalf("plan: %+v", result.Plan)
			}
		})
	}
}

func TestReadinessUsesAuthoredAbsolutePATH(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "unique-runtime"), []byte("fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	if failure := stdioReadiness(root, map[string]any{"command": "unique-runtime", "env": map[string]any{"PATH": "${PLUGIN_ROOT}"}}); failure != nil {
		t.Fatal(failure)
	}
	if failure := stdioReadiness(root, map[string]any{"command": "sh", "env": map[string]any{"PATH": root}}); failure == nil {
		t.Fatal("host PATH used despite authored override")
	}
}

func TestHelperUpgradeCannotMasqueradeAsExactRepair(t *testing.T) {
	for _, clientID := range []domain.ClientID{domain.ClientWindsurf, domain.ClientClaude} {
		t.Run(string(clientID), func(t *testing.T) {
			if !managedstdio.Supported() {
				t.Skip("native launcher unsupported")
			}
			service, store, _ := serviceFixture(t)
			source := func(body string) *managedstdio.Source {
				path := filepath.Join(t.TempDir(), "helper")
				if err := os.WriteFile(path, []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
				s, err := managedstdio.NewSource(path, body)
				if err != nil {
					t.Fatal(err)
				}
				return s
			}
			service.Stager = providers.Stager{LauncherSource: source("helper A fixture; never executed")}
			client := domain.DetectedClient{ClientID: clientID, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "windsurf")}
			if clientID == domain.ClientClaude {
				client.ExecutablePath = "/test/bin/claude"
				runner := &fakeClaudeLifecycleRunner{configRoot: client.ConfigRoot}
				service.Activator = providers.Activator{Runner: runner}
				service.NativeObserver = providers.NativeIdentityObserver{Runner: runner, Stager: service.Stager}
			}
			input := clinePackageInput(t, client, "1.0.0", "sha256:helper", "sha256:helper-manifest", "sh")
			input.Confirmed = true
			input.BackendExecutable = client.ExecutablePath
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			input.InstallationID = installed.InstallationID
			// A damaged artifact demands rematerialization; rebuilding with B must never
			// replace the receipt for A under the same exact-repair request.
			marker := filepath.Join(installed.Plan.ActivePath, "damage.txt")
			if err := os.WriteFile(marker, []byte("prior damage"), 0600); err != nil {
				t.Fatal(err)
			}
			stateBefore, _ := store.Load()
			before, _ := json.Marshal(stateBefore)
			service.Stager = providers.Stager{LauncherSource: source("helper B fixture; never executed")}
			result, err := service.Repair(context.Background(), input)
			if err == nil || !strings.Contains(err.Error(), "projection digest differs") || result.Mutated {
				t.Fatalf("repair=%+v err=%v", result, err)
			}
			stateAfter, _ := store.Load()
			after, _ := json.Marshal(stateAfter)
			if string(before) != string(after) {
				t.Fatal("exact repair changed state")
			}
			if body, err := os.ReadFile(marker); err != nil || string(body) != "prior damage" {
				t.Fatal("prior artifact changed")
			}
			// A controlled update can replace A with B; subsequent exact repair must use B.
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			oldBinding := onlyBinding(stateBefore.Installations[0])
			oldData := stateBefore.Installations[0].DataReceipts[oldBinding.DataReceiptID]
			dataMarker := filepath.Join(oldData.Locator, "continuity")
			if err := os.WriteFile(dataMarker, []byte("persistent"), 0600); err != nil {
				t.Fatal(err)
			}
			setEnvelopeVersion(t, &input.Envelope, "2.0.0", "sha256:helper-b", "sha256:helper-b-manifest")
			input.OperationID = "helper-b-update"
			updated, err := service.Update(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if !updated.Mutated {
				t.Fatal("B update did not commit")
			}
			stateB, _ := store.Load()
			bindingB := onlyBinding(stateB.Installations[0])
			if managedDigest(bindingB) == managedDigest(onlyBinding(stateBefore.Installations[0])) {
				t.Fatal("helper bytes did not change managed digest")
			}
			if stateB.Installations[0].DataReceipts[bindingB.DataReceiptID].Locator != oldData.Locator {
				t.Fatal("update changed data ownership")
			}
			if body, err := os.ReadFile(dataMarker); err != nil || string(body) != "persistent" {
				t.Fatal("update lost persistent data")
			}
			if err := os.WriteFile(marker, []byte("new damage"), 0600); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "helper-b-repair"
			repaired, err := service.Repair(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if !repaired.Mutated {
				t.Fatal("B exact repair did not commit")
			}
			stateRepaired, _ := store.Load()
			if managedDigest(onlyBinding(stateRepaired.Installations[0])) != managedDigest(bindingB) {
				t.Fatal("exact B repair changed receipt digest")
			}
			if body, err := os.ReadFile(dataMarker); err != nil || string(body) != "persistent" {
				t.Fatal("persistent data lost")
			}

			if clientID == domain.ClientClaude {
				return
			}
			nativePath := filepath.Join(client.ConfigRoot, "mcp_config.json")
			if err := os.Remove(nativePath); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "helper-b-native-repair"
			nativeRepaired, err := service.Repair(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if !nativeRepaired.Mutated {
				t.Fatal("B absent native config repair did not commit")
			}
			if _, err := os.Stat(nativePath); err != nil {
				t.Fatal("native config not restored")
			}
			if body, err := os.ReadFile(dataMarker); err != nil || string(body) != "persistent" {
				t.Fatal("native repair lost persistent data")
			}

		})
	}
}

func TestStaticUnsupportedUpdateUsesConfirmedRemoval(t *testing.T) {
	service, _, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
	input := openCodePluginInput(t, client, "1.0.0", "sha256:old", "sha256:old-manifest", "sh")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	candidate := openCodePluginInput(t, client, "2.0.0", "sha256:new", "sha256:new-manifest", "sh")
	candidate.InstallationID = installed.InstallationID
	candidate.OperationID = "static-update"
	candidate.Envelope.MCP.Servers["docs"] = domain.MCPServer{Type: "sse", Decoded: map[string]any{"type": "sse", "url": "https://example.invalid/events"}}
	preview, err := service.Update(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.RequiresConfirmation || preview.Mutated || len(domain.SelectedMCPNames(preview.Plan)) != 0 {
		t.Fatalf("preview=%+v", preview)
	}
	configPath := filepath.Join(client.ConfigRoot, "opencode.json")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "docs") {
		t.Fatal("existing entry missing before confirmation")
	}
	candidate.Confirmed = true
	updated, err := service.Update(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Mutated {
		t.Fatal("update did not commit")
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "docs") {
		t.Fatalf("unsupported native entry retained: %s", after)
	}
}

func TestClineHistoricalProjectionDigestCannotBeSilentlyRepaired(t *testing.T) {
	t.Setenv("CLINE_MCP_SETTINGS_PATH", filepath.Join(t.TempDir(), "cline-settings.json"))
	service, store, _ := serviceFixture(t)
	client := domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "cline")}
	input := clinePackageInput(t, client, "1.0.0", "sha256:cline-history", "sha256:cline-manifest", "sh")
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	projection := filepath.Join(installed.Plan.ActivePath, ".agentplugins-cline-native.json")
	body, err := os.ReadFile(projection)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	for _, server := range document["servers"].(map[string]any) {
		delete(server.(map[string]any), "cwd")
	}
	legacy, _ := json.Marshal(document)
	if err := os.WriteFile(projection, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	var verification *ports.VerificationError
	if err := service.Stager.Verify(context.Background(), installed.Plan.ActivePath, "historical-digest"); !errors.As(err, &verification) || verification.ActualDigest == "" {
		t.Fatalf("digest observation: %v", err)
	}
	state, _ := store.Load()
	for key, binding := range state.Installations[0].Clients {
		for i := range binding.NativeObjects {
			if binding.NativeObjects[i].Kind == "managed_package_directory" {
				binding.NativeObjects[i].ManagedDigest = verification.ActualDigest
			}
		}
		state.Installations[0].Clients[key] = binding
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	// Force exact rematerialization from the unchanged source revision.
	if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "damage"), []byte("damage"), 0600); err != nil {
		t.Fatal(err)
	}
	beforeState, _ := json.Marshal(state)
	nativeBefore, _ := os.ReadFile(os.Getenv("CLINE_MCP_SETTINGS_PATH"))
	input.InstallationID = installed.InstallationID
	input.OperationID = "historical-cline-repair"
	result, err := service.Repair(context.Background(), input)
	if err == nil || !strings.Contains(err.Error(), "projection digest differs") || result.Mutated {
		t.Fatalf("repair=%+v err=%v", result, err)
	}
	afterState, _ := store.Load()
	afterJSON, _ := json.Marshal(afterState)
	nativeAfter, _ := os.ReadFile(os.Getenv("CLINE_MCP_SETTINGS_PATH"))
	if string(beforeState) != string(afterJSON) || string(nativeBefore) != string(nativeAfter) {
		t.Fatal("historical repair changed state or native config")
	}
}

func TestMissingHelperOnlyExcludesManagedStdio(t *testing.T) {
	for _, clientID := range []domain.ClientID{domain.ClientWindsurf, domain.ClientClaude} {
		t.Run(string(clientID), func(t *testing.T) {
			service, _, _ := serviceFixture(t)
			client := domain.DetectedClient{ClientID: clientID, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "windsurf")}
			if clientID == domain.ClientClaude {
				client.ExecutablePath = "/test/bin/claude"
				runner := &fakeClaudeLifecycleRunner{configRoot: client.ConfigRoot}
				service.Activator = providers.Activator{Runner: runner}
				service.NativeObserver = providers.NativeIdentityObserver{Runner: runner, Stager: service.Stager}
			}
			input := clinePackageInput(t, client, "1.0.0", "sha256:missing-helper", "sha256:helper-manifest", "sh")
			input.Confirmed = true
			input.BackendExecutable = client.ExecutablePath
			input.Envelope.MCP.Servers["remote"] = domain.MCPServer{Type: "streamable-http", Decoded: map[string]any{"type": "streamable-http", "url": "https://example.invalid/mcp"}, Raw: json.RawMessage(`{"type":"streamable-http","url":"https://example.invalid/mcp"}`)}
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if got := domain.SelectedMCPNames(installed.Plan); !reflect.DeepEqual(got, []string{"remote"}) {
				t.Fatal(got)
			}
			if packageNeedsPluginData(input.Envelope, installed.Plan) {
				t.Fatal("unselected stdio demanded data")
			}

		})
	}
}

func TestFreshTargetCanSelectSubsetOfExistingInstallation(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprint(grouped), func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := clinePackageInput(t, client, "1.0.0", "sha256:new-target", "sha256:new-target-manifest", "sh")
			input.Confirmed = true
			if _, err := service.Add(context.Background(), input); err != nil {
				t.Fatal(err)
			}
			before, _ := store.Load()
			old := onlyBinding(before.Installations[0])
			input.Client = domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "windsurf")}
			input.OperationID = "fresh-target"
			var result AddResult
			var err error
			if grouped {
				var group GroupResult
				group, err = service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: "fresh-target-group"})
				if len(group.Targets) > 0 {
					result = group.Targets[0]
				}
			} else {
				result, err = service.Add(context.Background(), input)
			}
			if err != nil || !result.Mutated || len(domain.SelectedMCPNames(result.Plan)) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			after, _ := store.Load()
			if !reflect.DeepEqual(after.Installations[0].Clients[old.ClientBindingID], old) {
				t.Fatal("existing target changed")
			}
		})
	}
}

func TestUnexpectedReadinessIOIsNotAComponentSkip(t *testing.T) {
	for _, err := range []error{&os.PathError{Op: "stat", Path: "test", Err: syscall.EIO}, &os.PathError{Op: "stat", Path: "test", Err: os.ErrPermission}} {
		if !unexpectedPathIO(err) {
			t.Fatalf("I/O was masked: %v", err)
		}
	}
	for _, err := range []error{&os.PathError{Op: "stat", Path: "test", Err: os.ErrNotExist}, &os.PathError{Op: "stat", Path: "test", Err: syscall.ELOOP}} {
		if unexpectedPathIO(err) {
			t.Fatalf("expected readiness became fatal: %v", err)
		}
	}
}

func TestClaudeMissingHelperRetainsPreviouslySelectedStdio(t *testing.T) {
	service, _, _ := serviceFixture(t)
	envelope := domain.PackageEnvelope{SnapshotRoot: t.TempDir(), MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"local":  {Type: "stdio", Decoded: map[string]any{"command": "sh"}},
		"remote": {Type: "streamable-http"},
	}}}
	plan := domain.DeliveryPlan{ClientID: domain.ClientClaude, Components: []domain.ComponentDecision{
		{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportProjected},
		{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportProjected},
	}}
	if err := service.preflightComponents(envelope, &plan, true, map[string]bool{"local": true, "remote": true}); err == nil || !strings.Contains(err.Error(), "retaining the entire installed package") {
		t.Fatalf("previous selection silently withdrawn: %v", err)
	}
}
