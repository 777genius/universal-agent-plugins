package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIInitGeneratesBuildableModule(t *testing.T) {
	for _, platform := range []string{"claude", "codex-runtime", "codex-package", "gemini", "opencode", "cursor"} {
		t.Run(platform, func(t *testing.T) {
			root := RepoRoot(t)
			cliDir := filepath.Join(root, "cli")

			binDir := t.TempDir()
			bin := filepath.Join(binDir, "plugin-kit-ai")
			build := exec.Command("go", "build", "-o", bin, "./cmd/plugin-kit-ai")
			build.Dir = cliDir
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("build plugin-kit-ai: %v\n%s", err, out)
			}

			plugRoot := filepath.Join(t.TempDir(), "genplug")
			args := []string{"init", "genplug", "--platform", platform, "-o", plugRoot, "--extras"}
			if platform != "gemini" && platform != "codex-package" && platform != "opencode" && platform != "cursor" {
				args = append(args, "--runtime", "go")
			}
			run := exec.Command(bin, args...)
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			if platform == "gemini" || platform == "codex-package" || platform == "opencode" || platform == "cursor" {
				validate := exec.Command(bin, "validate", plugRoot, "--platform", platform, "--strict")
				validate.Env = append(os.Environ(), "GOWORK=off")
				if out, err := validate.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai validate: %v\n%s", err, out)
				}
				for _, rel := range []string{authoredRel("launcher.yaml"), "go.mod"} {
					if _, err := os.Stat(filepath.Join(plugRoot, rel)); !os.IsNotExist(err) {
						t.Fatalf("%s starter unexpectedly wrote %s", platform, rel)
					}
				}
				assertConfigTargetPortableMCPScaffold(t, plugRoot)
				assertConfigTargetRenderedOutputs(t, plugRoot, platform)
				return
			}

			env := newGoModuleEnv(t)
			wireGeneratedGoModuleToLocalSDK(t, plugRoot, env)

			tidy := exec.Command("go", "mod", "tidy")
			tidy.Dir = plugRoot
			tidy.Env = env
			if out, err := tidy.CombinedOutput(); err != nil {
				t.Fatalf("go mod tidy in generated module: %v\n%s", err, out)
			}

			validate := exec.Command(bin, "validate", plugRoot, "--platform", platform)
			validate.Env = env
			if out, err := validate.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai validate: %v\n%s", err, out)
			}

			test := exec.Command("go", "test", "./...")
			test.Dir = plugRoot
			test.Env = env
			if out, err := test.CombinedOutput(); err != nil {
				t.Fatalf("go test in generated module: %v\n%s", err, out)
			}

			vet := exec.Command("go", "vet", "./...")
			vet.Dir = plugRoot
			vet.Env = env
			if out, err := vet.CombinedOutput(); err != nil {
				t.Fatalf("go vet in generated module: %v\n%s", err, out)
			}
		})
	}
}

func assertConfigTargetPortableMCPScaffold(t *testing.T, root string) {
	t.Helper()
	rel := authoredRel("mcp", "servers.yaml")
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	for _, want := range []string{
		"api_version: v1",
		`url: "https://example.com/mcp"`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("portable MCP scaffold missing %q:\n%s", want, body)
		}
	}
}

func assertConfigTargetRenderedOutputs(t *testing.T, root, platform string) {
	t.Helper()
	switch platform {
	case "codex-package":
		body, err := os.ReadFile(filepath.Join(root, ".codex-plugin", "plugin.json"))
		if err != nil {
			t.Fatalf("read .codex-plugin/plugin.json: %v", err)
		}
		for _, want := range []string{`"mcpServers": "./.mcp.json"`, `"name": "genplug"`} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("codex-package output missing %q:\n%s", want, body)
			}
		}
		if _, err := os.Stat(filepath.Join(root, ".mcp.json")); err != nil {
			t.Fatalf("stat .mcp.json: %v", err)
		}
		if strings.Contains(string(body), `"apps": "./.app.json"`) {
			t.Fatalf("codex-package output unexpectedly enables empty app placeholder:\n%s", body)
		}
		if _, err := os.Stat(filepath.Join(root, ".app.json")); !os.IsNotExist(err) {
			t.Fatalf("unexpected .app.json generated for empty app placeholder")
		}
		rel := authoredRel("targets", "codex-package", "interface.json")
		interfaceBody, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !strings.Contains(string(interfaceBody), `"defaultPrompt": [`) {
			t.Fatalf("codex-package interface starter missing defaultPrompt array:\n%s", interfaceBody)
		}
	case "gemini":
		body, err := os.ReadFile(filepath.Join(root, "gemini-extension.json"))
		if err != nil {
			t.Fatalf("read gemini-extension.json: %v", err)
		}
		for _, want := range []string{`"mcpServers"`, `"https://example.com/mcp"`} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("gemini output missing %q:\n%s", want, body)
			}
		}
	case "opencode":
		body, err := os.ReadFile(filepath.Join(root, "opencode.json"))
		if err != nil {
			t.Fatalf("read opencode.json: %v", err)
		}
		for _, want := range []string{`"mcp"`, `"https://example.com/mcp"`} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("opencode output missing %q:\n%s", want, body)
			}
		}
		rel := authoredRel("targets", "opencode", "package.json")
		packageBody, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !strings.Contains(string(packageBody), `"@opencode-ai/plugin": "1.4.0"`) {
			t.Fatalf("opencode package.json missing helper dependency:\n%s", packageBody)
		}
	case "cursor":
		manifestBody, err := os.ReadFile(filepath.Join(root, ".cursor-plugin", "plugin.json"))
		if err != nil {
			t.Fatalf("read .cursor-plugin/plugin.json: %v", err)
		}
		if !strings.Contains(string(manifestBody), `"mcpServers": "./.mcp.json"`) {
			t.Fatalf("cursor manifest missing managed MCP ref:\n%s", manifestBody)
		}
		body, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
		if err != nil {
			t.Fatalf("read .mcp.json: %v", err)
		}
		for _, want := range []string{`"https://example.com/mcp"`, `"type": "http"`} {
			if !strings.Contains(string(body), want) {
				t.Fatalf("cursor output missing %q:\n%s", want, body)
			}
		}
	default:
		t.Fatalf("unsupported config platform %q", platform)
	}
}
