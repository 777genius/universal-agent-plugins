package providers_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestProjectKiroStdioUsesBoundManagedLauncherWithoutExecution(t *testing.T) {
	fixture := newKiroProjectFixture(t, true)
	beforeUser, beforeB := projectGeminiInventory(t, fixture.user), projectGeminiInventory(t, fixture.other)
	outcome, err := fixture.activator(nativeconfig.New()).Activate(context.Background(), fixture.request)
	kiroAssertPrepared(t, outcome, err)
	servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(fixture.request.Plan.NativeRegistryRoot, "settings", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	server := servers["docs"].(map[string]any)
	args := make([]string, 0, len(server["args"].([]any)))
	for _, arg := range server["args"].([]any) {
		args = append(args, arg.(string))
	}
	env := server["env"].(map[string]any)
	active := fixture.request.Plan.ActivePath
	data := filepath.Join(fixture.request.Plan.TargetContext.ProjectRoot, ".uap", "data", fixture.request.Plan.PhysicalArtifactID)
	wantArgs := []string{managedstdio.Mode, active, data, active, "plugin", "--", "node", active + "/server.js", data}
	if server["command"] != filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) || !reflect.DeepEqual(args, wantArgs) || server["cwd"] != nil || env["PLUGIN_ROOT"] != active || env["PLUGIN_DATA"] != data {
		t.Fatalf("native launcher binding = %+v; want args %q", server, wantArgs)
	}
	projectGeminiAssert(t, server["command"].(string), "trusted fixture bytes; never executed")
	if !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, fixture.user)) || !reflect.DeepEqual(beforeB, projectGeminiInventory(t, fixture.other)) {
		t.Fatal("stdio configuration changed other scopes")
	}
	activeBefore := projectGeminiInventory(t, active)
	objects, err := fixture.stager.ProjectActiveNative(context.Background(), fixture.envelope, fixture.request.Plan, fixture.request.Delivery.ArtifactDigest, data)
	if err != nil || len(objects) != 2 || !reflect.DeepEqual(activeBefore, projectGeminiInventory(t, active)) {
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
