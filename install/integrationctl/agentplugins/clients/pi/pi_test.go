package pi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestNamespaceAdmissionIsBoundedAndSpecific(t *testing.T) {
	contracttest.RunAdapter(t, New())
	for _, raw := range []string{`null`, `{"version":2,"source":"native"}`, `{"version":1,"source":"../escape"}`, `{"version":1,"source":"/absolute"}`, `{"version":1,"source":"native","command":"touch bad"}`, `{"version":1,"version":2,"source":"native"}`} {
		if support, _ := New().InterpretExtension(Namespace, json.RawMessage(raw)); support != domain.SupportUnsupported {
			t.Fatalf("admitted %s", raw)
		}
	}
	if support, _ := New().InterpretExtension("com.example.unknown", json.RawMessage(`{}`)); support != domain.SupportUnsupported {
		t.Fatal("unknown namespace admitted")
	}
	if support, _ := New().InterpretExtension(Namespace, json.RawMessage(`{"version":1,"source":"native"}`)); support != domain.SupportNative {
		t.Fatal("native namespace rejected")
	}
}

func TestProjectionBindsPhysicalPathDataAndCanonicalSource(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "profile")
	plan := domain.DeliveryPlan{ClientID: domain.ClientPi, Scope: domain.ScopeUser, NativeRegistryRoot: profile, TargetAnchor: profile, TargetRoot: filepath.Join(profile, "extensions"), PhysicalArtifactID: "TEST-plugin", ActivePath: filepath.Join(profile, "extensions", "TEST-plugin")}
	staging := filepath.Join(profile, ".agentplugins-staging-TEST")
	writePi(t, filepath.Join(staging, "native", "index.ts"), "export default function(pi) {}\n")
	raw := json.RawMessage(`{"version":1,"source":"native"}`)
	envelope := domain.PackageEnvelope{TreeDigest: "sha256:" + strings.Repeat("a", 64), Manifest: domain.PluginManifest{Extensions: map[string]json.RawMessage{Namespace: raw}}}
	manifest, _ := json.Marshal(envelope.Manifest)
	writePi(t, filepath.Join(staging, "plugin.json"), string(manifest))
	in := clients.ProjectionInput{StagingPath: staging, Plan: plan, Envelope: envelope, PluginDataPath: filepath.Join(root, "data")}
	if _, err := New().Project(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.TargetRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(staging, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	if _, err := New().ProjectActiveNative(context.Background(), plan.ActivePath, envelope, plan, in.PluginDataPath); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(plan.ActivePath, "index.ts"))
	if strings.Index(string(body), "dirname(current) !== descriptor.active_path") > strings.Index(string(body), "await import(") || strings.Contains(string(body), "realpathSync(descriptor.active_path)") {
		t.Fatal("bootstrap can import before physical binding guard")
	}
	if _, err := New().ProjectActiveNative(context.Background(), plan.ActivePath, envelope, plan, filepath.Join(root, "other-data")); err == nil {
		t.Fatal("changed data root accepted")
	}
	writePi(t, filepath.Join(plan.ActivePath, "index.ts"), "export default function() {}")
	if _, err := New().ProjectActiveNative(context.Background(), plan.ActivePath, envelope, plan, in.PluginDataPath); err == nil {
		t.Fatal("edited bootstrap accepted")
	}
}

func TestForeignDiscoveryOverridesArePreservedAndRefused(t *testing.T) {
	for _, name := range []string{"index.ts", "index.js", "package.json"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			body := "foreign"
			if name == "package.json" {
				body = `{"pi":{"extensions":["other.ts"]}}`
			}
			writePi(t, filepath.Join(root, name), body)
			if err := checkDiscoveryRoot(root); err == nil {
				t.Fatal("masking native root admitted")
			}
			actual, _ := os.ReadFile(filepath.Join(root, name))
			if string(actual) != body {
				t.Fatal("foreign bytes changed")
			}
		})
	}
}
func writePi(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
