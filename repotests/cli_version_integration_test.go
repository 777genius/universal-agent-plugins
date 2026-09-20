package pluginkitairepo_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginKitAIVersionCommand(t *testing.T) {
	bin := buildPluginKitAI(t)
	cmd := exec.Command(bin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai version: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"module:", "version:", "go:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plugin-kit-ai version output missing %q:\n%s", want, text)
		}
	}
}

func TestPluginKitAIVersionCommand_UsesInjectedReleaseVersion(t *testing.T) {
	root := RepoRoot(t)
	outPath := filepath.Join(t.TempDir(), "plugin-kit-ai")
	build := exec.Command("go", "build", "-ldflags", "-X main.version=v1.0.1", "-o", outPath, "./cmd/plugin-kit-ai")
	build.Dir = filepath.Join(root, "cli")
	buildOut, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build plugin-kit-ai with injected version: %v\n%s", err, buildOut)
	}

	cmd := exec.Command(outPath, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("plugin-kit-ai version: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "version: v1.0.1") {
		t.Fatalf("plugin-kit-ai version output missing injected version:\n%s", text)
	}
}
