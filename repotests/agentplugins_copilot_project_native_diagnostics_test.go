//go:build linux

package pluginkitairepo_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

const copilotProjectConfigDiagnosticLimit = 16 << 10

type copilotProjectConfigMemberChange struct {
	KeySHA256  string `json:"key_sha256"`
	BeforeType string `json:"before_type"`
	AfterType  string `json:"after_type"`
}

// Only held digests, counts and finite structural facts cross this boundary.
// Unknown profile keys, values, paths and errors never enter the artifact.
type copilotProjectConfigChange struct {
	ExpectedSHA256      string                             `json:"expected_sha256"`
	ObservedSHA256      string                             `json:"observed_sha256,omitempty"`
	BaselineBytes       int                                `json:"baseline_bytes"`
	ObservedBytes       *int                               `json:"observed_bytes,omitempty"`
	Status              string                             `json:"status"`
	SemanticallyEqual   *bool                              `json:"semantically_equal,omitempty"`
	TrustedFoldersMatch *bool                              `json:"trusted_folders_match,omitempty"`
	ChangedMembers      []copilotProjectConfigMemberChange `json:"changed_members,omitempty"`
}

func (f *copilotProjectAttempt) captureInstalled(t *testing.T, mcp, skill string) {
	t.Helper()
	for _, snapshot := range []struct{ path, name string }{
		{mcp, "mcp.json.installed"}, {skill, "uapcopilota-SKILL.md.installed"},
	} {
		body, err := projectNativeRuntimeRead(snapshot.path, copilotProjectBodyLimit)
		if err != nil {
			t.Fatal(err)
		}
		f.keep(t, snapshot.name, body)
	}
}

func (f *copilotProjectAttempt) protectedDigest(path, expected string) (string, error) {
	observed, err := projectNativeRuntimeDigest(path)
	if path == f.originalConfigPath && (err != nil || observed != expected) {
		change := copilotProjectConfigChange{ExpectedSHA256: expected, ObservedSHA256: observed, BaselineBytes: len(f.originalConfigBaseline), Status: "deadline"}
		if f.ctx == nil || f.ctx.Err() == nil {
			change = copilotProjectConfigDiagnostic(f.originalConfigBaseline, expected, observed, err, func() ([]byte, error) {
				return projectNativeRuntimeRead(path, copilotProjectConfigDiagnosticLimit)
			})
		}
		f.evidence["original_config_change"] = change
	}
	// The caller's original err/digest comparison remains the fatal guard.
	return observed, err
}

func copilotProjectConfigHash(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func copilotProjectConfigDiagnostic(baseline []byte, expected, observed string, guardErr error, read func() ([]byte, error)) copilotProjectConfigChange {
	change := copilotProjectConfigChange{ExpectedSHA256: expected, ObservedSHA256: observed, BaselineBytes: len(baseline), Status: "guard-read-refused"}
	if guardErr != nil {
		return change
	}
	change.Status = "baseline-refused"
	if len(baseline) > copilotProjectConfigDiagnosticLimit || copilotProjectConfigHash(baseline) != expected {
		return change
	}
	body, err := read()
	change.Status = "read-refused"
	if err != nil || len(body) > copilotProjectConfigDiagnosticLimit {
		return change
	}
	change.Status = "unstable"
	if copilotProjectConfigHash(body) != observed {
		return change
	}
	size := len(body)
	change.ObservedBytes = &size
	before, beforeErr := projectNativeRuntimeJSON(baseline)
	after, afterErr := projectNativeRuntimeJSON(body)
	change.Status = "invalid-json"
	if beforeErr != nil || afterErr != nil {
		return change
	}
	beforeObject, okBefore := before.(map[string]any)
	afterObject, okAfter := after.(map[string]any)
	change.Status = "non-object"
	if !okBefore || !okAfter {
		return change
	}
	change.ChangedMembers, okAfter = copilotProjectChangedMembers(beforeObject, afterObject)
	change.Status = "change-limit"
	if !okAfter {
		return change
	}
	equal := reflect.DeepEqual(beforeObject, afterObject)
	trusted := reflect.DeepEqual(beforeObject["trustedFolders"], afterObject["trustedFolders"])
	change.Status, change.SemanticallyEqual, change.TrustedFoldersMatch = "captured", &equal, &trusted
	return change
}

func copilotProjectChangedMembers(before, after map[string]any) ([]copilotProjectConfigMemberChange, bool) {
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	var changes []copilotProjectConfigMemberChange
	for key := range keys {
		old, hadOld := before[key]
		current, hasCurrent := after[key]
		if hadOld == hasCurrent && reflect.DeepEqual(old, current) {
			continue
		}
		if len(changes) == 32 {
			return nil, false
		}
		changes = append(changes, copilotProjectConfigMemberChange{KeySHA256: copilotProjectConfigHash([]byte(key)), BeforeType: copilotProjectConfigValueType(old, hadOld), AfterType: copilotProjectConfigValueType(current, hasCurrent)})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].KeySHA256 < changes[j].KeySHA256 })
	return changes, true
}

func copilotProjectConfigValueType(value any, present bool) string {
	if !present {
		return "absent"
	}
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		// Both maps came from the strict JSON decoder; only object remains.
		return "object"
	}
}
