package providers_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
)

func TestProjectOpenCodeRejectsContainedNativeAliases(t *testing.T) {
	for _, kind := range []string{"skills", "config"} {
		t.Run(kind, func(t *testing.T) {
			request, _, _ := projectOpenCodeRequest(t)
			project := request.Plan.TargetContext.ProjectRoot
			alias, target := filepath.Join(project, ".opencode"), filepath.Join(project, "other-native")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "config" {
				alias, target = filepath.Join(project, "opencode.jsonc"), filepath.Join(target, "opencode.jsonc")
				projectGeminiWrite(t, target, `{"mcp":{}}`)
				if err := os.Remove(alias); err != nil {
					t.Fatal(err)
				}
			}
			before := projectGeminiInventory(t, filepath.Join(project, "other-native"))
			if err := os.Symlink(target, alias); err != nil {
				t.Skipf("native symlink fixture unavailable: %v", err)
			}
			if _, err := opencode.New().Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, request); err == nil {
				t.Fatal("project accepted a contained native alias")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, filepath.Join(project, "other-native"))) {
				t.Fatal("refused native alias changed its contained target")
			}
		})
	}
}
