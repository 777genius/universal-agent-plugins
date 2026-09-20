package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestValidateGeminiHookEntrypoints(t *testing.T) {
	body := []byte(`{
  "hooks": {
    "SessionStart": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionStart"}]}],
    "SessionEnd": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionEnd"}]}],
    "BeforeModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeModel"}]}],
    "AfterModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterModel"}]}],
    "BeforeToolSelection": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeToolSelection"}]}],
    "BeforeAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeAgent"}]}],
    "AfterAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterAgent"}]}],
    "BeforeTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeTool"}]}],
    "AfterTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterTool"}]}]
  }
}`)
	mismatches, err := validateGeminiHookEntrypoints(body, "./bin/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(mismatches) != 0 {
		t.Fatalf("mismatches = %v", mismatches)
	}
}

func TestValidateGeminiHookEntrypointsMismatch(t *testing.T) {
	body := []byte(`{
  "hooks": {
    "SessionStart": [{"matcher":"resume","hooks":[{"type":"command","command":"./bin/other GeminiSessionStart"}]}]
  }
}`)
	mismatches, err := validateGeminiHookEntrypoints(body, "./bin/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(mismatches) == 0 {
		t.Fatal("expected mismatches")
	}
}

func TestGeminiExtensionDirBase(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	got := geminiExtensionDirBase(filepath.Join(cwd, "."))
	if got != filepath.Base(cwd) {
		t.Fatalf("base = %q, want %q", got, filepath.Base(cwd))
	}
}

func TestInferGeminiEntrypoint(t *testing.T) {
	t.Parallel()
	body := []byte(`{
  "hooks": {
    "SessionStart": [{"matcher":"*","hooks":[{"type":"command","command":"${extensionPath}${/}bin${/}demo GeminiSessionStart"}]}],
    "SessionEnd": [{"matcher":"*","hooks":[{"type":"command","command":"${extensionPath}${/}bin${/}demo GeminiSessionEnd"}]}]
  }
}`)
	got, ok := inferGeminiEntrypoint(body)
	if !ok {
		t.Fatal("expected entrypoint inference")
	}
	if got != "./bin/demo" {
		t.Fatalf("entrypoint = %q", got)
	}
}

func TestGeminiImportInfersEntrypointWhenLauncherSeeded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
  "hooks": {
    "SessionStart": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionStart"}]}],
    "SessionEnd": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionEnd"}]}],
    "BeforeModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeModel"}]}],
    "AfterModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterModel"}]}],
    "BeforeToolSelection": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeToolSelection"}]}],
    "BeforeAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeAgent"}]}],
    "AfterAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterAgent"}]}],
    "BeforeTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeTool"}]}],
    "AfterTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterTool"}]}]
  }
}`)
	if err := os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	launcher := &pluginmodel.Launcher{Runtime: "go", Entrypoint: "./bin/placeholder"}
	result, err := (geminiAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
		Launcher: launcher,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Launcher == nil {
		t.Fatal("expected launcher")
	}
	if result.Launcher.Entrypoint != "./bin/demo" {
		t.Fatalf("entrypoint = %q", result.Launcher.Entrypoint)
	}
}

func TestGeminiImportCreatesLauncherWhenHooksExposeEntrypoint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{
  "hooks": {
    "SessionStart": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionStart"}]}],
    "SessionEnd": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionEnd"}]}],
    "BeforeModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeModel"}]}],
    "AfterModel": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterModel"}]}],
    "BeforeToolSelection": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeToolSelection"}]}],
    "BeforeAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeAgent"}]}],
    "AfterAgent": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterAgent"}]}],
    "BeforeTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiBeforeTool"}]}],
    "AfterTool": [{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiAfterTool"}]}]
  }
}`)
	if err := os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := (geminiAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Launcher == nil {
		t.Fatal("expected inferred launcher")
	}
	if result.Launcher.Runtime != "go" {
		t.Fatalf("runtime = %q", result.Launcher.Runtime)
	}
	if result.Launcher.Entrypoint != "./bin/demo" {
		t.Fatalf("entrypoint = %q", result.Launcher.Entrypoint)
	}
}

func TestRefineGeminiHooksLayoutRejectsUnexpectedPath(t *testing.T) {
	t.Parallel()

	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("hooks", filepath.Join("targets", "gemini", "hooks", "extra.json"))

	err := refineGeminiHooksLayout(&state)
	if err == nil || !strings.Contains(err.Error(), "unsupported Gemini hooks layout") {
		t.Fatalf("err = %v", err)
	}
}

func TestImportedGeminiPrimaryContextNameFallsBackToRootGeminiDoc(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "GEMINI.md"), []byte("# Gemini\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := importedGeminiPrimaryContextName(root, geminiPackageMeta{})
	if got != "GEMINI.md" {
		t.Fatalf("context name = %q", got)
	}
}

func TestGeminiRenderGeneratesDefaultHooksFromLauncher(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "targets", "gemini", "contexts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "gemini", "package.yaml"), []byte("context_file_name: GEMINI.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "gemini", "contexts", "GEMINI.md"), []byte("# Gemini\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "demo-gemini", Version: "0.1.0", Description: "demo", Targets: []string{"gemini"}},
		Launcher: &pluginmodel.Launcher{Runtime: "go", Entrypoint: "./bin/demo"},
	}
	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "GEMINI.md"))
	artifacts, err := (geminiAdapter{}).Generate(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	var hooksBody []byte
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.ToSlash(filepath.Join("hooks", "hooks.json")) {
			hooksBody = artifact.Content
			break
		}
	}
	if len(hooksBody) == 0 {
		t.Fatal("expected generated hooks/hooks.json")
	}
	for _, want := range []string{
		`${extensionPath}${/}bin${/}demo GeminiSessionStart`,
		`${extensionPath}${/}bin${/}demo GeminiSessionEnd`,
		`${extensionPath}${/}bin${/}demo GeminiBeforeModel`,
		`${extensionPath}${/}bin${/}demo GeminiAfterModel`,
		`${extensionPath}${/}bin${/}demo GeminiBeforeToolSelection`,
		`${extensionPath}${/}bin${/}demo GeminiBeforeAgent`,
		`${extensionPath}${/}bin${/}demo GeminiAfterAgent`,
		`${extensionPath}${/}bin${/}demo GeminiBeforeTool`,
		`${extensionPath}${/}bin${/}demo GeminiAfterTool`,
	} {
		if !strings.Contains(string(hooksBody), want) {
			t.Fatalf("hooks/hooks.json missing %q:\n%s", want, hooksBody)
		}
	}
	if mismatches, err := validateGeminiHookEntrypoints(hooksBody, "./bin/demo"); err != nil {
		t.Fatal(err)
	} else if len(mismatches) != 0 {
		t.Fatalf("mismatches = %v", mismatches)
	}
}

func TestGeminiValidateIgnoresSharedHooksWhenGeminiHooksAreNotEnabled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"hooks":{"UserPromptSubmit":[{"hooks":[{"type":"command","command":"./bin/demo UserPromptSubmit"}]}]}}`
	if err := os.WriteFile(filepath.Join(root, "hooks", "hooks.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "multi-target",
			Version:     "0.1.0",
			Description: "demo",
			Targets:     []string{"claude", "codex-package", "gemini"},
		},
		Launcher: &pluginmodel.Launcher{Runtime: "python", Entrypoint: "./bin/demo"},
	}

	diagnostics := validateGeminiGeneratedHooks(root, graph, nil)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestGeminiRenderProjectsPackageMetaSettingsThemesAndContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "package.yaml"), "exclude_tools:\n  - Shell(tool)\nplan_directory: plans\ncontext_file_name: GEMINI.md\n")
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "settings", "api-key.yaml"), "name: API key\ndescription: Demo key\nenv_var: GEMINI_API_KEY\nsensitive: true\n")
	writeGeminiValidateFile(t, filepath.Join(root, "targets", "gemini", "themes", "sunrise.yaml"), "name: Sunrise\naccent: orange\n")

	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "demo-gemini", Version: "0.1.0", Description: "demo", Targets: []string{"gemini"}},
	}
	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "GEMINI.md"))
	state.AddComponent("settings", filepath.Join("targets", "gemini", "settings", "api-key.yaml"))
	state.AddComponent("themes", filepath.Join("targets", "gemini", "themes", "sunrise.yaml"))

	artifacts, err := (geminiAdapter{}).Generate(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, ok := artifactBody(artifacts, "gemini-extension.json")
	if !ok {
		t.Fatalf("artifacts missing gemini-extension.json: %+v", artifacts)
	}
	for _, want := range []string{
		`"excludeTools": [`,
		`"Shell(tool)"`,
		`"plan": {`,
		`"directory": "plans"`,
		`"contextFileName": "GEMINI.md"`,
		`"settings": [`,
		`"envVar": "GEMINI_API_KEY"`,
		`"themes": [`,
		`"accent": "orange"`,
	} {
		if !strings.Contains(manifestJSON, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifestJSON)
		}
	}
}

func TestGeminiManagedPathsIncludesGeneratedHooksForDedicatedRuntimeRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "targets", "gemini", "contexts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "gemini", "package.yaml"), []byte("context_file_name: GEMINI.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "gemini", "contexts", "GEMINI.md"), []byte("# Gemini\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "demo-gemini", Version: "0.1.0", Description: "demo", Targets: []string{"gemini"}},
		Launcher: &pluginmodel.Launcher{Runtime: "go", Entrypoint: "./bin/demo"},
	}
	state := pluginmodel.NewTargetState("gemini")
	state.SetDoc("package_metadata", filepath.Join("targets", "gemini", "package.yaml"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "GEMINI.md"))
	paths, err := (geminiAdapter{}).ManagedPaths(root, graph, state)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("expected managed paths")
	}
	foundHooks := false
	foundContext := false
	for _, path := range paths {
		if path == "hooks/hooks.json" {
			foundHooks = true
		}
		if path == "GEMINI.md" {
			foundContext = true
		}
	}
	if !foundHooks {
		t.Fatalf("managed paths = %v, want hooks/hooks.json", paths)
	}
	if !foundContext {
		t.Fatalf("managed paths = %v, want GEMINI.md", paths)
	}
}

func TestSelectGeminiPrimaryContextRejectsAmbiguousCandidates(t *testing.T) {
	t.Parallel()
	state := pluginmodel.NewTargetState("gemini")
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "alpha.md"))
	state.AddComponent("contexts", filepath.Join("targets", "gemini", "contexts", "beta.md"))

	_, ok, err := selectGeminiPrimaryContext(pluginmodel.PackageGraph{}, state, geminiPackageMeta{})
	if err == nil {
		t.Fatal("expected ambiguous context selection error")
	}
	if ok {
		t.Fatal("expected no selected context on ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("error = %q, want ambiguity message", err.Error())
	}
}

func TestValidateGeminiMCPServersRejectsMultipleTransports(t *testing.T) {
	t.Parallel()
	diagnostics := validateGeminiMCPServers("gemini-extension.json", map[string]any{
		"context7": map[string]any{
			"command": "npx -y @upstash/context7-mcp",
			"url":     "https://example.com/mcp",
		},
	})
	if len(diagnostics) == 0 {
		t.Fatal("expected diagnostics")
	}
	var found bool
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "exactly one transport") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want transport conflict", diagnostics)
	}
}

func TestValidateGeminiMCPServersRejectsArgsWithoutCommand(t *testing.T) {
	t.Parallel()
	diagnostics := validateGeminiMCPServers("gemini-extension.json", map[string]any{
		"context7": map[string]any{
			"url":  "https://example.com/mcp",
			"args": []any{"--stdio"},
		},
	})
	if len(diagnostics) == 0 {
		t.Fatal("expected diagnostics")
	}
	var found bool
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "may only use args with command-based stdio transport") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want args transport contract", diagnostics)
	}
}

func TestValidateGeminiMCPServersWarnsOnSpaceDelimitedCommand(t *testing.T) {
	t.Parallel()
	diagnostics := validateGeminiMCPServers("gemini-extension.json", map[string]any{
		"context7": map[string]any{
			"command": "npx -y @upstash/context7-mcp",
		},
	})
	var found bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == CodeGeminiMCPCommandStyle {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want command style warning", diagnostics)
	}
}
