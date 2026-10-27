//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Only the two Kimi constructors call this on their newly created fixture root,
// before any external baseline. Python's later private-profile check stays strict.
func kimiProjectPrivateFixtureRoot(root string) (result error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
		return fmt.Errorf("Kimi fresh fixture root is not canonical")
	}
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return fmt.Errorf("Kimi fresh fixture root has an alias")
	}
	before, err := os.Lstat(root)
	if err != nil || !kimiProjectFreshRootOwner(before) || (before.Mode() != os.ModeDir|0700 && before.Mode() != os.ModeDir|0755) {
		return fmt.Errorf("Kimi fresh fixture root type, owner or mode differs")
	}
	file, err := os.OpenFile(root, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("Kimi fresh fixture root could not be held")
	}
	defer func() {
		if err := file.Close(); err != nil {
			result = fmt.Errorf("Kimi fresh fixture root close failed")
		}
	}()
	held, err := file.Stat()
	if err != nil || !kimiProjectFreshRootMatches(before, held, before.Mode()) {
		return fmt.Errorf("Kimi fresh fixture root held identity differs")
	}
	if held.Mode() != os.ModeDir|0700 {
		if err := file.Chmod(0700); err != nil {
			return fmt.Errorf("Kimi fresh fixture root private mode could not be authored")
		}
	}
	after, heldErr := file.Stat()
	current, pathErr := os.Lstat(root)
	if heldErr != nil || pathErr != nil || !kimiProjectFreshRootMatches(before, after, os.ModeDir|0700) || !kimiProjectFreshRootMatches(after, current, os.ModeDir|0700) {
		return fmt.Errorf("Kimi fresh fixture root final identity differs")
	}
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return fmt.Errorf("Kimi fresh fixture root final ancestor differs")
	}
	return nil
}

func kimiProjectFreshRootMatches(expected, current os.FileInfo, mode os.FileMode) bool {
	return kimiProjectFreshRootOwner(current) && os.SameFile(expected, current) && current.Mode() == mode
}

func kimiProjectFreshRootOwner(info os.FileInfo) bool {
	if info == nil || !info.IsDir() {
		return false
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	return ok && identity.Uid == uint32(os.Getuid()) && identity.Gid == uint32(os.Getgid())
}

func TestKimiProjectNativePrivateRootAuthorsOnlyFreshOwnedDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "fresh-root")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "home", "foreign-config")
	projectNativeRuntimeWrite(t, child, []byte("held original child bytes\n"), 0600)
	before, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	wantTree := kimiProjectTree(t, filepath.Dir(child))
	var private os.FileInfo
	for range 2 {
		if err := kimiProjectPrivateFixtureRoot(root); err != nil {
			t.Fatal(err)
		}
		after, err := os.Lstat(root)
		if err != nil || !os.SameFile(before, after) || after.Mode() != os.ModeDir|0700 || !reflect.DeepEqual(wantTree, kimiProjectTree(t, filepath.Dir(child))) {
			t.Fatal("fresh Kimi root authoring changed identity or child authority")
		}
		if private != nil && !reflect.DeepEqual(private.Sys(), after.Sys()) {
			t.Fatal("private Kimi root was mutated on a repeated authoring check")
		}
		private = after
	}
}

func TestKimiProjectNativePrivateRootRefusesAliasesFilesAndUnknownModes(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatal(err)
	}
	leafAlias, ancestorAlias := filepath.Join(parent, "leaf-alias"), filepath.Join(parent, "ancestor-alias")
	if err := os.Symlink(target, leafAlias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parent, ancestorAlias); err != nil {
		t.Fatal(err)
	}
	ordinary := filepath.Join(parent, "ordinary")
	projectNativeRuntimeWrite(t, ordinary, []byte("unchanged ordinary bytes\n"), 0600)
	for _, root := range []string{leafAlias, filepath.Join(ancestorAlias, "target"), ordinary, "relative", "/"} {
		if err := kimiProjectPrivateFixtureRoot(root); err == nil {
			t.Fatalf("unsafe Kimi fixture root admitted: %s", root)
		}
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode() != os.ModeDir|0755 {
		t.Fatal("Kimi alias refusal changed its target mode")
	}
	for _, mode := range []os.FileMode{0777, 0750, 0700 | os.ModeSticky, 0755 | os.ModeSetgid, 0700 | os.ModeSetuid} {
		if err := os.Chmod(target, mode); err != nil {
			t.Fatal(err)
		}
		if err := kimiProjectPrivateFixtureRoot(target); err == nil {
			t.Fatalf("unknown Kimi root mode admitted: %v", mode)
		}
		info, err := os.Lstat(target)
		if err != nil || info.Mode() != os.ModeDir|mode {
			t.Fatal("Kimi refusal rewrote an unknown mode")
		}
	}
	body, err := os.ReadFile(ordinary)
	if err != nil || string(body) != "unchanged ordinary bytes\n" {
		t.Fatal("Kimi ordinary-file refusal changed bytes")
	}
}

func TestKimiProjectNativeWholeJSONRemainsBoundedAndExact(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"stage":"a","stage":"b"}`), []byte(`{"a":{"x":1,"x":2}}`),
		[]byte(`{"x":1e999}`), []byte(`{"x":NaN}`), []byte(`{} {}`), []byte(`[]`),
		{0xff}, bytes.Repeat([]byte(" "), kimiProjectNativeLimit+1),
		[]byte(`{"nested":` + strings.Repeat("[", 34) + strings.Repeat("]", 34) + `}`),
	} {
		if _, err := kimiProjectObject(body); err == nil {
			t.Fatalf("unsafe Kimi stage JSON admitted: %.80s", body)
		}
	}
	value, err := kimiProjectObject([]byte(`{"schema_version":1,"tools_call_count":0,"provider_configured":false}`))
	if err != nil || value["schema_version"] != json.Number("1") || value["tools_call_count"] != json.Number("0") || value["provider_configured"] != false {
		t.Fatalf("exact native count/version types lost: %v (%v)", value, err)
	}
}

func TestKimiProjectNativeEnvironmentKeepsOriginalHomeAndLiteralUpdateOptOut(t *testing.T) {
	t.Setenv("KIMI_CODE_NO_AUTO_UPDATE", "ambient-canary")
	t.Setenv("KIMI_API_KEY", "private-canary")
	root := t.TempDir()
	home := filepath.Join(root, "original-home")
	f := &kimiProjectNativeFixture{base: &nativeFixture{Root: root, Home: home}, client: filepath.Join(root, "client", "kimi")}
	environment := map[string]string{}
	for _, entry := range f.env() {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			t.Fatal("malformed controlled environment entry")
		}
		environment[key] = value
		if strings.Contains(value, "canary") {
			t.Fatal("ambient updater or credential entered native environment")
		}
	}
	if environment["HOME"] != home || environment["USERPROFILE"] != home || environment["KIMI_CODE_NO_AUTO_UPDATE"] != "1" {
		t.Fatal("original-home authority or literal update opt-out differs")
	}
}

func TestKimiProjectNativeMCPRequiresIndependentReceiptAndSourceAuthority(t *testing.T) {
	p := &kimiProjectNativePackage{Revision: "a-two", Data: "/owned/project/.uap/plugin-data/a", Binding: domain.ClientBinding{TargetLocator: "/owned/project/.uap/managed/a"}}
	active := p.Binding.TargetLocator
	good := map[string]any{"transport": "stdio", "command": filepath.Join(active, "io.github.777genius.agentplugins", "managed-stdio-v1", "agentplugins"), "args": []any{"--internal-stdio-v1", active, p.Data, active, "plugin", "--", "./bin/probe", "Kimi owned a", active + "/argument"}, "env": map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": p.Data, "UAP_TEST_ROOT": "/owned", "UAP_TEST_EVENTS": "/owned/a-events.jsonl", "UAP_TEST_NONCE": "nonce-a", "UAP_TEST_REVISION": "a-two"}}
	body, _ := json.Marshal(good)
	if err := kimiProjectValidateServer(good, p, "a", "/owned", "nonce"); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"launcher", "package", "data", "cwd", "anchor", "source-command", "source-argument", "nonce", "revision", "environment", "extra"} {
		actual, err := kimiProjectObject(body)
		if err != nil {
			t.Fatal(err)
		}
		args := actual["args"].([]any)
		env := actual["env"].(map[string]any)
		switch kind {
		case "launcher":
			actual["command"] = "/foreign/probe"
		case "package":
			args[1] = "/foreign/package"
		case "data":
			args[2] = "/foreign/data"
		case "cwd":
			args[3] = "/foreign/cwd"
		case "anchor":
			args[4] = "data"
		case "source-command":
			args[6] = "./bin/other"
		case "source-argument":
			args[7] = "wrong"
		case "nonce":
			env["UAP_TEST_NONCE"] = "wrong"
		case "revision":
			env["UAP_TEST_REVISION"] = "a-three"
		case "environment":
			env["PLUGIN_DATA"] = "/foreign/data"
		case "extra":
			actual["cwd"] = "/foreign"
		}
		if err := kimiProjectValidateServer(actual, p, "a", "/owned", "nonce"); err == nil {
			t.Fatalf("wrong Kimi %s projection accepted", kind)
		}
	}
}

func TestKimiProjectNativeManagedTreeSnapshotCannotAdoptCorruption(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bin", "probe")
	projectNativeRuntimeWrite(t, path, []byte("original inert probe bytes"), 0700)
	before := kimiProjectTree(t, root)
	if err := os.WriteFile(path, []byte("corrupt inert probe bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, kimiProjectTree(t, root)) {
		t.Fatal("managed probe corruption adopted into preservation baseline")
	}
	if err := os.WriteFile(path, []byte("original inert probe bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, kimiProjectTree(t, root)) {
		t.Fatal("managed probe permission drift adopted")
	}
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(root, "empty-foreign-dir")
	if err := os.Mkdir(empty, 0700); err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, kimiProjectTree(t, root)) {
		t.Fatal("empty managed directory addition adopted")
	}
	if err := os.Remove(empty); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "extra.json"), []byte("{}"), 0600)
	if reflect.DeepEqual(before, kimiProjectTree(t, root)) {
		t.Fatal("unrecorded managed file addition preserved")
	}
}
