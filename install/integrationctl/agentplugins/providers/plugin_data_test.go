package providers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginDataReceiptPreservesAndPurgesOnlyOwnedDirectory(t *testing.T) {
	t.Parallel()
	manager := PluginDataManager{Base: filepath.Join(t.TempDir(), "data")}
	receipt, created, err := manager.EnsureData(context.Background(), "installation", "backend", "user")
	if err != nil || !created {
		t.Fatalf("ensure data: created=%v err=%v", created, err)
	}
	marker := filepath.Join(receipt.Locator, "persistent-value")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	reused, created, err := manager.EnsureData(context.Background(), "installation", "backend", "user")
	if err != nil || created || reused.DataReceiptID != receipt.DataReceiptID {
		t.Fatalf("reuse = %+v created=%v err=%v", reused, created, err)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "keep" {
		t.Fatalf("persistent data was replaced: %q %v", body, err)
	}
	stale := receipt
	stale.OwnershipDigest = "sha256:stale"
	if err := manager.PurgeData(context.Background(), stale); err == nil {
		t.Fatal("stale receipt purged data")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("stale purge mutated data: %v", err)
	}
	if err := manager.PurgeData(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(receipt.Locator); !os.IsNotExist(err) {
		t.Fatalf("owned data survived purge: %v", err)
	}
}

func TestPluginDataMovedBackupKeepsOriginalLogicalLocator(t *testing.T) {
	manager := PluginDataManager{Base: filepath.Join(t.TempDir(), "data")}
	ctx := context.Background()
	receipt, _, err := manager.EnsureData(ctx, "installation", "backend", "user")
	if err != nil {
		t.Fatal(err)
	}
	originalMarker, err := os.ReadFile(filepath.Join(receipt.Locator, dataOwnershipMarker))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receipt.Locator, "user-data"), []byte("persistent bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(manager.Base, ".agentplugins-backup-test")
	if err := os.Rename(receipt.Locator, backup); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateDataAt(ctx, receipt, backup); err != nil {
		t.Fatal(err)
	}
	unchangedMarker, err := os.ReadFile(filepath.Join(backup, dataOwnershipMarker))
	if err != nil || string(unchangedMarker) != string(originalMarker) {
		t.Fatalf("marker rewritten: %q, %v", unchangedMarker, err)
	}
	if err := manager.ValidateData(ctx, receipt); err == nil {
		t.Fatal("validation ignored the physical path")
	}
	rewrittenReceipt := receipt
	rewrittenReceipt.Locator = backup
	if err := manager.ValidateDataAt(ctx, rewrittenReceipt, backup); err == nil {
		t.Fatal("backup locator substituted for original logical locator")
	}
	if err := os.WriteFile(filepath.Join(backup, dataOwnershipMarker), []byte(`{"state":"owned"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.ValidateDataAt(ctx, receipt, backup); err == nil {
		t.Fatal("foreign marker accepted")
	}
	body, err := os.ReadFile(filepath.Join(backup, "user-data"))
	if err != nil || string(body) != "persistent bytes" {
		t.Fatalf("backup bytes lost: %q %v", body, err)
	}
}

func TestPluginDataRejectsSymlinkMarker(t *testing.T) {
	manager := PluginDataManager{Base: filepath.Join(t.TempDir(), "data")}
	ctx := context.Background()
	receipt, _, err := manager.EnsureData(ctx, "installation", "backend", "user")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(receipt.Locator, dataOwnershipMarker)
	foreign := filepath.Join(t.TempDir(), "marker")
	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(foreign, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, marker); err != nil {
		t.Fatal(err)
	}
	if err := manager.PurgeData(ctx, receipt); err == nil {
		t.Fatal("symlink marker authorized purge")
	}
	got, err := os.ReadFile(foreign)
	if err != nil || string(got) != string(body) {
		t.Fatal("foreign marker changed")
	}
	if target, err := os.Readlink(marker); err != nil || target != foreign {
		t.Fatalf("marker changed: %q %v", target, err)
	}
}
