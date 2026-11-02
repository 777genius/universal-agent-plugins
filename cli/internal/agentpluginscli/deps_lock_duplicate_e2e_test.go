//go:build uappm_e2e

package agentpluginscli

import (
	"strings"
	"testing"
)

// The public install path must reject ambiguous portable lock JSON before any
// source fetch, native agent mutation or receipt rewrite. Even duplicated
// identical values are unsafe: another JSON implementation could keep the first.
func pmAgreementDuplicateJSONKeysFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name, anchor, injected, expectedError string
	}{
		{"root-same-value", "\"lock_version\":", "\"lock_version\": 1, ", ""},
		{"root-escaped-equivalent", "\"lock_version\":", "\"loc\\u006b_version\": 1, ", ""},
		{"root-different-value", "\"manifest_digest\":", "\"manifest_digest\": \"invalid\", ", ""},
		{"root-casefold-alias", "\"lock_version\":", "\"LOCK_VERSION\": 2, ", ""},
		{"package-casefold-source-key", "\"source_key\":", "\"SOURCE_KEY\": \"foreign-owner\", ", ""},
		{"nested-casefold-checksum", "\"sha256\":", "\"SHA256\": \"invalid\", ", ""},
		{"package-source-key", "\"source_key\":", "\"source_key\": \"foreign-owner\", ", ""},
		{"package-tree-digest", "\"tree_digest\":", "\"tree_digest\": \"sha256:untrusted\", ", ""},
		{"nested-source-object", "\"source\":", "\"source\": null, ", ""},
		{"nested-archive-checksum", "\"sha256\":", "\"sha256\": \"invalid\", ", ""},
		{"invalid-utf8-identity", "\"manifest_digest\": \"", string([]byte{0xff}), "UTF-8"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin := newPMArchiveOrigin(t, pmArchiveGraph{"a": nil})
			fixture := newPMArchiveEnvironment(t, origin, false)
			fixture.manifest(t, origin, []string{"a"}, map[string][]string{"gemini": {"a"}})
			fixture.run(t, "install", "--yes", "--format", "json")
			fixture.assertInstalled(t, pmArchiveTargets{"a": {"gemini"}})
			before := fixture.activeFiles(t)
			_, lockPath := fixture.app.dependencyPaths()
			lock := dependencyRead(t, lockPath)
			if !strings.Contains(lock, tc.anchor) {
				t.Fatalf("fixture no longer has security-relevant field %q", tc.anchor)
			}
			replacement := tc.injected + tc.anchor
			if tc.expectedError == "UTF-8" {
				// Preserve otherwise-valid JSON; only the authored byte is unsafe.
				replacement = tc.anchor + tc.injected
			}
			tampered := strings.Replace(lock, tc.anchor, replacement, 1)
			dependencyWrite(t, lockPath, tampered)
			networkBefore := origin.counts()
			_, err := fixture.executePM(t.Context(), t, "", false,
				"install", "--locked", "--yes", "--format", "json")
			expectedError := tc.expectedError
			if expectedError == "" {
				expectedError = "duplicate"
			}
			if err == nil || !strings.Contains(err.Error(), expectedError) {
				t.Fatalf("unsafe uap.lock was accepted or rejected for wrong reason: %v", err)
			}
			if got := dependencyRead(t, lockPath); got != tampered {
				t.Fatal("refused lock was rewritten")
			}
			pmArchiveAssertEqual(t, "duplicate lock left native and managed bytes/ownership untouched", before, fixture.activeFiles(t))
			pmArchiveAssertEqual(t, "duplicate lock triggered no source requests", networkBefore, origin.counts())
		})
	}
}
