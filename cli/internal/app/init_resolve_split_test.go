package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func TestResolvePackageOnlyTargetsRejectsLauncherBackedPlatform(t *testing.T) {
	t.Parallel()

	_, err := resolvePackageOnlyTargets(scaffold.InitTemplateOnlineService, InitOptions{
		Platform:         "codex-runtime",
		PlatformExplicit: true,
	}, "codex-runtime")
	if err == nil || !strings.Contains(err.Error(), "only supports package and workspace outputs") {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizeCustomLogicDefaultsSetsLauncherDefaults(t *testing.T) {
	t.Parallel()

	config := resolvedInitConfig{TemplateName: scaffold.InitTemplateCustomLogic}
	normalizeCustomLogicDefaults(InitOptions{}, &config)
	if config.Platform != "codex-runtime" || config.Runtime != scaffold.RuntimeGo {
		t.Fatalf("config = %+v", config)
	}
}

func TestValidateInitRuntimeOptionsRejectsTypeScriptWithoutNode(t *testing.T) {
	t.Parallel()

	err := validateInitRuntimeOptions(InitOptions{TypeScript: true}, &resolvedInitConfig{
		Platform: "claude",
		Runtime:  scaffold.RuntimeGo,
	})
	if err == nil || !strings.Contains(err.Error(), "--typescript requires --runtime node") {
		t.Fatalf("error = %v", err)
	}
}
