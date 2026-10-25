package installer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	deathManagedBoundary = "managed_package_committed_before_native_activation"
	deathNativeBoundary  = "native_skill_swapped_before_mcp_lock_and_commit"
	deathOldVersion      = "inert version one"
	deathNewVersion      = "inert version two"
	deathComponent       = "sample-notify"
)

type deathFixtureRunner struct{ calls int }

func (r *deathFixtureRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return (processadapter.OS{}).Run(ctx, command)
}

// Only the child uses this wrapper. All scope, ownership, projection and native
// mutation behavior delegates to the real adapter. The existing primitive seam
// pauses after the real native skill swap and before the MCP writer lock/write.
type deathBoundaryCodex struct {
	*codex.Adapter
	beforeMCPLock func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error)
}

func (a deathBoundaryCodex) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(a.beforeMCPLock)
	return a.Adapter.Activate(ctx, env, request)
}

func deathEngine(t *testing.T, base, project string, adapter clients.Adapter, committed func(context.Context, BindingFacts) error) (*Engine, *deathFixtureRunner) {
	t.Helper()
	if adapter == nil {
		adapter = codex.New()
	}
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(base, "user-uap")
	if project != "" {
		state = filepath.Join(project, ".uap")
	}
	runner := &deathFixtureRunner{}
	engine, err := New(Config{ProjectRoot: project, StateRoot: state, Registry: registry,
		Runner: runner, EnableNativeObserver: true, TrustedLocalPackages: true,
		HelperExecutable: filepath.Join(base, "codex-fixture"), OnCommittedBinding: committed})
	if err != nil {
		t.Fatal(err)
	}
	return engine, runner
}

func deathWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func deathRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func deathSkill(name, version string) string {
	return "---\nname: " + name + "\ndescription: Inert process-death fixture\n---\n" + version + "\n"
}

func deathPackage(t *testing.T, base, name, version string) string {
	t.Helper()
	root := filepath.Join(base, "package-"+name)
	deathWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":%q,"version":"1.0.0"}`, name))
	deathWrite(t, filepath.Join(root, "skills", name, "SKILL.md"), deathSkill(name, version))
	deathWrite(t, filepath.Join(root, "mcp.json"), fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{%q:{"type":"stdio","command":"./bin/probe","args":[%q],"env":{}}}}`, name, version))
	deathWrite(t, filepath.Join(root, "bin", "probe"), deathRead(t, filepath.Join(base, "codex-fixture")))
	if err := os.Chmod(filepath.Join(root, "bin", "probe"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func deathRequest(t *testing.T, engine *Engine, base, project string, op Operation) Request {
	t.Helper()
	scope, owner := domain.ScopeUser, "user-default"
	if project != "" {
		scope, owner = domain.ScopeProject, "project-default"
	}
	// Direct local SDK packages have the existing public source-key fallback
	// github::; this fixture fetches no GitHub source or vendor content.
	req := Request{Scope: scope, ProjectRoot: project, Operation: op,
		PackageRoot: filepath.Join(base, "package-"+deathComponent), ClientID: "codex",
		ClientConfigRoot: filepath.Join(base, "user", ".codex"), ClientExecutable: filepath.Join(base, "codex-fixture"),
		RequiredComponents: []string{"skills", "mcp"}, OperationID: "process-death-" + string(op),
		EnvironmentAuthority: &domain.EnvironmentAuthority{EnvironmentID: owner, Alias: deathComponent, ClientID: "codex", ExpectedAction: string(op), SourceKey: "github::"}}
	if op != OpInstall {
		view, err := engine.Inspect(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
			t.Fatalf("unexpected process-death baseline: %+v", view)
		}
		binding := view.Installations[0].Bindings[0]
		if binding.Owner == nil {
			t.Fatal("fixture installation lacks a real environment receipt")
		}
		req.InstallationID = view.Installations[0].InstallationID
		req.EnvironmentAuthority.ReceiptID = binding.Owner.ReceiptID
		req.EnvironmentAuthority.InstallationID = req.InstallationID
		req.EnvironmentAuthority.ClientBindingID = binding.BindingID
	}
	return req
}

func deathApply(t *testing.T, engine *Engine, req Request) Result {
	t.Helper()
	prepared, err := engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatalf("prepare %s: %v", req.Operation, err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := engine.Apply(t.Context(), prepared, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("apply %s: %+v, %v", req.Operation, result, err)
	}
	return result
}

func deathBinding(t *testing.T, base, project string) domain.ClientBinding {
	t.Helper()
	root := filepath.Join(base, "user-uap")
	if project != "" {
		root = filepath.Join(project, ".uap")
	}
	state, err := (statev2.Store{Path: filepath.Join(root, "state-v2.json"), ProjectRoot: project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("read genuine fixture state: %+v, %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("fixture binding absent")
	return domain.ClientBinding{}
}

func deathNativeObjects(binding domain.ClientBinding) []domain.NativeObjectOwnership {
	var out []domain.NativeObjectOwnership
	for _, object := range binding.NativeObjects {
		if object.Kind != "managed_package_directory" {
			out = append(out, object)
		}
	}
	return out
}

type deathTreeEntry struct {
	Mode os.FileMode
	Hash string
}

func deathTree(t *testing.T, root string) map[string]deathTreeEntry {
	t.Helper()
	out := map[string]deathTreeEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fact := deathTreeEntry{Mode: info.Mode()}
		if info.Mode()&os.ModeSymlink != 0 {
			fact.Hash, err = os.Readlink(path)
		} else if !entry.IsDir() {
			var file *os.File
			file, err = os.Open(path)
			if err == nil {
				hash := sha256.New()
				_, readErr := io.Copy(hash, file)
				err = file.Close()
				if readErr != nil {
					return readErr
				}
				fact.Hash = fmt.Sprintf("%x", hash.Sum(nil))
			}
		}
		out[relative] = fact
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func deathAssertTree(t *testing.T, root string, before map[string]deathTreeEntry) {
	t.Helper()
	if !reflect.DeepEqual(before, deathTree(t, root)) {
		t.Fatalf("process death/reopen changed retained tree %s", root)
	}
}

type deathBarrier struct {
	Boundary, TargetPath, AttemptID, BindingID, Backup string
}

func deathProveBoundary(t *testing.T, base, boundary string, before domain.ClientBinding, oldConfig string) deathBarrier {
	t.Helper()
	project := filepath.Join(base, "A")
	binding := deathBinding(t, base, project)
	if binding.Scope != string(domain.ScopeProject) || binding.ClientID != "codex" || binding.NativeProfileRoot != project || binding.ClientBindingID != before.ClientBindingID || binding.TargetLocator != before.TargetLocator || binding.DataReceiptID == "" || binding.DataReceiptID != before.DataReceiptID {
		t.Fatalf("boundary changed selected scope, binding or data authority: %+v", binding)
	}
	if binding.NativeActivationAttempt == "" || binding.PendingNativeIntent != nil || recordedBindingDigest(binding, "") == "" || recordedBindingDigest(binding, "") == recordedBindingDigest(before, "") {
		t.Fatalf("boundary lacks durable changed package/attempt identity: %+v", binding)
	}
	if !reflect.DeepEqual(deathNativeObjects(binding), deathNativeObjects(before)) {
		t.Fatal("boundary acknowledged new native ownership before the native commit")
	}
	if deathRead(t, filepath.Join(binding.TargetLocator, "skills", deathComponent, "SKILL.md")) != deathSkill(deathComponent, deathNewVersion) {
		t.Fatal("managed package was not replaced before the barrier")
	}
	if deathRead(t, filepath.Join(project, ".codex", "config.toml")) != oldConfig {
		t.Fatal("MCP config changed before its admitted commit boundary")
	}
	native := filepath.Join(project, ".agents", "skills", deathComponent, "SKILL.md")
	want := deathOldVersion
	barrier := deathBarrier{Boundary: boundary, TargetPath: binding.TargetLocator, AttemptID: binding.NativeActivationAttempt, BindingID: binding.ClientBindingID}
	backups, err := filepath.Glob(filepath.Join(project, ".agents", "skills", ".agentplugins-native-*"))
	if err != nil {
		t.Fatal(err)
	}
	if boundary == deathNativeBoundary {
		want = deathNewVersion
		if len(backups) != 1 {
			t.Fatalf("real native swap did not retain exactly one recovery root: %v", backups)
		}
		barrier.Backup = backups[0]
		if deathRead(t, filepath.Join(barrier.Backup, "old-"+deathComponent, "SKILL.md")) != deathSkill(deathComponent, deathOldVersion) {
			t.Fatal("real native swap lost the old skill backup")
		}
	} else if boundary != deathManagedBoundary || len(backups) != 0 {
		t.Fatalf("unexpected managed-only boundary/backups: %q %v", boundary, backups)
	}
	if deathRead(t, native) != deathSkill(deathComponent, want) {
		t.Fatalf("native skill differs at %s", boundary)
	}
	return barrier
}

func deathEmitAndBlock(t *testing.T, barrier deathBarrier) {
	t.Helper()
	if err := json.NewEncoder(os.Stdout).Encode(barrier); err != nil {
		t.Fatal(err)
	}
	var release [1]byte
	_, err := os.Stdin.Read(release[:])
	t.Fatalf("parent did not kill the child at its barrier: %v", err)
}
