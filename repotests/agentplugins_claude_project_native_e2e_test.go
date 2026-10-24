package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"
)

// The source-only project gate executes genuine pinned Claude read-only native
// listing/details after actual project UAP operations. Its opt-in and existing
// disposable-host guard run before any client process or package acquisition.
func TestAgentpluginsClaudeNativeProjectPluginLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source project native client execution")
	}
	nativeRequireDisposable(t)
	f := newClaudeProjectNativeFixture(t)
	stages := commandsStages()
	stages["native_mcp_initialization"] = nativeStage{Status: "not_evaluated", Reason: "skills-only fixture; no MCP process or model session"}
	stages["project_scope_preservation"] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	evidence := claudeProjectNativeIdentity(t, f)
	defer f.writeEvidence(t, evidence, stages)
	commandsNativeLifecycle(t, commandsNativeClient{
		target: "claude", pkg: f.base.PackageRoot, statePath: filepath.Join(f.project, ".uap", "state-v2.json"),
		run: f.run, skills: f.skills,
		dropManaged: func(t *testing.T, _ map[string]any) string {
			t.Helper()
			path := f.listedPath(t, f.project, "project-list-before-absent-repair")
			if path == "" {
				t.Fatal("native project plugin absent before missing-directory repair")
			}
			nativeRequireContained(t, f.project, path)
			if err := os.RemoveAll(path); err != nil {
				t.Fatal(err)
			}
			return path
		},
	}, stages)
	f.requireForeign(t)
	stages["project_scope_preservation"] = nativeStage{Status: "passed", Reason: "fresh project-B native listings omit project-A plugin; foreign user/cache/project A/project B bytes are exact"}
}
