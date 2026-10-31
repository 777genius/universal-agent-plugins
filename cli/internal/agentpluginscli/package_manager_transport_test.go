//go:build uappm_e2e

package agentpluginscli

import "testing"

// Reuse existing transport journeys in the portable qualification. These are
// additional selected coverage, not newly invented lifecycle implementations.
func packageManagerTransportCases() map[string]func(*testing.T) {
	cases := map[string]func(*testing.T){
		"nested-literal-plus": TestArchiveLiteralPlusSelectorSavedAndColdRestore,
	}
	for id, format := range map[string]string{
		"zip": "zip", "tar": "tar", "tar-gzip": "tar.gz", "tgz": "tgz", "extensionless": "extensionless",
	} {
		cases[id] = func(t *testing.T) { directSavedLifecycleCase(t, format) }
	}
	return cases
}
