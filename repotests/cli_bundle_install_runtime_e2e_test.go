package pluginkitairepo_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIBundleInstallPythonRequirementsFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for bundle install flow")
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
	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export before bundle install", nil)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_python_bundle.tar.gz")
	dest := filepath.Join(t.TempDir(), "installed")
	install := exec.Command(pluginKitAIBin, "bundle", "install", "--dest", dest, bundle)
	out, err := install.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle install python requirements: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Installed path: "+dest) {
		t.Fatalf("bundle install output missing installed path:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after bundle install:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after bundle install: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "codex-runtime", "plugin-kit-ai validate after bundle install", nil)
}

func TestPluginKitAIBundleInstallPythonPoetryFlow(t *testing.T) {
	if !pythonRuntimeAvailable() {
		t.Skip("python runtime not available for bundle install flow")
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
	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export before bundle install", env)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_python_bundle.tar.gz")
	dest := filepath.Join(t.TempDir(), "installed")
	install := exec.Command(pluginKitAIBin, "bundle", "install", "--dest", dest, bundle)
	install.Env = env
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bundle install poetry: %v\n%s", err, out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	doctor.Env = env
	out, err := doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after bundle install:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	bootstrap.Env = env
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap poetry after bundle install: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "codex-runtime", "plugin-kit-ai validate poetry after bundle install", env)
}

func TestPluginKitAIBundleInstallNodeTypeScriptFlow(t *testing.T) {
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
	exportProject(t, pluginKitAIBin, plugRoot, "codex-runtime", "plugin-kit-ai export before bundle install", nil)

	bundle := filepath.Join(plugRoot, "genplug_codex-runtime_node_bundle.tar.gz")
	dest := filepath.Join(t.TempDir(), "installed")
	install := exec.Command(pluginKitAIBin, "bundle", "install", "--dest", dest, bundle)
	out, err := install.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle install node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "runtime=node") {
		t.Fatalf("bundle install output missing runtime summary:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after bundle install:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after bundle install: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "codex-runtime", "plugin-kit-ai validate after bundle install", nil)
}

func TestPluginKitAIBundleInstallClaudeNodeTypeScriptFlow(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not in PATH")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not in PATH")
	}

	pluginKitAIBin := buildPluginKitAI(t)
	plugRoot := runtimeProjectRoot(t)
	run := exec.Command(pluginKitAIBin, "init", "genplug", "--platform", "claude", "--runtime", "node", "--typescript", "-o", plugRoot, "--extras")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai init claude node typescript: %v\n%s", err, out)
	}

	bootstrap := exec.Command(pluginKitAIBin, "bootstrap", plugRoot)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap before claude export: %v\n%s", err, out)
	}
	exportProject(t, pluginKitAIBin, plugRoot, "claude", "plugin-kit-ai export claude node typescript", nil)

	bundle := filepath.Join(plugRoot, "genplug_claude_node_bundle.tar.gz")
	dest := filepath.Join(t.TempDir(), "installed")
	install := exec.Command(pluginKitAIBin, "bundle", "install", "--dest", dest, bundle)
	out, err := install.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai bundle install claude node typescript: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "platform=claude runtime=node") {
		t.Fatalf("bundle install output missing claude runtime summary:\n%s", out)
	}

	doctor := exec.Command(pluginKitAIBin, "doctor", dest)
	out, err = doctor.CombinedOutput()
	if err == nil {
		t.Fatalf("expected doctor to require bootstrap after claude bundle install:\n%s", out)
	}
	if !strings.Contains(string(out), "Status: needs_bootstrap") {
		t.Fatalf("doctor output missing needs_bootstrap:\n%s", out)
	}

	bootstrap = exec.Command(pluginKitAIBin, "bootstrap", dest)
	if out, err := bootstrap.CombinedOutput(); err != nil {
		t.Fatalf("plugin-kit-ai bootstrap after claude bundle install: %v\n%s", err, out)
	}
	validateStrictProject(t, pluginKitAIBin, dest, "claude", "plugin-kit-ai validate after claude bundle install", nil)
}
