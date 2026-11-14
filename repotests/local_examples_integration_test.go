package pluginkitairepo_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocalExamples_RenderValidateAndNotify(t *testing.T) {
	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)

	cases := []struct {
		name    string
		dir     string
		runtime string
		ready   func() bool
		setup   func(t *testing.T, dir string)
	}{
		{
			name:    "codex-python-local",
			dir:     filepath.Join(root, "examples", "local", "codex-python-local"),
			runtime: "python",
			ready:   pythonRuntimeAvailable,
			setup:   setupPythonLocalExample,
		},
		{
			name:    "codex-node-local",
			dir:     filepath.Join(root, "examples", "local", "codex-node-local"),
			runtime: "node",
			ready:   nodeAndNPMAvailable,
			setup:   setupNodeLocalExample,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if !tc.ready() {
				t.Skipf("%s runtime not available", tc.runtime)
			}

			workDir := filepath.Join(t.TempDir(), tc.name)
			copyTree(t, tc.dir, workDir)

			runLocalExampleCmd(t, root, exec.Command(pluginKitAIBin, "generate", workDir, "--check"))
			tc.setup(t, workDir)
			runLocalExampleCmd(t, root, exec.Command(pluginKitAIBin, "validate", workDir, "--platform", "codex-runtime", "--strict"))

			entry := localExampleEntrypointPath(workDir, tc.runtime)
			cmd := launcherCommand(entry, "notify", `{"client":"codex-tui"}`)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run local example notify: %v\nstderr=%s", err, stderr.String())
			}
			if strings.TrimSpace(stdout.String()) != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if strings.TrimSpace(stderr.String()) != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func setupPythonLocalExample(t *testing.T, dir string) {
	t.Helper()
	pythonExe, err := findPythonExecutable()
	if err != nil {
		t.Skip(err.Error())
	}
	cmd := exec.Command(pythonExe, "-m", "venv", ".venv")
	cmd.Dir = dir
	runLocalExampleCmd(t, dir, cmd)
}

func setupNodeLocalExample(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("npm", "install")
	cmd.Dir = dir
	runLocalExampleCmd(t, dir, cmd)
}

func runLocalExampleCmd(t *testing.T, root string, cmd *exec.Cmd) {
	t.Helper()
	if cmd.Dir == "" {
		cmd.Dir = root
	}
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", cmd.String(), err, out)
	}
}

func nodeAndNPMAvailable() bool {
	if !nodeRuntimeAvailable() {
		return false
	}
	_, err := exec.LookPath("npm")
	return err == nil
}

func localExampleEntrypointPath(root, runtimeName string) string {
	name := filepath.Join(root, "bin", filepath.Base(root))
	switch {
	case runtimeName == "go" && runtime.GOOS == "windows":
		return name + ".exe"
	case runtime.GOOS == "windows":
		return name + ".cmd"
	default:
		return name
	}
}
