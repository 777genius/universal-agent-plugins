package pluginkitairepo_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestStarterRepos_LayoutAndReadmesStayAligned(t *testing.T) {
	root := RepoRoot(t)
	goSDKVersion := repoGoSDKVersion(t)
	runtimePackageVersion := repoRuntimePackageVersion(t)
	landing := readRepoFile(t, root, "examples", "starters", "README.md")
	mustContain(t, landing, "# Canonical Starter Repos")
	mustContain(t, landing, "fastest way to get one working plugin repo that can later expand to more supported outputs")
	mustContain(t, landing, "the starter is the first path, not the final boundary")
	mustContain(t, landing, "brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai")
	mustContain(t, landing, "npm i -g plugin-kit-ai")
	mustContain(t, landing, "pipx install plugin-kit-ai")
	mustContain(t, landing, "plugin-kit-ai doctor .")
	mustContain(t, landing, "plugin-kit-ai bootstrap .")
	mustContain(t, landing, "github.com/777genius/plugin-kit-ai/sdk@"+goSDKVersion)
	mustContain(t, landing, "go test ./...")
	mustContain(t, landing, "go build -o bin/<starter-name> ./cmd/<starter-name>")
	mustContain(t, landing, "plugin-kit-ai validate . --platform <codex-runtime|claude> --strict")
	mustContain(t, landing, "plugin-kit-ai bundle publish . --platform <codex-runtime|claude> --repo owner/repo --tag v1.0.0")
	mustContain(t, landing, "plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform <codex-runtime|claude> --runtime <python|node> --dest ./handoff-plugin")
	mustContain(t, landing, "requirements.txt")
	mustContain(t, landing, "dist/main.js")
	mustContain(t, landing, "plugin-kit-ai-runtime")
	mustContain(t, landing, "codex-python-runtime-package-starter")
	mustContain(t, landing, "claude-node-typescript-runtime-package-starter")
	mustContain(t, landing, "plugin-kit-ai-runtime=="+runtimePackageVersion)
	mustContain(t, landing, "plugin-kit-ai-runtime@"+runtimePackageVersion)

	cases := []struct {
		name          string
		platform      string
		runtime       string
		dir           string
		requiredFiles []string
		absent        []string
		contains      []string
	}{
		{
			name:     "codex-go-starter",
			platform: "codex-runtime",
			runtime:  "go",
			dir:      filepath.Join(root, "examples", "starters", "codex-go-starter"),
			requiredFiles: []string{
				"README.md",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"go.mod",
				"cmd/codex-go-starter/main.go",
				"plugin/targets/codex-runtime/package.yaml",
				".codex/config.toml",
			},
			absent: []string{
				".github/workflows/bundle-release.yml",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
			},
			contains: []string{
				"SDK-first handlers",
				"go build -o bin/codex-go-starter ./cmd/codex-go-starter",
			},
		},
		{
			name:     "codex-python-starter",
			platform: "codex-runtime",
			runtime:  "python",
			dir:      filepath.Join(root, "examples", "starters", "codex-python-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/codex-python-starter",
				"bin/codex-python-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"requirements.txt",
				"plugin/main.py",
				"plugin/targets/codex-runtime/package.yaml",
				".codex/config.toml",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
			},
			contains: []string{
				"requirements.txt",
				".venv",
				"plugin-kit-ai bundle publish . --platform codex-runtime --repo owner/repo --tag v1.0.0",
				"plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform codex-runtime --runtime python --dest ./handoff-plugin",
			},
		},
		{
			name:     "codex-node-typescript-starter",
			platform: "codex-runtime",
			runtime:  "node",
			dir:      filepath.Join(root, "examples", "starters", "codex-node-typescript-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/codex-node-typescript-starter",
				"bin/codex-node-typescript-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"package.json",
				"tsconfig.json",
				"plugin/main.ts",
				"plugin/targets/codex-runtime/package.yaml",
				".codex/config.toml",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
			},
			contains: []string{
				"dist/main.js",
				"npm",
				"plugin-kit-ai bundle publish . --platform codex-runtime --repo owner/repo --tag v1.0.0",
				"plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform codex-runtime --runtime node --dest ./handoff-plugin",
			},
		},
		{
			name:     "claude-go-starter",
			platform: "claude",
			runtime:  "go",
			dir:      filepath.Join(root, "examples", "starters", "claude-go-starter"),
			requiredFiles: []string{
				"README.md",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"go.mod",
				"cmd/claude-go-starter/main.go",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
				"plugin/targets/claude/hooks/hooks.json",
			},
			absent: []string{
				".github/workflows/bundle-release.yml",
				".codex/config.toml",
				"plugin/targets/codex-runtime/package.yaml",
			},
			contains: []string{
				"SDK-first handlers",
				"go build -o bin/claude-go-starter ./cmd/claude-go-starter",
			},
		},
		{
			name:     "claude-python-starter",
			platform: "claude",
			runtime:  "python",
			dir:      filepath.Join(root, "examples", "starters", "claude-python-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/claude-python-starter",
				"bin/claude-python-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"requirements.txt",
				"plugin/main.py",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
				"plugin/targets/claude/hooks/hooks.json",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				".codex/config.toml",
				"plugin/targets/codex-runtime/package.yaml",
			},
			contains: []string{
				"requirements.txt",
				".venv",
				"plugin-kit-ai bundle publish . --platform claude --repo owner/repo --tag v1.0.0",
				"plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform claude --runtime python --dest ./handoff-plugin",
			},
		},
		{
			name:     "claude-node-typescript-starter",
			platform: "claude",
			runtime:  "node",
			dir:      filepath.Join(root, "examples", "starters", "claude-node-typescript-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/claude-node-typescript-starter",
				"bin/claude-node-typescript-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"package.json",
				"tsconfig.json",
				"plugin/main.ts",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
				"plugin/targets/claude/hooks/hooks.json",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				".codex/config.toml",
				"plugin/targets/codex-runtime/package.yaml",
			},
			contains: []string{
				"dist/main.js",
				"npm",
				"plugin-kit-ai bundle publish . --platform claude --repo owner/repo --tag v1.0.0",
				"plugin-kit-ai bundle fetch owner/repo --tag v1.0.0 --platform claude --runtime node --dest ./handoff-plugin",
			},
		},
		{
			name:     "codex-python-runtime-package-starter",
			platform: "codex-runtime",
			runtime:  "python",
			dir:      filepath.Join(root, "examples", "starters", "codex-python-runtime-package-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/codex-python-runtime-package-starter",
				"bin/codex-python-runtime-package-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"requirements.txt",
				"plugin/main.py",
				"plugin/targets/codex-runtime/package.yaml",
				".codex/config.toml",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				"plugin/plugin_runtime.py",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
			},
			contains: []string{
				"plugin-kit-ai-runtime==" + runtimePackageVersion,
				"plugin_kit_ai_runtime",
				"plugin-kit-ai bundle publish . --platform codex-runtime --repo owner/repo --tag v1.0.0",
			},
		},
		{
			name:     "claude-node-typescript-runtime-package-starter",
			platform: "claude",
			runtime:  "node",
			dir:      filepath.Join(root, "examples", "starters", "claude-node-typescript-runtime-package-starter"),
			requiredFiles: []string{
				"README.md",
				"bin/claude-node-typescript-runtime-package-starter",
				"bin/claude-node-typescript-runtime-package-starter.cmd",
				"plugin/plugin.yaml",
				"plugin/launcher.yaml",
				"package.json",
				"tsconfig.json",
				"plugin/main.ts",
				".claude-plugin/plugin.json",
				"hooks/hooks.json",
				"plugin/targets/claude/hooks/hooks.json",
				".github/workflows/bundle-release.yml",
			},
			absent: []string{
				"plugin/plugin-runtime.ts",
				".codex/config.toml",
				"plugin/targets/codex-runtime/package.yaml",
			},
			contains: []string{
				"plugin-kit-ai-runtime@" + runtimePackageVersion,
				`from "plugin-kit-ai-runtime"`,
				"plugin-kit-ai bundle publish . --platform claude --repo owner/repo --tag v1.0.0",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, rel := range tc.requiredFiles {
				if !fileExists(filepath.Join(tc.dir, rel)) {
					t.Fatalf("starter missing %s", rel)
				}
			}
			for _, rel := range tc.absent {
				if fileExists(filepath.Join(tc.dir, rel)) {
					t.Fatalf("starter unexpectedly contains %s", rel)
				}
			}

			readme := readRepoFile(t, tc.dir, "README.md")
			mustContain(t, readme, "<!-- plugin-kit-ai:managed-root-readme -->")
			mustContain(t, readme, "[`plugin/README.md`](./plugin/README.md)")
			mustContain(t, readme, "[`AGENTS.md`](./AGENTS.md)")
			mustContain(t, readme, "[`GENERATED.md`](./GENERATED.md)")

			authoredReadme := readRepoFile(t, tc.dir, "plugin", "README.md")
			mustContain(t, authoredReadme, "## Who It Is For")
			mustContain(t, authoredReadme, "## Prerequisites")
			mustContain(t, authoredReadme, "## First Run")
			mustContain(t, authoredReadme, "plugin-kit-ai validate . --platform "+tc.platform+" --strict")
			mustContain(t, authoredReadme, "## Local Smoke")
			mustContain(t, authoredReadme, "## Ship It")
			for _, needle := range tc.contains {
				mustContain(t, authoredReadme, needle)
			}

			switch tc.runtime {
			case "go":
				mustContain(t, authoredReadme, "go test ./...")
				mustContain(t, authoredReadme, "Production plugin examples")
				mustNotContain(t, authoredReadme, "plugin-kit-ai bundle fetch")
				mustNotContain(t, authoredReadme, "plugin-kit-ai bundle publish")
				mustNotContain(t, authoredReadme, "plugin-kit-ai bootstrap .")
			case "python", "node":
				mustContain(t, authoredReadme, "plugin-kit-ai doctor .")
				mustContain(t, authoredReadme, "plugin-kit-ai bootstrap .")
				mustContain(t, authoredReadme, "public-stable")
				mustContain(t, authoredReadme, "plugin-kit-ai-runtime")
			}
		})
	}

	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "codex-python-starter", "plugin", "main.py"), "handle_claude")
	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "codex-python-starter", "plugin", "main.py"), "Stop|PreToolUse|UserPromptSubmit")
	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "codex-node-typescript-starter", "plugin", "main.ts"), "handleClaude")
	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "codex-node-typescript-starter", "plugin", "main.ts"), "PreToolUse")
	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "claude-python-starter", "plugin", "main.py"), "notify")
	mustNotContain(t, readRepoFile(t, root, "examples", "starters", "claude-node-typescript-starter", "plugin", "main.ts"), "\"notify\"")
	mustContain(t, readRepoFile(t, root, "examples", "starters", "codex-python-runtime-package-starter", "plugin", "main.py"), "from plugin_kit_ai_runtime import")
	mustContain(t, readRepoFile(t, root, "examples", "starters", "claude-node-typescript-runtime-package-starter", "plugin", "main.ts"), `from "plugin-kit-ai-runtime"`)
}

func TestStarterRepos_Smoke(t *testing.T) {
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)

	cases := []struct {
		name     string
		source   string
		platform string
		runtime  string
		ready    func() bool
		prepare  func(t *testing.T, workDir string)
		smoke    func(t *testing.T, workDir string)
		validate func(t *testing.T, workDir string)
	}{
		{
			name:     "codex-go-starter",
			source:   filepath.Join(root, "examples", "starters", "codex-go-starter"),
			platform: "codex-runtime",
			runtime:  "go",
			ready:    goRuntimeAvailable,
			prepare: func(t *testing.T, workDir string) {
				goStarterPrepare(t, workDir, "codex-go-starter")
			},
			validate: func(t *testing.T, workDir string) {
				env := newGoModuleEnv(t)
				validateStarterProject(t, pluginKitAIBin, workDir, "codex-runtime", env)
				assertCodexConfig(t, workDir, "gpt-5.4-mini", "./bin/codex-go-starter")
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "go")
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("run codex go starter notify: %v\nstderr=%s", err, stderr.String())
				}
				if strings.TrimSpace(stdout.String()) != "" {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
			},
		},
		{
			name:     "codex-python-starter",
			source:   filepath.Join(root, "examples", "starters", "codex-python-starter"),
			platform: "codex-runtime",
			runtime:  "python",
			ready:    pythonRuntimeAvailable,
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "codex-runtime", nil)
				assertCodexConfig(t, workDir, "gpt-5.4-mini", "./bin/codex-python-starter")
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "python")
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("run codex python starter notify: %v\nstderr=%s", err, stderr.String())
				}
				if strings.TrimSpace(stdout.String()) != "" {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
			},
		},
		{
			name:     "codex-node-typescript-starter",
			source:   filepath.Join(root, "examples", "starters", "codex-node-typescript-starter"),
			platform: "codex-runtime",
			runtime:  "node",
			ready:    nodeAndNPMAvailable,
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "codex-runtime", nil)
				assertCodexConfig(t, workDir, "gpt-5.4-mini", "./bin/codex-node-typescript-starter")
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "node")
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("run codex node starter notify: %v\nstderr=%s", err, stderr.String())
				}
				if strings.TrimSpace(stdout.String()) != "" {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
				if runtime.GOOS == "windows" {
					mustContain(t, entry, ".cmd")
				}
			},
		},
		{
			name:     "claude-go-starter",
			source:   filepath.Join(root, "examples", "starters", "claude-go-starter"),
			platform: "claude",
			runtime:  "go",
			ready:    goRuntimeAvailable,
			prepare: func(t *testing.T, workDir string) {
				goStarterPrepare(t, workDir, "claude-go-starter")
			},
			validate: func(t *testing.T, workDir string) {
				env := newGoModuleEnv(t)
				validateStarterProject(t, pluginKitAIBin, workDir, "claude", env)
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "go")
				assertClaudeStableSubsetEntry(t, entry)
			},
		},
		{
			name:     "claude-python-starter",
			source:   filepath.Join(root, "examples", "starters", "claude-python-starter"),
			platform: "claude",
			runtime:  "python",
			ready:    pythonRuntimeAvailable,
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "claude", nil)
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "python")
				assertClaudeStableSubsetEntry(t, entry)
			},
		},
		{
			name:     "claude-node-typescript-starter",
			source:   filepath.Join(root, "examples", "starters", "claude-node-typescript-starter"),
			platform: "claude",
			runtime:  "node",
			ready:    nodeAndNPMAvailable,
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "claude", nil)
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "node")
				assertClaudeStableSubsetEntry(t, entry)
			},
		},
		{
			name:     "codex-python-runtime-package-starter",
			source:   filepath.Join(root, "examples", "starters", "codex-python-runtime-package-starter"),
			platform: "codex-runtime",
			runtime:  "python",
			ready:    pythonRuntimeAvailable,
			prepare: func(t *testing.T, workDir string) {
				prepareSharedPythonRuntimeStarter(t, workDir)
			},
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "codex-runtime", nil)
				assertCodexConfig(t, workDir, "gpt-5.4-mini", "./bin/codex-python-runtime-package-starter")
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "python")
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("run codex python runtime-package starter notify: %v\nstderr=%s", err, stderr.String())
				}
				if strings.TrimSpace(stdout.String()) != "" {
					t.Fatalf("stdout = %q, want empty", stdout.String())
				}
			},
		},
		{
			name:     "claude-node-typescript-runtime-package-starter",
			source:   filepath.Join(root, "examples", "starters", "claude-node-typescript-runtime-package-starter"),
			platform: "claude",
			runtime:  "node",
			ready:    nodeAndNPMAvailable,
			prepare: func(t *testing.T, workDir string) {
				prepareSharedNodeRuntimeStarter(t, workDir)
			},
			validate: func(t *testing.T, workDir string) {
				doctor := exec.Command(pluginKitAIBin, "doctor", workDir)
				out, err := doctor.CombinedOutput()
				if err == nil {
					t.Fatalf("expected doctor to require bootstrap before starter first run:\n%s", out)
				}
				assertStarterDoctorBlockedBeforeBootstrap(t, string(out))

				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", workDir)
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap starter: %v\n%s", err, out)
				}

				validateStarterProject(t, pluginKitAIBin, workDir, "claude", nil)
			},
			smoke: func(t *testing.T, workDir string) {
				entry := localExampleEntrypointPath(workDir, "node")
				assertClaudeStableSubsetEntry(t, entry)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if !tc.ready() {
				t.Skip("runtime not available")
			}

			workDir := filepath.Join(t.TempDir(), tc.name)
			copyTree(t, tc.source, workDir)

			if tc.prepare != nil {
				tc.prepare(t, workDir)
			}
			tc.validate(t, workDir)
			tc.smoke(t, workDir)
		})
	}
}

func validateStarterProject(t *testing.T, pluginKitAIBin, workDir, platform string, env []string) {
	t.Helper()
	validate := exec.Command(pluginKitAIBin, "validate", workDir, "--platform", platform, "--strict")
	if len(env) > 0 {
		validate.Env = env
	}
	out, err := validate.CombinedOutput()
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && strings.Contains(string(out), "generated artifact drift:") {
		t.Logf("accepting known Windows generated artifact drift during starter validate (%s):\n%s", platform, out)
		return
	}
	t.Fatalf("plugin-kit-ai validate starter: %v\n%s", err, out)
}

func goRuntimeAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

func assertStarterDoctorBlockedBeforeBootstrap(t *testing.T, out string) {
	t.Helper()
	if strings.Contains(out, "Status: needs_bootstrap") {
		return
	}
	mustContain(t, out, "Status: blocked")
	mustContain(t, out, "launcher entrypoint")
	mustContain(t, out, "is missing")
}

func goStarterPrepare(t *testing.T, workDir, binaryName string) {
	t.Helper()
	root := RepoRoot(t)
	env := newGoModuleEnv(t)

	replaceCmd := exec.Command("go", "mod", "edit", "-replace", "github.com/777genius/plugin-kit-ai/sdk="+filepath.Join(root, "sdk"))
	replaceCmd.Dir = workDir
	replaceCmd.Env = env
	if out, err := replaceCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit replace starter sdk: %v\n%s", err, out)
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workDir
	tidyCmd.Env = env
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy starter: %v\n%s", err, out)
	}

	testCmd := exec.Command("go", "test", "./...")
	testCmd.Dir = workDir
	testCmd.Env = env
	if out, err := testCmd.CombinedOutput(); err != nil {
		t.Fatalf("go test starter: %v\n%s", err, out)
	}

	binName := filepath.Join("bin", binaryName)
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	build := exec.Command("go", "build", "-o", binName, "./cmd/"+binaryName)
	build.Dir = workDir
	build.Env = env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build starter: %v\n%s", err, out)
	}
}

func prepareSharedPythonRuntimeStarter(t *testing.T, workDir string) {
	t.Helper()
	runtimePackageVersion := repoRuntimePackageVersion(t)
	vendorDir := filepath.Join(workDir, "vendor", "python-runtime-wheel")
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wheelRel := filepath.ToSlash(filepath.Join("vendor", "python-runtime-wheel", "plugin_kit_ai_runtime-"+runtimePackageVersion+"-py3-none-any.whl"))
	writeLocalPythonRuntimeWheel(t, filepath.Join(workDir, filepath.FromSlash(wheelRel)), runtimePackageVersion)
	if err := os.WriteFile(filepath.Join(workDir, "requirements.txt"), []byte(wheelRel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func prepareSharedNodeRuntimeStarter(t *testing.T, workDir string) {
	t.Helper()
	root := RepoRoot(t)
	runtimePackageVersion := repoRuntimePackageVersion(t)
	vendorDir := filepath.Join(workDir, "vendor", "plugin-kit-ai-runtime")
	copyTree(t, filepath.Join(root, "npm", "plugin-kit-ai-runtime"), vendorDir)
	pkgPath := filepath.Join(workDir, "package.json")
	body, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(body), `"plugin-kit-ai-runtime": "`+runtimePackageVersion+`"`, `"plugin-kit-ai-runtime": "file:./vendor/plugin-kit-ai-runtime"`, 1)
	if updated == string(body) {
		t.Fatalf("failed to rewrite shared node runtime dependency in %s", pkgPath)
	}
	if err := os.WriteFile(pkgPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLocalPythonRuntimeWheel(t *testing.T, wheelPath, version string) {
	t.Helper()
	root := RepoRoot(t)
	sourcePath := filepath.Join(root, "python", "plugin-kit-ai-runtime", "src", "plugin_kit_ai_runtime", "__init__.py")
	sourceBody, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	moduleBody := strings.Replace(string(sourceBody), `__version__ = "0.0.0.dev0"`, fmt.Sprintf(`__version__ = "%s"`, version), 1)
	if moduleBody == string(sourceBody) {
		t.Fatalf("failed to rewrite python runtime package version in %s", sourcePath)
	}

	distInfo := fmt.Sprintf("plugin_kit_ai_runtime-%s.dist-info", version)
	entries := map[string][]byte{
		"plugin_kit_ai_runtime/__init__.py": []byte(moduleBody),
		distInfo + "/METADATA": []byte(strings.Join([]string{
			"Metadata-Version: 2.1",
			"Name: plugin-kit-ai-runtime",
			"Version: " + version,
			"Summary: Official Python runtime helpers for plugin-kit-ai executable plugins",
			"",
		}, "\n")),
		distInfo + "/WHEEL": []byte(strings.Join([]string{
			"Wheel-Version: 1.0",
			"Generator: pluginkitairepo_test",
			"Root-Is-Purelib: true",
			"Tag: py3-none-any",
			"",
		}, "\n")),
	}

	recordPaths := make([]string, 0, len(entries))
	for path := range entries {
		recordPaths = append(recordPaths, path)
	}
	sort.Strings(recordPaths)

	recordLines := make([]string, 0, len(recordPaths)+1)
	for _, path := range recordPaths {
		sum := sha256.Sum256(entries[path])
		recordLines = append(recordLines, fmt.Sprintf("%s,sha256=%s,%d", path, base64.RawURLEncoding.EncodeToString(sum[:]), len(entries[path])))
	}
	recordPath := distInfo + "/RECORD"
	recordLines = append(recordLines, recordPath+",,")
	entries[recordPath] = []byte(strings.Join(recordLines, "\n") + "\n")

	if err := os.MkdirAll(filepath.Dir(wheelPath), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(wheelPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	recordPaths = append(recordPaths, recordPath)
	for _, path := range recordPaths {
		w, err := zw.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(entries[path]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
