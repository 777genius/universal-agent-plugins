package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Build the actual public CLI once; the crash remains the already-qualified
// separate SDK process at the deterministic native barrier.
func deathBuildPublicCLI(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "agentplugins"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", binary, "./cli/cmd/agentplugins")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build actual public CLI: %v; %s", err, output)
	}
	return binary
}

// A new public CLI process must never convert an unresolved native attempt into
// a clean install or silently acknowledge recovery. No source/network is used.
func deathObservePublicCLI(t *testing.T, binary, base, project string, retained map[string]deathTreeEntry) {
	t.Helper()
	home := filepath.Join(base, "user")
	path := "/usr/bin:/bin"
	if runtime.GOOS == "windows" {
		systemRoot := os.Getenv("SystemRoot")
		if !filepath.IsAbs(systemRoot) {
			t.Fatal("Windows root is not absolute")
		}
		path = filepath.Join(systemRoot, "System32")
	}
	env := []string{
		"HOME=" + home, "USERPROFILE=" + home,
		"AGENTPLUGINS_HOME=" + filepath.Join(base, "user-uap"),
		"PATH=" + path, "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0",
		"XDG_CONFIG_HOME=" + filepath.Join(base, "xdg-config"),
		"XDG_CACHE_HOME=" + filepath.Join(base, "xdg-cache"),
		"XDG_DATA_HOME=" + filepath.Join(base, "xdg-data"),
		"XDG_STATE_HOME=" + filepath.Join(base, "xdg-state"),
	}
	for _, action := range []struct {
		name string
		args []string
	}{
		{"status", []string{"deps", "status"}},
		{"recover-dry-run", []string{"deps", "recover", "--dry-run"}},
		{"recover-confirmed", []string{"deps", "recover", "--yes"}},
	} {
		t.Run(action.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			argv := append([]string{"--scope", "project", "--project-root", project, "--format", "json"}, action.args...)
			command := exec.CommandContext(ctx, binary, argv...)
			command.Dir, command.Env = project, env
			body, runErr := command.CombinedOutput()
			if ctx.Err() != nil || len(body) > 64<<10 {
				t.Fatalf("public %s did not terminate with bounded output", action.name)
			}
			var doc map[string]any
			jsonErr := json.Unmarshal(body, &doc)
			// Recover must refuse this unresolved native identity, unlike
			// a fully committed directory-journal recovery in PR #704.
			if action.name == "recover-confirmed" && runErr == nil {
				t.Fatal("public CLI acknowledged an unqualified native attempt")
			}
			if runErr == nil {
				if jsonErr != nil || (doc["pending_recovery"] != true && !isRecoveryRequired(doc)) {
					t.Fatalf("public %s reported no pending recovery", action.name)
				}
			} else if !strings.Contains(strings.ToLower(string(body)), "native attempt") &&
				!strings.Contains(strings.ToLower(string(body)), "recovery") {
				t.Fatalf("public %s refused for an unrelated reason: %s", action.name, boundedDeathError(body))
			}
			deathAssertTree(t, project, retained)
		})
	}
}

func isRecoveryRequired(doc map[string]any) bool {
	recovery, _ := doc["recovery"].(map[string]any)
	return recovery["required"] == true
}

func boundedDeathError(body []byte) string {
	if len(body) > 300 {
		body = body[:300]
	}
	return fmt.Sprintf("%q", body)
}
