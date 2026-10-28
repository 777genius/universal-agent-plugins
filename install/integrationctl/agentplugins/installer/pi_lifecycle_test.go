package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
)

func TestPiPublicDirectoryLifecycle(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(pi.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{StateRoot: filepath.Join(root, "state"), Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "profile")
	source := filepath.Join(root, "source")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"test-pi","version":"1.0.0","extensions":{"com.universal-agent-plugins.pi":{"version":1,"source":"native"}}}`)
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) {}\n")
	write(filepath.Join(profile, "extensions", "foreign.ts"), "// foreign extension\n")
	req := Request{Operation: OpInstall, PackageRoot: source, ClientID: "pi", ClientConfigRoot: profile, InstallationID: "TEST-pi-lifecycle"}
	apply := func(req Request) Result {
		t.Helper()
		prepared, err := engine.Prepare(context.Background(), req)
		if err != nil {
			t.Fatalf("prepare %s: %v", req.Operation, err)
		}
		defer prepared.Close()
		result, err := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
		if err != nil || result.Outcome != OutcomeCompleted {
			t.Fatalf("apply %s: %+v %v", req.Operation, result, err)
		}
		return result
	}
	installed := apply(req)
	if installed.Binding.DataRoot == "" {
		t.Fatal("extension-only package has no owned plugin data")
	}
	active := installed.Binding.TargetPath
	if filepath.Dir(active) != filepath.Join(profile, "extensions") {
		t.Fatalf("not native discovery: %s", active)
	}
	req.Operation = OpUpdate
	write(filepath.Join(source, "native", "index.ts"), "export default function(pi) { /* updated */ }\n")
	apply(req)
	body, err := os.ReadFile(filepath.Join(active, "native", "index.ts"))
	if err != nil || !strings.Contains(string(body), "updated") {
		t.Fatal("native update missing", err)
	}
	if err := os.RemoveAll(active); err != nil {
		t.Fatal(err)
	}
	req.Operation = OpRepair
	apply(req)
	write(filepath.Join(active, "native", "index.ts"), "user edit\n")
	prepared, err := engine.Prepare(context.Background(), req)
	if err == nil {
		defer prepared.Close()
		_, err = engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
	}
	if err == nil {
		t.Fatal("repair overwrote user edits")
	}
	write(filepath.Join(active, "native", "index.ts"), "export default function(pi) { /* updated */ }\n")
	apply(Request{Operation: OpRemove, ClientID: "pi", ClientConfigRoot: profile, InstallationID: installed.InstallationID})
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatal("managed package remains", err)
	}
	if body, err := os.ReadFile(filepath.Join(profile, "extensions", "foreign.ts")); err != nil || string(body) != "// foreign extension\n" {
		t.Fatal("foreign extension changed", err)
	}
}
