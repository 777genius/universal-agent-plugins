package providers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestProjectCursorStdioUsesBoundManagedLauncherWithoutExecution(t *testing.T) {
	fixture := newCursorProjectFixture(t, true)
	beforeUser, beforeB := projectGeminiInventory(t, fixture.user), projectGeminiInventory(t, fixture.other)
	outcome, err := fixture.activator(nativeconfig.New()).Activate(context.Background(), fixture.request)
	cursorAssertPrepared(t, outcome, err)
	body, err := os.ReadFile(filepath.Join(fixture.request.Plan.NativeRegistryRoot, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			CWD     string            `json:"cwd"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	server := document.Servers["docs"]
	active := fixture.request.Plan.ActivePath
	data := filepath.Join(fixture.request.Plan.TargetContext.ProjectRoot, ".uap", "data", fixture.request.Plan.PhysicalArtifactID)
	wantArgs := []string{managedstdio.Mode, active, data, active, "plugin", "--argv-base64-v1", "bm9kZQ==",
		base64.StdEncoding.EncodeToString([]byte(active + "/server.js")), base64.StdEncoding.EncodeToString([]byte(data))}
	if server.Command != filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) || !reflect.DeepEqual(server.Args, wantArgs) || server.CWD != "" || server.Env["PLUGIN_ROOT"] != active || server.Env["PLUGIN_DATA"] != data {
		t.Fatalf("native launcher binding = %+v; want args %q", server, wantArgs)
	}
	projectGeminiAssert(t, server.Command, "trusted fixture bytes; never executed")
	if !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, fixture.user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, fixture.other)) {
		t.Fatal("stdio configuration changed other scopes")
	}
	activeBefore := projectGeminiInventory(t, active)
	objects, err := fixture.stager.ProjectActiveNative(context.Background(), fixture.envelope, fixture.request.Plan, fixture.request.Delivery.ArtifactDigest, data)
	if err != nil || len(objects) != 3 || !reflect.DeepEqual(activeBefore, projectGeminiInventory(t, active)) {
		t.Fatalf("stdio read-only projection = %+v, %v", objects, err)
	}
	for _, selectedData := range []string{"", filepath.Join(filepath.Dir(data), "other-receipt")} {
		if _, err := fixture.stager.ProjectActiveNative(context.Background(), fixture.envelope, fixture.request.Plan, fixture.request.Delivery.ArtifactDigest, selectedData); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
			t.Fatalf("different selected data locator was accepted: %v", err)
		}
		if !reflect.DeepEqual(activeBefore, projectGeminiInventory(t, active)) {
			t.Fatal("refused data locator changed immutable package bytes")
		}
	}
	fixture.stager.LauncherSource = nil
	if _, err := fixture.stager.Stage(context.Background(), fixture.envelope, fixture.request.Plan, "missing-launcher", domain.CompatibilityHints{}); err == nil {
		t.Fatal("project stdio staged without trusted launcher")
	}
}
