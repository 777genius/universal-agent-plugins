package agentpluginscli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyLockDuplicateKeysRejectedByStrictAndCompatibleReaders(t *testing.T) {
	requirements := environment.Requirements{SchemaVersion: environment.ManifestSchemaVersion}
	digest, err := environment.ManifestDigest(requirements)
	if err != nil {
		t.Fatal(err)
	}
	normal := environment.Lock{LockVersion: environment.LockVersion, ManifestDigest: digest}
	body, err := json.Marshal(normal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeDependencyLockBytes(body, requirements, false); err != nil {
		t.Fatalf("well-formed lock refused: %v", err)
	}
	if _, err := decodeDependencyLockBytes(body, requirements, true); err != nil {
		t.Fatalf("well-formed compatible lock refused: %v", err)
	}
	for _, tc := range []struct{ name, field, injected string }{
		{"identical-root", "\"lock_version\":", "\"lock_version\":1,"},
		{"different-root", "\"lock_version\":", "\"lock_version\":2,"},
		{"escaped-equivalent-root", "\"lock_version\":", "\"loc\\u006b_version\":1,"},
		{"duplicated-digest", "\"manifest_digest\":", "\"manifest_digest\":\"bogus\","},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hostile := []byte(strings.Replace(string(body), tc.field, tc.injected+tc.field, 1))
			if string(hostile) == string(body) {
				t.Fatal("fixture did not inject a duplicate JSON key")
			}
			for _, compatible := range []bool{false, true} {
				if _, err := decodeDependencyLockBytes(hostile, requirements, compatible); err == nil || !strings.Contains(err.Error(), "duplicate") {
					t.Fatalf("compatible=%t did not reject ambiguous lock: %v", compatible, err)
				}
			}
		})
	}
}
