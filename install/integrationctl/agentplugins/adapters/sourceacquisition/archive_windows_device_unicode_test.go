package sourceacquisition

import "testing"

// Real archive ingestion, not a string-only unit: invalid Win32 device names
// must be refused on every host before extraction writes any files.
func TestArchiveRefusesUnicodeDeviceAliasesAcrossFormats(t *testing.T) {
	for _, name := range []string{
		"COM¹", "com².txt", "LPT³.tar.gz",
		"publisher/COM¹/skill.md", "publisher/nested/lpt²/readme.md",
	} {
		t.Run(name, func(t *testing.T) {
			entries := []archiveFixtureEntry{{name: name, body: "inert"}}
			t.Run("ZIP", func(t *testing.T) {
				archiveRejectLocal(t, archiveFixtureZIP(t, entries, ""))
			})
			t.Run("TAR", func(t *testing.T) {
				archiveRejectLocal(t, archiveFixtureTAR(t, entries))
			})
		})
	}
}
