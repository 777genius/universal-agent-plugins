package codex

import (
	"os"
	"path/filepath"
	"testing"
)

// The directory name is Codex's own and was observed on Codex 0.162.0 for this
// marketplace and plugin, so a change of the formula fails here first.
const (
	observedMarketplace = "agentplugins-5ee57608bcb3"
	observedPlugin      = "local-native-plugin-probe"
	observedDataDir     = "e380e494d08453224af00faa13dea0c26c7168d306573b9d62f691bce8eea760"
)

func TestPurgeCodexNativeDataRemovesOnlyThePluginDirectory(t *testing.T) {
	profile := tempRoot(t)
	base := filepath.Join(profile, "plugins", "data", "agent-plugins")
	target := filepath.Join(base, observedDataDir)
	sibling := filepath.Join(base, "other-plugin-data")
	for _, dir := range []string{target, sibling} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := purgeCodexNativeData(profile, observedMarketplace, observedPlugin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("plugin data was not purged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sibling, "state.json")); err != nil {
		t.Fatalf("another plugin's data was touched: %v", err)
	}
	if err := purgeCodexNativeData(profile, observedMarketplace, observedPlugin); err != nil {
		t.Fatalf("an absent directory must not fail: %v", err)
	}
}

func TestPurgeCodexNativeDataRefusesASymlinkedDirectory(t *testing.T) {
	profile := tempRoot(t)
	base := filepath.Join(profile, "plugins", "data", "agent-plugins")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := tempRoot(t)
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, observedDataDir)); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := purgeCodexNativeData(profile, observedMarketplace, observedPlugin); err == nil {
		t.Fatal("a symlinked data directory must be refused")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("the symlink target was deleted: %v", err)
	}
}
