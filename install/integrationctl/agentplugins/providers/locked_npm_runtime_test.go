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

func TestLockedRuntimeReplacesOwnedCacheWhenMarkerChanges(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
		t.Fatal(err)
	}
	runtimeRoot := filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath))
	configPath := filepath.Join(runtimeRoot, "runtime.json")
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(body), `"omit_optional":false`, `"omit_optional":true`, 1)
	if changed == string(body) {
		t.Fatal("fixture did not contain omit_optional")
	}
	if err := os.WriteFile(configPath, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
		t.Fatal(err)
	}
	if *runs != 2 {
		t.Fatalf("npm runs = %d, want two", *runs)
	}
	lockBody, err := os.ReadFile(filepath.Join(runtimeRoot, "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(lockBody)
	store := filepath.Join(dataPath, "npm-runtime")
	target := filepath.Join(store, hex.EncodeToString(sum[:]))
	markerBody, err := os.ReadFile(filepath.Join(target, runtimeMarkerName))
	if err != nil {
		t.Fatal(err)
	}
	var marker lockedRuntimeMarker
	if err := decodeStrictJSON(markerBody, &marker); err != nil || !marker.OmitOptional {
		t.Fatalf("new marker = %+v, error = %v", marker, err)
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	retired := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".retired-") {
			retired++
			oldBody, err := os.ReadFile(filepath.Join(store, entry.Name(), "runtime", runtimeMarkerName))
			if err != nil || !strings.Contains(string(oldBody), `"omit_optional":false`) {
				t.Fatalf("old runtime was not preserved: %s, %v", entry.Name(), err)
			}
		}
	}
	if retired != 1 {
		t.Fatalf("retired runtimes = %d, want one", retired)
	}
}

func TestLockedRuntimeRepairsMissingEntrypointWithoutDeletingOldCache(t *testing.T) {
	manager, envelope, plan, dataPath, runs := lockedRuntimeFixture(t)
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
		t.Fatal(err)
	}
	lockBody, err := os.ReadFile(filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(lockedRuntimePath), "package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(lockBody)
	store := filepath.Join(dataPath, "npm-runtime")
	target := filepath.Join(store, hex.EncodeToString(sum[:]))
	entrypoint := filepath.Join(target, "node_modules", "@example", "mcp", "cli.js")
	if err := os.Remove(entrypoint); err != nil {
		t.Fatal(err)
	}
	if err := manager.PrepareRuntime(context.Background(), envelope, plan, dataPath); err != nil {
		t.Fatal(err)
	}
	if *runs != 2 {
		t.Fatalf("npm runs = %d, want two", *runs)
	}
	if _, err := os.Stat(entrypoint); err != nil {
		t.Fatalf("repaired entrypoint: %v", err)
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	foundOld := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".retired-") {
			_, err := os.Stat(filepath.Join(store, entry.Name(), "runtime", runtimeMarkerName))
			foundOld = err == nil
		}
	}
	if !foundOld {
		t.Fatal("damaged old runtime was not preserved")
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
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), ".lock.retired-") {
			t.Fatalf("unexpected runtime store entry after failure: %s", entry.Name())
		}
	}
}

func TestLockedRuntimeReclaimsDeadOwnerAndRetiresRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process liveness is checked conservatively")
	}
	root := t.TempDir()
	lockPath := filepath.Join(root, "runtime.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockPath, "owner.json"), []byte(`{"pid":1073741823,"token":"dead"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := reclaimRuntimeLock(lockPath)
	if err != nil || !reclaimed {
		t.Fatalf("reclaim dead owner: %v, %v", reclaimed, err)
	}
	owner, err := recordFreshRuntimeLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseRuntimeLock(lockPath, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release left active lock: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("retired lock count = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), ".retired-") {
			t.Fatalf("unexpected entry %s", entry.Name())
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), runtimeLockReclaimMarker)); err != nil {
			t.Fatalf("retired lock lacks immutable marker: %v", err)
		}
	}
}

func recordFreshRuntimeLock(lockPath string) (string, error) {
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		return "", err
	}
	return recordRuntimeLockOwner(lockPath)
}

func TestLockedRuntimeNeverReclaimsLiveOwner(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "runtime.lock")
	owner, err := recordFreshRuntimeLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	reclaimed, err := reclaimRuntimeLock(lockPath)
	if err != nil || reclaimed {
		t.Fatalf("live lock was reclaimed: %v, %v", reclaimed, err)
	}
	if err := releaseRuntimeLock(lockPath, owner); err != nil {
		t.Fatal(err)
	}
}

func TestLockedRuntimeReclaimsAgedOwnerlessLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "runtime.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := reclaimRuntimeLock(lockPath)
	if err != nil || reclaimed {
		t.Fatalf("fresh ownerless lock was reclaimed: %v, %v", reclaimed, err)
	}
	old := time.Now().Add(-31 * time.Second)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	reclaimed, err = reclaimRuntimeLock(lockPath)
	if err != nil || !reclaimed {
		t.Fatalf("aged ownerless lock was not reclaimed: %v, %v", reclaimed, err)
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

func TestRuntimeStderrTailBoundsDiagnostics(t *testing.T) {
	var writer runtimeStderrTail
	for _, chunk := range []string{strings.Repeat("a", 900), strings.Repeat("b", 900), strings.Repeat("c", 2000)} {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if got := string(writer.tail); got != strings.Repeat("c", 1200) {
		t.Fatalf("stderr tail = %q", got)
	}
}
