package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
	"github.com/777genius/plugin-kit-ai/plugininstall"
)

type fakeInstaller struct {
	called bool
	res    plugininstall.Result
	err    error
}

func (f *fakeInstaller) Install(ctx context.Context, p plugininstall.Params) (plugininstall.Result, error) {
	f.called = true
	if p.Owner != "a" || p.Repo != "b" {
		return plugininstall.Result{}, errors.New("bad params")
	}
	return f.res, f.err
}

func TestInstallRunner_usesFake(t *testing.T) {
	t.Parallel()
	f := &fakeInstaller{}
	r := NewInstallRunner(f)
	_, err := r.Install(context.Background(), plugininstall.Params{Owner: "a", Repo: "b", Tag: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.called {
		t.Fatal("fake not called")
	}
}

func TestNewInstallRunner_nilUsesFacadeType(t *testing.T) {
	t.Parallel()
	r := NewInstallRunner(nil)
	if _, ok := r.Installer.(plugininstallFacade); !ok {
		t.Fatalf("want plugininstallFacade, got %T", r.Installer)
	}
}

func TestInitRunner_unknownPlatform(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{ProjectName: "okname", Platform: "bad-platform"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInitRunner_onlineServiceTemplateDoesNotRequireGeminiNameByDefault(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "DemoPlugin")
	got, err := r.Run(InitOptions{
		ProjectName: "DemoPlugin",
		Template:    scaffold.InitTemplateOnlineService,
		OutputDir:   out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "mcp", "servers.yaml"),
		filepath.Join("plugin", "README.md"),
		"CLAUDE.md",
		"AGENTS.md",
		"README.md",
		filepath.Join(".claude-plugin", "plugin.json"),
		filepath.Join(".codex-plugin", "plugin.json"),
		filepath.Join(".cursor-plugin", "plugin.json"),
		".mcp.json",
		"opencode.json",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "gemini-extension.json")); !os.IsNotExist(err) {
		t.Fatalf("expected gemini-extension.json to stay absent by default, err=%v", err)
	}
}

func TestInitRunner_onlineServiceTemplateExplicitGeminiStillRequiresKebabCase(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{
		ProjectName:      "DemoPlugin",
		Template:         scaffold.InitTemplateOnlineService,
		Platform:         "gemini",
		PlatformExplicit: true,
		OutputDir:        filepath.Join(t.TempDir(), "DemoPlugin"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "must be lowercase kebab-case") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitRunner_claudeStableDefault(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "claude", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		"go.mod",
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "launcher.yaml"),
		filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"),
		filepath.Join("plugin", "targets", "claude", "settings.json"),
		filepath.Join("plugin", "targets", "claude", "lsp.json"),
		filepath.Join("plugin", "targets", "claude", "user-config.json"),
		filepath.Join("plugin", "targets", "claude", "manifest.extra.json"),
		filepath.Join("cmd", "genplug", "main.go"),
		filepath.Join(".claude-plugin", "plugin.json"),
		filepath.Join("hooks", "hooks.json"),
		"CLAUDE.md",
		"AGENTS.md",
		"README.md",
		"Makefile",
		".goreleaser.yml",
		filepath.Join("plugin", "skills", "genplug", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	assertRuntimeTestAssetsExist(t, out, "claude")
	hooksBody, err := os.ReadFile(filepath.Join(out, "plugin", "targets", "claude", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	hooks := string(hooksBody)
	for _, want := range []string{`"Stop"`, `"PreToolUse"`, `"UserPromptSubmit"`} {
		if !strings.Contains(hooks, want) {
			t.Fatalf("default Claude hooks missing %s:\n%s", want, hooks)
		}
	}
	for _, unwanted := range []string{`"SessionStart"`, `"WorktreeRemove"`} {
		if strings.Contains(hooks, unwanted) {
			t.Fatalf("default Claude hooks unexpectedly contain %s:\n%s", unwanted, hooks)
		}
	}
	mainBody, err := os.ReadFile(filepath.Join(out, "cmd", "genplug", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainGo := string(mainBody)
	for _, want := range []string{
		"app.Claude().OnStop",
		"return claude.Allow()",
		"app.Claude().OnPreToolUse",
		"return claude.PreToolAllow()",
		"app.Claude().OnUserPromptSubmit",
		"return claude.UserPromptAllow()",
	} {
		if !strings.Contains(mainGo, want) {
			t.Fatalf("default Claude main.go missing %q:\n%s", want, mainGo)
		}
	}
	if !strings.Contains(mainGo, "OnStop") || !strings.Contains(mainGo, "OnUserPromptSubmit") {
		t.Fatalf("default Claude main.go missing stable handlers:\n%s", mainGo)
	}
	if strings.Contains(mainGo, "OnSessionStart") {
		t.Fatalf("default Claude main.go unexpectedly contains extended handler:\n%s", mainGo)
	}
	assertDevSDKReplace(t, out)
}

func TestInitRunner_claudeWithoutExtrasStaysMinimal(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "claude", OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "targets", "claude", "settings.json"),
		filepath.Join("plugin", "targets", "claude", "lsp.json"),
		filepath.Join("plugin", "targets", "claude", "user-config.json"),
		filepath.Join("plugin", "targets", "claude", "manifest.extra.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to stay absent, err=%v", rel, err)
		}
	}
}

func TestInitRunner_claudeExtendedHooks(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{
		ProjectName:         "genplug",
		Platform:            "claude",
		OutputDir:           out,
		ClaudeExtendedHooks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	hooksBody, err := os.ReadFile(filepath.Join(out, "plugin", "targets", "claude", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	hooks := string(hooksBody)
	for _, want := range []string{`"Stop"`, `"SessionStart"`, `"WorktreeRemove"`} {
		if !strings.Contains(hooks, want) {
			t.Fatalf("extended Claude hooks missing %s:\n%s", want, hooks)
		}
	}
	mainBody, err := os.ReadFile(filepath.Join(out, "cmd", "genplug", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainGo := string(mainBody)
	for _, want := range []string{"OnStop", "OnSessionStart", "OnWorktreeRemove"} {
		if !strings.Contains(mainGo, want) {
			t.Fatalf("extended Claude main.go missing %s:\n%s", want, mainGo)
		}
	}
	assertRuntimeTestAssetsExist(t, out, "claude")
	for _, rel := range []string{
		filepath.Join("fixtures", "claude", "SessionStart.json"),
		filepath.Join("goldens", "claude", "SessionStart.stdout"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected stable-only runtime test assets, but %s exists: %v", rel, err)
		}
	}
}

func TestInitRunner_claudeExtendedHooksRejectedOutsideClaude(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{
		ProjectName:         "genplug",
		Platform:            "codex-runtime",
		OutputDir:           filepath.Join(t.TempDir(), "genplug"),
		ClaudeExtendedHooks: true,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--claude-extended-hooks is only supported with --platform claude") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_geminiPackagingStarter(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "gemini", OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "targets", "gemini", "package.yaml"),
		filepath.Join("plugin", "targets", "gemini", "contexts", "GEMINI.md"),
		"CLAUDE.md",
		"AGENTS.md",
		"GEMINI.md",
		"gemini-extension.json",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	manifestBody, err := os.ReadFile(filepath.Join(out, "gemini-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(manifestBody)
	for _, want := range []string{`"name": "genplug"`, `"contextFileName": "GEMINI.md"`} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("gemini manifest missing %q:\n%s", want, manifest)
		}
	}
	for _, rel := range []string{
		"go.mod",
		filepath.Join("plugin", "launcher.yaml"),
		filepath.Join("cmd", "genplug", "main.go"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected gemini runtime scaffold file %s", rel)
		}
	}
	assertRuntimeTestAssetsAbsent(t, out)
}

func TestInitRunner_geminiPackagingStarterWithPortableMCP(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "gemini", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "mcp", "servers.yaml"),
		"gemini-extension.json",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	mcpBody, err := os.ReadFile(filepath.Join(out, "plugin", "mcp", "servers.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"api_version: v1",
		`url: "https://example.com/mcp"`,
		"- \"gemini\"",
	} {
		if !strings.Contains(string(mcpBody), want) {
			t.Fatalf("gemini portable MCP missing %q:\n%s", want, mcpBody)
		}
	}
	manifestBody, err := os.ReadFile(filepath.Join(out, "gemini-extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"mcpServers"`, `"https://example.com/mcp"`} {
		if !strings.Contains(string(manifestBody), want) {
			t.Fatalf("gemini-extension.json missing %q:\n%s", want, manifestBody)
		}
	}
}

func TestInitRunner_geminiRejectsInvalidExtensionNameEarly(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "Demo_Extension")
	_, err := r.Run(InitOptions{ProjectName: "Demo_Extension", Platform: "gemini", OutputDir: out})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid Gemini extension name") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_geminiRejectsRuntimeFlag(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "gemini", Runtime: "python", OutputDir: out})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime is not supported with --platform gemini") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_geminiGoRuntimeStarter(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "gemini", Runtime: "go", OutputDir: out})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "launcher.yaml"),
		"go.mod",
		filepath.Join("cmd", "genplug", "main.go"),
		filepath.Join("plugin", "targets", "gemini", "package.yaml"),
		filepath.Join("plugin", "targets", "gemini", "contexts", "GEMINI.md"),
		filepath.Join("plugin", "targets", "gemini", "hooks", "hooks.json"),
		"CLAUDE.md",
		"AGENTS.md",
		"hooks/hooks.json",
		"gemini-extension.json",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	readmeBody, err := os.ReadFile(filepath.Join(out, "plugin", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(readmeBody)
	for _, want := range []string{
		"plugin-kit-ai test` and `plugin-kit-ai dev` stay focused on the stable Claude/Codex runtime fixture lanes",
		"gemini extensions link .",
		"plugin-kit-ai validate . --platform gemini --strict",
		"plugin-kit-ai inspect . --target gemini",
		"plugin-kit-ai capabilities --mode runtime --platform gemini",
		"make test-gemini-runtime",
		"make test-gemini-runtime-live",
		"`gemini.*Continue()` helpers mean a true no-op Gemini hook response",
		"Gemini treats `SessionStart` and `SessionEnd` as advisory hooks",
		"`gemini.BeforeModelOverrideRequestValue(...)`",
		"`gemini.BeforeModelSyntheticResponseValue(...)`",
		"`gemini.AfterModelReplaceResponseValue(...)`",
		"`gemini.BeforeToolRewriteInputValue(...)`",
		"`gemini.AfterToolAddContext(...)`",
		"`gemini.AfterToolTailCallValue(...)`",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("gemini runtime README missing %q:\n%s", want, readme)
		}
	}
	rootReadmeBody, err := os.ReadFile(filepath.Join(out, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"This file is generated by `plugin-kit-ai generate`.",
		"[`plugin/README.md`](./plugin/README.md)",
		"[`AGENTS.md`](./AGENTS.md)",
	} {
		if !strings.Contains(string(rootReadmeBody), want) {
			t.Fatalf("gemini runtime root README missing %q:\n%s", want, rootReadmeBody)
		}
	}
	mainBody, err := os.ReadFile(filepath.Join(out, "cmd", "genplug", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	mainGo := string(mainBody)
	for _, want := range []string{
		"return gemini.SessionStartContinue()",
		"return gemini.SessionEndContinue()",
		"app.Gemini().OnBeforeTool",
		"return gemini.BeforeToolContinue()",
		"app.Gemini().OnAfterTool",
		"return gemini.AfterToolContinue()",
	} {
		if !strings.Contains(mainGo, want) {
			t.Fatalf("gemini runtime main.go missing %q:\n%s", want, mainGo)
		}
	}
	assertDevSDKReplace(t, out)
}

func assertDevSDKReplace(t *testing.T, root string) {
	t.Helper()
	path := defaultGoSDKReplacePath()
	if strings.TrimSpace(path) == "" {
		return
	}
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	want := `replace github.com/777genius/plugin-kit-ai/sdk => "` + path + `"`
	if !strings.Contains(string(body), want) {
		t.Fatalf("go.mod missing local sdk replace %q:\n%s", want, string(body))
	}
}

func TestInitRunner_opencodeWorkspaceStarter(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "opencode", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "mcp", "servers.yaml"),
		filepath.Join("plugin", "targets", "opencode", "package.yaml"),
		filepath.Join("plugin", "targets", "opencode", "package.json"),
		filepath.Join("plugin", "targets", "opencode", "commands", "genplug.md"),
		filepath.Join("plugin", "targets", "opencode", "agents", "genplug.md"),
		filepath.Join("plugin", "targets", "opencode", "themes", "genplug.json"),
		filepath.Join("plugin", "targets", "opencode", "tools", "genplug.ts"),
		filepath.Join("plugin", "targets", "opencode", "plugins", "example.js"),
		"CLAUDE.md",
		"AGENTS.md",
		"opencode.json",
		"README.md",
		filepath.Join("plugin", "skills", "genplug", "SKILL.md"),
		filepath.Join(".opencode", "skills", "genplug", "SKILL.md"),
		filepath.Join(".opencode", "commands", "genplug.md"),
		filepath.Join(".opencode", "agents", "genplug.md"),
		filepath.Join(".opencode", "themes", "genplug.json"),
		filepath.Join(".opencode", "tools", "genplug.ts"),
		filepath.Join(".opencode", "plugins", "example.js"),
		filepath.Join(".opencode", "package.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		filepath.Join("plugin", "launcher.yaml"),
		"go.mod",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected opencode starter file %s", rel)
		}
	}
	assertRuntimeTestAssetsAbsent(t, out)
	skillBody, err := os.ReadFile(filepath.Join(out, "plugin", "skills", "genplug", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"name: genplug",
		"description: Portable shared skill stub for genplug.",
		"execution_mode: docs_only",
		"supported_agents:",
		"  - claude",
		"  - codex",
	} {
		if !strings.Contains(string(skillBody), want) {
			t.Fatalf("OpenCode skill stub missing %q:\n%s", want, skillBody)
		}
	}
	pluginBody, err := os.ReadFile(filepath.Join(out, "plugin", "targets", "opencode", "plugins", "example.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pluginBody), "export const ExamplePlugin = async") {
		t.Fatalf("unexpected OpenCode plugin starter:\n%s", pluginBody)
	}
	if strings.Contains(string(pluginBody), "export default") {
		t.Fatalf("OpenCode plugin starter still uses deprecated export default shape:\n%s", pluginBody)
	}
	opencodeBody, err := os.ReadFile(filepath.Join(out, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"mcp"`, `"https://example.com/mcp"`} {
		if !strings.Contains(string(opencodeBody), want) {
			t.Fatalf("opencode.json missing %q:\n%s", want, opencodeBody)
		}
	}
	packageJSONBody, err := os.ReadFile(filepath.Join(out, "plugin", "targets", "opencode", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"@opencode-ai/plugin": "1.4.0"`, `"type": "module"`} {
		if !strings.Contains(string(packageJSONBody), want) {
			t.Fatalf("OpenCode package.json missing %q:\n%s", want, packageJSONBody)
		}
	}
	readmeBody, err := os.ReadFile(filepath.Join(out, "plugin", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"official-style named async plugin exports",
		"plugin/targets/opencode/tools/",
		"@opencode-ai/plugin",
		"plugin/mcp/servers.yaml",
	} {
		if !strings.Contains(string(readmeBody), want) {
			t.Fatalf("OpenCode README missing %q:\n%s", want, readmeBody)
		}
	}
	rootReadmeBody, err := os.ReadFile(filepath.Join(out, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootReadmeBody), "[`plugin/README.md`](./plugin/README.md)") {
		t.Fatalf("OpenCode root README missing plugin pointer:\n%s", rootReadmeBody)
	}
}

func TestInitRunner_opencodeRejectsRuntimeFlag(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "opencode", Runtime: "python", OutputDir: out})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime is not supported with --platform opencode") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_cursorStarter(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "cursor", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "mcp", "servers.yaml"),
		filepath.Join("plugin", "skills", "genplug", "SKILL.md"),
		"CLAUDE.md",
		"AGENTS.md",
		"README.md",
		filepath.Join(".cursor-plugin", "plugin.json"),
		".mcp.json",
		filepath.Join("skills", "genplug", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		filepath.Join("plugin", "launcher.yaml"),
		"go.mod",
		filepath.Join("plugin", "targets", "cursor", "rules", "project.mdc"),
		filepath.Join(".cursor", "rules", "project.mdc"),
		filepath.Join(".cursor", "mcp.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected cursor starter file %s", rel)
		}
	}
	assertRuntimeTestAssetsAbsent(t, out)
	mcpBody, err := os.ReadFile(filepath.Join(out, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(mcpBody, &doc); err != nil {
		t.Fatalf("parse .mcp.json: %v\n%s", err, mcpBody)
	}
	server, ok := doc["docs"]
	if !ok {
		t.Fatalf(".mcp.json missing docs server:\n%s", mcpBody)
	}
	if got := strings.TrimSpace(fmt.Sprint(server["type"])); got != "http" {
		t.Fatalf("cursor example-http type = %q want http\n%s", got, mcpBody)
	}
	if got := strings.TrimSpace(fmt.Sprint(server["url"])); got != "https://example.com/mcp" {
		t.Fatalf("cursor example-http url = %q want https://example.com/mcp\n%s", got, mcpBody)
	}
}

func TestInitRunner_cursorWorkspaceStarter(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "cursor-workspace", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "mcp", "servers.yaml"),
		"CLAUDE.md",
		"AGENTS.md",
		"README.md",
		filepath.Join("plugin", "targets", "cursor-workspace", "rules", "project.mdc"),
		filepath.Join(".cursor", "rules", "project.mdc"),
		filepath.Join(".cursor", "mcp.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		filepath.Join("plugin", "launcher.yaml"),
		"go.mod",
		filepath.Join(".cursor-plugin", "plugin.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected cursor workspace starter file %s", rel)
		}
	}
	assertRuntimeTestAssetsAbsent(t, out)
}

func TestInitRunner_cursorRejectsRuntimeFlag(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "cursor", Runtime: "python", OutputDir: out})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime is not supported with --platform cursor") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_codexRuntime(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "codex-runtime", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "launcher.yaml"),
		filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"),
		"CLAUDE.md",
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	assertRuntimeTestAssetsExist(t, out, "codex-runtime")
	for _, rel := range []string{
		filepath.Join(".codex-plugin", "plugin.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected codex runtime starter file %s", rel)
		}
	}
}

func TestInitRunner_codexRuntimePython(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "codex-runtime", Runtime: "python", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "launcher.yaml"),
		filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"),
		filepath.Join("plugin", "main.py"),
		filepath.Join("bin", "genplug"),
		filepath.Join(".github", "workflows", "bundle-release.yml"),
		"CLAUDE.md",
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	assertRuntimeTestAssetsExist(t, out, "codex-runtime")
	for _, rel := range []string{
		filepath.Join(".codex-plugin", "plugin.json"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected codex runtime starter file %s", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(out, ".plugin-kit-ai", "project.toml")); !os.IsNotExist(err) {
		t.Fatalf("unsupported old manifest should not be generated, stat err = %v", err)
	}
}

func TestInitRunner_codexRuntimePythonRuntimePackage(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{
		ProjectName:           "genplug",
		Platform:              "codex-runtime",
		Runtime:               "python",
		RuntimePackage:        true,
		RuntimePackageVersion: scaffold.DefaultRuntimePackageVersion,
		OutputDir:             out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	if _, err := os.Stat(filepath.Join(out, "plugin", "plugin_runtime.py")); !os.IsNotExist(err) {
		t.Fatalf("vendored helper should stay absent, stat err=%v", err)
	}
	reqBody, err := os.ReadFile(filepath.Join(out, "requirements.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reqBody), "plugin-kit-ai-runtime=="+scaffold.DefaultRuntimePackageVersion) {
		t.Fatalf("requirements.txt missing shared runtime package:\n%s", reqBody)
	}
	mainBody, err := os.ReadFile(filepath.Join(out, "plugin", "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainBody), "from plugin_kit_ai_runtime import") {
		t.Fatalf("main.py missing shared runtime import:\n%s", mainBody)
	}
}

func TestInitRunner_codexRuntimeNodeTypeScript(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{
		ProjectName: "genplug",
		Platform:    "codex-runtime",
		Runtime:     "node",
		TypeScript:  true,
		OutputDir:   out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "launcher.yaml"),
		"package.json",
		"tsconfig.json",
		filepath.Join("plugin", "main.ts"),
		filepath.Join("bin", "genplug"),
		"CLAUDE.md",
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	assertRuntimeTestAssetsExist(t, out, "codex-runtime")
}

func TestInitRunner_codexRuntimeNodeTypeScriptRuntimePackage(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{
		ProjectName:           "genplug",
		Platform:              "codex-runtime",
		Runtime:               "node",
		TypeScript:            true,
		RuntimePackage:        true,
		RuntimePackageVersion: scaffold.DefaultRuntimePackageVersion,
		OutputDir:             out,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	if _, err := os.Stat(filepath.Join(out, "plugin", "plugin-runtime.ts")); !os.IsNotExist(err) {
		t.Fatalf("vendored helper should stay absent, stat err=%v", err)
	}
	mainBody, err := os.ReadFile(filepath.Join(out, "plugin", "main.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainBody), `from "plugin-kit-ai-runtime"`) {
		t.Fatalf("main.ts missing shared runtime import:\n%s", mainBody)
	}
	pkgBody, err := os.ReadFile(filepath.Join(out, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pkgBody), `"plugin-kit-ai-runtime": "`+scaffold.DefaultRuntimePackageVersion+`"`) {
		t.Fatalf("package.json missing shared runtime dependency:\n%s", pkgBody)
	}
	assertRuntimeTestAssetsExist(t, out, "codex-runtime")
}

func assertRuntimeTestAssetsExist(t *testing.T, root, platform string) {
	t.Helper()
	var rels []string
	switch platform {
	case "claude":
		rels = []string{
			filepath.Join("fixtures", "claude", "Stop.json"),
			filepath.Join("fixtures", "claude", "PreToolUse.json"),
			filepath.Join("fixtures", "claude", "UserPromptSubmit.json"),
			filepath.Join("goldens", "claude", "Stop.stdout"),
			filepath.Join("goldens", "claude", "Stop.stderr"),
			filepath.Join("goldens", "claude", "Stop.exitcode"),
			filepath.Join("goldens", "claude", "PreToolUse.stdout"),
			filepath.Join("goldens", "claude", "PreToolUse.stderr"),
			filepath.Join("goldens", "claude", "PreToolUse.exitcode"),
			filepath.Join("goldens", "claude", "UserPromptSubmit.stdout"),
			filepath.Join("goldens", "claude", "UserPromptSubmit.stderr"),
			filepath.Join("goldens", "claude", "UserPromptSubmit.exitcode"),
		}
	case "codex-runtime":
		rels = []string{
			filepath.Join("fixtures", "codex-runtime", "Notify.json"),
			filepath.Join("goldens", "codex-runtime", "Notify.stdout"),
			filepath.Join("goldens", "codex-runtime", "Notify.stderr"),
			filepath.Join("goldens", "codex-runtime", "Notify.exitcode"),
		}
	default:
		t.Fatalf("unsupported platform %q", platform)
	}
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}

func assertRuntimeTestAssetsAbsent(t *testing.T, root string) {
	t.Helper()
	for _, rel := range []string{"fixtures", "goldens"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to stay absent, err=%v", rel, err)
		}
	}
}

func TestInitRunner_claudeNodeExtrasEmitBundleReleaseWorkflow(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	_, err := r.Run(InitOptions{
		ProjectName: "genplug",
		Platform:    "claude",
		Runtime:     "node",
		OutputDir:   out,
		Extras:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	workflowBody, err := os.ReadFile(filepath.Join(out, ".github", "workflows", "bundle-release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowBody)
	for _, want := range []string{
		"actions/setup-node@v6",
		"777genius/universal-agent-plugins/setup-plugin-kit-ai@v1",
		"plugin-kit-ai validate . --platform claude --strict",
		"plugin-kit-ai bundle publish . --platform claude --repo ${{ github.repository }} --tag ${{ github.ref_name }}",
	} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("workflow missing %q:\n%s", want, workflow)
		}
	}
}

func TestInitRunner_TypeScriptRejectedOutsideNodeRuntime(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{
		ProjectName: "genplug",
		Platform:    "codex-runtime",
		TypeScript:  true,
		OutputDir:   filepath.Join(t.TempDir(), "genplug"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--typescript requires --runtime node") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_RuntimePackageRejectedOutsidePythonNode(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{
		ProjectName:    "genplug",
		Platform:       "codex-runtime",
		RuntimePackage: true,
		OutputDir:      filepath.Join(t.TempDir(), "genplug"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime-package requires --runtime python or --runtime node") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_RuntimePackageVersionRejectedWithoutRuntimePackage(t *testing.T) {
	t.Parallel()
	var r InitRunner
	_, err := r.Run(InitOptions{
		ProjectName:           "genplug",
		Platform:              "codex-runtime",
		Runtime:               "python",
		RuntimePackageVersion: scaffold.DefaultRuntimePackageVersion,
		OutputDir:             filepath.Join(t.TempDir(), "genplug"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime-package-version requires --runtime-package") {
		t.Fatalf("error = %q", err)
	}
}

func TestInitRunner_codexPackage(t *testing.T) {
	t.Parallel()
	var r InitRunner
	out := filepath.Join(t.TempDir(), "genplug")
	got, err := r.Run(InitOptions{ProjectName: "genplug", Platform: "codex-package", OutputDir: out, Extras: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != out {
		t.Fatalf("out = %q, want %q", got, out)
	}
	for _, rel := range []string{
		filepath.Join("plugin", "plugin.yaml"),
		filepath.Join("plugin", "mcp", "servers.yaml"),
		filepath.Join("plugin", "targets", "codex-package", "package.yaml"),
		filepath.Join("plugin", "targets", "codex-package", "interface.json"),
		filepath.Join("plugin", "targets", "codex-package", "app.json"),
		"CLAUDE.md",
		"AGENTS.md",
		filepath.Join(".codex-plugin", "plugin.json"),
		".mcp.json",
		filepath.Join("plugin", "skills", "genplug", "SKILL.md"),
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
	for _, rel := range []string{
		filepath.Join("plugin", "launcher.yaml"),
		filepath.Join(".codex", "config.toml"),
		"go.mod",
	} {
		if _, err := os.Stat(filepath.Join(out, rel)); !os.IsNotExist(err) {
			t.Fatalf("unexpected codex package starter file %s", rel)
		}
	}
	assertRuntimeTestAssetsAbsent(t, out)
	manifestBody, err := os.ReadFile(filepath.Join(out, ".codex-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"mcpServers": "./.mcp.json"`, `"name": "genplug"`} {
		if !strings.Contains(string(manifestBody), want) {
			t.Fatalf("codex plugin manifest missing %q:\n%s", want, manifestBody)
		}
	}
	if strings.Contains(string(manifestBody), `"apps": "./.app.json"`) {
		t.Fatalf("codex plugin manifest unexpectedly enables empty app placeholder:\n%s", manifestBody)
	}
	if _, err := os.Stat(filepath.Join(out, ".app.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected .app.json generated for empty app placeholder")
	}
	interfaceBody, err := os.ReadFile(filepath.Join(out, "plugin", "targets", "codex-package", "interface.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(interfaceBody), `"defaultPrompt": [`) {
		t.Fatalf("codex interface starter missing defaultPrompt array:\n%s", interfaceBody)
	}
}
