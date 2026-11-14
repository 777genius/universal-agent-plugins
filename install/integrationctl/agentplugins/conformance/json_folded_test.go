package conformance

import (
	"strings"
	"testing"
)

// Locks opt into Unicode-simple-fold identity, because encoding/json v1
// accepts case-insensitive struct field names. Other standard JSON consumers
// retain their prior exact-key semantics.
func TestRejectDuplicateJSONKeysFoldedPortableLock(t *testing.T) {
	for _, body := range []string{
		`{"packages":[],"PACKAGES":[]}`,
		`{"source":{"source_key":"a","SOURCE_KEY":"b"}}`,
		`{"lock_version":1,"loc\u212a_version":2}`,
		`{"lock_version":1,"LOCK_VERSION":2}`,
		`{"source":{"source_key":"a","ſource_key":"b"}}`,
		`{"lock_version":1,"lock_version":1}`,
	} {
		t.Run(body, func(t *testing.T) {
			if err := RejectDuplicateJSONKeysFolded([]byte(body)); err == nil || !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("ambiguous lock keys accepted: %q, %v", body, err)
			}
		})
	}
	for _, body := range []string{
		`{"lock_version":1,"manifest_digest":"sha256:a","packages":[]}`,
		`{"source":{"source_key":"a","tree_digest":"b"},"other":[{"foo":1},{"FOO":2}]}`,
	} {
		if err := RejectDuplicateJSONKeysFolded([]byte(body)); err != nil {
			t.Fatalf("well-formed lock keys rejected: %q: %v", body, err)
		}
	}
	// The old public JSON validator has exact-key semantics. Keep that
	// compatibility surface unchanged for standards/extension payloads.
	if err := RejectDuplicateJSONKeys([]byte(`{"name":1,"NAME":2}`)); err != nil {
		t.Fatalf("unrelated exact-key parser behavior changed: %v", err)
	}
}
