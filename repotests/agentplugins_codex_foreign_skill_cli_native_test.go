package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Real pinned Codex discovery and the actual source-built public UAP CLI.
// No injected detector, vendor login, model request or foreign-file adoption.
func TestAgentpluginsCodexNativeForeignSkillCLIRefusal(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in real Codex project E2E")
	}
	nativeRequireDisposable(t)
	f := newCodexProjectNativeFixture(t)
	stages := map[string]nativeStage{
		"native_foreign_discovery": {Status: "not_evaluated"},
		"public_cli_refusal":      {Status: "not_evaluated"},
		"foreign_preservation":    {Status: "not_evaluated"},
	}
	evidence := f.evidence(t, stages)
	defer func() {
		f.writeEvidence(t, evidence)
		if err := nativeStageFailures(stages); err != nil {
			t.Error(err)
		}
	}()

	const name = "benchmark-note"
	const sourceCommit = "7155140441b93db3a7b36dcd7e00291cafe608f9"
	const sourcePath = "tests/fixtures/uap_apm/skills/benchmark-note"
	target := filepath.Join(f.base.Project, ".agents", "skills", name, "SKILL.md")
	f.foreign(t, target, "---\nname: benchmark-note\ndescription: Human-owned foreign Skill\n---\n\nDo not overwrite.\n")
	before := nativeTreeDigest(t, filepath.Join(f.base.Project, ".agents", "skills"))

	// Genuine Codex app-server sees the conflicting repo Skill before UAP.
	r := startNativeRPC(t, f.base, f.client, "foreign-before")
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_foreign_skill_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var listing struct {
		Data []struct {
			CWD    string `json:"cwd"`
			Errors []any  `json:"errors"`
			Skills []struct {
				Name, Path, Scope string
				PluginID          string `json:"pluginId"`
				Enabled           bool
			} `json:"skills"`
		} `json:"data"`
	}
	body := r.must(t, "skills/list", map[string]any{"cwds": []string{f.base.Project}, "forceReload": true})
	r.close()
	if err := json.Unmarshal(body, &listing); err != nil || len(listing.Data) != 1 ||
		!openCodeSamePath(listing.Data[0].CWD, f.base.Project) || len(listing.Data[0].Errors) != 0 {
		t.Fatal("genuine Codex project Skill inventory incomplete")
	}
	matches := 0
	for _, skill := range listing.Data[0].Skills {
		if skill.Name != name {
			continue
		}
		if !skill.Enabled || skill.Scope != "repo" || skill.PluginID != "" || !openCodeSamePath(skill.Path, target) {
			t.Fatal("Codex did not bind the foreign Skill to this repository")
		}
		matches++
	}
	if matches != 1 {
		t.Fatalf("Codex found %d foreign Skill entries, want exactly one", matches)
	}
	stages["native_foreign_discovery"] = nativeStage{Status: "passed", Reason: "genuine pinned Codex skills/list saw the exact unmanaged repository Skill"}

	argv := []string{"skills", "add", "777genius/universal-agent-plugins", "--ref", sourceCommit,
		"--path", sourcePath, "--skill", name, "--target", "codex", "--scope", "project",
		"--project-root", f.base.Project, "--no-ui", "--yes", "--format", "json"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.installer, argv...)
	cmd.Env = append(f.base.env(filepath.Dir(f.client)), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	cmd.Dir = f.base.Project
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	var exit *exec.ExitError
	var refusal struct {
		Code, Message string
	}
	if !errors.As(runErr, &exit) || exit.ExitCode() != 1 ||
		json.Unmarshal(stdout.Bytes(), &refusal) != nil || ctx.Err() != nil {
		t.Fatalf("public CLI did not return one structured exit-1 refusal: %v", runErr)
	}
	if refusal.Code != "FOREIGN_DESTINATION" ||
		!strings.Contains(refusal.Message, ".agents/skills/benchmark-note/SKILL.md") ||
		strings.Contains(refusal.Message, f.base.Project) || strings.Contains(refusal.Message, f.base.Home) ||
		strings.Contains(stderr.String(), f.base.Project) {
		t.Fatalf("unsafe or missing typed project-relative refusal: %q", refusal.Code)
	}
	nativeJSON(t, filepath.Join(f.base.Root, "foreign-skill-public-refusal.json"), refusal)
	stages["public_cli_refusal"] = nativeStage{Status: "passed", Reason: "actual external UAP CLI and genuine Codex detection produced typed relative-path refusal"}
	if got := nativeTreeDigest(t, filepath.Join(f.base.Project, ".agents", "skills")); got != before {
		t.Fatal("refused collision mutated foreign Skill or its neighbor")
	}
	for _, relative := range []string{"uap.toml", "uap.lock", filepath.Join(".uap", "state-v2.json")} {
		if _, err := os.Lstat(filepath.Join(f.base.Project, relative)); !os.IsNotExist(err) {
			t.Fatalf("refusal created false project state at %s: %v", relative, err)
		}
	}
	f.requireForeign(t)
	stages["foreign_preservation"] = nativeStage{Status: "passed", Reason: "occupied Skill, neighbor and foreign Codex config remained exact; no saved project install"}
	evidence["source_coordinate"] = "777genius/universal-agent-plugins@" + sourceCommit + "//" + sourcePath
	evidence["claimed_release"] = false
}
