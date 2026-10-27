//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"testing"
	"unicode"
	"unicode/utf8"
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
	BaselineJSONValid   *bool                              `json:"baseline_json_valid,omitempty"`
	ObservedJSONValid   *bool                              `json:"observed_json_valid,omitempty"`
	StrictJSONFailure   string                             `json:"strict_json_failure,omitempty"`
	BaselineJSONReason  string                             `json:"baseline_json_reason,omitempty"`
	ObservedJSONReason  string                             `json:"observed_json_reason,omitempty"`
	SemanticallyEqual   *bool                              `json:"semantically_equal,omitempty"`
	TrustedFoldersMatch *bool                              `json:"trusted_folders_match,omitempty"`
	ChangedMembers      []copilotProjectConfigMemberChange `json:"changed_members,omitempty"`
	LeadingCommentProbe *copilotProjectConfigPrefix        `json:"leading_comment_probe,omitempty"`
}

type copilotProjectConfigPrefix struct {
	Status              string `json:"status"`
	PrefixBytes         int    `json:"prefix_bytes,omitempty"`
	CommentLines        int    `json:"comment_lines,omitempty"`
	PrefixSHA256        string `json:"prefix_sha256,omitempty"`
	SuffixBytes         int    `json:"suffix_bytes,omitempty"`
	SuffixJSONReason    string `json:"suffix_json_reason,omitempty"`
	SemanticallyEqual   *bool  `json:"semantically_equal,omitempty"`
	TrustedFoldersMatch *bool  `json:"trusted_folders_match,omitempty"`
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
		change = copilotProjectInvalidJSON(change, baseline, body, beforeErr, afterErr)
		if original, ok := before.(map[string]any); beforeErr == nil && ok && change.ObservedJSONReason == "syntax" {
			change.LeadingCommentProbe = copilotProjectConfigLeadingComments(original, body)
		}
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

func copilotProjectConfigLeadingComments(original map[string]any, body []byte) *copilotProjectConfigPrefix {
	end, lines, status := copilotProjectConfigCommentEnd(body)
	result := &copilotProjectConfigPrefix{Status: status}
	if status != "candidate" {
		return result
	}
	result.Status, result.PrefixBytes, result.CommentLines = "suffix-refused", end, lines
	result.PrefixSHA256, result.SuffixBytes = copilotProjectConfigHash(body[:end]), len(body)-end
	value, err := projectNativeRuntimeJSON(body[end:])
	result.SuffixJSONReason = copilotProjectJSONErrorKind(err)
	if err != nil {
		return result
	}
	current, ok := value.(map[string]any)
	if !ok {
		return result
	}
	equal := reflect.DeepEqual(original, current)
	beforeTrust, hadTrust := original["trustedFolders"]
	afterTrust, hasTrust := current["trustedFolders"]
	trusted := hadTrust == hasTrust && reflect.DeepEqual(beforeTrust, afterTrust)
	result.Status, result.SemanticallyEqual, result.TrustedFoldersMatch = "captured", &equal, &trusted
	return result
}

// This private projection never changes the original invalid-json/digest result.
func copilotProjectConfigCommentEnd(body []byte) (int, int, string) {
	if len(body) > copilotProjectConfigDiagnosticLimit || !utf8.Valid(body) {
		return 0, 0, "prefix-refused"
	}
	end, lines := 0, 0
	for {
		for end < len(body) && bytes.IndexByte([]byte(" \t\r\n"), body[end]) >= 0 {
			end++
			if end > 4096 {
				return 0, 0, "prefix-refused"
			}
		}
		if !bytes.HasPrefix(body[end:], []byte("//")) {
			break
		}
		newline := bytes.IndexByte(body[end:], '\n')
		if newline < 0 || end+newline+1 > 4096 || lines == 16 {
			return 0, 0, "prefix-refused"
		}
		comment := body[end : end+newline+1]
		if bytes.IndexFunc(comment, func(r rune) bool { return unicode.IsControl(r) && r != '\t' && r != '\r' && r != '\n' }) >= 0 {
			return 0, 0, "prefix-refused"
		}
		end, lines = end+newline+1, lines+1
	}
	if lines == 0 {
		return 0, 0, "absent"
	}
	if end == len(body) || body[end] != '{' {
		return 0, 0, "prefix-refused"
	}
	return end, lines, "candidate"
}

func copilotProjectInvalidJSON(change copilotProjectConfigChange, baseline, observed []byte, beforeErr, afterErr error) copilotProjectConfigChange {
	beforeValid, afterValid := json.Valid(baseline), json.Valid(observed)
	change.BaselineJSONValid, change.ObservedJSONValid = &beforeValid, &afterValid
	change.BaselineJSONReason, change.ObservedJSONReason = copilotProjectJSONErrorKind(beforeErr), copilotProjectJSONErrorKind(afterErr)
	switch {
	case beforeErr != nil && afterErr != nil:
		change.StrictJSONFailure = "both"
	case beforeErr != nil:
		change.StrictJSONFailure = "baseline"
	default:
		change.StrictJSONFailure = "observed"
	}
	return change
}

func copilotProjectJSONErrorKind(err error) string {
	if err == nil {
		return "valid"
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return "syntax"
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "unexpected-eof"
	}
	// Only exact static parser reasons become finite labels. Error contents
	// from an unrecognized parser path never cross the evidence boundary.
	switch err.Error() {
	case "duplicate JSON key":
		return "duplicate-key"
	case "JSON body limit":
		return "body-limit"
	case "JSON structure limit":
		return "structure-limit"
	case "JSON key":
		return "key"
	case "JSON object end":
		return "object-end"
	case "JSON array end":
		return "array-end"
	case "unexpected JSON delimiter":
		return "unexpected-delimiter"
	case "nonfinite JSON number":
		return "nonfinite-number"
	case "JSON trailing data":
		return "trailing-data"
	default:
		return "other"
	}
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
