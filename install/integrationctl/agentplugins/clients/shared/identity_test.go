package shared

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestInspectUnqualifiedPluginRootSkipsHostileSiblings(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "owned")
	if err := os.MkdirAll(filepath.Join(owned, ".cursor-plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(owned, ".cursor-plugin", "plugin.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "ccc")); err != nil {
		t.Fatal(err)
	}

	finding, err := InspectUnqualifiedPluginRoot(root, "demo", owned, true)
	if err != nil || finding != clients.RegistryExpected {
		t.Fatalf("finding=%v err=%v", finding, err)
	}

	foreign := filepath.Join(root, "foreign")
	if err := os.MkdirAll(filepath.Join(foreign, ".cursor-plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, ".cursor-plugin", "plugin.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	finding, err = InspectUnqualifiedPluginRoot(root, "demo", owned, true)
	if err != nil || finding != clients.RegistryCollision {
		t.Fatalf("collision finding=%v err=%v", finding, err)
	}
}

func TestSameOwnedPluginDirectoryAcceptsPathAliasAndRejectsLeafSymlink(t *testing.T) {
	root := t.TempDir()
	owned := filepath.Join(root, "owned")
	if err := os.MkdirAll(owned, 0o700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(owned)
	if err != nil {
		t.Fatal(err)
	}
	if !SameOwnedPluginDirectory(owned, resolved) {
		t.Fatalf("alias %q vs %q should be the owned directory", owned, resolved)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(owned, alias); err != nil {
		t.Fatal(err)
	}
	if SameOwnedPluginDirectory(alias, owned) {
		t.Fatal("directory symlink with a different leaf must not count as owned")
	}
	if EquivalentLocalPath("", owned) || EquivalentLocalPath(owned, "   ") || SameOwnedPluginDirectory(owned, "") {
		t.Fatal("empty paths must not compare as the same location")
	}
}
