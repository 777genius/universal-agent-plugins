//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
