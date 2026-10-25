//go:build linux

package main

import (
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestFixtureAncestorsHaveExplicitSealedOrWritableRoles(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0755); err != nil {
		t.Fatal(err)
	}
	c := &controller{request: sealedRequest{Base: base, Input: inputRequest{UID: os.Getuid(), GID: os.Getgid()}}}
	if err := c.fixtures(); err != nil {
		t.Fatal(err)
	}
	// The builder UID is root in the real guarded controller. Running this pure
	// fixture test as an ordinary CI user still checks actual owner roles/modes.
	roles := map[string]os.FileMode{base: 0755, filepath.Join(base, "probes"): 0755, filepath.Join(base, "home"): 0755}
	for _, label := range probeLabels {
		root := filepath.Join(base, "probes", label)
		roles[root] = 0755
		for _, leaf := range []string{"profile", "extensions", "results"} {
			roles[filepath.Join(root, leaf)] = 0700
		}
		roles[filepath.Join(root, "profile", "User")] = 0755
		for _, cache := range []string{"globalStorage", "workspaceStorage", "History", "snippets"} {
			roles[filepath.Join(root, "profile", "User", cache)] = 0700
		}
	}
	for path, want := range roles {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		uid, err := statUID(info)
		if err != nil || uid != uint32(os.Getuid()) || !info.IsDir() || info.Mode().Perm() != want {
			t.Fatalf("native path authority %s: uid=%d mode=%v error=%v", path, uid, info.Mode(), err)
		}
		if want == 0755 && info.Mode().Perm()&0005 != 0005 {
			t.Fatal("sealed ancestor is not traversable", path)
		}
	}
}
func TestImmutableDirectoryNeedsOwnerModesAndNarrowStickyException(t *testing.T) {
	path := t.TempDir()
	owner := uint32(os.Getuid())
	for _, tc := range []struct {
		mode            os.FileMode
		sticky, allowed bool
	}{{0755, false, true}, {0777, false, false}, {0777 | os.ModeSticky, false, false}, {0777 | os.ModeSticky, true, true}} {
		if err := os.Chmod(path, tc.mode); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if immutableDirectory(info, owner, tc.sticky) != tc.allowed || immutableDirectory(info, owner+1, tc.sticky) {
			t.Fatalf("owner/mode authority mismatch mode=%v", info.Mode())
		}
	}
}
func TestReadyGateWaitsForCompleteSingleLinkAndPreservesForeignLeaves(t *testing.T) {
	root := t.TempDir()
	final := filepath.Join(root, "observation.json")
	pending := filepath.Join(root, ".observation.pending")
	ready := filepath.Join(root, "observation.ready")
	if err := requireFreshResultPaths(final); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pending, []byte(`{"partial":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := admitReadyResult(final); err != nil || ok {
		t.Fatalf("pending became observation %t %v", ok, err)
	}
	if err := os.WriteFile(pending, []byte(`{"complete":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(pending, final); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := admitReadyResult(final); err != nil || ok {
		t.Fatalf("two-link intermediate admitted %t %v", ok, err)
	}
	if err := os.Remove(pending); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ready, nil, 0600); err != nil {
		t.Fatal(err)
	}
	body, ok, err := admitReadyResult(final)
	if err != nil || !ok || string(body) != `{"complete":true}` {
		t.Fatalf("final not admitted %q %t %v", body, ok, err)
	}
	if err := requireFreshResultPaths(final); err == nil {
		t.Fatal("preexisting protocol adopted")
	}
	if err := os.Remove(ready); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(final, ready); err != nil {
		t.Fatal(err)
	}
	if _, _, err := admitReadyResult(final); err == nil {
		t.Fatal("aliased ready adopted")
	}
	if string(mustReadInert(t, final)) != `{"complete":true}` {
		t.Fatal("foreign final was modified")
	}
}
func mustReadInert(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
func TestObserverPinsMatchEveryExactCopiedSource(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "scripts", "vscode-workspace-observer")
	if len(observerPins) != 4 {
		t.Fatal("observer closure differs")
	}
	for _, name := range []string{"package.json", "extension.cjs", "shape.cjs", "runner.cjs"} {
		digest, err := hashFile(filepath.Join(root, name), 1<<20)
		if err != nil || observerPins[name] != digest {
			t.Fatalf("observer source pin mismatch %s %v", name, err)
		}
	}
}
func TestDisplayMetadataRejectsWrongPIDOwnerTypeAndReplacedAsset(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "uap-vsc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	lock := filepath.Join(root, "lock")
	body := []byte(strconv.Itoa(os.Getpid()) + "\n")
	if err := os.WriteFile(lock, body, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", filepath.Join(root, "private-inert-socket"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	lockInfo, err := os.Lstat(lock)
	if err != nil {
		t.Fatal(err)
	}
	socketInfo, err := os.Lstat(filepath.Join(root, "private-inert-socket"))
	if err != nil {
		t.Fatal(err)
	}
	proof := isolationProof{PID: os.Getpid(), UID: os.Getuid()}
	if err := validateDisplayReady(body, lockInfo, socketInfo, proof); err != nil {
		t.Fatal(err)
	}
	bad := proof
	bad.PID++
	if validateDisplayReady(body, lockInfo, socketInfo, bad) == nil {
		t.Fatal("foreign server PID admitted")
	}
	bad = proof
	bad.UID++
	if validateDisplayReady(body, lockInfo, socketInfo, bad) == nil {
		t.Fatal("foreign server owner admitted")
	}
	if validateDisplayReady(body, lockInfo, lockInfo, proof) == nil {
		t.Fatal("ordinary file became display socket")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, body, 0600); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.Lstat(lock)
	if err != nil {
		t.Fatal(err)
	}
	if sameFile(lockInfo, replacement) {
		t.Fatal("replacement retained display-unlink authority")
	}
	if string(mustReadInert(t, lock)) != string(body) {
		t.Fatal("replacement bytes changed")
	}
}
func TestFixedTrustedCommandsKeepIsolatedLiteralSyntaxAndClosedOperands(t *testing.T) {
	base := "/tmp/uap-vscode-sealed-inert"
	args := []string{"-I", "-c", cleanupCode, base, "1", "2"}
	cmd, err := fixedSystemCommand(t.Context(), "/usr/bin/python3", args)
	if err != nil || !reflect.DeepEqual(cmd.Args, append([]string{"/usr/bin/python3"}, args...)) {
		t.Fatalf("fixed cleanup argv %v %v", cmd, err)
	}
	args[2] = "unreviewed script"
	if _, err := fixedSystemCommand(t.Context(), "/usr/bin/python3", args); err == nil {
		t.Fatal("unreviewed cleanup code admitted")
	}
	for _, path := range []string{"/usr/bin/bash", "/bin/python3"} {
		if _, err := fixedSystemCommand(t.Context(), path, nil); err == nil {
			t.Fatal("unreviewed program admitted", path)
		}
	}
	if _, err := fixedSystemCommand(t.Context(), "/usr/bin/xauth", []string{"-f", base + "/auth/authority", "source", "-"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixedSystemCommand(t.Context(), "/usr/bin/xauth", []string{"-f", "/tmp/foreign", "source", "-"}); err == nil {
		t.Fatal("foreign xauth operand admitted")
	}
}
func TestWatchedFixtureParentAliasRefusesDespiteUnchangedOrdinaryLeaf(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "profile")
	user := filepath.Join(profile, "User")
	if err := os.MkdirAll(user, 0755); err != nil {
		t.Fatal(err)
	}
	watched := filepath.Join(user, "settings.json")
	body := []byte(`{"chat.mcp.autostart":"never"}`)
	if err := os.WriteFile(watched, body, 0444); err != nil {
		t.Fatal(err)
	}
	if _, err := exactDirectory(filepath.Dir(watched), os.Getuid()); err != nil {
		t.Fatal("ordinary fixture ancestry refused", err)
	}
	if err := os.Rename(user, user+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("User-original", user); err != nil {
		t.Fatal(err)
	}
	if string(mustReadInert(t, watched)) != string(body) {
		t.Fatal("alias fixture changed leaf bytes")
	}
	if _, err := snapshot([]string{watched}); err == nil || !strings.Contains(err.Error(), "directory ancestor is not ordinary") {
		t.Fatalf("watched file parent alias admitted or refused for the wrong reason: %v", err)
	}
}
