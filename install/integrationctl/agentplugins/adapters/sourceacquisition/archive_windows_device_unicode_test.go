package sourceacquisition

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Real archive ingestion, not a string-only unit: invalid Win32 device names
// must be refused on every host before extraction writes any files.
func TestArchiveRefusesUnicodeDeviceAliasesAcrossFormats(t *testing.T) {
	for _, name := range []string{
		"COM¹", "com².txt", "LPT³.tar.gz",
		"publisher/COM¹/skill.md", "publisher/nested/lpt²/readme.md",
	} {
		t.Run(name, func(t *testing.T) {
			// Keep an otherwise valid Agent Plugins package in the archive. A
			// negative result must not be caused merely by a missing plugin.json.
			entries := append(archivePackageEntries(""), archiveFixtureEntry{name: name, body: "inert"})
			t.Run("ZIP", func(t *testing.T) {
				archiveRejectLocal(t, archiveFixtureZIP(t, entries, ""))
			})
			t.Run("TAR", func(t *testing.T) {
				archiveRejectLocal(t, archiveFixtureTAR(t, entries))
			})
		})
	}
}

// Prove that we did not accidentally ban healthy Unicode names. Both archive
// formats must materialize exact non-device names through the real acquirer.
func TestArchiveRetainsValidUnicodeOutsideDeviceNamespace(t *testing.T) {
	for _, tc := range []struct {
		name string
		pack func(*testing.T, []archiveFixtureEntry) []byte
	}{
		{"ZIP", func(t *testing.T, entries []archiveFixtureEntry) []byte {
			return archiveFixtureZIP(t, entries, "")
		}},
		{"TAR", archiveFixtureTAR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := append(archivePackageEntries(""), archiveFixtureEntry{
				name: "docs/report¹.txt", body: "portable unicode content",
			})
			payload := tc.pack(t, entries)
			snapshot, err := (Acquirer{TempRoot: t.TempDir()}).AcquireArchive(
				t.Context(), sourcespec.Archive{URL: archiveLocalFile(t, payload)}, "", "")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := packagedigest.Remove(snapshot); err != nil {
					t.Errorf("remove acquired snapshot: %v", err)
				}
			})
			body, err := os.ReadFile(filepath.Join(snapshot.Root, "docs", "report¹.txt"))
			if err != nil || string(body) != "portable unicode content" {
				t.Fatalf("valid Unicode archive content=%q, err=%v", body, err)
			}
		})
	}
}
