//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestQoderCycleArgvAcceptsOnlyExactHeldOrOneSourceEmbeddedEntrypoint(t *testing.T) {
	expected := make([]string, 0, 7)
	expected = append(expected, "/held/qodercli", "plugins", "list", "--json", "--plugin-dir", "/held/view")
	cases := []struct {
		args []string
		form string
	}{
		{append([]string{}, expected...), "direct"},
		{append([]string{expected[0], "/$bunfs/root/qodercli.js"}, expected[1:]...), "compiled-embedded"},
		{append([]string{"bun", "/$bunfs/root/qodercli"}, expected[1:]...), "compiled-embedded"},
		{nil, "refused"},
		{append([]string{"/usr/bin/bun", "/$bunfs/root/qodercli"}, expected[1:]...), "refused"},
		{append([]string{"bun", "/$bunfs/root/qodercli.js"}, expected[1:]...), "refused"},
		{append([]string{"bun", "/$bunfs/root/./qodercli"}, expected[1:]...), "refused"},
		{append([]string{"bun", "/$bunfs/root/qodercli", "foreign"}, expected[1:]...), "refused"},
		{[]string{"bun", "/$bunfs/root/qodercli"}, "refused"},
		{append([]string{"/foreign/qodercli", "/$bunfs/root/qodercli.js"}, expected[1:]...), "refused"},
		{append([]string{expected[0], "/private/build/entry.js"}, expected[1:]...), "refused"},
		{append([]string{expected[0], "/$bunfs/root/../entry.js"}, expected[1:]...), "refused"},
		{append([]string{expected[0], "/$bunfs/root/entry\n.js"}, expected[1:]...), "refused"},
		{append([]string{expected[0], "/$bunfs/root/entry.js", "foreign"}, expected[1:]...), "refused"},
	}
	for i, tc := range cases {
		if got := qoderCycleArgvForm(tc.args, expected); got != tc.form {
			t.Fatal(i, got, tc.form)
		}
	}
	if qoderCycleArgvForm(nil, nil) != "refused" || qoderCycleArgvForm(expected, append(expected, "foreign")) != "refused" {
		t.Fatal("absent or changed argument authority")
	}
}

func qoderCycleLogInert(t *testing.T) (*qoderCycleLogs, []string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".qoder")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	recipe := map[string]string{".": "directory:20000000700"}
	logs, err := newQoderCycleLogs(context.Background(), root, recipe)
	if err != nil {
		t.Fatal(err)
	}
	recipe["PRIVATE_FOREIGN"] = "unknown"
	if len(logs.original) != 1 || len(logs.expected) != 1 {
		t.Fatal("original recipe aliased caller state")
	}
	return logs, []string{"/held/qodercli", "plugins", "list", "--json"}
}

func qoderCycleLogAuthor(t *testing.T, l *qoderCycleLogs, args []string, number int) (time.Time, time.Time, string) {
	t.Helper()
	stamp := time.Date(2026, 10, 28, 12, 34, number, 789000000, time.UTC)
	id := fmt.Sprintf("2026-10-28T12-34-%02d-789Z-a1b2c3-p%d", number, 12345+number)
	root := filepath.Join(l.root, "logs", "runs", id)
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"run_id": id, "started_at": stamp.Format(qoderLogTimeLayout), "pid": 12345 + number, "ppid": 12, "process_role": "main", "cli_version": "1.1.78", "node_version": "v24.0.0", "platform": "linux", "arch": "x64", "cwd": l.root, "project_id": "uap-qoder-private-project", "argv": args, "env_flags": map[string]any{"debug": false, "remote_worker": false, "ci": true}, "log_files": map[string]any{"process": "qodercli.log"}})
	projectNativeRuntimeWrite(t, filepath.Join(root, "manifest.json"), body, 0644)
	projectNativeRuntimeWrite(t, filepath.Join(root, "qodercli.log"), []byte("uap-qoder-private-log-body\n"), 0644)
	latest := filepath.Join(l.root, "logs", "latest")
	if number > 1 {
		if err := os.Remove(latest); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("runs/"+id, latest); err != nil {
		t.Fatal(err)
	}
	return stamp.Add(-time.Millisecond), stamp.Add(time.Millisecond), id
}

func TestQoderCycleLogLedgerDerivesFirstAndAccumulatedRunsWithoutBaselineAdoption(t *testing.T) {
	l, expected := qoderCycleLogInert(t)
	args := append([]string{expected[0], "/$bunfs/root/qodercli.js"}, expected[1:]...)
	before, _ := json.Marshal(l.original)
	for number := 1; number <= 2; number++ {
		if number == 2 {
			args = append([]string{"bun", "/$bunfs/root/qodercli"}, expected[1:]...)
		}
		start, end, _ := qoderCycleLogAuthor(t, l, args, number)
		transition, err := l.native(l.root, expected, start, end)
		if err != nil {
			t.Fatal(err)
		}
		if transition.Kind != "one-run" || transition.Form != "compiled-embedded" || transition.Run == nil || transition.Run.PIDProvenance != "manifest-only" || !transition.Run.ExactAddedRunClosure || !transition.Run.PreexistingRecordsStable || len(l.runs) != number {
			t.Fatal("source-defined run transition differs")
		}
		raw, _ := json.Marshal(transition)
		if bytes.Contains(raw, []byte("uap-qoder-private-")) || bytes.Contains(raw, []byte("$bunfs")) || len(raw) > 16<<10 {
			t.Fatal("log/argv/private data escaped bounded projection")
		}
		if err := l.check(); err != nil {
			t.Fatal(err)
		}
	}
	after, _ := json.Marshal(l.original)
	if !bytes.Equal(before, after) || len(l.expected) != 10 {
		t.Fatal("original map renewed or accumulated closure differs")
	}
	path := filepath.Join(l.root, "logs", "runs", l.runs[0], "qodercli.log")
	if err := os.WriteFile(path, []byte("changed private bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if l.check() == nil {
		t.Fatal("old admitted task run was renewed")
	}
}

func TestQoderCycleLogLedgerRefusesForeignOldAliasAndUnknownNativeChanges(t *testing.T) {
	for _, fault := range []string{"extra", "cwd", "argv", "time", "manifest-link", "latest-alias", "mode"} {
		t.Run(fault, func(t *testing.T) {
			l, expected := qoderCycleLogInert(t)
			args := append([]string{expected[0], "/$bunfs/root/qodercli.js"}, expected[1:]...)
			start, end, id := qoderCycleLogAuthor(t, l, args, 1)
			cwd := l.root
			switch fault {
			case "extra":
				projectNativeRuntimeWrite(t, filepath.Join(l.root, "PRIVATE_FOREIGN"), []byte("private"), 0600)
			case "cwd":
				cwd = "/foreign"
			case "argv":
				expected = append(expected, "foreign")
			case "time":
				start = start.Add(time.Second)
				end = end.Add(time.Second)
			case "manifest-link":
				path := filepath.Join(l.root, "logs", "runs", id, "manifest.json")
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".original", path); err != nil {
					t.Fatal(err)
				}
			case "latest-alias":
				path := filepath.Join(l.root, "logs", "latest")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("runs/../runs/"+id, path); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(filepath.Join(l.root, "logs", "runs", id, "qodercli.log"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			old := map[string]string{}
			for key, value := range l.expected {
				old[key] = value
			}
			if _, err := l.native(cwd, expected, start, end); err == nil {
				t.Fatal("foreign/unmeasured log transition accepted")
			}
			if !reflect.DeepEqual(old, l.expected) || len(l.runs) != 0 {
				t.Fatal("refused native inventory became owned")
			}
		})
	}
	l, _ := qoderCycleLogInert(t)
	projectNativeRuntimeWrite(t, filepath.Join(l.root, "PRIVATE"), []byte("unchanged-only must refuse"), 0600)
	if _, err := l.unchanged(); err == nil {
		t.Fatal("public call may not acquire hypothetical logger allowance")
	}
	if _, err := newQoderCycleLogs(context.Background(), l.root, map[string]string{".": "directory:20000000700"}); err == nil {
		t.Fatal("initial foreign profile record adopted")
	}
}

func TestQoderCyclePrivacyChecksRawBeforeBase64AndEscapedStrings(t *testing.T) {
	for _, raw := range [][]byte{[]byte("uap-qoder-private-canary"), []byte(`{"safe":"uap-qoder-\u0070rivate-canary"}`), {0xff}, bytes.Repeat([]byte{'x'}, (1<<20)+1)} {
		if !qoderCyclePrivate(raw) {
			t.Fatal("private/raw boundary widened")
		}
	}
	for _, raw := range [][]byte{nil, []byte("ordinary fixture\n"), []byte(`{"safe":"fixture"}`)} {
		if qoderCyclePrivate(raw) {
			t.Fatal("safe raw refused")
		}
	}
}
