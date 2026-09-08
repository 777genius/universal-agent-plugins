package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestPluginServiceBootstrapPythonCreatesVenvAndInstallsRequirements(t *testing.T) {
	restoreBootstrapHelpers(t)
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, "requirements.txt", "requests==2.32.0\n")

	var svc PluginService
	result, err := svc.Bootstrap(context.Background(), PluginBootstrapOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Project: lane=codex-runtime runtime=python manager=requirements.txt (pip)",
		"Runtime requirement: Python 3.10+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Python before running the plugin",
		"Detected Python manager: requirements.txt (pip)",
		"Ran: python -m venv .venv",
		"Ran: python -m pip install -r requirements.txt",
		"Canonical Python environment source: repo-local .venv",
		"Next: plugin-kit-ai validate . --platform codex-runtime --strict",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	venvPython := filepath.Join(dir, ".venv", "bin", "python3")
	if runtime.GOOS == "windows" {
		venvPython = filepath.Join(dir, ".venv", "Scripts", "python.exe")
	}
	if _, err := os.Stat(venvPython); err != nil {
		t.Fatalf("expected virtualenv interpreter: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".venv", "pip-installed.txt")); err != nil {
		t.Fatalf("expected pip install marker: %v", err)
	}
}

func TestPluginServiceBootstrapPoetryReportsManagerOwnedEnv(t *testing.T) {
	restoreBootstrapHelpers(t)
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, "pyproject.toml", "[tool.poetry]\nname='demo'\n")

	var svc PluginService
	result, err := svc.Bootstrap(context.Background(), PluginBootstrapOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Detected Python manager: poetry",
		"Ran: poetry install --no-root",
		"Canonical Python environment source: manager-owned env",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestPluginServiceBootstrapNodePNPMTypeScriptRunsInstallAndBuild(t *testing.T) {
	restoreBootstrapHelpers(t)
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, "tsconfig.json", "{}\n")
	writeBootstrapProjectFile(t, dir, "package.json", `{"scripts":{"build":"tsc -p tsconfig.json"}}`)
	writeBootstrapProjectFile(t, dir, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec node \"$ROOT/dist/main.js\" \"$@\"\n")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))

	var svc PluginService
	result, err := svc.Bootstrap(context.Background(), PluginBootstrapOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Project: lane=codex-runtime runtime=node manager=pnpm",
		"Runtime requirement: Node.js 20+ installed on the machine running the plugin",
		"Runtime install hint: Go is the recommended path when you want users to avoid installing Node.js before running the plugin",
		"Detected Node manager: pnpm",
		"Ran: pnpm install --frozen-lockfile",
		"Ran: pnpm run build",
		"Next: plugin-kit-ai validate . --platform codex-runtime --strict",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules", ".installed")); err != nil {
		t.Fatalf("expected pnpm install marker: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "dist", "main.js")); err != nil {
		t.Fatalf("expected built output marker: %v", err)
	}
}

func TestPluginServiceBootstrapNodeNPMDisablesAuditAndFundingRequests(t *testing.T) {
	for _, tc := range []struct {
		name        string
		lockfile    bool
		wantCommand string
		wantMarker  string
	}{
		{name: "install", wantCommand: "Ran: npm install --no-audit --no-fund", wantMarker: "npm-install"},
		{name: "ci", lockfile: true, wantCommand: "Ran: npm ci --no-audit --no-fund", wantMarker: "npm-ci"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreBootstrapHelpers(t)
			dir := t.TempDir()
			writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
			writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
			writeBootstrapProjectFile(t, dir, "package.json", `{"type":"module"}`)
			if tc.lockfile {
				writeBootstrapProjectFile(t, dir, "package-lock.json", `{}`)
			}

			var svc PluginService
			result, err := svc.Bootstrap(context.Background(), PluginBootstrapOptions{Root: dir})
			if err != nil {
				t.Fatal(err)
			}
			if output := strings.Join(result.Lines, "\n"); !strings.Contains(output, tc.wantCommand) {
				t.Fatalf("output missing %q:\n%s", tc.wantCommand, output)
			}
			marker, err := os.ReadFile(filepath.Join(dir, "node_modules", ".installed"))
			if err != nil {
				t.Fatalf("read npm install marker: %v", err)
			}
			if got := string(marker); got != tc.wantMarker {
				t.Fatalf("npm install marker = %q, want %q", got, tc.wantMarker)
			}
		})
	}
}

func TestPluginServiceBootstrapGoIsNoOp(t *testing.T) {
	restoreBootstrapHelpers(t)
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: go\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexit 0\n")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))

	var svc PluginService
	result, err := svc.Bootstrap(context.Background(), PluginBootstrapOptions{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	if !strings.Contains(output, "Bootstrap not required for Go projects") {
		t.Fatalf("output = %s", output)
	}
}

func TestBootstrapHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_BOOTSTRAP_HELPER") != "1" {
		return
	}
	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+1 >= len(args) {
		os.Exit(2)
	}
	name := filepath.Base(args[sep+1])
	cmdArgs := args[sep+2:]
	switch name {
	case "python", "python3", "python.exe":
		runBootstrapPythonHelper(cmdArgs)
	case "npm", "pnpm", "yarn", "bun":
		runBootstrapNodeHelper(name, cmdArgs)
	case "uv", "poetry", "pipenv":
		runBootstrapPythonManagerHelper(name, cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "unexpected helper %s", name)
		os.Exit(2)
	}
}

func runBootstrapPythonHelper(args []string) {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintln(os.Stdout, "Python 3.11.0")
		return
	}
	if len(args) >= 3 && args[0] == "-m" && args[1] == "venv" {
		venvRoot := args[2]
		interpreter := filepath.Join(venvRoot, "bin", "python3")
		if runtime.GOOS == "windows" {
			interpreter = filepath.Join(venvRoot, "Scripts", "python.exe")
		}
		mustWriteHelperFile(interpreter, "bootstrap-helper", 0o755)
		return
	}
	if len(args) >= 5 && args[0] == "-m" && args[1] == "pip" && args[2] == "install" && args[3] == "-r" {
		mustWriteHelperFile(filepath.Join(".venv", "pip-installed.txt"), args[4], 0o644)
		return
	}
	fmt.Fprintf(os.Stderr, "unexpected python helper args: %v", args)
	os.Exit(2)
}

func runBootstrapPythonManagerHelper(name string, args []string) {
	switch name {
	case "uv":
		if len(args) == 1 && args[0] == "sync" {
			mustWriteHelperFile(filepath.Join(".venv", "uv-synced.txt"), "ok", 0o644)
			return
		}
	case "poetry":
		if len(args) == 3 && args[0] == "env" && args[1] == "info" && args[2] == "--path" {
			fmt.Fprintln(os.Stderr, "no active poetry env")
			os.Exit(1)
		}
		if len(args) == 2 && args[0] == "install" && args[1] == "--no-root" {
			mustWriteHelperFile(filepath.Join(".venv", "poetry-installed.txt"), "ok", 0o644)
			return
		}
	case "pipenv":
		if len(args) == 1 && args[0] == "--venv" {
			fmt.Fprintln(os.Stderr, "no active pipenv env")
			os.Exit(1)
		}
		if len(args) == 1 && (args[0] == "sync" || args[0] == "install") {
			mustWriteHelperFile(filepath.Join(".venv", "pipenv-installed.txt"), args[0], 0o644)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "unexpected python manager helper %s args: %v", name, args)
	os.Exit(2)
}

func runBootstrapNodeHelper(name string, args []string) {
	switch name {
	case "npm":
		if len(args) == 3 &&
			(args[0] == "install" || args[0] == "ci") &&
			args[1] == "--no-audit" &&
			args[2] == "--no-fund" {
			mustWriteHelperFile(filepath.Join("node_modules", ".installed"), name+"-"+args[0], 0o644)
			return
		}
		if len(args) == 2 && args[0] == "run" && args[1] == "build" {
			mustWriteHelperFile(filepath.Join("dist", "main.js"), "console.log('ok')\n", 0o644)
			return
		}
	case "pnpm":
		if len(args) == 2 && args[0] == "install" && args[1] == "--frozen-lockfile" {
			mustWriteHelperFile(filepath.Join("node_modules", ".installed"), "pnpm-install", 0o644)
			return
		}
		if len(args) == 2 && args[0] == "run" && args[1] == "build" {
			mustWriteHelperFile(filepath.Join("dist", "main.js"), "console.log('ok')\n", 0o644)
			return
		}
	case "yarn":
		if len(args) == 2 && args[0] == "install" && (args[1] == "--immutable" || args[1] == "--frozen-lockfile") {
			mustWriteHelperFile(filepath.Join("node_modules", ".installed"), "yarn-install", 0o644)
			return
		}
		if len(args) == 1 && args[0] == "build" {
			mustWriteHelperFile(filepath.Join("dist", "main.js"), "console.log('ok')\n", 0o644)
			return
		}
	case "bun":
		if len(args) == 1 && args[0] == "install" {
			mustWriteHelperFile(filepath.Join("node_modules", ".installed"), "bun-install", 0o644)
			return
		}
		if len(args) == 2 && args[0] == "run" && args[1] == "build" {
			mustWriteHelperFile(filepath.Join("dist", "main.js"), "console.log('ok')\n", 0o644)
			return
		}
	}
	fmt.Fprintf(os.Stderr, "unexpected node helper %s args: %v", name, args)
	os.Exit(2)
}

func restoreBootstrapHelpers(t *testing.T) {
	t.Helper()
	prevLookPath := runtimecheck.LookPath
	prevRunCommand := runtimecheck.RunCommand
	prevCommand := bootstrapCommandContext
	runtimecheck.LookPath = func(name string) (string, error) {
		switch name {
		case "python", "python3", "npm", "pnpm", "yarn", "bun", "uv", "poetry", "pipenv":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		cmdArgs := []string{"-test.run=TestBootstrapHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command(os.Args[0], cmdArgs...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GO_WANT_BOOTSTRAP_HELPER=1")
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	bootstrapCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=TestBootstrapHelperProcess", "--", name}
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_BOOTSTRAP_HELPER=1")
		return cmd
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = prevLookPath
		runtimecheck.RunCommand = prevRunCommand
		bootstrapCommandContext = prevCommand
	})
}

func minimalBootstrapManifest() string {
	return `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["codex-runtime"]
`
}

func writeBootstrapProjectFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWriteHelperFile(path, body string, mode os.FileMode) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func mustChmodBootstrapExecutable(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
