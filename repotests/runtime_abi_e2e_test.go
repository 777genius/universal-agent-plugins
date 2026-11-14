package pluginkitairepo_test

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func TestPluginKitAIRuntimeABIPassthrough(t *testing.T) {
	cases := []struct {
		runtime string
		ready   func() bool
	}{
		{runtime: "go", ready: func() bool { return true }},
		{runtime: "python", ready: pythonRuntimeAvailable},
		{runtime: "node", ready: nodeRuntimeAvailable},
		{runtime: "shell", ready: shellRuntimeAvailable},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.runtime, func(t *testing.T) {
			if !tc.ready() {
				t.Skipf("%s runtime not available", tc.runtime)
			}

			for _, platform := range []string{"claude", "codex-runtime"} {
				platform := platform
				t.Run(platform, func(t *testing.T) {
					pluginKitAIBin := buildPluginKitAI(t)
					plugRoot := runtimeProjectRoot(t)
					goEnv := newGoModuleEnv(t)

					initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", platform, "--runtime", tc.runtime, "-o", plugRoot)
					if out, err := initCmd.CombinedOutput(); err != nil {
						t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
					}

					if tc.runtime == "go" {
						wireGeneratedGoModuleToLocalSDK(t, plugRoot, goEnv)
						tidyCmd := exec.Command("go", "mod", "tidy")
						tidyCmd.Dir = plugRoot
						tidyCmd.Env = goEnv
						if out, err := tidyCmd.CombinedOutput(); err != nil {
							t.Fatalf("go mod tidy generated entrypoint: %v\n%s", err, out)
						}
					}
					overwriteRuntimeWithABIFixture(t, plugRoot, tc.runtime)
					if tc.runtime == "python" || tc.runtime == "node" {
						bootstrapCmd := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
						bootstrapCmd.Env = append(os.Environ(), "GOWORK=off")
						if out, err := bootstrapCmd.CombinedOutput(); err != nil {
							t.Fatalf("plugin-kit-ai bootstrap: %v\n%s", err, out)
						}
					}

					validateGeneratedProject(t, pluginKitAIBin, plugRoot, platform, goEnv, "plugin-kit-ai validate")

					if tc.runtime == "go" {
						buildGeneratedGoEntrypoint(t, plugRoot, goEnv)
					}

					entry := generatedEntrypointPath(plugRoot, tc.runtime)

					switch platform {
					case "claude":
						stdout, stderr, err := runProcess(entry, []string{"Stop"}, `{"hook_event_name":"Stop","payload":"claude"}`)
						if err != nil {
							t.Fatalf("run claude ABI fixture: %v\nstderr=%s", err, stderr)
						}
						if stdout != `{"hook_event_name":"Stop","payload":"claude"}` {
							t.Fatalf("stdout = %q", stdout)
						}
						if stderr != "Stop" {
							t.Fatalf("stderr = %q, want %q", stderr, "Stop")
						}
					case "codex-runtime":
						stdout, stderr, err := runProcess(entry, []string{"notify", `{"client":"codex-tui","payload":"codex"}`}, "")
						if err == nil {
							t.Fatal("expected non-zero exit")
						}
						var exitErr *exec.ExitError
						if !asExitError(err, &exitErr) {
							t.Fatalf("expected ExitError, got %v", err)
						}
						if exitErr.ExitCode() != 7 {
							t.Fatalf("exit code = %d, want 7", exitErr.ExitCode())
						}
						if stdout != "" {
							t.Fatalf("stdout = %q, want empty", stdout)
						}
						if stderr != `{"client":"codex-tui","payload":"codex"}` {
							t.Fatalf("stderr = %q", stderr)
						}
					}
				})
			}
		})
	}
}

func TestPluginKitAIPythonLauncherPrefersProjectVenvOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific")
	}
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available")
	}

	systemPython, err := findPythonExecutable()
	if err != nil {
		t.Skip(err.Error())
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	initCmd := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}

	venvPython := filepath.Join(plugRoot, ".venv", "Scripts", "python.exe")
	if err := os.MkdirAll(filepath.Dir(venvPython), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := linkOrCopyFile(systemPython, venvPython); err != nil {
		t.Fatalf("prepare project python runtime: %v", err)
	}
	writeRuntimeFile(t, plugRoot, filepath.Join(platformmeta.CanonicalAuthoredRoot, "main.py"), pythonExecutableProbeSource())

	validateGeneratedProject(t, pluginKitAIBin, plugRoot, "codex-runtime", nil, "plugin-kit-ai validate")

	stdout, stderr, err := runProcess(generatedEntrypointPath(plugRoot, "python"), []string{"notify", `{"client":"codex-tui"}`}, "")
	if err != nil {
		t.Fatalf("run python launcher: %v\nstderr=%s", err, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout), strings.ToLower(filepath.Clean(venvPython))) {
		t.Fatalf("stdout = %q, want path containing %q", stdout, venvPython)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func buildGeneratedGoEntrypoint(t *testing.T, plugRoot string, env []string) {
	t.Helper()
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
}

func overwriteRuntimeWithABIFixture(t *testing.T, root, runtimeName string) {
	t.Helper()
	switch runtimeName {
	case "go":
		writeRuntimeFile(t, root, filepath.Join("cmd", "genplug", "main.go"), abiGoSource())
	case "python":
		writeRuntimeFile(t, root, filepath.Join(platformmeta.CanonicalAuthoredRoot, "main.py"), abiPythonSource())
	case "node":
		writeRuntimeFile(t, root, filepath.Join(platformmeta.CanonicalAuthoredRoot, "main.mjs"), abiNodeSource())
	case "shell":
		writeRuntimeFile(t, root, filepath.Join("scripts", "main.sh"), abiShellSource())
	default:
		t.Fatalf("unsupported runtime %q", runtimeName)
	}
}

func runProcess(entry string, args []string, stdin string) (stdout string, stderr string, err error) {
	cmd := launcherCommand(entry, args...)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func generatedEntrypointPath(root, runtimeName string) string {
	name := filepath.Join(root, "bin", "genplug")
	switch {
	case runtimeName == "go" && runtime.GOOS == "windows":
		return name + ".exe"
	case runtime.GOOS == "windows":
		return name + ".cmd"
	default:
		return name
	}
}

func runtimeProjectRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "project with spaces")
}

func nodeRuntimeAvailable() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

func shellRuntimeAvailable() bool {
	_, err := exec.LookPath("bash")
	return err == nil
}

func findPythonExecutable() (string, error) {
	if runtime.GOOS == "windows" {
		if path, err := exec.LookPath("python"); err == nil {
			return path, nil
		}
		return exec.LookPath("python3")
	}
	return exec.LookPath("python3")
}

func linkOrCopyFile(src, dst string) error {
	if err := os.Link(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func asExitError(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	*target = exitErr
	return true
}

func abiGoSource() string {
	return `package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, "usage: main <hook-name>\n")
		os.Exit(1)
	}
	hookName := os.Args[1]
	if hookName == "notify" {
		if len(os.Args) < 3 {
			fmt.Fprint(os.Stderr, "missing notify payload\n")
			os.Exit(1)
		}
		fmt.Fprint(os.Stderr, os.Args[2])
		os.Exit(7)
	}
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprint(os.Stdout, string(body))
	fmt.Fprint(os.Stderr, hookName)
}
`
}

func abiPythonSource() string {
	return `import sys


def main():
    if len(sys.argv) < 2:
        sys.stderr.write("usage: main.py <hook-name>\n")
        return 1

    hook_name = sys.argv[1]
    if hook_name == "notify":
        if len(sys.argv) < 3:
            sys.stderr.write("missing notify payload\n")
            return 1
        sys.stderr.write(sys.argv[2])
        return 7

    sys.stdout.write(sys.stdin.read())
    sys.stderr.write(hook_name)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
`
}

func abiNodeSource() string {
	return `import fs from "node:fs";

function main() {
  const hookName = process.argv[2];
  if (!hookName) {
    process.stderr.write("usage: main.mjs <hook-name>\n");
    return 1;
  }
  if (hookName === "notify") {
    const payload = process.argv[3];
    if (!payload) {
      process.stderr.write("missing notify payload\n");
      return 1;
    }
    process.stderr.write(payload);
    return 7;
  }
  const body = fs.readFileSync(0, "utf8");
  process.stdout.write(body);
  process.stderr.write(hookName);
  return 0;
}

process.exit(main());
`
}

func abiShellSource() string {
	return `#!/usr/bin/env bash
set -euo pipefail

hook_name="${1:-}"
if [[ -z "$hook_name" ]]; then
  echo "usage: main.sh <hook-name>" >&2
  exit 1
fi

if [[ "$hook_name" == "notify" ]]; then
  if [[ $# -lt 2 ]]; then
    echo "missing notify payload" >&2
    exit 1
  fi
  printf '%s' "$2" >&2
  exit 7
fi

payload="$(cat)"
printf '%s' "$payload"
printf '%s' "$hook_name" >&2
`
}

func pythonExecutableProbeSource() string {
	return `import sys


def main():
    sys.stdout.write(sys.executable)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
`
}
