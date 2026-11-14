package app

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestPluginServiceExportPythonBundleExcludesProjectVenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows export fixture remains stricter than the archive exclusion behavior under test; unix coverage exercises the bundle exclusion contract")
	}
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) {
		switch name {
		case "python", "python3":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		base := filepath.Base(name)
		if len(args) == 1 && args[0] == "--version" {
			switch base {
			case "python", "python3", "python.exe":
				return "Python 3.11.0", nil
			}
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})

	dir := t.TempDir()
	entrypoint := "./bin/demo"
	launcherRel := filepath.Join("bin", "demo")
	venvInterpreter := filepath.Join(".venv", "bin", "python3")
	launcherBody := "#!/usr/bin/env bash\nexec python \"$ROOT/plugin/main.py\" \"$@\"\n"
	if runtime.GOOS == "windows" {
		entrypoint = "./bin/demo.cmd"
		launcherRel = filepath.Join("bin", "demo.cmd")
		venvInterpreter = filepath.Join(".venv", "Scripts", "python.exe")
		launcherBody = "@echo off\r\npython \"%~dp0..\\plugin\\main.py\" %*\r\n"
	}
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: python\nentrypoint: "+entrypoint+"\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, launcherRel, launcherBody)
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "main.py"), "print('ok')\n")
	writeBootstrapProjectFile(t, dir, "requirements.txt", "requests==2.32.0\n")
	writeBootstrapProjectFile(t, dir, venvInterpreter, "ok")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, launcherRel))
	renderExportTarget(t, dir, "codex-runtime")

	var svc PluginService
	result, err := svc.Export(PluginExportOptions{Root: dir, Platform: "codex-runtime"})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	if !strings.Contains(output, "Exported bundle:") {
		t.Fatalf("unexpected output:\n%s", output)
	}

	bundlePath := filepath.Join(dir, "demo_codex-runtime_python_bundle.tar.gz")
	entries := readExportArchive(t, bundlePath)
	expectedLauncher := "bin/demo"
	if runtime.GOOS == "windows" {
		expectedLauncher = "bin/demo.cmd"
	}
	for _, want := range []string{
		".plugin-kit-ai-export.json",
		"plugin/plugin.yaml",
		"plugin/launcher.yaml",
		".codex/config.toml",
		expectedLauncher,
		"plugin/main.py",
		"requirements.txt",
	} {
		if _, ok := entries[want]; !ok {
			t.Fatalf("bundle missing %s", want)
		}
	}
	if _, ok := entries[".venv/bin/python3"]; ok {
		t.Fatal("bundle unexpectedly included .venv")
	}
	if _, ok := entries[".venv/Scripts/python.exe"]; ok {
		t.Fatal("bundle unexpectedly included .venv")
	}

	var metadata map[string]any
	if err := json.Unmarshal(entries[".plugin-kit-ai-export.json"], &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["runtime"] != "python" ||
		metadata["bootstrap_model"] != "repo-local .venv" ||
		metadata["runtime_requirement"] != "Python 3.10+ installed on the machine running the plugin" {
		t.Fatalf("metadata = %#v", metadata)
	}
	if !strings.Contains(output, "Runtime requirement: Python 3.10+ installed on the machine running the plugin") {
		t.Fatalf("unexpected output:\n%s", output)
	}
}

func TestPluginServiceExportShellBundlePreservesScripts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell export assertions are unix-oriented")
	}
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: shell\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec \"$ROOT/scripts/main.sh\" \"$@\"\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "scripts", "main.sh"))
	renderExportTarget(t, dir, "codex-runtime")

	var svc PluginService
	if _, err := svc.Export(PluginExportOptions{Root: dir, Platform: "codex-runtime"}); err != nil {
		t.Fatal(err)
	}
	entries := readExportArchive(t, filepath.Join(dir, "demo_codex-runtime_shell_bundle.tar.gz"))
	if _, ok := entries["scripts/main.sh"]; !ok {
		t.Fatal("bundle missing scripts/main.sh")
	}
}

func TestPluginServiceExportExcludesExistingBundleOutputInsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell export assertions are unix-oriented")
	}
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "plugin", "existing-bundle.tar.gz")
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: shell\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec \"$ROOT/scripts/main.sh\" \"$@\"\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "existing-bundle.tar.gz"), "stale tar content")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "scripts", "main.sh"))
	renderExportTarget(t, dir, "codex-runtime")

	var svc PluginService
	if _, err := svc.Export(PluginExportOptions{
		Root:     dir,
		Platform: "codex-runtime",
		Output:   outputPath,
	}); err != nil {
		t.Fatal(err)
	}

	entries := readExportArchive(t, outputPath)
	if _, ok := entries["plugin/existing-bundle.tar.gz"]; ok {
		t.Fatal("bundle unexpectedly included its own output path")
	}
}

func renderExportTarget(t *testing.T, root, target string) {
	t.Helper()
	result, err := pluginmanifest.Generate(root, target)
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(root, result.Artifacts); err != nil {
		t.Fatal(err)
	}
}

func TestPluginServiceExportRejectsGoRuntime(t *testing.T) {
	dir := t.TempDir()
	writeBootstrapProjectFile(t, dir, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeBootstrapProjectFile(t, dir, "plugin/launcher.yaml", "runtime: go\nentrypoint: ./bin/demo\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	writeBootstrapProjectFile(t, dir, filepath.Join(".codex", "config.toml"), "model = \"gpt-5.4-mini\"\nnotify = [\"./bin/demo\", \"notify\"]\n")
	writeBootstrapProjectFile(t, dir, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexit 0\n")
	mustChmodBootstrapExecutable(t, filepath.Join(dir, "bin", "demo"))

	var svc PluginService
	_, err := svc.Export(PluginExportOptions{Root: dir, Platform: "codex-runtime"})
	if err == nil || !strings.Contains(err.Error(), "interpreted runtimes") {
		t.Fatalf("error = %v", err)
	}
}

func readExportArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		out[hdr.Name] = body
	}
	return out
}
