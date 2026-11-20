package agentpluginscli

import (
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

// The free-disk guard measures the volume the source acquirer stages files on
// and admits when that cannot be measured.
func TestPrefetchGuardMeasuresTheAcquirerStagingVolume(t *testing.T) {
	staging := t.TempDir()
	var measured string
	free, known := uint64(0), true
	original := prefetchFreeDisk
	prefetchFreeDisk = func(path string) (uint64, bool) {
		measured = path
		return free, known
	}
	t.Cleanup(func() { prefetchFreeDisk = original })

	app := App{SourceAcquirer: sourceacquisition.Acquirer{TempRoot: staging}, DependenciesRoot: t.TempDir()}
	guard := app.prefetchGuard()
	for _, test := range []struct {
		free  uint64
		known bool
		admit bool
	}{
		{free: prefetchMinFreeBytes - 1, known: true, admit: false},
		{free: prefetchMinFreeBytes, known: true, admit: true},
		{free: 0, known: false, admit: true}, // statfs failed: fail-open
	} {
		free, known = test.free, test.known
		if got := guard.admit(); got != test.admit || measured != staging {
			t.Fatalf("free=%d known=%v: admit=%v measured %q, want admit=%v on %q", test.free, test.known, got, measured, test.admit, staging)
		}
	}

	// An acquirer without a staging directory uses the system temp directory,
	// as os.MkdirTemp("") does.
	guard = App{SourceAcquirer: sourceacquisition.Acquirer{}}.prefetchGuard()
	free, known = prefetchMinFreeBytes, true
	guard.admit()
	if measured != existingAncestor(os.TempDir()) {
		t.Fatalf("measured %q, want the system temp directory", measured)
	}
}
