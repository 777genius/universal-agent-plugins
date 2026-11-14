package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Reuse the existing SDK process-death harness and exact native lock seam. The
// child invokes the real workspace adapter; it runs no VS Code or MCP process.
func TestProjectVSCodeProcessDeathKeepsNativeAttemptFenced(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { deathAllowSnapshotCleanup(t, base) })
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	deathWrite(t, filepath.Join(base, "codex-fixture"), deathRead(t, helper))
	if err := os.Chmod(filepath.Join(base, "codex-fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	deathPackage(t, base, deathComponent, deathOldVersion)
	freshPackage := deathPackage(t, base, "fresh-death", deathOldVersion)
	user, a, b := filepath.Join(base, "user"), filepath.Join(base, "A"), filepath.Join(base, "B")
	deathWrite(t, filepath.Join(user, "Code", "User", "mcp.json"), `{"servers":{"USER":{"type":"http","url":"https://foreign.test/mcp"}}}`)
	deathWrite(t, filepath.Join(base, "user-uap", "keep"), "original user state")
	var aEngine *Engine
	for _, project := range []string{a, b} {
		deathWrite(t, filepath.Join(project, ".vscode", "mcp.json"), "{// foreign comment\n\"inputs\":[],\"servers\":{\"foreign\":{\"type\":\"http\",\"url\":\"https://foreign.test/mcp\"},},}\n")
		deathWrite(t, filepath.Join(project, ".github", "skills", "foreign", "SKILL.md"), "foreign skill")
		engine, _ := deathEngine(t, base, project, vscode.NewProject(), nil)
		result := deathApply(t, engine, workspaceDeathRequest(t, engine, base, project, OpInstall))
		deathWrite(t, filepath.Join(result.Binding.DataRoot, "keep"), "retained data")
		if project == a {
			aEngine = engine
		}
	}
	before, oldConfig := deathBinding(t, base, a), deathRead(t, filepath.Join(a, ".vscode", "mcp.json"))
	update := workspaceDeathRequest(t, aEngine, base, a, OpUpdate)
	userBefore, userState, bBefore := deathTree(t, user), deathTree(t, filepath.Join(base, "user-uap")), deathTree(t, b)
	deathPackage(t, base, deathComponent, deathNewVersion)
	deathKillAtBarrierChild(t, "TestProjectVSCodeProcessDeathChild", base, deathNativeBoundary, func(observed deathBarrier) {
		if want := deathProveBoundary(t, base, deathNativeBoundary, before, oldConfig); !reflect.DeepEqual(observed, want) {
			t.Fatalf("workspace child barrier differs from persisted facts: %+v, want %+v", observed, want)
		}
	})
	retained := deathTree(t, a)
	hostCalls := 0
	fresh, runner := deathEngine(t, base, a, vscode.NewProject(), func(context.Context, BindingFacts) error { hostCalls++; return nil })
	view, err := fresh.Inspect(t.Context())
	if err == nil || !view.Recovery.Required || view.Recovery.Reason != "native attempt has no persisted intent; retain uncertainty" || len(view.Installations) != 1 {
		t.Fatalf("workspace reopen lost pending native attempt: %+v %v", view, err)
	}
	for _, operation := range []Operation{OpUpdate, OpRepair, OpRemove, OpInstall} {
		req := update
		authority := *update.EnvironmentAuthority
		req.EnvironmentAuthority = &authority
		req.Operation, req.OperationID, authority.ExpectedAction = operation, "workspace-after-death-"+string(operation), string(operation)
		if operation == OpInstall {
			req.PackageRoot, req.InstallationID = freshPackage, ""
			authority.Alias, authority.ReceiptID, authority.InstallationID, authority.ClientBindingID = "fresh-death", "", "", ""
		}
		handle, err := fresh.Prepare(t.Context(), req)
		if err == nil {
			result, applyErr := fresh.Apply(t.Context(), handle, Decision{Confirmed: true})
			err = applyErr
			if result.Mutated || result.Outcome != OutcomeRecovery {
				t.Fatalf("workspace %s crossed unresolved attempt: %+v %v", operation, result, err)
			}
		}
		if handle != nil {
			if closeErr := handle.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		}
		blocked := errors.Is(err, ErrRecoveryRequired) || err != nil && strings.Contains(err.Error(), "native activation attempt") && strings.Contains(err.Error(), "unresolved")
		if operation == OpRemove && errors.Is(err, ErrInvalidRequest) && strings.Contains(err.Error(), "removal binding scope or native attempt differs") {
			blocked = true
		}
		if !blocked {
			t.Fatalf("workspace %s refused for an unrelated reason: %v", operation, err)
		}
		deathAssertTree(t, a, retained)
	}
	if result, err := fresh.RecoverEnvironment(t.Context(), view, "project-default"); !errors.Is(err, ErrRecoveryRequired) || result.Outcome != OutcomeRecovery {
		t.Fatalf("workspace recovery guessed native ownership: %+v %v", result, err)
	}
	again, err := fresh.Inspect(t.Context())
	if err == nil || !reflect.DeepEqual(view, again) || runner.calls != 0 || hostCalls != 0 {
		t.Fatalf("workspace retry changed pending facts or invoked process/host: %+v %v; native=%d host=%d", again, err, runner.calls, hostCalls)
	}
	deathAssertTree(t, a, retained)
	deathAssertTree(t, user, userBefore)
	deathAssertTree(t, filepath.Join(base, "user-uap"), userState)
	deathAssertTree(t, b, bBefore)
	deathProveBoundary(t, base, deathNativeBoundary, before, oldConfig)
}

func workspaceDeathRequest(t *testing.T, engine *Engine, base, project string, op Operation) Request {
	t.Helper()
	req := deathRequest(t, engine, base, project, op)
	req.ClientID, req.ClientConfigRoot, req.ClientExecutable = "vscode", filepath.Join(base, "user", "Code"), ""
	req.EnvironmentAuthority.ClientID = "vscode"
	return req
}

type workspaceDeathAdapter struct {
	*vscode.ProjectAdapter
	beforeMCPLock func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error)
}

func (a workspaceDeathAdapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	env.NativeConfig = nativeconfig.NewWithLockAcquirer(a.beforeMCPLock)
	return a.ProjectAdapter.Activate(ctx, env, request)
}

func TestProjectVSCodeProcessDeathChild(t *testing.T) {
	var args []string
	for i, arg := range os.Args {
		if arg == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		t.Skip("disposable parent-controlled SDK child only")
	}
	if len(args) != 2 || args[1] != deathNativeBoundary {
		t.Fatal("invalid workspace process-death fixture arguments")
	}
	base, project := args[0], filepath.Join(args[0], "A")
	before, oldConfig := deathBinding(t, base, project), deathRead(t, filepath.Join(project, ".vscode", "mcp.json"))
	adapter := workspaceDeathAdapter{ProjectAdapter: vscode.NewProject(), beforeMCPLock: func(paths nativeconfig.Paths, codec nativeconfig.Codec) (func() error, error) {
		if codec != nativeconfig.CodecVSCode || paths.JSON != filepath.Join(project, ".vscode", "mcp.json") || paths.JSONC != "" {
			t.Fatalf("workspace primitive selected another codec/path: %+v %s", paths, codec)
		}
		deathEmitAndBlock(t, deathProveBoundary(t, base, deathNativeBoundary, before, oldConfig))
		return nil, fmt.Errorf("unreachable parent-controlled workspace boundary")
	}}
	engine, _ := deathEngine(t, base, project, adapter, nil)
	deathApply(t, engine, workspaceDeathRequest(t, engine, base, project, OpUpdate))
	t.Fatal("workspace SDK update completed without the selected kill boundary")
}
