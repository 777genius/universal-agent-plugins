package providers

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestManagedCodexMarketplaceRegisteredAcceptsFilesystemAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating directory symlinks requires elevated Windows privileges")
	}
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	managed := filepath.Join(root, "managed")
	aliasRoot := filepath.Join(root, "alias")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(managed, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, aliasRoot); err != nil {
		t.Fatal(err)
	}
	config := "[marketplaces.agentplugins-test]\nsource_type = \"local\"\nsource = " + strconv.Quote(filepath.Join(aliasRoot, "managed")) + "\n"
	if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	registered, err := managedCodexMarketplaceRegistered(configRoot, "agentplugins-test", managed)
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Fatal("filesystem aliases should identify the same managed artifact")
	}
}

func TestManagedCodexMarketplaceRegisteredRejectsDifferentResolvedPath(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "config")
	managed := filepath.Join(root, "managed")
	other := filepath.Join(root, "other")
	for _, directory := range []string{configRoot, managed, other} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	config := "[marketplaces.agentplugins-test]\nsource_type = \"local\"\nsource = " + strconv.Quote(other) + "\n"
	if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	registered, err := managedCodexMarketplaceRegistered(configRoot, "agentplugins-test", managed)
	if err == nil || registered {
		t.Fatalf("different resolved path must fail closed: registered=%v err=%v", registered, err)
	}
}

func TestEquivalentLocalPathRejectsUnresolvedAlias(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	for _, pair := range [][2]string{{missing, root}, {root, missing}} {
		if equivalentLocalPath(pair[0], pair[1]) {
			t.Fatalf("unresolved alias must fail closed: %q and %q", pair[0], pair[1])
		}
	}
}

func TestEquivalentLocalPathAcceptsIdenticalCleanedPath(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	if !equivalentLocalPath(missing+string(filepath.Separator)+".", missing) {
		t.Fatal("identical cleaned paths must remain accepted without filesystem evidence")
	}
}
