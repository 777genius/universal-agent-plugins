package hermes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"

	"gopkg.in/yaml.v3"
)

type sessionHistoryFixture struct {
	req                          clients.SessionRequest
	owner                        domain.NativeObjectOwnership
	env                          clients.Env
	root, code, state, temporary string
	ledger, log, response        []byte
}

func historyWrite(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
}

func sessionHistoryStdioBinding(t *testing.T, req *clients.SessionRequest) string {
	t.Helper()
	req.Bindings[0].TargetLocator = filepath.Join(req.ProjectRoot, "inert-package")
	command := filepath.Join(req.Bindings[0].TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	object := &req.Bindings[0].NativeObjects[1]
	prior := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecHermes, Name: object.LogicalName, Digest: object.ManagedDigest}
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: object.Path}, Codec: nativeconfig.CodecHermes,
		Action: nativeconfig.ActionUpdate, Name: object.LogicalName, Owned: &prior,
		Server: nativeconfig.Server{Type: "stdio", Command: command, Args: []string{"inert-only"}}})
	if err != nil {
		t.Fatal(err)
	}
	object.ManagedDigest = receipt.Digest
	return object.Path
}

func newSessionHistoryFixture(t *testing.T) sessionHistoryFixture {
	t.Helper()
	req, owner := newProjectSessionFixture(t)
	sessionBootstrapFixture(t, req.ProjectRoot)
	root := projectSessionRoot(req.ProjectRoot)
	configPath := sessionHistoryStdioBinding(t, &req)
	bundle, err := qualifyHermesBundle(req.Client.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	code := filepath.Join(bundle.root, "hermes-agent")
	install := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(code))))[:12]
	ledger := []byte(fmt.Sprintf(`[{"pid":41,"create_time":1700000001.5,"purpose":"mcp-helper","install":%q,"spawner_pid":37,"spawner_create":1700000000.5,"registered_at":1700000002.5,"argv":"inert opaque fixture","host":"","port":null,"profile":"","hermes_home":"","isolated":false,"boot_start":null,"spawner_boot":null,"boot_id":null}]`, install))
	newline := "\n"
	if runtime.GOOS == "windows" {
		newline = "\r\n"
	}
	log := []byte(newline + "2026-11-14 12:05:11,913 ===== starting MCP server 'owned' =====" + newline)
	historyWrite(t, filepath.Join(root, "spawn-ledger.json"), ledger, 0o600)
	historyWrite(t, filepath.Join(root, "spawn-ledger.json.lock"), nil, 0o644)
	historyWrite(t, filepath.Join(root, "logs", "mcp-stderr.log"), log, 0o644)
	state, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	temporary := filepath.Join(state, "original-tmp")
	if err := os.Mkdir(temporary, 0o700); err != nil {
		t.Fatal(err)
	}
	base := clients.SessionPlan{Environment: map[string]string{"TMPDIR": temporary, "TMP": "", "TEMP": "", "HERMES_SCRATCH_DIR": "",
		"XDG_STATE_HOME": state, "HERMES_INSTALL_ROOT": "", "HERMES_GATEWAY_LOCK_DIR": ""}}.ChildEnvironment(os.Environ())
	env := sessionInventoryEnvironment(t, clients.SessionPlan{Directory: req.ProjectRoot}, nil)
	env.SessionEnvironment = base
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := yaml.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	response, err := json.Marshal(config["mcp_servers"])
	if err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []string{filepath.Join(req.Client.ConfigRoot, "user-sentinel"), filepath.Join(t.TempDir(), "sibling-sentinel")} {
		historyWrite(t, foreign, []byte("original foreign bytes"), 0o600)
		t.Cleanup(func() {
			body, err := os.ReadFile(foreign)
			if err != nil || string(body) != "original foreign bytes" {
				t.Error("passive history observation changed foreign bytes", err)
			}
		})
	}
	return sessionHistoryFixture{req, owner, env, root, code, state, temporary, ledger, log, response}
}

type sessionHistoryRunner struct {
	*sessionInventoryRunner
	response []byte
}

func (runner sessionHistoryRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := runner.sessionInventoryRunner.Run(ctx, command)
	if len(runner.calls) == 3 {
		result.Stdout = append([]byte(nil), runner.response...)
	}
	return result, err
}

func (fixture sessionHistoryFixture) plan(t *testing.T) clients.SessionPlan {
	t.Helper()
	plan, err := New().PlanSession(t.Context(), fixture.env, fixture.req)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestProjectSessionPassiveHistoryUsesOnlyOriginalInspectSource(t *testing.T) {
	f := newSessionHistoryFixture(t)
	plan := f.plan(t)
	runner := &sessionInventoryRunner{plan: plan}
	f.env.Runner = sessionHistoryRunner{runner, f.response}
	if err := New().VerifySession(t.Context(), f.env, plan); err != nil || len(runner.calls) != 3 {
		t.Fatal("unchanged passive history failed actual adapter verification", err)
	}
	if !reflect.DeepEqual(plan, f.plan(t)) {
		t.Fatal("unchanged history changed the final inspection plan")
	}
	for name, wanted := range map[string][]byte{"spawn-ledger.json": f.ledger, "spawn-ledger.json.lock": {}, "logs/mcp-stderr.log": f.log} {
		actual, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(actual, wanted) {
			t.Fatal("inspection changed registry/log history", name, err)
		}
	}
	f.req.Inspect = false
	if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil {
		t.Fatal("registry history authorized a general native session")
	}
	plan.Arguments = nil
	if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 3 {
		t.Fatal("changed inspect mode reached another child", err)
	}
	plan.Directory = t.TempDir()
	if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 3 {
		t.Fatal("another project inherited original runtime authority", err)
	}
}

func (fixture sessionHistoryFixture) unmeasuredLog(change string) []byte {
	body := append([]byte(nil), fixture.log...)
	switch change {
	case "log-output":
		return append(body, []byte("foreign child stderr")...)
	case "log-name":
		return bytes.ReplaceAll(body, []byte("'owned'"), []byte("'foreign'"))
	case "log-date":
		return bytes.ReplaceAll(body, []byte("2026-11-14"), []byte("2026-02-30"))
	case "log-year":
		return bytes.ReplaceAll(body, []byte("2026-11-14"), []byte("0000-11-14"))
	case "log-size":
		return bytes.Repeat(body, 10)
	}
	return body
}

func TestProjectSessionPassiveHistoryRefusesUnmeasuredContents(t *testing.T) {
	for _, change := range []string{"duplicate", "extra", "purpose", "install", "pid", "time", "port", "isolated", "empty", "oversized", "lock", "unpaired", "log-output", "log-name", "log-date", "log-year", "log-size", "unknown-lock", "mode", "hardlink"} {
		t.Run(change, func(t *testing.T) {
			f := newSessionHistoryFixture(t)
			path, body, mode := filepath.Join(f.root, "spawn-ledger.json"), f.ledger, os.FileMode(0o600)
			switch change {
			case "duplicate":
				body = bytes.Replace(body, []byte(`"pid":41`), []byte(`"pid":41,"pid":41`), 1)
			case "extra":
				body = bytes.Replace(body, []byte(`"pid":41`), []byte(`"extra":0,"pid":41`), 1)
			case "purpose":
				body = bytes.ReplaceAll(body, []byte("mcp-helper"), []byte("serve"))
			case "install":
				body = bytes.Replace(body, []byte(`"install":"`), []byte(`"install":"foreign`), 1)
			case "pid":
				body = bytes.Replace(body, []byte(`"pid":41`), []byte(`"pid":41.0`), 1)
			case "time":
				body = bytes.ReplaceAll(body, []byte("1700000001.5"), []byte("null"))
			case "port":
				body = bytes.ReplaceAll(body, []byte(`"port":null`), []byte(`"port":8080`))
			case "isolated":
				body = bytes.ReplaceAll(body, []byte(`"isolated":false`), []byte(`"isolated":null`))
			case "empty":
				body = []byte("[]")
			case "oversized":
				body = append(body, bytes.Repeat([]byte(" "), 4097)...)
			case "lock":
				path, body, mode = path+".lock", []byte("x"), 0o644
			case "unpaired":
				if err := os.Remove(path + ".lock"); err != nil {
					t.Fatal(err)
				}
			case "log-output", "log-name", "log-date", "log-year", "log-size":
				path, body, mode = filepath.Join(f.root, "logs", "mcp-stderr.log"), f.unmeasuredLog(change), 0o644
			case "unknown-lock":
				path, body, mode = filepath.Join(f.root, "logs", ".__errors.lock"), nil, 0o644
			case "mode":
				if runtime.GOOS == "windows" {
					t.Skip("POSIX permission mode; Windows owner/file checks are separate")
				}
				mode = 0o644
			case "hardlink":
				if err := os.Link(path, filepath.Join(t.TempDir(), "same-inode")); err != nil {
					t.Fatal(err)
				}
			}
			historyWrite(t, path, body, mode)
			runner := &sessionInventoryRunner{}
			f.env.Runner = runner
			if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil || len(runner.calls) != 0 {
				t.Fatal("unmeasured runtime history reached native planning", err)
			}
		})
	}
}

func (fixture sessionHistoryFixture) replaceHistoryParent(t *testing.T, change string) {
	t.Helper()
	path := filepath.Join(fixture.root, "logs")
	if change == "root-parent" {
		path = filepath.Dir(fixture.root)
	}
	old := filepath.Join(filepath.Dir(path), filepath.Base(path)+"-original")
	if change == "log-parent" {
		old = filepath.Join(t.TempDir(), "original-logs")
	}
	if err := os.Rename(path, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if change == "root-parent" {
		if err := os.Rename(filepath.Join(old, "uap-session"), fixture.root); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.CopyFS(path, os.DirFS(old)); err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" {
			for _, directory := range []string{path, filepath.Join(path, "curator")} {
				if err := os.Chmod(directory, 0o700); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func (fixture sessionHistoryFixture) replaceHistoryFile(t *testing.T, path string) {
	t.Helper()
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, filepath.Join(t.TempDir(), "original")); err != nil {
		t.Fatal(err)
	}
	historyWrite(t, path, fixture.ledger, 0o600)
	if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func TestProjectSessionPassiveHistoryCannotAdoptLaterRevision(t *testing.T) {
	for _, change := range []string{"bytes", "same-bytes-new-file", "missing", "log-parent", "root-parent", "startup-stamp", "gateway", "selector", "new-log", "cancel"} {
		t.Run(change, func(t *testing.T) {
			f := newSessionHistoryFixture(t)
			if change == "new-log" {
				if err := os.Remove(filepath.Join(f.root, "logs", "mcp-stderr.log")); err != nil {
					t.Fatal(err)
				}
			}
			plan := f.plan(t)
			path := filepath.Join(f.root, "spawn-ledger.json")
			switch change {
			case "bytes":
				historyWrite(t, path, bytes.ReplaceAll(f.ledger, []byte("1700000002.5"), []byte("1700000003.5")), 0o600)
			case "same-bytes-new-file":
				f.replaceHistoryFile(t, path)
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "log-parent", "root-parent":
				f.replaceHistoryParent(t, change)
			case "startup-stamp":
				historyWrite(t, filepath.Join(f.code, "install-stamp.json"), []byte(`{"commit":"forged"}`), 0o600)
			case "gateway":
				if err := os.MkdirAll(filepath.Join(f.state, "hermes", "gateway-locks"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "selector":
				f.env.SessionEnvironment = clients.SessionPlan{Environment: map[string]string{"TMPDIR": f.temporary + "-other"}}.ChildEnvironment(f.env.SessionEnvironment)
			case "new-log":
				historyWrite(t, filepath.Join(f.root, "logs", "mcp-stderr.log"), f.log, 0o644)
			}
			ctx := t.Context()
			if change == "cancel" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			runner := &sessionInventoryRunner{plan: plan}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			if err := New().VerifySession(ctx, f.env, plan); err == nil || len(runner.calls) != 0 {
				t.Fatal("later revision reached the first native child", err)
			}
			if change == "bytes" || change == "same-bytes-new-file" || change == "log-parent" || change == "selector" || change == "new-log" {
				fresh := f.plan(t)
				if fresh.ConfigDigest == plan.ConfigDigest {
					t.Fatal("original commitment did not distinguish a new valid snapshot")
				}
			}
		})
	}
}

func TestProjectSessionPassiveHistoryStopsAtFirstObservedMutation(t *testing.T) {
	for _, change := range []string{"ledger", "absence", "stamp"} {
		t.Run(change, func(t *testing.T) {
			f := newSessionHistoryFixture(t)
			plan := f.plan(t)
			runner := &sessionInventoryRunner{plan: plan, editAt: 1, edit: func() {
				switch change {
				case "ledger":
					historyWrite(t, filepath.Join(f.root, "spawn-ledger.json"), bytes.ReplaceAll(f.ledger, []byte("1700000002.5"), []byte("1700000003.5")), 0o600)
				case "absence":
					if err := os.Mkdir(filepath.Join(f.state, "hermes"), 0o700); err != nil {
						t.Fatal(err)
					}
				case "stamp":
					historyWrite(t, filepath.Join(f.code, "install-stamp.json"), []byte(`{}`), 0o600)
				}
			}}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 1 {
				t.Fatal("changed original startup authority reached another child", err)
			}
		})
	}
}

func TestProjectSessionPassiveStartupEligibilityIsBoundedAndOriginal(t *testing.T) {
	for _, change := range []string{"install", "gateway-env", "all-scratch", "python-strip", "missing-state", "state-link", "gateway-dir", "stamp", "journal", "unicode-code", "empty-env"} {
		t.Run(change, func(t *testing.T) {
			f := newSessionHistoryFixture(t)
			overrides := map[string]string{}
			switch change {
			case "install":
				overrides["HERMES_INSTALL_ROOT"] = f.code
			case "gateway-env":
				overrides["HERMES_GATEWAY_LOCK_DIR"] = filepath.Join(f.state, "foreign")
			case "all-scratch":
				overrides["HERMES_SCRATCH_DIR"] = f.temporary
			case "python-strip":
				overrides["TMPDIR"] = "\x1c\x1f"
			case "missing-state":
				overrides["XDG_STATE_HOME"] = filepath.Join(f.state, "absent")
			case "state-link":
				alias := filepath.Join(f.state, "alias")
				if err := os.Symlink(f.state, alias); err != nil {
					if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
						t.Skip("Windows does not grant the symbolic-link privilege")
					}
					t.Fatal(err)
				}
				overrides["XDG_STATE_HOME"] = alias
			case "gateway-dir":
				if err := os.MkdirAll(filepath.Join(f.state, "hermes", "gateway-locks"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "stamp":
				historyWrite(t, filepath.Join(f.code, "install-stamp.json"), []byte(`{}`), 0o600)
			case "journal":
				historyWrite(t, filepath.Join(f.code, ".hermes-update-zip-swap"), []byte(`{}`), 0o600)
			case "unicode-code":
				original := filepath.Dir(f.code)
				moved := original + "-straße"
				if err := os.Rename(original, moved); err != nil {
					t.Fatal(err)
				}
				f.req.Client.ExecutablePath = filepath.Join(moved, "bin", filepath.Base(f.req.Client.ExecutablePath))
			}
			f.env.SessionEnvironment = clients.SessionPlan{Environment: overrides}.ChildEnvironment(f.env.SessionEnvironment)
			if change == "empty-env" {
				f.env.SessionEnvironment = []string{}
			}
			runner := &sessionInventoryRunner{}
			f.env.Runner = runner
			if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil || len(runner.calls) != 0 {
				t.Fatal("ineligible startup state reached native planning", err)
			}
		})
	}
}

func TestProjectSessionPassiveAbsenceKeepsOriginalHeldParents(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			f := newSessionHistoryFixture(t)
			if existing {
				if err := os.Mkdir(filepath.Join(f.state, "hermes"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			bundle, err := qualifyHermesBundle(f.req.Client.ExecutablePath)
			if err != nil {
				t.Fatal(err)
			}
			state, err := captureSessionPassive(t.Context(), bundle, f.env.SessionEnvironment)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := state.close(); err != nil {
					t.Error(err)
				}
			})
			name := filepath.Join(f.state, "hermes")
			if existing {
				name = filepath.Join(name, "gateway-locks")
			}
			if err := os.Mkdir(name, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := state.rejoin(t.Context()); err == nil {
				t.Fatal("original first-missing component was replaced")
			}
		})
	}
	if captured := sessionEnvironment([]string{}); captured == nil || len(captured) != 0 {
		t.Fatal("empty base acquired ambient environment")
	}
	if captured := sessionEnvironment(nil); !reflect.DeepEqual(captured, os.Environ()) {
		t.Fatal("nil base lost legacy ambient capture")
	}
}

func TestProjectSessionPassiveOriginalBudgetPreventsNativeStart(t *testing.T) {
	f := newSessionHistoryFixture(t)
	plan := f.plan(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := &sessionInventoryRunner{plan: plan}
	f.env.Runner = sessionHistoryRunner{runner, f.response}
	if err := New().VerifySession(ctx, f.env, plan); !errors.Is(err, context.Canceled) || len(runner.calls) != 0 {
		t.Fatal("expired original context reached native command", err)
	}
}
