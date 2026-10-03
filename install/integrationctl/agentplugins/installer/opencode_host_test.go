package installer

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

func openCodeTestRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp(".", "TEST-opencode-installer-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("TMPDIR", root)
	return root
}
func buildOpenCodeTarget(t *testing.T, root, version, mode string) string {
	t.Helper()
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "opencode")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	command := exec.CommandContext(testCtx(t), "go", "build", "-ldflags", "-X main.version="+version+" -X main.mode="+mode, "-o", executable, "../adapters/clientdetect/testdata/opencode_probe.go")
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native host fixture: %s %v", body, err)
	}
	return executable
}
func openCodeEngine(t *testing.T, cfg Config) *Engine {
	t.Helper()
	var err error
	cfg.Registry, err = clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	cfg.TrustedLocalPackages = true
	engine, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
func openCodeRequest(t *testing.T, root, executable string) Request {
	t.Helper()
	packageRoot := filepath.Join(root, "package")
	writePackage(t, packageRoot, executable)
	return Request{Operation: OpInstall, ClientID: "opencode", ClientExecutable: executable,
		ClientConfigRoot: filepath.Join(root, "config"), PackageRoot: packageRoot, RequiredComponents: []string{"skills", "mcp"}}
}

// Regression: real Prepare must consume the exact selected V2 while PATH points
// at V1. A pure config_v2 selection must stop before preview/staging/activation,
// rather than silently writing config_v1. Single explicit target shape counts.
func TestOpenCodePrepareExplicitV2BlocksUnavailableCodec(t *testing.T) {
	root := openCodeTestRoot(t)
	v1 := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
	v2 := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	calls, handoffs := 0, 0
	var observed clientdetect.ProbeEvidence
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: v1,
		OpenCodeProbeEnvironment: []string{"PATH=" + filepath.Dir(v1)},
		OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			calls++
			evidence, err := clientdetect.ProbeOpenCodeTarget(ctx, target)
			observed = evidence
			return evidence, err
		}, OnCommittedBinding: func(context.Context, BindingFacts) error { handoffs++; return nil }})
	req := openCodeRequest(t, root, v1)
	req.Targets = []ClientTarget{{ClientID: "opencode", ClientConfigRoot: req.ClientConfigRoot, ClientExecutable: v2}}
	handle, err := engine.Prepare(testCtx(t), req)
	if !errors.Is(err, clients.ErrOpenCodeAdapterUnavailable) || handle != nil || calls != 1 || observed.Version != "2.0.21" || handoffs != 0 {
		t.Fatalf("target: handle=%v calls=%d evidence=%+v err=%v", handle, calls, observed, err)
	}
	if _, err := os.Stat(engine.cfg.ManagedRoot); !os.IsNotExist(err) {
		t.Fatalf("staged before adapter availability: %v", err)
	}
	if _, err := os.Stat(req.ClientConfigRoot); !os.IsNotExist(err) {
		t.Fatalf("native effect: %v", err)
	}
}

// Regression: changes to Request, Config env, injected port input, or returned
// Plan/profile/selection cannot rewrite a prepared native target. Read-only
// discovery/inspection/recovery and receipt-authorized removal never probe.
func TestOpenCodeLifecycleCopiesAndReadOnlyZeroProbes(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
	calls, handoffs := 0, 0
	env := []string{"PATH="}
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: env,
		OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			calls++
			evidence, err := clientdetect.ProbeOpenCodeTarget(ctx, target)
			target.Environment[0] = "PATH=mutated-by-port"
			return evidence, err
		}, OnCommittedBinding: func(context.Context, BindingFacts) error { handoffs++; return nil }})
	engine.Discover()
	if _, err := engine.Inspect(testCtx(t)); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("constructor/discovery/inspection probed")
	}
	env[0] = "PATH=caller-change"
	req := openCodeRequest(t, root, executable)
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handle.Close() }()
	if calls != 1 {
		t.Fatalf("Prepare probes: %d", calls)
	}
	plan := handle.Plan()
	if plan.OpenCodeProfile == nil || plan.OpenCodeProfile.ConfigDialect != opencodehost.DialectV1 {
		t.Fatalf("profile: %+v", plan.OpenCodeProfile)
	}
	original := handle.Plan()
	req.RequiredComponents[0] = "caller-change"
	req.ClientExecutable = filepath.Join(root, "wrong-target")
	req.ClientConfigRoot = filepath.Join(root, "wrong-root")
	plan.OpenCodeProfile.Capabilities[opencodehost.MCPStdio] = opencodehost.SupportUnsupported
	plan.OpenCodeSelections[0].Adapter = opencodehost.ConfigV2
	plan.Delivery.Components[0].Support = "caller-change"
	if !reflect.DeepEqual(handle.Plan(), original) {
		t.Fatal("presentation alias changed private plan")
	}
	result, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted || calls != 2 || handoffs != 1 {
		t.Fatalf("Apply: %+v probes=%d handoffs=%d %v", result, calls, handoffs, err)
	}
	raw, err := os.ReadFile(filepath.Join(original.ConfigRoot, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	if config["mcp"].(map[string]any)["sample-notify"] == nil {
		t.Fatal("V1 native entry missing")
	}
	if _, err := os.Stat(filepath.Join(original.ConfigRoot, "skills", "sample-notify", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	inspection, err := engine.Inspect(testCtx(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Recover(testCtx(t), inspection); err != nil {
		t.Fatal(err)
	}
	remove, err := engine.Prepare(testCtx(t), Request{Operation: OpRemove, ClientID: "opencode", ClientConfigRoot: original.ConfigRoot, InstallationID: result.InstallationID})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = remove.Close() }()
	if _, err := engine.Apply(testCtx(t), remove, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("read-only/recovery/remove probed: %d", calls)
	}
}

// Regression: equal-version executable/symlink/root changes or failed authority
// must return plan_changed before directories, ProgressStage, or host handoff.
// Nested links followed by .. must bind the payload actually executed, rather
// than the same-version executable at the lexically cleaned path.
func TestOpenCodeApplyRevalidatesBeforeEffects(t *testing.T) {
	for _, change := range []string{"bytes", "symlink", "nested_symlink", "nested_wrapper", "root", "absent", "failed", "wrapper"} {
		t.Run(change, func(t *testing.T) {
			if runtime.GOOS == "windows" && (change == "symlink" || change == "nested_symlink" || change == "nested_wrapper" || change == "root") {
				t.Skip("symlink fixture requires Unix")
			}
			root := openCodeTestRoot(t)
			executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
			replacement := buildOpenCodeTarget(t, filepath.Join(root, "other"), "1.18.33", "prefix")
			selected := executable
			if change == "symlink" {
				selected = filepath.Join(root, "selected")
				if err := os.Symlink(executable, selected); err != nil {
					t.Fatal(err)
				}
			}
			if change == "nested_symlink" || change == "nested_wrapper" {
				buildOpenCodeTarget(t, root, "1.18.33", "prefix") // lexical decoy A
				if err := os.MkdirAll(filepath.Join(root, "v1", "sub"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "v1", "sub"), filepath.Join(root, "dirlink")); err != nil {
					t.Fatal(err)
				}
				selected = filepath.Join(root, "selected")
				if err := os.Symlink("dirlink/../opencode", selected); err != nil {
					t.Fatal(err)
				}
			}
			req := openCodeRequest(t, root, selected)
			if change == "root" {
				actualRoot := filepath.Join(root, "config-a")
				if err := os.MkdirAll(actualRoot, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(actualRoot, req.ClientConfigRoot); err != nil {
					t.Fatal(err)
				}
			}
			effects := 0
			fail := false
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: replacement, OpenCodeProbeEnvironment: []string{"PATH="},
				OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
					if fail {
						return clientdetect.ProbeEvidence{}, errors.New("fixture failed authority")
					}
					return clientdetect.ProbeOpenCodeTarget(ctx, target)
				}, Progress: func(ProgressEvent) { effects++ }, OnCommittedBinding: func(context.Context, BindingFacts) error { effects++; return nil }})
			handle, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = handle.Close() }()
			if change == "nested_symlink" || change == "nested_wrapper" {
				if _, err := os.Stat(filepath.Join(root, "v1", "observed.json")); err != nil {
					t.Fatalf("actual nested payload B did not run: %v", err)
				}
				if _, err := os.Stat(filepath.Join(root, "observed.json")); !os.IsNotExist(err) {
					t.Fatalf("lexical decoy A ran: %v", err)
				}
			}
			effects = 0
			switch change {
			case "bytes", "nested_symlink":
				body, err := os.ReadFile(replacement)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(executable, body, 0700); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Remove(selected); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(replacement, selected); err != nil {
					t.Fatal(err)
				}
			case "root":
				other := filepath.Join(root, "config-b")
				if err := os.MkdirAll(other, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(req.ClientConfigRoot); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, req.ClientConfigRoot); err != nil {
					t.Fatal(err)
				}
			case "absent":
				if err := os.Remove(executable); err != nil {
					t.Fatal(err)
				}
			case "failed":
				fail = true
			case "wrapper", "nested_wrapper":
				if err := os.WriteFile(executable, []byte("#!/bin/sh\necho executed > \""+filepath.Join(root, "wrapper-ran")+"\"\nexec \""+replacement+"\" --version\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			result, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
			if !errors.Is(err, ErrPlanChanged) || result.Outcome != OutcomeConflict || result.Reason != "plan_changed" || effects != 0 {
				t.Fatalf("boundary: %+v effects=%d %v", result, effects, err)
			}
			if _, err := os.Stat(engine.cfg.ManagedRoot); !os.IsNotExist(err) {
				t.Fatalf("ensureDirs ran: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "wrapper-ran")); !os.IsNotExist(err) {
				t.Fatalf("unsupported wrapper executed: %v", err)
			}
		})
	}
}

// Regression: the identical-output pending OnCommittedBinding retry is still
// an effect, and cannot bypass target authority just because NoChange is true.
func TestOpenCodePendingHandoffRevalidates(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
	replacement := buildOpenCodeTarget(t, filepath.Join(root, "other"), "1.18.33", "prefix")
	handoffs := 0
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: replacement, OpenCodeProbeEnvironment: []string{"PATH="},
		OnCommittedBinding: func(context.Context, BindingFacts) error { handoffs++; return nil }})
	req := openCodeRequest(t, root, executable)
	first, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()
	result, err := engine.Apply(testCtx(t), first, Decision{Confirmed: true})
	if err != nil || handoffs != 1 || result.InstallationID == "" {
		t.Fatalf("pending fixture: %+v %v", result, err)
	}
	// Independent persisted host facts: this fixture has no authentication
	// action. Settle that separate phase so the preview can be a no-op.
	state, err := engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Authentication = domain.AuthenticationNotRequired
		state.Installations[0].Clients[key] = binding
	}
	if err := engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	req.InstallationID = result.InstallationID
	retry, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = retry.Close() }()
	if !retry.Plan().NoChange {
		t.Fatalf("fixture did not reach identical-output retry: first=%+v retry=%+v", result.Client, retry.Plan())
	}
	// Plant an independently acknowledged pending handoff. An interrupted
	// native write has a different unresolved-attempt guard and cannot reach
	// this retry branch; that guard is deliberately preserved.
	state, err = engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for key, binding := range state.Installations[0].Clients {
		binding.Activation = domain.ActivationFailed
		state.Installations[0].Clients[key] = binding
	}
	if err := engine.store.Save(state); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, body, 0700); err != nil {
		t.Fatal(err)
	}
	result, err = engine.Apply(testCtx(t), retry, Decision{Confirmed: true})
	if !errors.Is(err, ErrPlanChanged) || result.Reason != "plan_changed" || handoffs != 1 {
		t.Fatalf("pending boundary: %+v handoffs=%d %v", result, handoffs, err)
	}
}

// Regression: update must not lose the handle client in compatibility checks;
// repair and projection refresh must use the same authority boundary as add.
// Existing bindings without an explicit target cannot fall back to PATH.
func TestOpenCodeExistingMutationsRequireAndRevalidateTarget(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
	replacement := buildOpenCodeTarget(t, filepath.Join(root, "other"), "1.18.33", "prefix")
	original, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := os.ReadFile(replacement)
	if err != nil {
		t.Fatal(err)
	}
	probes, effects := 0, 0
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: replacement, OpenCodeProbeEnvironment: []string{"PATH="},
		OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
			probes++
			return clientdetect.ProbeOpenCodeTarget(ctx, target)
		}, Progress: func(ProgressEvent) { effects++ }, OnCommittedBinding: func(context.Context, BindingFacts) error { effects++; return nil }})
	req := openCodeRequest(t, root, executable)
	initial, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(testCtx(t), initial, Decision{Confirmed: true})
	_ = initial.Close()
	if err != nil {
		t.Fatal(err)
	}
	req.InstallationID = result.InstallationID
	if err := os.WriteFile(filepath.Join(req.PackageRoot, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, op := range []Operation{OpUpdate, OpRepair, OpRefreshProjection} {
		t.Run(string(op), func(t *testing.T) {
			req.Operation = op
			missing := req
			missing.ClientExecutable = ""
			before := probes
			if handle, err := engine.Prepare(testCtx(t), missing); !errors.Is(err, ErrHostTargetRequired) || handle != nil || probes != before {
				t.Fatalf("missing target: handle=%v probes=%d %v", handle, probes-before, err)
			}
			handle, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			if plan := handle.Plan(); plan.OpenCodeProfile == nil || plan.OpenCodeProfile.Version != "1.18.33" || probes != before+1 {
				t.Fatalf("prepared authority: %+v probes=%d", plan, probes-before)
			}
			if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil || probes != before+2 {
				t.Fatalf("stable apply: probes=%d %v", probes-before, err)
			}
			_ = handle.Close()
			handle, err = engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = handle.Close() }()
			if err := os.WriteFile(executable, changed, 0700); err != nil {
				t.Fatal(err)
			}
			effects = 0
			result, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
			if !errors.Is(err, ErrPlanChanged) || result.Reason != "plan_changed" || effects != 0 {
				t.Fatalf("changed target: %+v effects=%d %v", result, effects, err)
			}
			if err := os.WriteFile(executable, original, 0700); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Regression: removing the last declarations through Update still reconciles
// prior native receipts and may retry a pending handoff. An empty new envelope
// must not bypass explicit authority or send config_v1 cleanup to V2.
func TestOpenCodeMetadataUpdateFencesPriorNativeEffects(t *testing.T) {
	for _, change := range []string{"bytes", "root", "stable", "v2"} {
		t.Run(change, func(t *testing.T) {
			if runtime.GOOS == "windows" && change == "root" {
				t.Skip("symlink fixture requires Unix")
			}
			root := openCodeTestRoot(t)
			executable := buildOpenCodeTarget(t, filepath.Join(root, "v1"), "1.18.33", "ok")
			version := "1.18.33"
			if change == "v2" {
				version = "2.0.21"
			}
			replacement := buildOpenCodeTarget(t, filepath.Join(root, "other"), version, "prefix")
			probes, effects := 0, 0
			engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: replacement, OpenCodeProbeEnvironment: []string{"PATH="},
				OpenCodeProbe: func(ctx context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
					probes++
					return clientdetect.ProbeOpenCodeTarget(ctx, target)
				}, Progress: func(ProgressEvent) { effects++ }, OnCommittedBinding: func(context.Context, BindingFacts) error { effects++; return nil }})
			req := openCodeRequest(t, root, executable)
			nativeRoot := req.ClientConfigRoot
			initial, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.Apply(testCtx(t), initial, Decision{Confirmed: true})
			_ = initial.Close()
			if err != nil {
				t.Fatal(err)
			}
			req.Operation, req.InstallationID, req.RequiredComponents = OpUpdate, result.InstallationID, nil
			if err := os.RemoveAll(filepath.Join(req.PackageRoot, "skills")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(req.PackageRoot, "mcp.json")); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(req.PackageRoot, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sample-notify","version":"1.0.1"}`), 0600); err != nil {
				t.Fatal(err)
			}
			missing := req
			missing.ClientExecutable = ""
			before := probes
			if handle, err := engine.Prepare(testCtx(t), missing); !errors.Is(err, ErrHostTargetRequired) || handle != nil || probes != before {
				t.Fatalf("empty proposal lost prior authority: handle=%v probes=%d %v", handle, probes-before, err)
			}
			if change == "v2" {
				req.ClientExecutable = replacement
				if handle, err := engine.Prepare(testCtx(t), req); !errors.Is(err, clients.ErrOpenCodeAdapterUnavailable) || handle != nil {
					t.Fatalf("config_v1 cleanup sent to V2: handle=%v %v", handle, err)
				}
				return
			}
			handle, err := engine.Prepare(testCtx(t), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = handle.Close() }()
			if len(handle.Plan().Delivery.Components) != 0 || handle.Plan().OpenCodeProfile == nil || handle.Plan().NoChange {
				t.Fatalf("fixture is not a metadata-only update: %+v", handle.Plan())
			}
			paths := []string{filepath.Join(nativeRoot, "opencode.json"), filepath.Join(nativeRoot, "skills", "sample-notify", "SKILL.md"), engine.cfg.StateFile}
			prior := make([][]byte, len(paths))
			for i, path := range paths {
				prior[i], err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			switch change {
			case "bytes":
				body, err := os.ReadFile(replacement)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(executable, body, 0700); err != nil {
					t.Fatal(err)
				}
			case "root":
				other := filepath.Join(root, "config-b")
				if err := os.Mkdir(other, 0700); err != nil {
					t.Fatal(err)
				}
				preserved := filepath.Join(root, "config-a")
				if err := os.Rename(req.ClientConfigRoot, preserved); err != nil {
					t.Fatal(err)
				}
				paths[0] = filepath.Join(preserved, "opencode.json")
				paths[1] = filepath.Join(preserved, "skills", "sample-notify", "SKILL.md")
				if err := os.Symlink(other, req.ClientConfigRoot); err != nil {
					t.Fatal(err)
				}
			}
			effects = 0
			result, err = engine.Apply(testCtx(t), handle, Decision{Confirmed: true})
			if change == "stable" {
				if err != nil || !result.Mutated {
					t.Fatalf("stable metadata update: %+v %v", result, err)
				}
				body, err := os.ReadFile(paths[0])
				if err != nil {
					t.Fatal(err)
				}
				var config map[string]any
				if err := json.Unmarshal(body, &config); err != nil || config["mcp"].(map[string]any)["sample-notify"] != nil {
					t.Fatalf("old MCP was not removed: %s %v", body, err)
				}
				if _, err := os.Stat(paths[1]); !os.IsNotExist(err) {
					t.Fatalf("old skill was not removed: %v", err)
				}
				// After cleanup there are no proposed or previous native objects.
				// Pending handoff alone must still require/revalidate the target.
				state, err := engine.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				for key, binding := range state.Installations[0].Clients {
					if len(opencode.OpenCodeObjects(binding.NativeObjects)) != 0 {
						t.Fatal("metadata update retained native receipts")
					}
					binding.Activation = domain.ActivationFailed
					state.Installations[0].Clients[key] = binding
				}
				if err := engine.store.Save(state); err != nil {
					t.Fatal(err)
				}
				if pending, err := engine.Prepare(testCtx(t), missing); !errors.Is(err, ErrHostTargetRequired) || pending != nil {
					t.Fatalf("pending-only target missing: handle=%v %v", pending, err)
				}
				pending, err := engine.Prepare(testCtx(t), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = pending.Close() }()
				changed, err := os.ReadFile(replacement)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(executable, changed, 0700); err != nil {
					t.Fatal(err)
				}
				effects = 0
				result, err = engine.Apply(testCtx(t), pending, Decision{Confirmed: true})
				if !errors.Is(err, ErrPlanChanged) || result.Reason != "plan_changed" || effects != 0 {
					t.Fatalf("pending-only bypass: %+v effects=%d %v", result, effects, err)
				}
				return
			}
			if !errors.Is(err, ErrPlanChanged) || result.Outcome != OutcomeConflict || result.Reason != "plan_changed" || effects != 0 {
				t.Fatalf("empty proposal effects bypass: %+v effects=%d %v", result, effects, err)
			}
			for i, path := range paths {
				body, err := os.ReadFile(path)
				if err != nil || string(body) != string(prior[i]) {
					t.Fatalf("prior owned state changed at %s: %v", path, err)
				}
			}
		})
	}
}

// Regression: withholding the V2 MCP codec cannot disable qualified directory
// skills, nor may a skills-only install rewrite a foreign V2 JSONC config.
func TestOpenCodeV2SkillsIndependentOfMCPCodecAndObservers(t *testing.T) {
	root := openCodeTestRoot(t)
	executable := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	req := openCodeRequest(t, root, executable)
	if err := os.Remove(filepath.Join(req.PackageRoot, "mcp.json")); err != nil {
		t.Fatal(err)
	}
	req.RequiredComponents = []string{"skills"}
	if err := os.MkdirAll(req.ClientConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	foreign := []byte("{ // owned by the user\n \"mcp\": {\"servers\": {\"foreign\": {\"command\": [\"foreign\"]}}}}\n")
	configPath := filepath.Join(req.ClientConfigRoot, "opencode.jsonc")
	if err := os.WriteFile(configPath, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	engine := openCodeEngine(t, Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: executable, OpenCodeProbeEnvironment: []string{"PATH="}})
	handle, err := engine.Prepare(testCtx(t), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = handle.Close() }()
	plan := handle.Plan()
	if len(plan.OpenCodeSelections) != 1 || plan.OpenCodeSelections[0].Adapter != opencodehost.SkillDirectory || plan.OpenCodeProfile.Capabilities[opencodehost.ObserverCompletion] != opencodehost.Unverified {
		t.Fatalf("qualification: %+v", plan)
	}
	if _, err := engine.Apply(testCtx(t), handle, Decision{Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(req.ClientConfigRoot, "skills", "sample-notify", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(after) != string(foreign) {
		t.Fatalf("foreign config changed: %s %v", after, err)
	}
}

// A grouped request must not bypass the single-target host snapshot by using
// the main registry's broader group support. Neither an unknown nor an explicit
// V2 target may reach package assessment, host probing, snapshots, or effects.
func TestOpenCodeMutatingGroupRefusesBeforePreparation(t *testing.T) {
	root := openCodeTestRoot(t)
	v2 := buildOpenCodeTarget(t, filepath.Join(root, "v2"), "2.0.21", "ok")
	packageRoot := filepath.Join(root, "package")
	writePackage(t, packageRoot, v2)
	registry, err := clients.NewRegistry(claude.New(), opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []Operation{OpInstall, OpUpdate, OpRepair} {
		for _, executable := range []string{"", v2} {
			for _, reverse := range []bool{false, true} {
				name := string(operation) + "/unknown"
				if executable != "" {
					name = string(operation) + "/v2"
				}
				if reverse {
					name += "/reversed"
				}
				t.Run(name, func(t *testing.T) {
					sandbox := filepath.Join(root, filepath.FromSlash(name))
					assessments, probes, progress, handoffs := 0, 0, 0, 0
					engine, err := New(Config{StateRoot: filepath.Join(sandbox, "state"), Registry: registry, HelperExecutable: v2,
						Assess: func(_ context.Context, _ string, digest string) (Assessment, error) {
							assessments++
							return Assessment{TreeDigest: digest, Outcome: AssessmentAllow}, nil
						}, OpenCodeProbe: func(context.Context, clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
							probes++
							return clientdetect.ProbeEvidence{}, errors.New("unexpected group host probe")
						}, Progress: func(ProgressEvent) { progress++ },
						OnCommittedBinding: func(context.Context, BindingFacts) error { handoffs++; return nil }})
					if err != nil {
						t.Fatal(err)
					}
					targets := []ClientTarget{
						{ClientID: "claude", ClientConfigRoot: filepath.Join(sandbox, "claude")},
						{ClientID: "opencode", ClientConfigRoot: filepath.Join(sandbox, "opencode"), ClientExecutable: executable},
					}
					if reverse {
						targets[0], targets[1] = targets[1], targets[0]
					}
					handle, err := engine.Prepare(testCtx(t), Request{Operation: operation, PackageRoot: packageRoot,
						InstallationID: "00000000-0000-4000-8000-000000000081", RequiredComponents: []string{"mcp", "skills"}, Targets: targets})
					if handle != nil {
						defer func() { _ = handle.Close() }()
					}
					if !errors.Is(err, ErrUnsupported) || handle != nil || assessments != 0 || probes != 0 || progress != 0 || handoffs != 0 {
						t.Fatalf("group reached preparation: handle=%v assess=%d probe=%d progress=%d handoff=%d err=%v", handle, assessments, probes, progress, handoffs, err)
					}
					for _, path := range []string{engine.cfg.StateRoot, engine.cfg.TempRoot, engine.cfg.ManagedRoot, engine.cfg.LockFile,
						filepath.Join(sandbox, "claude"), filepath.Join(sandbox, "opencode")} {
						if _, err := os.Lstat(path); !os.IsNotExist(err) {
							t.Fatalf("group created owned/native path %s: %v", path, err)
						}
					}
				})
			}
		}
	}
}
