package providers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestDefaultOpenCodeClientPreparationIsolatesProfileAndRevalidation(t *testing.T) {
	root, binary, parent := defaultOpenCodeProbeFixture(t)
	registry, err := clients.NewRegistry(opencode.New())
	if err != nil {
		t.Fatal(err)
	}
	preparer := NewOpenCodeClientPreparation(registry)
	preparer.environment = []string{"PATH="}
	client := domain.DetectedClient{ClientID: domain.ClientOpenCode, ExecutablePath: binary, ConfigRoot: filepath.Join(root, "selected-config")}
	envelope := domain.PackageEnvelope{Skills: map[string]domain.Skill{"docs": {Name: "docs"}}}
	host, err := preparer.PrepareClient(t.Context(), envelope, client, nil, binary, false)
	if err != nil || host == nil {
		t.Fatalf("default preparation: %v %v", host, err)
	}
	assertDefaultOpenCodeProbePrivate(t, binary, parent)
	client.OpenCodeHost = host
	if err := preparer.RevalidateClient(t.Context(), client, domain.DeliveryPlan{OpenCodeHost: host}); err != nil {
		t.Fatalf("private roots changed frozen authority: %v", err)
	}
	assertDefaultOpenCodeProbePrivate(t, binary, parent)
	if _, err := os.Stat(client.ConfigRoot); !os.IsNotExist(err) {
		t.Fatalf("selected native config was created: %v", err)
	}
}

// Exercise the default composition with native inert bytes, never a real agent.
func defaultOpenCodeProbeFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "opencode")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.CommandContext(t.Context(), "go", "build", "-ldflags", "-X main.version=2.0.21 -X main.mode=isolated-ok", "-o", binary, "../adapters/clientdetect/testdata/opencode_probe.go")
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build inert native fixture: %s %v", body, err)
	}
	parent := filepath.Join(root, "ambient-profile")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "canary"), []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
		t.Setenv(key, parent)
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, root)
	}
	t.Setenv("OPENCODE_CONFIG_CONTENT", "parent-secret")
	t.Setenv("AGENTPLUGINS_TEST_CREDENTIAL", "parent-secret")
	return root, binary, parent
}

func assertDefaultOpenCodeProbePrivate(t *testing.T, binary, parent string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "observed.json"))
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		CWD      string
		Env      []string
		Roots    map[string]string
		Config   string
		Failures []string
	}
	if err := json.Unmarshal(body, &observed); err != nil {
		t.Fatal(err)
	}
	if len(observed.Failures) != 0 || strings.TrimSpace(observed.Config) != "{}" {
		t.Fatalf("unsafe native startup: %+v", observed)
	}
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if !strings.HasPrefix(observed.Roots[key], observed.CWD+string(os.PathSeparator)) {
			t.Fatalf("%s escaped private probe roots: %+v", key, observed)
		}
	}
	for _, entry := range observed.Env {
		if strings.Contains(entry, parent) || strings.Contains(entry, "parent-secret") {
			t.Fatalf("inherited parent profile or credentials: %s", entry)
		}
	}
	if _, err := os.Stat(observed.CWD); !os.IsNotExist(err) {
		t.Fatalf("private roots survived completed probe: %v", err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 || entries[0].Name() != "canary" {
		t.Fatalf("parent profile changed: %v %v", entries, err)
	}
	canary, err := os.ReadFile(filepath.Join(parent, "canary"))
	if err != nil || string(canary) != "unchanged" {
		t.Fatalf("parent bytes changed: %s %v", canary, err)
	}
}
