package commands_test

import (
	"os"
	"testing"
)

func pathFixAlias(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func pathFixWindowsContracts(*testing.T, pathFixBinaries) {}
