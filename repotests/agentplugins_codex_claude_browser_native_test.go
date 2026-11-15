package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The Linux native lane replaces only the dual-user fixture's first install
// with Chromium driving the public CLI's own bundled browser. Its existing
// post-install native inventories, repeat and removals qualify these same bytes.
func nativeBrowserSkillsInstall(t *testing.T, f *nativeFixture, installer string, env []string) {
	t.Helper()
	if runtime.GOOS != "linux" || os.Getenv("GITHUB_ACTIONS") != "true" ||
		os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("browser/native proof requires the opted-in disposable Linux hosted lane")
	}
	node := nativeBinary(t, "AGENTPLUGINS_NATIVE_BROWSER_NODE")
	python := nativeBinary(t, "AGENTPLUGINS_NATIVE_BROWSER_PYTHON")
	source := os.Getenv("AGENTPLUGINS_NATIVE_BROWSER_SOURCE")
	cache := os.Getenv("PLAYWRIGHT_BROWSERS_PATH")
	if !filepath.IsAbs(source) || !filepath.IsAbs(cache) {
		t.Fatal("browser/native proof requires exact source and Chromium paths")
	}
	ui := filepath.Join(source, "ui", "skills-picker")
	playwright := filepath.Join(ui, "node_modules", "@playwright", "test", "cli.js")
	driver := filepath.Join(source, "scripts", "terminal-ui", "native_browser.py")
	for _, path := range []string{playwright, driver} {
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("browser/native tooling file unavailable: %s: %v", path, err)
		}
	}
	childEnv := make([]string, 0, len(env)+12)
	pathValue := ""
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			pathValue = value
		} else if !strings.HasPrefix(entry, "TERM=") {
			childEnv = append(childEnv, entry)
		}
	}
	childEnv = append(childEnv,
		"PATH="+filepath.Dir(node)+string(os.PathListSeparator)+pathValue,
		"TERM=xterm-256color", "CI=1",
		"PLAYWRIGHT_BROWSERS_PATH="+cache,
		"AGENTPLUGINS_NATIVE_BROWSER_LIVE=1",
		"AGENTPLUGINS_NATIVE_BROWSER_PYTHON="+python,
		"AGENTPLUGINS_NATIVE_BROWSER_DRIVER="+driver,
		"AGENTPLUGINS_NATIVE_BROWSER_ROOT="+f.Root,
		"AGENTPLUGINS_NATIVE_BROWSER_PROJECT="+f.Project,
		"AGENTPLUGINS_NATIVE_BROWSER_INSTALLER="+installer,
		"AGENTPLUGINS_NATIVE_BROWSER_SKILL_SOURCE="+nativeRawUserSkillSource,
		"AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=1", "GITHUB_ACTIONS=true", "RUNNER_ENVIRONMENT=github-hosted")
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, node, playwright, "test", "tests/browser/native-skills.spec.ts",
		"--workers=1", "--retries=0", "--forbid-only", "--reporter=json",
		"--output="+filepath.Join(f.Root, "browser-output"))
	command.Dir, command.Env, command.WaitDelay = ui, childEnv, 5*time.Second
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.Output()
	// A navigation failure may include the initial fragment in an error. The
	// capability stays private even in failed structured/transcript artifacts.
	privateURL := regexp.MustCompile("#capability=[^[:space:]\"']+")
	stdout = privateURL.ReplaceAll(stdout, []byte("#capability=[redacted]"))
	diagnostic := privateURL.ReplaceAll(stderr.Bytes(), []byte("#capability=[redacted]"))
	if len(stdout) > 1<<20 || len(diagnostic) > 1<<20 {
		t.Fatal("browser/native report exceeded its 1 MiB evidence bound")
	}
	nativeWrite(t, filepath.Join(f.Root, "native-browser-playwright.json"), stdout, 0600)
	nativeWrite(t, filepath.Join(f.Root, "native-browser-stderr.log"), diagnostic, 0600)
	if err != nil || ctx.Err() != nil {
		if len(diagnostic) > 65536 {
			diagnostic = diagnostic[len(diagnostic)-65536:]
		}
		t.Fatalf("browser/native install failed: %v; deadline=%v\n%s", err, ctx.Err(), diagnostic)
	}
	var report struct {
		Stats struct {
			Expected, Unexpected, Skipped, Flaky int
		}
	}
	if err := json.Unmarshal(stdout, &report); err != nil || report.Stats.Expected != 1 ||
		report.Stats.Unexpected != 0 || report.Stats.Skipped != 0 || report.Stats.Flaky != 0 {
		t.Fatalf("browser/native proof did not execute exactly one passing, unskipped scenario: %+v; %v", report.Stats, err)
	}
}
