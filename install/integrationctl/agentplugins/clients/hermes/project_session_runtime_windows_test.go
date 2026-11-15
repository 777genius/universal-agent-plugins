package hermes

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func windowsSessionLogFixture(t *testing.T, only bool) (sessionHistoryFixture, string) {
	t.Helper()
	f := newSessionHistoryFixture(t)
	if only {
		for _, name := range []string{"spawn-ledger.json", "spawn-ledger.json.lock", "logs/mcp-stderr.log"} {
			if err := os.Remove(filepath.Join(f.root, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := filepath.Join(f.root, "logs", ".__agent.lock")
	historyWrite(t, path, nil, 0o666)
	return f, path
}

func TestProjectSessionWindowsAgentLockIsImmutableInspectData(t *testing.T) {
	for _, only := range []bool{false, true} {
		t.Run(map[bool]string{false: "four-roles", true: "lock-only"}[only], func(t *testing.T) {
			f, path := windowsSessionLogFixture(t, only)
			plan := f.plan(t)
			runner := &sessionInventoryRunner{plan: plan}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			if err := New().VerifySession(t.Context(), f.env, plan); err != nil || len(runner.calls) != 3 {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, f.plan(t)) {
				t.Fatal("unchanged Windows history changed final plan")
			}
			body, err := os.ReadFile(path)
			if err != nil || len(body) != 0 {
				t.Fatal("inspection changed original empty lock", err)
			}
			f.req.Inspect = false
			if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil {
				t.Fatal("Windows lock authorized noninspect")
			}
			plan.Arguments = nil
			if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 3 {
				t.Fatal("changed inspection mode reached another child", err)
			}
		})
	}
}

func TestProjectSessionWindowsAgentLockRefusesUnmeasuredLeaves(t *testing.T) {
	for _, change := range []string{"nonempty", "readonly", "directory", "hardlink", "symlink", "errors", "case", "suffix"} {
		t.Run(change, func(t *testing.T) {
			f, path := windowsSessionLogFixture(t, true)
			switch change {
			case "nonempty":
				historyWrite(t, path, []byte("x"), 0o666)
			case "readonly":
				if err := os.Chmod(path, 0o444); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o666) })
			case "hardlink":
				if err := os.Link(path, filepath.Join(t.TempDir(), "same-file")); err != nil {
					t.Fatal(err)
				}
			case "directory", "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if change == "directory" {
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				} else {
					foreign := filepath.Join(t.TempDir(), "foreign")
					historyWrite(t, foreign, nil, 0o666)
					if err := os.Symlink(foreign, path); err != nil {
						if errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) {
							t.Skip("Windows does not grant the symbolic-link privilege")
						}
						t.Fatal(err)
					}
				}
			case "errors", "case", "suffix":
				name := map[string]string{"errors": ".__errors.lock", "case": ".__AGENT.lock", "suffix": ".__agent.lock.extra"}[change]
				if err := os.Rename(path, path+"-intermediate"); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(path+"-intermediate", filepath.Join(filepath.Dir(path), name)); err != nil {
					t.Fatal(err)
				}
			}
			runner := &sessionInventoryRunner{}
			f.env.Runner = runner
			if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil || len(runner.calls) != 0 {
				t.Fatal("unmeasured Windows log lock reached planning", err)
			}
		})
	}
}

func TestProjectSessionWindowsAgentLockRejectsAnotherOwner(t *testing.T) {
	f, path := windowsSessionLogFixture(t, true)
	token := windows.GetCurrentProcessToken()
	original, err := sessionTokenDefaultOwner(token, windows.GetTokenInformation)
	if err != nil {
		t.Fatal(err)
	}
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if original.Equals(user.User.Sid) {
		t.Skip("Current token has no distinct TokenUser owner for this fixture")
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, user.User.Sid, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, original, nil, nil, nil); err != nil {
			t.Error(err)
		}
	})
	runner := &sessionInventoryRunner{}
	f.env.Runner = runner
	if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil || len(runner.calls) != 0 {
		t.Fatal("different physical owner acquired history authority", err)
	}
}

func replaceWindowsHistoryLogParent(t *testing.T, path string) {
	t.Helper()
	logs := filepath.Dir(path)
	old := filepath.Join(t.TempDir(), "original-logs")
	if err := os.Rename(logs, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(logs, 0o700); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(old)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(old, entry.Name()), filepath.Join(logs, entry.Name())); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProjectSessionWindowsAgentLockKeepsFirstMetadataAndLogsParent(t *testing.T) {
	for _, change := range []string{"replacement", "mtime", "missing", "appearance", "logs-parent"} {
		t.Run(change, func(t *testing.T) {
			f, path := windowsSessionLogFixture(t, true)
			if change == "appearance" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			plan := f.plan(t)
			switch change {
			case "replacement", "mtime":
				before, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				stamp := before.ModTime().Add(time.Second)
				if change == "replacement" {
					if err := os.Rename(path, filepath.Join(t.TempDir(), "original")); err != nil {
						t.Fatal(err)
					}
					historyWrite(t, path, nil, 0o666)
					stamp = before.ModTime()
				}
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "appearance":
				historyWrite(t, path, nil, 0o666)
			case "logs-parent":
				replaceWindowsHistoryLogParent(t, path)
			}
			runner := &sessionInventoryRunner{plan: plan}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 0 {
				t.Fatal("new Windows lock revision replaced original plan", err)
			}
			if f.plan(t).ConfigDigest == plan.ConfigDigest {
				t.Fatal("new valid Windows snapshot lost metadata or parent identity")
			}
		})
	}
}

func TestProjectSessionWindowsAgentLockAloneRequiresEveryPassiveGate(t *testing.T) {
	for _, change := range []string{"stamp", "journal", "install", "scratch", "state", "cancel"} {
		t.Run(change, func(t *testing.T) {
			f, _ := windowsSessionLogFixture(t, true)
			overrides := map[string]string{}
			ctx := t.Context()
			switch change {
			case "stamp", "journal":
				name := map[string]string{"stamp": "install-stamp.json", "journal": ".hermes-update-zip-swap"}[change]
				historyWrite(t, filepath.Join(f.code, name), []byte("{}"), 0o600)
			case "install":
				overrides["HERMES_INSTALL_ROOT"] = f.code
			case "scratch":
				overrides["HERMES_SCRATCH_DIR"] = f.temporary
			case "state":
				if err := os.MkdirAll(filepath.Join(f.state, "hermes", "gateway-locks"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "cancel":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			f.env.SessionEnvironment = clients.SessionPlan{Environment: overrides}.ChildEnvironment(f.env.SessionEnvironment)
			runner := &sessionInventoryRunner{}
			f.env.Runner = runner
			if _, err := New().PlanSession(ctx, f.env, f.req); err == nil || len(runner.calls) != 0 {
				t.Fatal("lock-only history bypassed startup passivity", err)
			}
		})
	}
}

func TestProjectSessionWindowsAgentLockGrowthReadsOnlyOneActualByte(t *testing.T) {
	f, path := windowsSessionLogFixture(t, true)
	plan := f.plan(t)
	runner := &sessionInventoryRunner{plan: plan}
	f.env.Runner = sessionHistoryRunner{runner, f.response}
	execution, err := New().BeginSessionExecution(t.Context(), f.env, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = execution.Close() }()
	member := execution.(*sessionInspectHistory).state.members[3]
	if member.name != "logs/.__agent.lock" || len(member.body) != 0 {
		t.Fatal("capture did not retain original empty lock")
	}
	historyWrite(t, path, make([]byte, 8192), 0o666)
	// Exercise the production held reader after a writer grows that original
	// file, without replacing the captured handle or its original metadata.
	body, err := member.readBody()
	if err != nil || len(body) != 1 {
		t.Fatal("growing held lock escaped its actual one-byte read", len(body), err)
	}
	position, err := member.file.Seek(0, io.SeekCurrent)
	if err != nil || position != 1 {
		t.Fatal("held lock body read exceeded the actual one-byte cap", position, err)
	}
	if err := member.rejoin(execution.Context()); err == nil {
		t.Fatal("growing original lock was accepted")
	}
	if len(runner.calls) != 0 {
		t.Fatal("growth control started native observation")
	}
}

func TestProjectSessionWindowsAgentLockRequiresItsCapturedCatalogRole(t *testing.T) {
	f, _ := windowsSessionLogFixture(t, true)
	plan := f.plan(t)
	bundle, err := qualifyHermesBundle(plan.Executable)
	if err != nil {
		t.Fatal(err)
	}
	_, state, err := acquireSessionSource(t.Context(), plan.Directory, f.owner, plan.Servers,
		sessionRuntimeSelection{true, bundle, f.env.SessionEnvironment})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = state.close() }()
	admitted := state.admitted()
	if admitted["logs/.__agent.lock"] == nil {
		t.Fatal("actual capture did not bind the Windows role")
	}
	delete(admitted, "logs/.__agent.lock")
	err = verifySessionBootstrap(t.Context(), state.root.root, state.root.info, admitted)
	if err == nil || !strings.Contains(err.Error(), "fixed catalog") && !strings.Contains(err.Error(), "unknown or unsafe member") {
		t.Fatal("three-role catalog independently admitted the Windows lock", err)
	}
	if err := verifySessionBootstrap(t.Context(), state.root.root, state.root.info, state.admitted()); err != nil {
		t.Fatal("restoring only the original captured role did not restore catalog eligibility", err)
	}
}
