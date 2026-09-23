package providers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func lockedRuntimeFixture(t *testing.T) (PluginDataManager, domain.PackageEnvelope, domain.DeliveryPlan, string, *int) {
	t.Helper()
	root := t.TempDir()
	snapshot := filepath.Join(root, "source")
	runtimeRoot := filepath.Join(snapshot, filepath.FromSlash(lockedRuntimePath))
	if err := os.MkdirAll(runtimeRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := fmt.Sprintf(`{"name":"test","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"dependencies":{"@example/mcp":"1.2.3"}},"node_modules/@example/mcp":{"version":"1.2.3","resolved":"https://registry.npmjs.org/@example/mcp/-/mcp-1.2.3.tgz","integrity":"sha512-%s"}}}`,
		base64.StdEncoding.EncodeToString(make([]byte, 64)))
	sum := sha256.Sum256([]byte(lock))
	files := map[string]string{
		"runtime.json":      fmt.Sprintf(`{"schema_version":1,"package":"@example/mcp","version":"1.2.3","entrypoint":"node_modules/@example/mcp/cli.js","package_lock_sha256":"sha256:%s","omit_optional":false}`, hex.EncodeToString(sum[:])),
		"package.json":      `{"name":"test","version":"1.0.0","private":true,"dependencies":{"@example/mcp":"1.2.3"}}`,
		"package-lock.json": lock,
		"launcher.mjs":      "// fixture",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(runtimeRoot, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runs := new(int)
	manager := PluginDataManager{Base: filepath.Join(root, "plugin-data")}
	manager.RunNPM = func(_ context.Context, temporary, _ string, _ bool) error {
		*runs++
		entrypoint := filepath.Join(temporary, "node_modules", "@example", "mcp", "cli.js")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(temporary, "node_modules", ".bin"), 0o700); err != nil {
			return err
		}
		return os.WriteFile(entrypoint, []byte("// fixture"), 0o600)
	}
	envelope := domain.PackageEnvelope{SnapshotRoot: snapshot, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
		"mcp": {Name: "mcp", Type: "stdio", Decoded: map[string]any{
			"command": "node", "args": []any{lockedLauncherArg},
		}},
	}}}
	plan := domain.DeliveryPlan{Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "mcp", Support: domain.SupportNative}}}
	receipt, _, err := manager.EnsureData(context.Background(), "installation", "backend", "user")
	if err != nil {
		t.Fatal(err)
	}
	return manager, envelope, plan, receipt.Locator, runs
}

func TestLockedRuntimePreparedOnceBeforeActivation(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	for range 2 {
		if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
			t.Fatal(err)
		}
	}
	if *runs != 1 {
		t.Fatalf("npm runs = %d, want one", *runs)
	}
	body, err := os.ReadFile(filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath), "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	target := filepath.Join(dataPath, "npm-runtime", hex.EncodeToString(sum[:]))
	if _, err := os.Stat(filepath.Join(target, runtimeMarkerName)); err != nil {
		t.Fatalf("runtime marker: %v", err)
	}
	if _, err := os.Stat(target + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock after preparation: %v", err)
	}
}

func TestLockedRuntimeNotSelectedStaysInert(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	plan.Components[0].Support = domain.SupportUnsupported
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
		t.Fatal(err)
	}
	if *runs != 0 {
		t.Fatal("unselected runtime ran npm")
	}
}

func TestLockedRuntimeRejectsUntrustedLockBeforeNPM(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	runtimeRoot := filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath))
	lockPath := filepath.Join(runtimeRoot, "package-lock.json")
	body, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.ReplaceAll(string(body), "registry.npmjs.org", "attacker.example")
	if err := os.WriteFile(lockPath, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(changed))
	config := fmt.Sprintf(`{"schema_version":1,"package":"@example/mcp","version":"1.2.3","entrypoint":"node_modules/@example/mcp/cli.js","package_lock_sha256":"sha256:%s","omit_optional":false}`, hex.EncodeToString(sum[:]))
	if err := os.WriteFile(filepath.Join(runtimeRoot, "runtime.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err == nil || !strings.Contains(err.Error(), "registry.npmjs.org") {
		t.Fatalf("unexpected validation result: %v", err)
	}
	if *runs != 0 {
		t.Fatal("untrusted lock ran npm")
	}
}

func TestLockedRuntimeNPMFailureLeavesNoPreparedTarget(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	manager.RunNPM = func(context.Context, string, string, bool) error {
		*runs++
		return errors.New("offline")
	}
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("unexpected npm failure: %v", err)
	}
	if *runs != 1 {
		t.Fatalf("npm runs = %d", *runs)
	}
	entries, err := os.ReadDir(filepath.Join(dataPath, "npm-runtime"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("runtime store after failure: %v, %v", entries, err)
	}
}

func TestLockedRuntimeConcurrentPreparationRunsNPMOnce(t *testing.T) {
	manager, envelope, plan, dataPath, _ := lockedRuntimeFixture(t)
	var runs atomic.Int32
	manager.RunNPM = func(_ context.Context, temporary, _ string, _ bool) error {
		runs.Add(1)
		time.Sleep(150 * time.Millisecond)
		entrypoint := filepath.Join(temporary, "node_modules", "@example", "mcp", "cli.js")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(temporary, "node_modules", ".bin"), 0o700); err != nil {
			return err
		}
		return os.WriteFile(entrypoint, []byte("// fixture"), 0o600)
	}
	var group sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			errs <- manager.PrepareRuntime(context.Background(), envelope, plan, dataPath)
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("concurrent npm runs = %d, want one", got)
	}
}

// Set UAP_LOCKED_RUNTIME_FIXTURES to a disposable registry checkout to verify
// the real bridge metadata without making installer CI depend on another repo.
func TestLockedRuntimeRegistryFixtures(t *testing.T) {
	registry := os.Getenv("UAP_LOCKED_RUNTIME_FIXTURES")
	if registry == "" {
		t.Skip("set UAP_LOCKED_RUNTIME_FIXTURES for the cross-repository local check")
	}
	for _, plugin := range []string{"playwright", "chrome-devtools", "context7", "firebase", "hubspot-developer"} {
		t.Run(plugin, func(t *testing.T) {
			root := filepath.Join(registry, "plugins", plugin, filepath.FromSlash(lockedRuntimePath))
			config, err := os.ReadFile(filepath.Join(root, "runtime.json"))
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err != nil {
				t.Fatal(err)
			}
			lock, err := os.ReadFile(filepath.Join(root, "package-lock.json"))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := validateLockedRuntime(config, manifest, lock); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLockedRuntimeRejectsSymlinkEntrypoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires Windows developer mode")
	}
	manager, envelope, plan, dataPath, _ := lockedRuntimeFixture(t)
	outside := filepath.Join(dataPath, "outside.js")
	if err := os.WriteFile(outside, []byte("// not an installed package"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager.RunNPM = func(_ context.Context, temporary, _ string, _ bool) error {
		entrypoint := filepath.Join(temporary, "node_modules", "@example", "mcp", "cli.js")
		if err := os.MkdirAll(filepath.Dir(entrypoint), 0o700); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(temporary, "node_modules", ".bin"), 0o700); err != nil {
			return err
		}
		return os.Symlink(outside, entrypoint)
	}
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err == nil ||
		!strings.Contains(err.Error(), "unsafe locked npm entrypoint") {
		t.Fatalf("symlinked entrypoint accepted: %v", err)
	}
}
