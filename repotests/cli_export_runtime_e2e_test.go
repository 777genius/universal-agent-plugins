package pluginkitairepo_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIExportPythonRequirementsBundleFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for export flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	writeRuntimeFile(t, plugRoot, "requirements.txt", "requests==2.32.0\n")

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before export: %v\n%s", err, out)
	}

	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export python requirements", nil)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_python_bundle.tar.gz")
	unpackRoot := filepath.Join(t.TempDir(), "bundle")
	extractTarGz(t, bundle, unpackRoot)
	if _, err := os.Stat(filepath.Join(unpackRoot, ".venv")); !os.IsNotExist(err) {
		t.Fatalf("expected export bundle to exclude .venv, err=%v", err)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", unpackRoot)
	out, err := doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after unpack:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", unpackRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after unpack: %v\n%s", err, out)
	}

	validateStrictProject(t, pluginKitAIBin, unpackRoot, "codex-runtime", "plugin-kit-ai validate after unpack", nil)
}

func TestPluginKitAIExportPythonPoetryBundleFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for export flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "python", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}
	writeRuntimeFile(t, plugRoot, "pyproject.toml", "[tool.poetry]\nname = 'genplug'\nversion = '0.1.0'\ndescription = 'demo'\nauthors = ['demo <demo@example.com>']\n")

	pythonExe := mustPythonExecutable(t)
	shimDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePythonManagerShim(t, shimDir, "poetry", pythonExe)
	env := append(os.Environ(), "GOWORK=off", "PATH="+shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	bootstrap.Env = env
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before export: %v\n%s", err, out)
	}

	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export python poetry", env)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_python_bundle.tar.gz")
	unpackRoot := filepath.Join(t.TempDir(), "bundle")
	extractTarGz(t, bundle, unpackRoot)
	if _, err := os.Stat(filepath.Join(unpackRoot, "external-env")); !os.IsNotExist(err) {
		t.Fatalf("expected export bundle to exclude manager-owned env, err=%v", err)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", unpackRoot)
	doctor.Env = env
	out, err := doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after unpack:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", unpackRoot)
	bootstrap.Env = env
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap poetry after unpack: %v\n%s", err, out)
	}

	validateStrictProject(t, pluginKitAIBin, unpackRoot, "codex-runtime", "plugin-kit-ai validate poetry after unpack", env)
}

func TestPluginKitAIExportNodeTypeScriptBundleFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before export: %v\n%s", err, out)
	}

	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export node typescript", nil)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_node_bundle.tar.gz")
	unpackRoot := filepath.Join(t.TempDir(), "bundle")
	extractTarGz(t, bundle, unpackRoot)
	if _, err := os.Stat(filepath.Join(unpackRoot, "dist", "main.js")); err != nil {
		t.Fatalf("expected export bundle to include built output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(unpackRoot, "node_modules")); !os.IsNotExist(err) {
		t.Fatalf("expected export bundle to exclude node_modules, err=%v", err)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", unpackRoot)
	out, err := doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after unpack:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", unpackRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after unpack: %v\n%s", err, out)
	}

	validateStrictProject(t, pluginKitAIBin, unpackRoot, "codex-runtime", "plugin-kit-ai validate after unpack", nil)
}

func TestPluginKitAIExportShellBundleFlow(t *testing.T) {
	if !shellRuntimeAvailable() {
		t.Skip("bash runtime not available for shell export flow")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "codex-runtime", "--runtime", "shell", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init shell runtime: %v\n%s", err, out)
	}

	validateStrictProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai validate shell runtime before export", nil)
	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export shell bundle", nil)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_shell_bundle.tar.gz")
	unpackRoot := filepath.Join(t.TempDir(), "bundle")
	extractTarGz(t, bundle, unpackRoot)
	if _, err := os.Stat(filepath.Join(unpackRoot, "scripts", "main.sh")); err != nil {
		t.Fatalf("expected export bundle to preserve shell target: %v", err)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", unpackRoot)
	out, err := doctor.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai doctor after shell unpack: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Status: ready") {
		t.Fatalf("doctor output missing ready status after shell unpack:\n%s", out)
	}

	validateStrictProject(t, pluginKitAIBin, unpackRoot, "codex-runtime", "plugin-kit-ai validate shell runtime after unpack", nil)
}

func extractTarGz(t *testing.T, archivePath, dest string) {
	t.Helper()
	f, err := os.Open(archivePath)
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
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(dest, filepath.FromSlash(hdr.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, body, os.FileMode(hdr.Mode)); err != nil {
			t.Fatal(err)
		}
	}
}
