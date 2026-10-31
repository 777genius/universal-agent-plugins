package providers_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectOpenCodeReadOnlyProjectionRequiresSelectedDataAuthority(t *testing.T) {
	request, other, user := projectOpenCodeRequest(t)
	plan := request.Plan
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportNative}}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"docs": {Name: "docs", Type: "stdio", Decoded: map[string]any{"command": "go", "args": []any{"version"}, "env": map[string]any{"DATA": "${PLUGIN_DATA}"}}},
	}}}
	data := filepath.Join(plan.TargetContext.ProjectRoot, ".uap", "plugin-data", "selected")
	// Rebuild this test-owned package as stdio before taking its immutable
	// baseline; Project refuses authored use of its private projection path.
	if err := os.Remove(filepath.Join(plan.ActivePath, opencode.OpenCodeProjectionFile)); err != nil {
		t.Fatal(err)
	}
	adapter := opencode.New()
	want, err := adapter.Project(t.Context(), clients.ProjectionInput{StagingPath: plan.ActivePath, Envelope: envelope, Plan: plan, PluginDataPath: data})
	if err != nil {
		t.Fatal(err)
	}
	before, beforeUser, beforeOther := projectGeminiInventory(t, plan.TargetContext.ProjectRoot), projectGeminiInventory(t, user), projectGeminiInventory(t, other)
	got, err := adapter.ProjectActiveNative(t.Context(), plan.ActivePath, envelope, plan, data)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("exact selected data projection = %+v, %v", got, err)
	}
	for _, changed := range []string{"", "relative-data", filepath.Join(other, ".uap", "plugin-data", "selected"), data + "-other"} {
		if got, err := adapter.ProjectActiveNative(t.Context(), plan.ActivePath, envelope, plan, changed); !errors.Is(err, domain.ErrProjectionRefreshRequired) || len(got) != 0 {
			t.Fatalf("changed data authority %q accepted: %+v, %v", changed, got, err)
		}
	}
	changed := plan
	changed.ActivePath += "-other"
	if _, err := adapter.ProjectActiveNative(t.Context(), plan.ActivePath, envelope, changed, data); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
		t.Fatalf("changed active package authority accepted: %v", err)
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, plan.TargetContext.ProjectRoot)) || !reflect.DeepEqual(beforeUser, projectGeminiInventory(t, user)) || !reflect.DeepEqual(beforeOther, projectGeminiInventory(t, other)) {
		t.Fatal("read-only projection mutated package/native/user/project-B bytes")
	}
}
