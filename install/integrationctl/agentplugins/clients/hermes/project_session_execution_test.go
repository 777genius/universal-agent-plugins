package hermes

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func sessionInspectHistoryFixture(t *testing.T) sessionHistoryFixture {
	t.Helper()
	f := newSessionHistoryFixture(t)
	if runtime.GOOS == "windows" {
		historyWrite(t, filepath.Join(f.root, "logs", ".__agent.lock"), nil, 0o666)
	}
	return f
}

func TestProjectSessionInspectRetainsOpaqueHistoryThroughFinalCommand(t *testing.T) {
	f := sessionInspectHistoryFixture(t)
	f.req.Inspect = false
	if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil {
		t.Fatal("ordinary launch admitted inspect-only history")
	}
	f.req.Inspect = true
	plan := f.plan(t)
	runner := &sessionInventoryRunner{plan: plan}
	f.env.Runner = sessionHistoryRunner{runner, f.response}
	parent, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	execution, err := New().BeginSessionExecution(parent, f.env, plan)
	if err != nil || execution == nil {
		t.Fatal("original history capture failed", err)
	}
	t.Cleanup(func() { _ = execution.Close() })
	deadline, _ := parent.Deadline()
	if actual, ok := execution.Context().Deadline(); !ok || !actual.Equal(deadline) {
		t.Fatal("inspect renewed the original deadline")
	}
	f.env.Runner = execution.Runner()
	if err := New().VerifySession(execution.Context(), f.env, plan); err != nil {
		t.Fatal(err)
	}
	_, err = execution.Runner().Run(execution.Context(), legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...),
		Dir: plan.Directory, Env: plan.ChildEnvironment(f.env.SessionEnvironment), StdoutLimitBytes: 1 << 20})
	if err != nil || execution.Revalidate() != nil || len(runner.calls) != 4 {
		t.Fatal("original history did not survive all four metadata commands", err)
	}
	retained := execution.(*sessionInspectHistory).state.members[0].file
	if err := execution.Close(); err != nil {
		t.Fatal(err)
	}
	if err := retained.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original history handle remained live", err)
	}
	if !errors.Is(execution.Context().Err(), context.Canceled) || execution.Revalidate() == nil {
		t.Fatal("closed history retained execution authority")
	}
}

func TestProjectSessionInspectRejectsHistoryMutationAtEveryCommand(t *testing.T) {
	for index := range 4 {
		at := index + 1
		t.Run([]string{"version", "config-path", "inventory", "final"}[index], func(t *testing.T) {
			f := sessionInspectHistoryFixture(t)
			plan := f.plan(t)
			path := filepath.Join(f.root, "spawn-ledger.json")
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan, editAt: at, edit: func() {
				body := bytes.ReplaceAll(f.ledger, []byte("1700000002.5"), []byte("1700000003.5"))
				historyWrite(t, path, body, 0o600)
				if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
					t.Fatal(err)
				}
				after, err := os.Stat(path)
				if err != nil || !sameSessionMember(before, after) {
					t.Fatal("mutation did not preserve the existing metadata predicate", err)
				}
			}}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			execution, err := New().BeginSessionExecution(t.Context(), f.env, plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = execution.Close() }()
			f.env.Runner = execution.Runner()
			err = New().VerifySession(execution.Context(), f.env, plan)
			if at == 4 {
				if err != nil {
					t.Fatal("failed before the final-command mutation", err)
				}
				_, err = execution.Runner().Run(execution.Context(), legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...),
					Dir: plan.Directory, Env: plan.ChildEnvironment(f.env.SessionEnvironment), StdoutLimitBytes: 1 << 20})
			}
			if err == nil || len(runner.calls) != at {
				t.Fatal("same-size, same-mtime changed history was adopted", at, runner.calls, err)
			}
		})
	}
}

func TestProjectSessionInspectRefusesUnmeasuredHistoryBeforeNative(t *testing.T) {
	cases := []string{"ledger-only", "lock-only", "nonempty-lock", "oversize-ledger", "oversize-stderr", "unknown-lock"}
	if runtime.GOOS != "windows" {
		cases = append(cases, "lock-mode", "windows-lock")
	}
	for _, change := range cases {
		t.Run(change, func(t *testing.T) {
			f := sessionInspectHistoryFixture(t)
			path, body := filepath.Join(f.root, "spawn-ledger.json.lock"), []byte("changed")
			switch change {
			case "ledger-only":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "lock-only":
				if err := os.Remove(filepath.Join(f.root, "spawn-ledger.json")); err != nil {
					t.Fatal(err)
				}
			case "oversize-ledger":
				path, body = filepath.Join(f.root, "spawn-ledger.json"), make([]byte, 4097)
			case "oversize-stderr":
				path, body = filepath.Join(f.root, "logs", "mcp-stderr.log"), make([]byte, (1<<20)+1)
			case "unknown-lock":
				path, body = filepath.Join(f.root, "logs", ".__errors.lock"), nil
			case "windows-lock":
				path, body = filepath.Join(f.root, "logs", ".__agent.lock"), nil
			case "lock-mode":
				if err := os.Chmod(path, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if change != "ledger-only" && change != "lock-only" && change != "lock-mode" {
				historyWrite(t, path, body, 0o644)
			}
			if _, err := New().PlanSession(t.Context(), f.env, f.req); err == nil {
				t.Fatal("unmeasured history was admitted", change)
			}
		})
	}
}

func TestProjectSessionInspectPreservesAbsenceAndOriginalContext(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	plan, err := prepareProjectSession(t.Context(), req, owner)
	if err != nil {
		t.Fatal(err)
	}
	runner := &sessionInventoryRunner{plan: plan}
	env := sessionInventoryEnvironment(t, plan, runner)
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if execution, err := New().BeginSessionExecution(canceled, env, plan); !errors.Is(err, context.Canceled) || execution != nil {
		t.Fatal("pre-canceled capture retained authority", err)
	}
	execution, err := New().BeginSessionExecution(t.Context(), env, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = execution.Close() }()
	env.Runner = execution.Runner()
	if err := New().VerifySession(t.Context(), env, plan); err == nil || len(runner.calls) != 0 {
		t.Fatal("different context reached a native command", err)
	}
	for _, name := range []string{"spawn-ledger.json", "spawn-ledger.json.lock"} {
		mode := os.FileMode(0o600)
		if strings.HasSuffix(name, ".lock") {
			mode = 0o644
		}
		historyWrite(t, filepath.Join(projectSessionRoot(req.ProjectRoot), name), nil, mode)
	}
	if err := execution.Revalidate(); err == nil || !strings.Contains(err.Error(), "appeared after original absence") || len(runner.calls) != 0 {
		t.Fatal("new history replaced captured absence", err)
	}
}

func TestProjectSessionExecutionJoinsFirstPlanFileAndMtime(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "mtime", true: "same-bytes-file"}[replacement], func(t *testing.T) {
			f := sessionInspectHistoryFixture(t)
			plan := f.plan(t)
			path := filepath.Join(f.root, "spawn-ledger.json")
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			stamp := before.ModTime().Add(time.Second)
			if replacement {
				if err := os.Rename(path, filepath.Join(t.TempDir(), "original")); err != nil {
					t.Fatal(err)
				}
				historyWrite(t, path, f.ledger, 0o600)
				stamp = before.ModTime()
			}
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan}
			f.env.Runner = sessionHistoryRunner{runner, f.response}
			if execution, err := New().BeginSessionExecution(t.Context(), f.env, plan); err == nil || execution != nil {
				t.Fatal("Begin adopted changed first-plan history", err)
			}
			if err := New().VerifySession(t.Context(), f.env, plan); err == nil || len(runner.calls) != 0 {
				t.Fatal("changed first-plan history reached a child", err)
			}
			if fresh := f.plan(t); fresh.ConfigDigest == plan.ConfigDigest {
				t.Fatal("new valid capture did not distinguish original metadata")
			}
		})
	}
}

func TestProjectSessionExecutionKeepsOrdinaryBootstrapAndHistoryAbsence(t *testing.T) {
	for _, addHistory := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary-bootstrap", true: "new-history"}[addHistory], func(t *testing.T) {
			req, owner := newProjectSessionFixture(t)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan, editAt: 1, edit: func() { sessionBootstrapFixture(t, req.ProjectRoot) }}
			env := sessionInventoryEnvironment(t, plan, runner)
			execution, err := New().BeginSessionExecution(t.Context(), env, plan)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = execution.Close() }()
			env.Runner = execution.Runner()
			if err := New().VerifySession(execution.Context(), env, plan); err != nil || len(runner.calls) != 3 {
				t.Fatal(err)
			}
			if execution.(*sessionInspectHistory).state.logs != nil {
				t.Fatal("new logs directory replaced original absence authority")
			}
			if addHistory {
				historyWrite(t, filepath.Join(projectSessionRoot(req.ProjectRoot), "logs", "mcp-stderr.log"), nil, 0o644)
			}
			_, err = execution.Runner().Run(execution.Context(), legacyports.Command{Argv: append([]string{plan.Executable}, plan.Arguments...),
				Dir: plan.Directory, Env: plan.ChildEnvironment(sessionEnvironment(env.SessionEnvironment)), StdoutLimitBytes: 1 << 20})
			if addHistory && (err == nil || len(runner.calls) != 3) || !addHistory && (err != nil || len(runner.calls) != 4) {
				t.Fatal("ordinary bootstrap/history absence boundary changed", err)
			}
		})
	}
}
