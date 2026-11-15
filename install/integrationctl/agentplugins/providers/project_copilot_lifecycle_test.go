package providers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real loader/planner/stager/native kernel; no Copilot or server process runs.
func TestProjectCopilotComposedLifecycleSelectsEffectiveConfig(t *testing.T) {
	for _, rootConfig := range []bool{false, true} {
		t.Run(map[bool]string{false: "github-fallback", true: "root-precedence"}[rootConfig], func(t *testing.T) {
			f := newCopilotProjectFixture(t, false, rootConfig)
			user, other := projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
			activator := f.activator(nativeconfig.New())
			outcome, err := activator.Activate(context.Background(), f.request)
			copilotPrepared(t, outcome, err)
			projectGeminiAssert(t, filepath.Join(f.project, ".github", "skills", "docs", "SKILL.md"), copilotSkill)
			config, err := os.ReadFile(f.config)
			if err != nil || !strings.Contains(string(config), "managed.test/one") || !strings.Contains(string(config), `"enabled":false`) || !strings.Contains(string(config), `"type":"http"`) {
				t.Fatalf("Copilot config lost transport or foreign settings: %s, %v", config, err)
			}
			if rootConfig {
				projectGeminiAssert(t, filepath.Join(f.project, ".github", "mcp.json"), copilotForeignConfig)
			}
			f.request.PreviousNativeObjects = outcome.NativeObjects
			for _, verify := range []bool{false, true} {
				f.request.VerifyOnly = verify
				outcome, err = activator.Activate(context.Background(), f.request)
				copilotPrepared(t, outcome, err)
			}
			for range 2 {
				removed, err := activator.Deactivate(context.Background(), f.remove(outcome.NativeObjects))
				if err != nil || !removed.ExternalRemovalComplete {
					t.Fatalf("exact repeated Copilot remove = %+v, %v", removed, err)
				}
			}
			if _, err := os.Stat(filepath.Join(f.project, ".github", "skills", "docs")); !os.IsNotExist(err) {
				t.Fatalf("Copilot managed skill survived: %v", err)
			}
			remaining, err := os.ReadFile(f.config)
			if err != nil || strings.Contains(string(remaining), "managed.test") || !strings.Contains(string(remaining), "foreign.test") {
				t.Fatalf("Copilot removal changed foreign MCP data: %s, %v", remaining, err)
			}
			projectGeminiAssert(t, filepath.Join(f.data, "keep.txt"), "persistent data\n")
			remainingUser := projectGeminiInventory(t, f.user)
			trust := filepath.Join(f.user, "config.json")
			copilotAssertProjectTrust(t, trust, f.project)
			projectGeminiAssert(t, trust+".agentplugins.lock", "")
			delete(remainingUser, trust)
			delete(remainingUser, trust+".agentplugins.lock")
			if !reflect.DeepEqual(user, remainingUser) || !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
				t.Fatal("Copilot project lifecycle changed unrelated user settings or B")
			}
		})
	}
}

func copilotAssertProjectTrust(t *testing.T, path, project string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config, map[string]any{"trustedFolders": []any{project}}) {
		t.Fatal("Copilot project lifecycle did not retain only its reviewed folder trust")
	}
}

func TestProjectCopilotUpdatesRepairsAndDropsMCP(t *testing.T) {
	f := newCopilotProjectFixture(t, false, false)
	a := f.activator(nativeconfig.New())
	out, err := a.Activate(context.Background(), f.request)
	copilotPrepared(t, out, err)
	f.request.PreviousNativeObjects = out.NativeObjects
	projectGeminiWrite(t, filepath.Join(f.source, "skills", "docs", "SKILL.md"), strings.Replace(copilotSkill, "ONE", "TWO", 1))
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"sse","url":"https://managed.test/two"}}}`)
	f.stage(t, "update")
	out, err = a.Activate(context.Background(), f.request)
	copilotPrepared(t, out, err)
	skill := filepath.Join(f.project, ".github", "skills", "docs")
	if err := os.RemoveAll(skill); err != nil {
		t.Fatal(err)
	}
	f.request.PreviousNativeObjects = out.NativeObjects
	out, err = a.Activate(context.Background(), f.request)
	copilotPrepared(t, out, err)
	projectGeminiAssert(t, filepath.Join(skill, "SKILL.md"), strings.Replace(copilotSkill, "ONE", "TWO", 1))
	projectGeminiWrite(t, filepath.Join(f.source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{}}`)
	f.request.PreviousNativeObjects = out.NativeObjects
	f.stage(t, "drop-mcp")
	out, err = a.Activate(context.Background(), f.request)
	copilotPrepared(t, out, err)
	body, err := os.ReadFile(f.config)
	if err != nil || strings.Contains(string(body), "managed.test") || !strings.Contains(string(body), "foreign.test") {
		t.Fatalf("dropped MCP/foreign config = %s, %v", body, err)
	}
}

func TestProjectCopilotRejectsAttestationBeforeAutomaticFallback(t *testing.T) {
	f := newCopilotProjectFixture(t, false, false)
	before := projectGeminiInventory(t, f.base)
	f.request.ActivationComplete = true
	for _, support := range []domain.SupportLevel{domain.SupportNative, domain.SupportPrepared} {
		f.request.Plan.Components[0].Support = support
		if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil || !strings.Contains(err.Error(), "cannot be established by activation attestation") {
			t.Fatalf("Copilot project attestation bypass: %v", err)
		}
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("Copilot attestation created effects")
	}
}
