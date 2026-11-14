//go:build linux

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestReadonlyModePreservesOriginalDropChainAndRejectsOtherAuthority(t *testing.T) {
	r := inertRequest()
	systems := map[string]systemFact{}
	for _, name := range researchSystemNames("") {
		path := "/usr/bin/" + name
		if name == "python3" {
			path = "/usr/bin/python3.12"
		}
		systems[name] = systemFact{Path: path, SHA: strings.Repeat("a", 64)}
	}
	original, err := fixedCommand("/tmp/uap-grok-acp-sealed-owned", "/tmp/input", r, systems)
	if err != nil {
		t.Fatal(err)
	}
	systems["bwrap"] = systemFact{Path: "/usr/bin/bwrap", SHA: strings.Repeat("b", 64)}
	if _, err := fixedCommand("/tmp/uap-grok-acp-sealed-owned", "/tmp/input", r, systems); err == nil {
		t.Fatal("old mode accepted a fifth executable")
	}
	r.Mode = readonlyMode
	selected, err := fixedCommand("/tmp/uap-grok-acp-sealed-owned", "/tmp/input", r, systems)
	if err != nil || !reflect.DeepEqual(selected, original) {
		t.Fatal("readonly changed the existing root drop chain", selected, err)
	}
	for _, change := range []func(*request){
		func(r *request) { r.Scope = "user" },
		func(r *request) { r.Scope = "b" },
		func(r *request) { r.Phase = "locked-writer" },
	} {
		changed := r
		change(&changed)
		if _, err := fixedCommand("/tmp/sealed", "/tmp/input", changed, systems); err == nil {
			t.Fatal("readonly admitted another scope or phase")
		}
	}
	systems["bwrap"] = systemFact{Path: "/tmp/bwrap", SHA: strings.Repeat("b", 64)}
	if _, err := fixedCommand("/tmp/sealed", "/tmp/input", r, systems); err == nil {
		t.Fatal("readonly admitted an alternate primitive")
	}
	delete(systems, "bwrap")
	if _, err := fixedCommand("/tmp/sealed", "/tmp/input", r, systems); err == nil {
		t.Fatal("readonly fell back without bwrap")
	}
}

func TestReadonlyBudgetsRetainOneOriginWithoutChangingOldReserve(t *testing.T) {
	if researchActiveLimit(readonlyMode) != 25*time.Second {
		t.Fatal("readonly active deadline changed")
	}
	for _, mode := range []string{"", sessionMode, lockMode, cliMode} {
		if researchActiveLimit(mode) != 30*time.Second {
			t.Fatal("old active deadline changed", mode)
		}
	}
	now, err := monoNow()
	if err != nil {
		t.Fatal(err)
	}
	r := inertRequest()
	r.Start, r.Mode = now-int64(31*time.Second), readonlyMode
	if _, cancel, err := researchFinalBudget(t.Context(), r); err == nil {
		cancel()
		t.Fatal("readonly renewed its expired final deadline")
	}
	if err := readonlyFinalCheck(r); err == nil {
		t.Fatal("readonly publication admitted an expired original clock")
	}
	r.Mode = ""
	ctx, cancel, err := researchFinalBudget(t.Context(), r)
	if err != nil {
		t.Fatal("legacy 35-second reserve changed", err)
	}
	defer cancel()
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 4*time.Second || time.Until(deadline) < 2*time.Second {
		t.Fatal("legacy original reserve was renewed or shortened")
	}
	for _, limit := range []time.Duration{25 * time.Second, 30 * time.Second} {
		if _, err := remaining(10, 10+int64(limit), limit); err == nil {
			t.Fatal("exact expired boundary admitted")
		}
	}
}

func readonlyTestRoot(t *testing.T) (string, *os.Root) {
	t.Helper()
	path := t.TempDir()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(resolved)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return resolved, root
}

func TestReadonlyOverflowFitsSignedIdentityWithoutCoercion(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"ordinary", "65534\n", 65534},
		{"signed-maximum", "2147483647\n", 2147483647},
		{"signed-overflow", "2147483648\n", 0},
		{"unsigned-maximum", "4294967295\n", 0},
		{"zero", "0\n", 0},
		{"negative", "-1\n", 0},
		{"leading-zero", "065534\n", 0},
		{"missing-newline", "65534", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "overflow-id")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := readonlyOverflow(path)
			if tc.want == 0 {
				if err == nil || got != 0 {
					t.Fatalf("unrepresentable or noncanonical identity admitted: got=%d err=%v", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("canonical bounded identity refused: got=%d want=%d err=%v", got, tc.want, err)
			}
		})
	}
}

func TestReadonlyInputRefusesAliasSpecialAndUnownedPermissions(t *testing.T) {
	path, root := readonlyTestRoot(t)
	leaf := filepath.Join(path, "leaf")
	if err := os.WriteFile(leaf, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, count, err := readonlyInput(t.Context(), root, os.Getuid(), os.Getgid()); err != nil || count != 2 {
		t.Fatal("original ordinary input refused", count, err)
	}
	for _, kind := range []string{"symlink", "hardlink", "fifo", "writable"} {
		t.Run(kind, func(t *testing.T) {
			member := filepath.Join(path, "extra")
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink("/outside/not-read", member)
			case "hardlink":
				err = os.Link(leaf, member)
			case "fifo":
				err = unix.Mkfifo(member, 0o600)
			case "writable":
				err = os.WriteFile(member, nil, 0o600)
				if err == nil {
					err = os.Chmod(member, 0o666)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(member); err != nil {
					t.Error(err)
				}
			}()
			if _, _, err := readonlyInput(t.Context(), root, os.Getuid(), os.Getgid()); err == nil {
				t.Fatal("unsafe input member admitted")
			}
		})
	}
}

func TestReadonlyInputDigestBindsOriginalFileIdentity(t *testing.T) {
	outer := t.TempDir()
	path := filepath.Join(outer, "selected")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(path, "leaf")
	if err := os.WriteFile(leaf, []byte("same bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = root.Close() }()
	before, count, err := readonlyInput(t.Context(), root, os.Getuid(), os.Getgid())
	if err != nil || count != 2 {
		t.Fatal(err)
	}
	if err := os.Rename(leaf, filepath.Join(outer, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leaf, []byte("same bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, afterCount, err := readonlyInput(t.Context(), root, os.Getuid(), os.Getgid())
	if err != nil || afterCount != count || before == after {
		t.Fatal("same-byte replacement became original authority", err)
	}
	body, err := os.ReadFile(filepath.Join(outer, "original"))
	if err != nil || string(body) != "same bytes" {
		t.Fatal("original retained file changed", err)
	}
}

func TestReadonlyInputBoundsAndCancellationRefuseBeforeTraversal(t *testing.T) {
	rows := make(map[string][]int64, 4096)
	for index := range 4096 {
		rows[strconv.Itoa(index)] = nil
	}
	if err := readonlyInputRows(t.Context(), nil, ".", 1000, 1000, rows); err == nil {
		t.Fatal("full census began another traversal")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := readonlyInput(ctx, nil, 1000, 1000); err == nil {
		t.Fatal("canceled census began traversal")
	}
}

func TestReadonlyReceiptRetainsOriginalGrantAndRejectsMissingProof(t *testing.T) {
	now, err := monoNow()
	if err != nil {
		t.Fatal(err)
	}
	r := inertRequest()
	r.Mode, r.Start = readonlyMode, now
	grant := &readonlyGrant{InputSHA: strings.Repeat("a", 64), InputCount: 2}
	state := &scopeState{readonly: grant}
	body, err := json.Marshal(map[string]any{"readonly": map[string]any{"authority": grant}})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.verifyReadonlyResult(body, r); err != nil {
		t.Fatal("same recorded grant refused", err)
	}
	changed := strings.Replace(string(body), strings.Repeat("a", 64), strings.Repeat("b", 64), 1)
	if err := state.verifyReadonlyResult([]byte(changed), r); err == nil {
		t.Fatal("new grant replaced original authority")
	}
	if err := (&scopeState{}).verifyReadonlyResult(body, r); err == nil {
		t.Fatal("missing original authority admitted")
	}
	r.Start = now - int64(30*time.Second)
	if err := state.verifyReadonlyResult(body, r); err == nil {
		t.Fatal("late proof admitted")
	}
}

func TestReadonlyClosedProofDoesNotAddFieldsToOldResultSchema(t *testing.T) {
	r := inertRequest()
	raw := inertResult(t, r)
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	body["readonly"] = map[string]any{"authority": map[string]any{}, "stage": "runtime",
		"control_complete": false, "original": nil, "control_marker": nil}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(encoded, r, 1); err == nil {
		t.Fatal("old result schema accepted readonly proof")
	}
	r.Mode = readonlyMode
	body["mode"] = readonlyMode
	encoded, err = json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(encoded, r, 1); err != nil {
		t.Fatal("closed negative readonly result refused", err)
	}
	body["semantic_complete"] = nil
	encoded, err = json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(encoded, r, 1); err == nil {
		t.Fatal("null semantic flag became false")
	}
}
