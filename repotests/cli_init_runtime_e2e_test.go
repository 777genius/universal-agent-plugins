package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestPluginKitAIInitGoRuntimeLauncherFlow(t *testing.T) {
	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			env := newGoModuleEnv(t)

			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "go", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			if platform == "codex-runtime" {
				assertCodexConfig(t, plugRoot, "gpt-5.4-mini", "./bin/genplug")
			}

			wireGeneratedGoModuleToLocalSDK(t, plugRoot, env)
			tidy := exec.Command("go", "mod", "tidy")
			tidy.Dir = plugRoot
			tidy.Env = env
			if out, err := tidy.CombinedOutput(); err != nil {
				t.Fatalf("go mod tidy: %v\n%s", err, out)
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, env, "plugin-kit-ai validate")

			binName := "genplug"
			if runtime.GOOS == "windows" {
				binName += ".exe"
			}
			build := exec.Command("go", "build", "-o", filepath.Join("bin", binName), "./cmd/genplug")
			build.Dir = plugRoot
			build.Env = env
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go build generated entrypoint: %v\n%s", err, out)
			}

			entry := filepath.Join(plugRoot, "bin", binName)
			switch platform {
			case "codex-runtime":
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("run go codex launcher: %v\n%s", err, out)
				}
				if strings.TrimSpace(string(out)) != "" {
					t.Fatalf("stdout = %q, want empty", string(out))
				}
			case "claude":
				assertClaudeStableSubsetEntry(t, entry)
			}
		})
	}
}

func TestPluginKitAIInitGeminiGoRuntimeLauncherFlow(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	env := newGoModuleEnv(t)

	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "gemini", "--runtime", "go", "-o", plugRoot, "--extras")
	initOut, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, initOut)
	}
	initText := string(initOut)
	for _, want := range []string{
		`Created plugin "genplug" at ` + plugRoot,
		`cd ` + strconv.Quote(plugRoot),
		"Portable MCP starter: " + authoredSlash("mcp", "servers.yaml"),
		"go test ./...",
		"plugin-kit-ai generate .",
		"plugin-kit-ai generate --check .",
		"plugin-kit-ai validate . --platform gemini --strict",
		"plugin-kit-ai inspect . --target gemini",
		"plugin-kit-ai capabilities --mode runtime --platform gemini",
		"make test-gemini-runtime",
		"gemini extensions link .",
		"make test-gemini-runtime-live",
		"See README.md for Gemini runtime steps",
	} {
		if !strings.Contains(initText, want) {
			t.Fatalf("init output missing %q:\n%s", want, initText)
		}
	}
	for _, unwanted := range []string{
		"plugin-kit-ai test .",
		"plugin-kit-ai dev .",
	} {
		if strings.Contains(initText, unwanted) {
			t.Fatalf("init output unexpectedly contains %q:\n%s", unwanted, initText)
		}
	}

	validateGeneratedProject(t, pluginKitAIBin, plugRoot, "gemini", env, "plugin-kit-ai validate")

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = plugRoot
	tidy.Env = env
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, out)
	}

	binName := "genplug"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join("bin", binName), "./cmd/genplug")
	build.Dir = plugRoot
	build.Env = env
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build generated entrypoint: %v\n%s", err, out)
	}

	generate := exec.Command(pluginKitAIBin, "generate", plugRoot)
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai generate: %v\n%s", err, out)
	}
	validateGeneratedProject(t, pluginKitAIBin, plugRoot, "gemini", env, "plugin-kit-ai validate after generate")

	report := inspectGeneratedProject(t, pluginKitAIBin, plugRoot, "gemini")
	target := requireInspectTarget(t, report, "gemini")
	mustHaveManagedArtifacts(t, target.ManagedArtifacts, "gemini-extension.json", "hooks/hooks.json")

	entry := filepath.Join(plugRoot, "bin", binName)
	assertGeminiRuntimeEntry(t, entry)

	hooksBody, err := os.ReadFile(filepath.Join(plugRoot, "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"command": "${extensionPath}${/}bin${/}genplug GeminiSessionStart"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiSessionEnd"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiBeforeModel"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiAfterModel"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiBeforeToolSelection"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiBeforeAgent"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiAfterAgent"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiBeforeTool"`,
		`"command": "${extensionPath}${/}bin${/}genplug GeminiAfterTool"`,
	} {
		if !strings.Contains(string(hooksBody), want) {
			t.Fatalf("generated hooks missing %q:\n%s", want, hooksBody)
		}
	}
}

func TestPluginKitAIInitNodeRuntimeSupportsTypeScriptBuildThroughLauncher(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			if platform == "codex-runtime" {
				assertCodexConfig(t, plugRoot, "gpt-5.4-mini", "./bin/genplug")
			}

			doctor := exec.Command(pluginKitAIBin, "doctor", plugRoot)
			out, err := doctor.CombinedOutput()
			if err == nil {
				t.Fatalf("expected doctor to report not ready before bootstrap:\n%s", out)
			}
			if !strings.Contains(string(out), "Status: needs_bootstrap") {
				t.Fatalf("doctor output missing needs_bootstrap status:\n%s", out)
			}

			validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", platform)
			out, err = validate.CombinedOutput()
			if err == nil {
				t.Fatalf("expected validate failure before bootstrap:\n%s", out)
			}
			if !strings.Contains(string(out), "TypeScript scaffold expects built output") {
				t.Fatalf("validate output missing TS-specific guidance:\n%s", out)
			}

			bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
			if out, err := bootstrap.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai bootstrap: %v\n%s", err, out)
			}

			doctor = exec.Command(pluginKitAIBin, "doctor", plugRoot)
			if out, err := doctor.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai doctor after bootstrap: %v\n%s", err, out)
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, nil, "plugin-kit-ai validate after bootstrap")

			entry := filepath.Join(plugRoot, "bin", "genplug")
			if runtime.GOOS == "windows" {
				entry += ".cmd"
			}
			switch platform {
			case "codex-runtime":
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("run TS-over-node codex launcher: %v\n%s", err, out)
				}
				if strings.TrimSpace(string(out)) != "" {
					t.Fatalf("stdout = %q, want empty", string(out))
				}
			case "claude":
				assertClaudeStableSubsetEntry(t, entry)
			}
		})
	}
}

func TestPluginKitAIInitRuntimePackageModeWritesSharedHelperImports(t *testing.T) {
	pluginKitAIBin := buildPluginKitAI(t)
	runtimePackageVersion := repoRuntimePackageVersion(t)

	t.Run("python", func(t *testing.T) {
		plugRoot := runtimeProjectRoot(t)
		run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "--runtime-package", "--runtime-package-version", runtimePackageVersion, "-o", plugRoot)
		if out, err := run.CombinedOutput(); err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(plugRoot, authoredRel("plugin_runtime.py"))); !os.IsNotExist(err) {
			t.Fatalf("vendored python helper should stay absent, stat err=%v", err)
		}
		mainBody, err := os.ReadFile(filepath.Join(plugRoot, authoredRel("main.py")))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(mainBody), "from plugin_kit_ai_runtime import") {
			t.Fatalf("main.py missing shared runtime import:\n%s", mainBody)
		}
		reqBody, err := os.ReadFile(filepath.Join(plugRoot, "requirements.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(reqBody), "plugin-kit-ai-runtime=="+runtimePackageVersion) {
			t.Fatalf("requirements.txt missing shared runtime dependency:\n%s", reqBody)
		}
	})

	t.Run("node-typescript", func(t *testing.T) {
		plugRoot := runtimeProjectRoot(t)
		run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "node", "--typescript", "--runtime-package", "--runtime-package-version", runtimePackageVersion, "-o", plugRoot)
		if out, err := run.CombinedOutput(); err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(plugRoot, authoredRel("plugin-runtime.ts"))); !os.IsNotExist(err) {
			t.Fatalf("vendored node helper should stay absent, stat err=%v", err)
		}
		mainBody, err := os.ReadFile(filepath.Join(plugRoot, authoredRel("main.ts")))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(mainBody), `from "plugin-kit-ai-runtime"`) {
			t.Fatalf("main.ts missing shared runtime import:\n%s", mainBody)
		}
		pkgBody, err := os.ReadFile(filepath.Join(plugRoot, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(pkgBody), `"plugin-kit-ai-runtime": "`+runtimePackageVersion+`"`) {
			t.Fatalf("package.json missing shared runtime dependency:\n%s", pkgBody)
		}
	})
}

func TestPluginKitAIInitRuntimePackageModeAutoPinsReleasedCLIVersion(t *testing.T) {
	goSDKVersion := repoGoSDKVersion(t)
	runtimePackageVersion := repoRuntimePackageVersion(t)
	pluginKitAIBin := buildPluginKitAIWithVersion(t, goSDKVersion)

	t.Run("python", func(t *testing.T) {
		plugRoot := runtimeProjectRoot(t)
		run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "--runtime-package", "-o", plugRoot)
		out, err := run.CombinedOutput()
		if err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "Shared helper dependency: plugin-kit-ai-runtime@"+runtimePackageVersion) {
			t.Fatalf("init output missing shared helper dependency line:\n%s", out)
		}
		reqBody, err := os.ReadFile(filepath.Join(plugRoot, "requirements.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(reqBody), "plugin-kit-ai-runtime=="+runtimePackageVersion) {
			t.Fatalf("requirements.txt missing released-cli runtime package pin:\n%s", reqBody)
		}
	})

	t.Run("node-typescript", func(t *testing.T) {
		plugRoot := runtimeProjectRoot(t)
		run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "node", "--typescript", "--runtime-package", "-o", plugRoot)
		out, err := run.CombinedOutput()
		if err != nil {
			t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "Shared helper dependency: plugin-kit-ai-runtime@"+runtimePackageVersion) {
			t.Fatalf("init output missing shared helper dependency line:\n%s", out)
		}
		pkgBody, err := os.ReadFile(filepath.Join(plugRoot, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(pkgBody), `"plugin-kit-ai-runtime": "`+runtimePackageVersion+`"`) {
			t.Fatalf("package.json missing released-cli runtime package pin:\n%s", pkgBody)
		}
	})
}

func TestPluginKitAIInitNodeRuntimePNPMDoctorBootstrapFlow(t *testing.T) {
	if !nodeRuntimeAvailable() {
		t.Skip("node not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	writeRuntimeFile(t, plugRoot, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	shimDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		writeRuntimeFile(t, shimDir, "pnpm.cmd", "@echo off\r\nif \"%1\"==\"install\" (\r\n  if not exist node_modules mkdir node_modules\r\n  > node_modules\\.installed echo ok\r\n  exit /b 0\r\n)\r\nif \"%1\"==\"run\" if \"%2\"==\"build\" (\r\n  if not exist dist mkdir dist\r\n  > dist\\main.js echo console.log('ok')\r\n  exit /b 0\r\n)\r\necho unexpected pnpm args %* 1>&2\r\nexit /b 1\r\n")
	} else {
		writeRuntimeFile(t, shimDir, "pnpm", "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"$1\" == \"install\" ]]; then\n  mkdir -p node_modules\n  printf 'ok' > node_modules/.installed\n  exit 0\nfi\nif [[ \"$1\" == \"run\" && \"$2\" == \"build\" ]]; then\n  mkdir -p dist\n  printf \"console.log('ok')\\n\" > dist/main.js\n  exit 0\nfi\necho \"unexpected pnpm args: $*\" >&2\nexit 1\n")
	}

	env := append(os.Environ(), "GOWORK=off", "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	doctor := exec.Command(pluginKitAIBin, "doctor", plugRoot)
	doctor.Env = env
	out, err := doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to report not ready before bootstrap:\n%s", out)
	}
	if !strings.Contains(string(out), "manager=pnpm") || !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing pnpm readiness guidance:\n%s", out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	bootstrap.Env = env
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap with pnpm shim: %v\n%s", err, out)
	}

	doctor = exec.Command(pluginKitAIBin, "doctor", plugRoot)
	doctor.Env = env
	if out, err := doctor.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai doctor after pnpm bootstrap: %v\n%s", err, out)
	}

	validateGeneratedProject(t, pluginKitAIBin, plugRoot, "codex-runtime", env, "plugin-kit-ai validate after pnpm bootstrap")
}

func TestPluginKitAIInitPythonRuntimeWithRequirementsDoctorBootstrapFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for launcher flow")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "python", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			writeRuntimeFile(t, plugRoot, "requirements.txt", "requests==2.32.0\n")

			doctor := exec.Command(pluginKitAIBin, "doctor", plugRoot)
			out, err := doctor.CombinedOutput()
			if err == nil {
				t.Fatalf("expected doctor to require bootstrap:\n%s", out)
			}
			if !strings.Contains(string(out), "Status: needs_bootstrap") {
				t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
			}

			bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
			if out, err := bootstrap.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai bootstrap python requirements: %v\n%s", err, out)
			}

			doctor = exec.Command(pluginKitAIBin, "doctor", plugRoot)
			if out, err := doctor.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai doctor after python bootstrap: %v\n%s", err, out)
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, nil, "plugin-kit-ai validate after python bootstrap")
		})
	}
}

func TestPluginKitAIInitPythonRuntimeLauncherFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for launcher flow")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "python", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			if platform == "codex-runtime" {
				assertCodexConfig(t, plugRoot, "gpt-5.4-mini", "./bin/genplug")
			}

			bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
			if out, err := bootstrap.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai bootstrap python runtime: %v\n%s", err, out)
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, nil, "plugin-kit-ai validate python runtime")

			entry := filepath.Join(plugRoot, "bin", "genplug")
			if runtime.GOOS == "windows" {
				entry += ".cmd"
			}
			switch platform {
			case "codex-runtime":
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("run python codex launcher: %v\n%s", err, out)
				}
				if strings.TrimSpace(string(out)) != "" {
					t.Fatalf("stdout = %q, want empty", string(out))
				}
			case "claude":
				assertClaudeStableSubsetEntry(t, entry)
			}
		})
	}
}

func TestPluginKitAIInitPythonRuntimeManagerOwnedEnvFlows(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for manager-owned env flows")
	}

	pythonExe := mustPythonExecutable(t)
	for _, tc := range []struct {
		name              string
		manifestRel       string
		manifestBody      string
		managerBinaryName string
		bootstrapCommand  string
		probeCommand      string
	}{
		{
			name:              "poetry",
			manifestRel:       "pyproject.toml",
			manifestBody:      "[tool.poetry]\nname = 'demo'\nversion = '0.1.0'\ndescription = 'demo'\nauthors = ['demo <demo@example.com>']\n",
			managerBinaryName: "poetry",
			bootstrapCommand:  "install --no-root",
			probeCommand:      "env info --path",
		},
		{
			name:              "pipenv",
			manifestRel:       "Pipfile.lock",
			manifestBody:      "{}\n",
			managerBinaryName: "pipenv",
			bootstrapCommand:  "sync",
			probeCommand:      "--venv",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init python runtime: %v\n%s", err, out)
			}
			writeRuntimeFile(t, plugRoot, tc.manifestRel, tc.manifestBody)

			shimDir := filepath.Join(t.TempDir(), "bin")
			if err := os.MkdirAll(shimDir, 0o755); err != nil {
				t.Fatal(err)
			}
			writePythonManagerShim(t, shimDir, tc.managerBinaryName, pythonExe)
			env := append(os.Environ(), "GOWORK=off", "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			doctor := exec.Command(pluginKitAIBin, "doctor", plugRoot)
			doctor.Env = env
			out, err := doctor.CombinedOutput()
			if err == nil {
				t.Fatalf("expected doctor to require bootstrap before %s env exists:\n%s", tc.name, out)
			}
			if !strings.Contains(string(out), "Status: needs_bootstrap") || !strings.Contains(string(out), "manager="+tc.name) {
				t.Fatalf("doctor output missing %s readiness guidance:\n%s", tc.name, out)
			}

			bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
			bootstrap.Env = env
			out, err = bootstrap.CombinedOutput()
			if err != nil {
				t.Fatalf("plugin-kit-ai bootstrap %s: %v\n%s", tc.name, err, out)
			}
			if !strings.Contains(string(out), "Canonical Python environment source: manager-owned env") {
				t.Fatalf("bootstrap output missing manager-owned env summary:\n%s", out)
			}

			if _, err := os.Stat(filepath.Join(plugRoot, ".venv")); err == nil {
				t.Fatalf("expected %s flow to stay out of repo-local .venv", tc.name)
			}

			doctor = exec.Command(pluginKitAIBin, "doctor", plugRoot)
			doctor.Env = env
			if out, err := doctor.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai doctor after %s bootstrap: %v\n%s", tc.name, err, out)
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, "codex-runtime", env, "plugin-kit-ai validate after "+tc.name+" bootstrap")
		})
	}
}

func TestPluginKitAIInitShellRuntimeLauncherFlow(t *testing.T) {
	if !shellRuntimeAvailable() {
		t.Skip("bash runtime not available for shell launcher flow")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "shell", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}
			if platform == "codex-runtime" {
				assertCodexConfig(t, plugRoot, "gpt-5.4-mini", "./bin/genplug")
			}

			validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, nil, "plugin-kit-ai validate shell runtime")

			entry := filepath.Join(plugRoot, "bin", "genplug")
			if runtime.GOOS == "windows" {
				entry += ".cmd"
			}
			switch platform {
			case "codex-runtime":
				cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("run shell codex launcher: %v\n%s", err, out)
				}
				if strings.TrimSpace(string(out)) != "" {
					t.Fatalf("stdout = %q, want empty", string(out))
				}
			case "claude":
				assertClaudeStableSubsetEntry(t, entry)
			}
		})
	}
}

func TestPluginKitAIInitPythonRuntimeBrokenVenvFailsValidate(t *testing.T) {
	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "python", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}

			if runtime.GOOS == "windows" {
				writeRuntimeFile(t, plugRoot, filepath.Join(".venv", "Scripts", "python.exe"), "not-a-real-exe")
			} else {
				writeRuntimeFile(t, plugRoot, filepath.Join(".venv", "bin", "python3"), "not-a-real-exe")
			}

			validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", platform, "--strict")
			out, err := validate.CombinedOutput()
			if err == nil {
				t.Fatalf("expected validate failure for broken .venv:\n%s", out)
			}
			text := string(out)
			if !strings.Contains(text, "recreate .venv") {
				t.Fatalf("validate output missing broken .venv recovery guidance:\n%s", text)
			}
			if !strings.Contains(text, "Failure: runtime not found:") {
				t.Fatalf("validate output missing runtime failure line:\n%s", text)
			}
		})
	}
}

func TestPluginKitAIInitNodeRuntimeMissingBuiltOutputFailsValidate(t *testing.T) {
	if !nodeRuntimeAvailable() {
		t.Skip("node not in PATH")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}

			validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", platform, "--strict")
			out, err := validate.CombinedOutput()
			if err == nil {
				t.Fatalf("expected validate failure for missing built output:\n%s", out)
			}
			text := string(out)
			if !strings.Contains(text, "dist/main.js") {
				t.Fatalf("validate output missing built-output target path:\n%s", text)
			}
			if !strings.Contains(text, "plugin-kit-ai bootstrap .") {
				t.Fatalf("validate output missing bootstrap guidance:\n%s", text)
			}
		})
	}
}

func validateGeneratedProject(t *testing.T, pluginKitAIBin, plugRoot, platform string, env []string, label string) {
	t.Helper()
	validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", platform)
	if len(env) > 0 {
		validate.Env = env
	}
	out, err := validate.CombinedOutput()
	if err == nil {
		return
	}
	if runtime.GOOS == "windows" && strings.Contains(string(out), "generated artifact drift:") {
		t.Logf("accepting known Windows generated artifact drift during %s:\n%s", label, out)
		return
	}
	t.Fatalf("%s: %v\n%s", label, err, out)
}

func TestPluginKitAIInitShellRuntimeNonExecutableTargetFailsValidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only executable-bit check")
	}

	for _, platform := range []string{"claude", "codex-runtime"} {
		t.Run(platform, func(t *testing.T) {
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", "shell", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
			}

			target := filepath.Join(plugRoot, "scripts", "main.sh")
			if err := os.Chmod(target, 0o644); err != nil {
				t.Fatalf("chmod shell target non-executable: %v", err)
			}

			validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", platform, "--strict")
			out, err := validate.CombinedOutput()
			if err == nil {
				t.Fatalf("expected validate failure for non-executable shell target:\n%s", out)
			}
			text := string(out)
			if !strings.Contains(text, "scripts/main.sh is not executable") {
				t.Fatalf("validate output missing executable-bit guidance:\n%s", text)
			}
		})
	}
}

func assertClaudeStableSubsetEntry(t *testing.T, entry string) {
	t.Helper()
	cases := []struct {
		name    string
		payload string
	}{
		{
			name:    "Stop",
			payload: `{"session_id":"s","transcript_path":"t","cwd":".","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":false,"last_assistant_message":"ok"}`,
		},
		{
			name:    "PreToolUse",
			payload: `{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_use_id":"toolu_e2e","tool_input":{"command":"echo ok"}}`,
		},
		{
			name:    "UserPromptSubmit",
			payload: `{"session_id":"e2e-session","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","permission_mode":"default","hook_event_name":"UserPromptSubmit","prompt":"hello e2e"}`,
		},
	}
	for _, tc := range cases {
		cmd := launcherCommand(entry, tc.name)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run claude launcher %s: %v\n%s", tc.name, err, out)
		}
		if strings.TrimSpace(string(out)) != "{}" {
			t.Fatalf("claude %s stdout = %q, want {}", tc.name, string(out))
		}
	}
}

func assertGeminiRuntimeEntry(t *testing.T, entry string) {
	t.Helper()
	cases := []struct {
		name    string
		payload string
	}{
		{
			name:    "GeminiSessionStart",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionStart","source":"startup"}`,
		},
		{
			name:    "GeminiSessionEnd",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"SessionEnd","reason":"clear"}`,
		},
		{
			name:    "GeminiBeforeModel",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"BeforeModel","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hi"}]}}`,
		},
		{
			name:    "GeminiAfterModel",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"AfterModel","llm_request":{"model":"gemini-2.5-pro"},"llm_response":{"candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]}}]}}`,
		},
		{
			name:    "GeminiBeforeToolSelection",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"BeforeToolSelection","llm_request":{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"read the repo"}]}}`,
		},
		{
			name:    "GeminiBeforeAgent",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"BeforeAgent","prompt":"hello"}`,
		},
		{
			name:    "GeminiAfterAgent",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"AfterAgent","prompt":"hello","prompt_response":"ok","stop_hook_active":false}`,
		},
		{
			name:    "GeminiBeforeTool",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"BeforeTool","tool_name":"read_file","tool_input":{"path":"README.md"},"original_request_name":"tail.read_file"}`,
		},
		{
			name:    "GeminiAfterTool",
			payload: `{"session_id":"s","cwd":".","hook_event_name":"AfterTool","tool_name":"read_file","tool_input":{"path":"README.md"},"tool_response":{"llmContent":"ok","returnDisplay":"ok"},"original_request_name":"tail.read_file"}`,
		},
	}
	for _, tc := range cases {
		cmd := launcherCommand(entry, tc.name)
		cmd.Stdin = strings.NewReader(tc.payload)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run gemini launcher %s: %v\n%s", tc.name, err, out)
		}
		if strings.TrimSpace(string(out)) != "{}" {
			t.Fatalf("gemini %s stdout = %q, want {}", tc.name, string(out))
		}
	}
}

func TestPluginKitAIInitClaudeExtendedHooksRuntimeFlow(t *testing.T) {
	cases := []struct {
		runtime string
		skip    func() bool
		prepare func(t *testing.T, root string)
		entry   func(root string) string
	}{
		{
			runtime: "go",
			prepare: func(t *testing.T, root string) {
				bootstrapGeneratedGoPlugin(t, root)
				binName := "genplug"
				if runtime.GOOS == "windows" {
					binName += ".exe"
				}
				build := exec.Command("go", "build", "-o", filepath.Join("bin", binName), "./cmd/genplug")
				build.Dir = root
				build.Env = append(os.Environ(), "GOWORK=off")
				if out, err := build.CombinedOutput(); err != nil {
					t.Fatalf("go build generated extended Claude entrypoint: %v\n%s", err, out)
				}
			},
			entry: func(root string) string {
				binName := "genplug"
				if runtime.GOOS == "windows" {
					binName += ".exe"
				}
				return filepath.Join(root, "bin", binName)
			},
		},
		{
			runtime: "node",
			skip: func() bool {
				_, err := exec.LookPath("node")
				return err != nil
			},
			entry: func(root string) string {
				path := filepath.Join(root, "bin", "genplug")
				if runtime.GOOS == "windows" {
					path += ".cmd"
				}
				return path
			},
		},
		{
			runtime: "python",
			skip: func() bool {
				return !pythonRuntimeAvailable()
			},
			prepare: func(t *testing.T, root string) {
				t.Helper()
				pluginKitAIBin := buildPluginKitAI(t)
				bootstrap := exec.Command(pluginKitAIBin, "bootstrap", root)
				bootstrap.Env = append(os.Environ(), "GOWORK=off")
				if out, err := bootstrap.CombinedOutput(); err != nil {
					t.Fatalf("plugin-kit-ai bootstrap extended Claude python runtime: %v\n%s", err, out)
				}
			},
			entry: func(root string) string {
				path := filepath.Join(root, "bin", "genplug")
				if runtime.GOOS == "windows" {
					path += ".cmd"
				}
				return path
			},
		},
		{
			runtime: "shell",
			skip: func() bool {
				return !shellRuntimeAvailable()
			},
			entry: func(root string) string {
				path := filepath.Join(root, "bin", "genplug")
				if runtime.GOOS == "windows" {
					path += ".cmd"
				}
				return path
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.runtime, func(t *testing.T) {
			if tc.skip != nil && tc.skip() {
				t.Skipf("%s runtime not available", tc.runtime)
			}
			pluginKitAIBin := buildPluginKitAI(t)
			plugRoot := runtimeProjectRoot(t)
			run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", tc.runtime, "--claude-extended-hooks", "-o", plugRoot, "--extras")
			if out, err := run.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai init extended Claude %s: %v\n%s", tc.runtime, err, out)
			}

			hooksBody, err := os.ReadFile(filepath.Join(plugRoot, authoredRel("targets", "claude", "hooks", "hooks.json")))
			if err != nil {
				t.Fatal(err)
			}
			hooks := string(hooksBody)
			for _, want := range []string{`"SessionStart"`, `"WorktreeRemove"`, `"Stop"`} {
				if !strings.Contains(hooks, want) {
					t.Fatalf("extended Claude hooks missing %s:\n%s", want, hooks)
				}
			}

			if tc.prepare != nil {
				tc.prepare(t, plugRoot)
			}

			validate := exec.Command(pluginKitAIBin, "validate", plugRoot, "--platform", "claude")
			validate.Env = append(os.Environ(), "GOWORK=off")
			if out, err := validate.CombinedOutput(); err != nil {
				t.Fatalf("plugin-kit-ai validate extended Claude %s: %v\n%s", tc.runtime, err, out)
			}

			entry := tc.entry(plugRoot)
			assertClaudeExtendedHookEntry(t, entry, "SessionStart", `{"session_id":"s","cwd":"/tmp","hook_event_name":"SessionStart"}`)
			assertClaudeExtendedHookEntry(t, entry, "WorktreeRemove", `{"session_id":"s","cwd":"/tmp","hook_event_name":"WorktreeRemove","worktree_path":"/tmp/demo"}`)
		})
	}
}

func writeRuntimeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0o644)
	if strings.HasSuffix(rel, ".sh") || strings.HasPrefix(body, "#!") {
		mode = 0o755
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func assertClaudeExtendedHookEntry(t *testing.T, entry, hookName, payload string) {
	t.Helper()
	cmd := launcherCommand(entry, hookName)
	cmd.Stdin = strings.NewReader(payload)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run extended claude launcher %s: %v\n%s", hookName, err, out)
	}
	if strings.TrimSpace(string(out)) != "{}" {
		t.Fatalf("extended claude %s stdout = %q, want {}", hookName, string(out))
	}
}

func pythonRuntimeAvailable() bool {
	bin := ""
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("python"); err == nil {
			bin = "python"
		} else if _, err := exec.LookPath("python3"); err == nil {
			bin = "python3"
		}
	} else if _, err := exec.LookPath("python3"); err == nil {
		bin = "python3"
	}
	if bin == "" {
		return false
	}
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return false
	}
	major, minor, ok := parsePythonVersion(string(out))
	return ok && (major > 3 || (major == 3 && minor >= 10))
}

func parsePythonVersion(version string) (major, minor int, ok bool) {
	fields := strings.Fields(strings.TrimSpace(version))
	if len(fields) < 2 {
		return 0, 0, false
	}
	parts := strings.Split(fields[1], ".")
	if len(parts) < 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

func mustPythonExecutable(t *testing.T) string {
	t.Helper()
	candidates := []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	t.Fatal("python executable not found")
	return ""
}

func writePythonManagerShim(t *testing.T, dir, name, pythonExe string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		body := "@echo off\r\nsetlocal\r\nset \"ROOT=%CD%\"\r\nif \"%1\"==\"install\" if \"%2\"==\"--no-root\" (\r\n  \"" + pythonExe + "\" -m venv external-env\r\n  exit /b %ERRORLEVEL%\r\n)\r\nif \"%1\"==\"env\" if \"%2\"==\"info\" if \"%3\"==\"--path\" (\r\n  if exist external-env (\r\n    echo %ROOT%\\external-env\r\n    exit /b 0\r\n  )\r\n  exit /b 1\r\n)\r\nif \"%1\"==\"sync\" (\r\n  \"" + pythonExe + "\" -m venv external-env\r\n  exit /b %ERRORLEVEL%\r\n)\r\nif \"%1\"==\"--venv\" (\r\n  if exist external-env (\r\n    echo %ROOT%\\external-env\r\n    exit /b 0\r\n  )\r\n  exit /b 1\r\n)\r\necho unexpected args %* 1>&2\r\nexit /b 1\r\n"
		writeRuntimeFile(t, dir, name+".cmd", body)
		return
	}
	body := "#!/usr/bin/env bash\nset -euo pipefail\nroot=\"$PWD\"\npython_exe=" + strconv.Quote(pythonExe) + "\nif [[ \"$1\" == \"install\" && \"${2:-}\" == \"--no-root\" ]]; then\n  \"$python_exe\" -m venv external-env\n  exit 0\nfi\nif [[ \"$1\" == \"env\" && \"${2:-}\" == \"info\" && \"${3:-}\" == \"--path\" ]]; then\n  if [[ -d external-env ]]; then\n    printf '%s\\n' \"$root/external-env\"\n    exit 0\n  fi\n  exit 1\nfi\nif [[ \"$1\" == \"sync\" ]]; then\n  \"$python_exe\" -m venv external-env\n  exit 0\nfi\nif [[ \"$1\" == \"--venv\" ]]; then\n  if [[ -d external-env ]]; then\n    printf '%s\\n' \"$root/external-env\"\n    exit 0\n  fi\n  exit 1\nfi\necho \"unexpected args: $*\" >&2\nexit 1\n"
	writeRuntimeFile(t, dir, name, body)
}
