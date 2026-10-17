package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestUpsertKimiInstalledRecordWritesLocalPathEntry(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	pluginRoot := filepath.Join(config, "plugins", "managed", "demo-0123456789ab")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := upsertKimiInstalledRecord(config, "kimi-basic", pluginRoot); err != nil {
		t.Fatal(err)
	}
	file, err := readKimiInstalled(config)
	if err != nil {
		t.Fatal(err)
	}
	if file.Version != 1 || len(file.Plugins) != 1 {
		t.Fatalf("installed.json = %+v", file)
	}
	record := file.Plugins[0]
	if record.ID != "kimi-basic" || record.Source != "local-path" || !record.Enabled || !samePath(record.Root, pluginRoot) {
		t.Fatalf("record = %+v", record)
	}
	if err := upsertKimiInstalledRecord(config, "kimi-basic", pluginRoot); err != nil {
		t.Fatal(err)
	}
	file, err = readKimiInstalled(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Plugins) != 1 {
		t.Fatalf("duplicate Kimi records = %+v", file.Plugins)
	}
}

func TestUpsertKimiInstalledRecordRejectsForeignRoot(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	if err := upsertKimiInstalledRecord(config, "kimi-basic", filepath.Join(config, "foreign")); err != nil {
		t.Fatal(err)
	}
	err := upsertKimiInstalledRecord(config, "kimi-basic", filepath.Join(config, "ours"))
	if err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveKimiInstalledRecordDropsMatchingIdentity(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	pluginRoot := filepath.Join(config, "plugins", "managed", "demo")
	if err := upsertKimiInstalledRecord(config, "kimi-basic", pluginRoot); err != nil {
		t.Fatal(err)
	}
	if err := removeKimiInstalledRecord(config, "kimi-basic", pluginRoot); err != nil {
		t.Fatal(err)
	}
	file, err := readKimiInstalled(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Plugins) != 0 {
		t.Fatalf("plugins after remove = %+v", file.Plugins)
	}
}

func TestInspectKimiRegistryReportsCollisionAndOwnership(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	active := filepath.Join(config, "plugins", "managed", "demo")
	plan := domain.DeliveryPlan{DeclaredName: "kimi-basic", ActivePath: active, NativeRegistryRoot: config}
	finding, err := inspectKimiRegistry(plan, nil)
	if err != nil || finding != registryClear {
		t.Fatalf("empty registry = %v %v", finding, err)
	}
	if err := upsertKimiInstalledRecord(config, "kimi-basic", filepath.Join(config, "foreign")); err != nil {
		t.Fatal(err)
	}
	finding, err = inspectKimiRegistry(plan, nil)
	if err != nil || finding != registryCollision {
		t.Fatalf("foreign registry = %v %v", finding, err)
	}

	ownedConfig := t.TempDir()
	ownedActive := filepath.Join(ownedConfig, "plugins", "managed", "demo")
	if err := upsertKimiInstalledRecord(ownedConfig, "kimi-basic", ownedActive); err != nil {
		t.Fatal(err)
	}
	ownedPlan := domain.DeliveryPlan{DeclaredName: "kimi-basic", ActivePath: ownedActive, NativeRegistryRoot: ownedConfig}
	finding, err = inspectKimiRegistry(ownedPlan, &domain.ClientBinding{})
	if err != nil || finding != registryExpected {
		t.Fatalf("owned registry = %v %v", finding, err)
	}
}
