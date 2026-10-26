package installer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type qoderSDKNoNative struct{ calls int }

func (r *qoderSDKNoNative) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, fmt.Errorf("forbidden Qoder native command %v", c.Argv)
}

type qoderSDKFixture struct {
	project, user, pkg, pin string
	sequence                int
	engine                  *installer.Engine
	request                 installer.Request
}

func newQoderSDKFixture(t *testing.T, project, user, pkg string, enabled bool, runner *qoderSDKNoNative) *qoderSDKFixture {
	t.Helper()
	registry, err := clients.NewRegistry(qoder.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	config := installer.Config{ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"), Registry: registry, EnableNativeObserver: enabled, TrustedLocalPackages: true, HelperExecutable: helper}
	if enabled {
		config.Runner = runner
	}
	engine, err := installer.New(config)
	if err != nil {
		t.Fatal(err)
	}
	return &qoderSDKFixture{project: project, user: user, pkg: pkg, engine: engine, request: installer.Request{Scope: domain.ScopeProject, ProjectRoot: project, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "qoder", ClientConfigRoot: user, InstallationID: "00000000-0000-4000-8000-000000000078", OperationID: "qoder-project", RequiredComponents: []string{"skills"}}}
}
func (f *qoderSDKFixture) execute(t *testing.T, req installer.Request) (installer.Result, error) {
	t.Helper()
	f.sequence++
	req.OperationID = fmt.Sprintf("qoder-%s-%d", req.Operation, f.sequence)
	handle, err := f.engine.Prepare(t.Context(), req)
	if err != nil {
		return installer.Result{}, err
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	return f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
}
func (f *qoderSDKFixture) apply(t *testing.T, op installer.Operation) installer.Result {
	t.Helper()
	req := f.request
	req.Operation, req.OperationID = op, "qoder-"+string(op)
	if op == installer.OpRepair && f.pin != "" {
		req.PackageRoot = f.pin
	}
	out, err := f.execute(t, req)
	if err != nil || out.Outcome != installer.OutcomeCompleted && out.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("%s: %+v %v", op, out, err)
	}
	return out
}
func (f *qoderSDKFixture) state(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.project, ".uap", "state-v2.json"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
func (f *qoderSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.project, ".uap", "state-v2.json"), ProjectRoot: f.project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state %+v %v", state, err)
	}
	if len(state.Installations[0].DataReceipts) != 0 {
		t.Fatal("unselected MCP created persistent data ownership")
	}
	for _, b := range state.Installations[0].Clients {
		return b
	}
	t.Fatal("missing binding")
	return domain.ClientBinding{}
}
func qoderSDKTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if e.IsDir() {
			out[rel] = "directory"
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestProjectQoderPublicPreparedLifecycleNeverCallsNative(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("observer-%t", enabled), func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			a, b, user, pkg := filepath.Join(base, "A"), filepath.Join(base, "B"), filepath.Join(base, "home", ".qoder"), filepath.Join(base, "package")
			for _, root := range []string{a, b, user} {
				claudeSDKWrite(t, filepath.Join(root, "keep"), "original foreign")
			}
			claudeSDKWrite(t, filepath.Join(user, "plugins", "installed_plugins_v2.json"), `{"foreign":"original global native state"}`)
			claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-qoder","version":"1.0.0"}`)
			skill := "---\nname: docs\ndescription: Qoder Prepared fixture\n---\nPinned original.\n"
			claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), skill)
			claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"never","args":["${PLUGIN_DATA}"]}}}`)
			runner := &qoderSDKNoNative{}
			other := newQoderSDKFixture(t, b, user, pkg, enabled, runner)
			other.apply(t, installer.OpInstall)
			bBefore, userBefore := qoderSDKTree(t, b), qoderSDKTree(t, user)
			f := newQoderSDKFixture(t, a, user, pkg, enabled, runner)
			reject := func(op installer.Operation) {
				t.Helper()
				req := f.request
				req.Operation, req.ActivationComplete = op, true
				before := f.state(t)
				if out, err := f.execute(t, req); err == nil {
					t.Fatalf("attestation accepted %+v", out)
				}
				if f.state(t) != before || runner.calls != 0 {
					t.Fatal("refusal changed state or invoked native")
				}
			}
			reject(installer.OpInstall)
			installed := f.apply(t, installer.OpInstall)
			if !installed.Mutated {
				t.Fatal("first installation lost mutation")
			}
			binding := f.binding(t)
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != filepath.Join(a, ".uap", "managed", "clients", "qoder") || len(binding.NativeObjects) != 1 || binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
				t.Fatalf("bundle claimed native ownership %+v", binding)
			}
			view := filepath.Join(binding.TargetLocator, ".agentplugins-qoder-native")
			if claudeSDKBytes(t, filepath.Join(view, "skills", "docs", "SKILL.md")) != skill {
				t.Fatal("actual native view missing")
			}
			if !strings.Contains(strings.Join(installed.ManualActions, " "), view) {
				t.Fatal("native import boundary not retained", installed.ManualActions)
			}
			data := filepath.Join(a, ".uap", "caller-data", "keep")
			claudeSDKWrite(t, data, "persistent caller data")
			for _, op := range []installer.Operation{installer.OpInstall, installer.OpUpdate, installer.OpRepair} {
				reject(op)
				before := f.state(t)
				out := f.apply(t, op)
				if out.Mutated || !out.NoChange || before != f.state(t) {
					t.Fatalf("intact Prepared %s not stable %+v", op, out)
				}
			}
			sequence := f.sequence
			f = newQoderSDKFixture(t, a, user, pkg, enabled, runner)
			f.sequence = sequence
			beforeCalls := runner.calls
			status, err := f.engine.Inspect(t.Context())
			if err != nil || status.Recovery.Required || runner.calls != beforeCalls {
				t.Fatalf("status not host-free %+v %v", status, err)
			}
			// The public SDK requires an explicit package root. Its caller supplies
			// an immutable captured pin; the saved CLI separately tests cache resolution.
			pin := filepath.Join(base, "immutable-pin")
			if err := filetree.CopyDir(pkg, pin); err != nil {
				t.Fatal(err)
			}
			f.pin = pin
			advanced := strings.Replace(skill, "Pinned original.", "Advanced source.", 1)
			claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), advanced)
			req := f.request
			req.Operation = installer.OpRepair
			stateBefore := f.state(t)
			if out, err := f.execute(t, req); err == nil || !strings.Contains(err.Error(), "install cannot change an active revision") || out.Mutated || stateBefore != f.state(t) {
				t.Fatalf("advanced explicit SDK repair root bypassed revision guard: %+v %v", out, err)
			}
			claudeSDKWrite(t, filepath.Join(view, "skills", "docs", "SKILL.md"), "outside drift")
			if out := f.apply(t, installer.OpRepair); !out.Mutated {
				t.Fatal("drift repair hid mutation")
			}
			if got := f.binding(t); got.Activation != domain.ActivationPrepared || got.Verification != domain.VerificationInstalled {
				t.Fatalf("repaired file proof lost: %+v", got)
			}
			if claudeSDKBytes(t, filepath.Join(view, "skills", "docs", "SKILL.md")) != skill {
				t.Fatal("repair followed later source")
			}
			if err := os.RemoveAll(binding.TargetLocator); err != nil {
				t.Fatal(err)
			}
			f.apply(t, installer.OpRepair)
			if claudeSDKBytes(t, filepath.Join(view, "skills", "docs", "SKILL.md")) != skill {
				t.Fatal("missing repair followed later source")
			}
			if out := f.apply(t, installer.OpUpdate); !out.Mutated {
				t.Fatal("changed source update hid mutation")
			}
			if claudeSDKBytes(t, filepath.Join(view, "skills", "docs", "SKILL.md")) != advanced {
				t.Fatal("update did not deliver current source")
			}
			current := f.binding(t)
			if current.ClientBindingID != binding.ClientBindingID || current.TargetLocator != binding.TargetLocator {
				t.Fatal("update changed exact authority")
			}
			f.apply(t, installer.OpRemove)
			if _, err := os.Lstat(binding.TargetLocator); !os.IsNotExist(err) {
				t.Fatal("bundle survived removal", err)
			}
			if claudeSDKBytes(t, data) != "persistent caller data" || runner.calls != 0 || !reflect.DeepEqual(userBefore, qoderSDKTree(t, user)) || !reflect.DeepEqual(bBefore, qoderSDKTree(t, b)) {
				t.Fatal("project operation changed another authority or native runtime")
			}
		})
	}
}
