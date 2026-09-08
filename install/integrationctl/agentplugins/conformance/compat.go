// Package conformance decodes captured standard documents without filesystem or runtime access.
package conformance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
)

// SchemaRegistry is an offline schema capability. Implementations must never fetch rules.
type SchemaRegistry interface {
	Supports(string) bool
	Validate(string, any) error
}

// InstallerDecoder preserves historical loading dispositions. Its raw domain results
// and errors are internal service values, not public report DTOs. Author callers use Decoder.
type InstallerDecoder struct {
	Registry SchemaRegistry
	issue    func(code, item string, cause error)
}

func sha256Digest(body []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(body)) }

// The compatibility entrypoints preserve the native caller's duplicate boundaries.
func DecodeJSONObject(body []byte) (map[string]json.RawMessage, map[string]any, error) {
	return decodeJSONObject(body)
}
func DecodeJSON(body []byte, target any) error          { return decodeJSON(body, target) }
func DecodeRawJSONObject(body []byte, target any) error { return decodeRawJSONObject(body, target) }
func RejectDuplicateJSONKeys(body []byte) error         { return rejectDuplicateJSONKeys(body) }

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
