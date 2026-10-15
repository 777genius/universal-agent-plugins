//go:build uapdeps_e2e

package repotests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The default non-skipped CLI/SDK acceptance suite lives in
// cli/internal/agentpluginscli/deps_e2e_test.go. This additional binary/remote
// transport harness is optional.
// TestUAPDependenciesTwoRootLockTransfer exercises the real CLI with explicit
// disposable roots. It is opt-in because it requires the caller's reviewed test
// package source and built agentplugins binary; no user profile is used.
func TestUAPDependenciesTwoRootLockTransfer(t *testing.T) {
	binary := os.Getenv("AGENTPLUGINS_E2E_BINARY")
	fixture := os.Getenv("UAP_DEPS_E2E_MANIFEST")
	if binary == "" || fixture == "" {
		t.Skip("set AGENTPLUGINS_E2E_BINARY and UAP_DEPS_E2E_MANIFEST to run the dependency E2E")
	}
	if !filepath.IsAbs(binary) || !filepath.IsAbs(fixture) {
		t.Fatal("E2E binary and manifest paths must be absolute")
	}
	rootA, rootB, previewRoot := filepath.Join(t.TempDir(), "root-a"), filepath.Join(t.TempDir(), "root-b"), filepath.Join(t.TempDir(), "preview")
	for _, root := range []string{rootA, rootB, previewRoot} {
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	manifestA, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootA, "uap.toml"), manifestA, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previewRoot, "uap.toml"), manifestA, 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(root string, args ...string) string {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = root
		command.Env = append(os.Environ(), "AGENTPLUGINS_HOME="+root, "HOME="+filepath.Join(root, "home"))
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", strings.Join(args, " "), err, output)
		}
		return string(output)
	}
	run(previewRoot, "deps", "lock", "--dry-run", "--format", "json")
	for _, name := range []string{"uap.lock", "state-v2.json", "store", "mutation.lock", "environment.lock"} {
		if _, err := os.Lstat(filepath.Join(previewRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("dry-run durable target %s exists: %v", name, err)
		}
	}
	run(rootA, "deps", "lock")
	run(rootA, "deps", "sync", "--locked", "--yes", "--format", "json")
	first := run(rootA, "deps", "status", "--format", "json")
	lockA, err := os.ReadFile(filepath.Join(rootA, "uap.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "uap.toml"), manifestA, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootB, "uap.lock"), lockA, 0o600); err != nil {
		t.Fatal(err)
	}
	run(rootB, "deps", "sync", "--locked", "--yes", "--format", "json")
	second := run(rootB, "deps", "status", "--format", "json")
	var left, right map[string]any
	if json.Unmarshal([]byte(first), &left) != nil || json.Unmarshal([]byte(second), &right) != nil {
		t.Fatalf("status is not JSON:\n%s\n%s", first, second)
	}
	if left["manifest_digest"] != right["manifest_digest"] || left["lock_digest"] != right["lock_digest"] {
		t.Fatalf("portable digests differ across roots:\n%s\n%s", first, second)
	}
	if strings.Contains(first, "not_synced") || strings.Contains(second, "not_synced") {
		t.Fatalf("restored root is not synced:\n%s\n%s", first, second)
	}
}
