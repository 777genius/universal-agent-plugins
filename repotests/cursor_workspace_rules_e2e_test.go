package pluginkitairepo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const cursorWorkspaceRuleBody = `---
description: "Cursor workspace rules projection guard"
alwaysApply: true
---

- RULE MARKER: when explicitly asked to use the MCP tool release_checks, do so exactly once.`

// TestCursorWorkspaceRulesProjectionWithoutLiveAgents is the non-live guard for
// the cursor-workspace rules lane used by the portable MCP live smoke: rules
// authored under plugin/targets/cursor-workspace/rules/** must render verbatim
// into .cursor/rules/project.mdc and pass generate --check plus
// validate --strict without any live agent.
func TestCursorWorkspaceRulesProjectionWithoutLiveAgents(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := filepath.Join(t.TempDir(), "cursor-workspace-rules")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "cursor-workspace-rules"
version: "0.1.0"
description: "cursor workspace rules projection guard"
targets:
  - "cursor-workspace"
`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "cursor-workspace", "rules", "project.mdc"), cursorWorkspaceRuleBody)

	env := append(os.Environ(), "GOWORK=off")
	generate := exec.CommandContext(context.Background(), pluginKitAIBin, "generate", workDir)
	generate.Env = env
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai generate: %v\n%s", err, out)
	}
	generateCheck := exec.CommandContext(context.Background(), pluginKitAIBin, "generate", workDir, "--check")
	generateCheck.Env = env
	if out, err := generateCheck.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai generate --check: %v\n%s", err, out)
	}
	validate := exec.CommandContext(context.Background(), pluginKitAIBin, "validate", workDir, "--platform", "cursor-workspace", "--strict")
	validate.Env = env
	if out, err := validate.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai validate --strict: %v\n%s", err, out)
	}

	body, err := os.ReadFile(filepath.Join(workDir, ".cursor", "rules", "project.mdc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`description: "Cursor workspace rules projection guard"`,
		"alwaysApply: true",
		"RULE MARKER: when explicitly asked to use the MCP tool release_checks, do so exactly once.",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf(".cursor/rules/project.mdc missing authored rule %q:\n%s", want, body)
		}
	}
}

// TestCursorTargetRulesAreRejectedInPhaseOne pins the phase-1 rejection path:
// repo-local Cursor rules must not be authored under plugin/targets/cursor/
// (that lane only accepts skills and MCP); they belong in
// plugin/targets/cursor-workspace/rules/**.
func TestCursorTargetRulesAreRejectedInPhaseOne(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := filepath.Join(t.TempDir(), "cursor-target-rules")
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "cursor-target-rules"
version: "0.1.0"
description: "cursor target rules rejection"
targets:
  - "cursor"
`)
	mustWriteRepoFile(t, workDir, filepath.Join("plugin", "targets", "cursor", "rules", "project.mdc"), cursorWorkspaceRuleBody)

	env := append(os.Environ(), "GOWORK=off")
	for _, args := range [][]string{
		{"generate", workDir},
		{"validate", workDir, "--platform", "cursor", "--strict"},
	} {
		cmd := exec.CommandContext(context.Background(), pluginKitAIBin, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("plugin-kit-ai %s accepted plugin/targets/cursor/rules:\n%s", strings.Join(args, " "), out)
		}
		for _, want := range []string{
			"target cursor does not support",
			"plugin/targets/cursor/...",
			"in phase 1",
		} {
			if !strings.Contains(string(out), want) {
				t.Fatalf("plugin-kit-ai %s rejection missing %q:\n%s", strings.Join(args, " "), want, out)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(workDir, ".cursor", "rules", "project.mdc")); !os.IsNotExist(err) {
		t.Fatalf("rejected plugin/targets/cursor/rules must not render .cursor/rules/project.mdc; stat err = %v", err)
	}
}
