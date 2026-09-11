package providers

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestManagedCodexMarketplaceRegisteredExtendedWindowsPath(t *testing.T) {
	root := t.TempDir()
	managed := filepath.Join(root, "managed")
	foreign := filepath.Join(root, "foreign")
	for _, path := range []string{managed, foreign} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	extendedPath := func(path string) string {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(absolute, `\\?\`) {
			return absolute
		}
		if strings.HasPrefix(absolute, `\\`) {
			return `\\?\UNC\` + strings.TrimPrefix(absolute, `\\`)
		}
		return `\\?\` + absolute
	}
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"managed", extendedPath(managed), true},
		{"foreign", extendedPath(foreign), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prove the extended path exists so a rejection cannot hide a bad fixture.
			if _, err := os.Stat(tc.source); err != nil {
				t.Fatal(err)
			}
			configRoot := t.TempDir()
			config := "[marketplaces.agentplugins-test]\nsource_type = \"local\"\nsource = " + strconv.Quote(tc.source) + "\n"
			if err := os.WriteFile(filepath.Join(configRoot, "config.toml"), []byte(config), 0o600); err != nil {
				t.Fatal(err)
			}
			registered, err := managedCodexMarketplaceRegistered(configRoot, "agentplugins-test", managed)
			if tc.want {
				if err != nil || !registered {
					t.Fatalf("same directory must be accepted: registered=%v err=%v", registered, err)
				}
			} else if registered || err == nil || !strings.Contains(err.Error(), "no longer points at the managed artifact") {
				t.Fatalf("foreign directory must fail ownership check: registered=%v err=%v", registered, err)
			}
		})
	}
}
