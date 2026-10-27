//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func windComponentInert(t *testing.T) *windComponentFixture {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	base := &nativeFixture{Root: root, Project: filepath.Join(root, "project"), Home: filepath.Join(root, "home"), Probe: filepath.Join(root, "probe"), Events: filepath.Join(root, "events.jsonl"), Nonce: "inert-private-nonce"}
	f := &projectNativeCLIAttempt{base: base, spec: projectNativeCLISpec{client: "windsurf", version: "3000.11.205"}, started: time.Now(), ctx: ctx, cancel: cancel, client: filepath.Join(root, "vendor-bin", "devin"), installer: filepath.Join(root, "installer"), git: filepath.Join(root, "git"), other: filepath.Join(root, "project-b"), outside: filepath.Join(root, "outside"), used: map[string]bool{}, files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, evidence: map[string]any{}}
	for _, role := range windProfileRoles {
		if err := os.Mkdir(filepath.Join(root, role), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{base.Project, f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{base.Probe, f.installer} {
		projectNativeRuntimeWrite(t, path, []byte("inert owned bytes; never executed\n"), 0700)
		f.immutable[path] = nativeSHA(t, path)
	}
	updater, body := projectNativeCLIUpdater("windsurf")
	path := filepath.Join(root, "xdg-config", filepath.FromSlash(updater))
	projectNativeRuntimeWrite(t, path, body, 0600)
	f.protected[path] = nativeSHA(t, path)
	w := newWindComponentFixture(f)
	w.prepare(t)
	return w
}
func TestWindsurfComponentCommandsKeepExactClosedAuthority(t *testing.T) {
	w := windComponentInert(t)
	native, public, git := 0, 0, 0
	for _, label := range windComponentLabels {
		selected, err := w.selectCommand(label)
		if err != nil || w.admitCommand(label, selected) != nil {
			t.Fatal("closed selected command", label, err)
		}
		if selected.Native {
			native++
		} else if strings.HasPrefix(label, "add-") {
			public++
		} else {
			git++
		}
		for _, mutation := range []string{"cwd", "argv", "timeout", "native", "scope"} {
			changed := selected
			changed.Argv = append([]string{}, selected.Argv...)
			switch mutation {
			case "cwd":
				changed.CWD = w.f.base.Project + "/other"
			case "argv":
				changed.Argv = append(changed.Argv, "--respect-workspace-trust=false")
			case "timeout":
				changed.Timeout++
			case "native":
				changed.Native = !changed.Native
			case "scope":
				changed.Scope = "unknown"
			}
			if w.admitCommand(label, changed) == nil {
				t.Fatal("wrong command accepted", label, mutation)
			}
		}
		w.completed = append(w.completed, label)
		w.f.used[label] = true
		if w.admitCommand(label, selected) == nil {
			t.Fatal("repeated command accepted")
		}
	}
	if native != 18 || public != 3 || git != 2 || len(w.completed) != 23 {
		t.Fatal("closed component count")
	}
	if _, err := w.selectCommand("native-attempt"); err == nil {
		t.Fatal("old startup route admitted")
	}
	if w.f.used["native-attempt"] {
		t.Fatal("old diagnostic eligibility flag changed")
	}
}
func TestWindsurfComponentFullProfileSnapshotsAreNonmutatingAndPrivate(t *testing.T) {
	w := windComponentInert(t)
	root := w.f.base.Root
	path := filepath.Join(root, "home", "PRIVATE-CONTROL\nname")
	projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY https://private.invalid/token"), 0600)
	before, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("snapshot changed profile", err)
	}
	if err = os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(root, "home", ".codeium", "windsurf", "skills", "foreign-user", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		projectNativeRuntimeWrite(t, filepath.Join(root, "xdg-cache", string(rune('a'+i))), []byte("added"), 0600)
	}
	changed, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	d := windProfileDifference(before, changed)
	if d.Total.Added != 4 || d.Total.Removed != 1 || d.Total.Changed < 3 || len(d.Details) > 3 || d.Omitted != d.Total.Added+d.Total.Removed+d.Total.Changed-len(d.Details) {
		t.Fatal("exact complete profile delta counts", d)
	}
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > 2<<10 || strings.Contains(string(raw), "PRIVATE") || strings.Contains(string(raw), root) || strings.Contains(string(raw), "private.invalid") {
		t.Fatal("finite profile projection privacy")
	}
	again, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil || !reflect.DeepEqual(changed, again) {
		t.Fatal("projection mutated profile")
	}
}
func TestWindsurfComponentProfileRefusesAliasesLinksSpecialFilesAndBounds(t *testing.T) {
	for _, kind := range []string{"symlink", "ancestor-alias", "hardlink", "fifo", "owner", "file-bound", "entries", "depth", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			w := windComponentInert(t)
			root := w.f.base.Root
			path := filepath.Join(root, "tmp", "hostile")
			switch kind {
			case "symlink":
				if err := os.Symlink(w.f.base.Probe, path); err != nil {
					t.Fatal(err)
				}
			case "ancestor-alias":
				if err := os.Rename(filepath.Join(root, "tmp"), filepath.Join(root, "tmp-real")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "tmp-real"), filepath.Join(root, "tmp")); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(w.f.base.Probe, path); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				windContractFIFO(t, path)
			case "owner":
				projectNativeRuntimeWrite(t, path, []byte("owner"), 0600)
				if os.Getuid() != 0 {
					t.Skip("actual owner-change control requires root")
				}
				if err := os.Chown(path, 1, 1); err != nil {
					t.Fatal(err)
				}
			case "file-bound":
				file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				truncateErr := file.Truncate((64 << 20) + 1)
				closeErr := file.Close()
				if truncateErr != nil || closeErr != nil {
					t.Fatal("sparse bound fixture")
				}
			case "entries":
				for i := range 4097 {
					name := filepath.Join(root, "tmp", time.Unix(int64(i), 0).Format("150405"))
					projectNativeRuntimeWrite(t, name, []byte{}, 0600)
				}
			case "depth":
				for i := range 65 {
					path = filepath.Join(path, "d")
					_ = i
				}
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "deadline":
				ctx, cancel := context.WithCancel(w.f.ctx)
				cancel()
				w.f.ctx = ctx
			}
			got, err := windProfileSnapshot(w.f.ctx, root)
			if err == nil || got != nil {
				t.Fatal("hostile complete frame admitted", kind)
			}
			if strings.Contains(err.Error(), root) || strings.Contains(err.Error(), "hostile") {
				t.Fatal("private path leaked")
			}
		})
	}
}
