//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const windsurfConfigDiagnosticLimit = 64 << 10

type windsurfConfigField struct {
	Present         bool   `json:"present"`
	Type            string `json:"type"`
	MatchesAuthored bool   `json:"matches_authored"`
}

type windsurfConfigAdditionalFields struct {
	Documented         map[string]string            `json:"documented"`
	UnknownCount       int                          `json:"unknown_count"`
	UnknownObservation *windsurfConfigUnknownMember `json:"unknown_observation,omitempty"`
}

type windsurfConfigChange struct {
	ExpectedSHA256   string                          `json:"expected_sha256,omitempty"`
	ObservedSHA256   string                          `json:"observed_sha256,omitempty"`
	GuardError       bool                            `json:"guard_error"`
	Status           string                          `json:"status"`
	StrictJSONValid  *bool                           `json:"strict_json_valid,omitempty"`
	TopLevelFields   *int                            `json:"top_level_fields,omitempty"`
	AutoUpdate       *windsurfConfigField            `json:"auto_update,omitempty"`
	Permissions      *windsurfConfigField            `json:"permissions,omitempty"`
	AdditionalFields *windsurfConfigAdditionalFields `json:"additional_fields,omitempty"`
}

func (f *projectNativeCLIAttempt) captureWindsurfConfigChange(path, expected, observed string, guardErr error) {
	if f.spec.client != "windsurf" || f.base == nil || path != filepath.Join(f.base.Root, "xdg-config", "devin", "config.json") || !f.used["native-attempt"] {
		return
	}
	for _, suffix := range []string{".command.json", ".stdout.log", ".stderr.log"} {
		if _, ok := f.files["native-attempt"+suffix]; !ok {
			return
		}
	}
	if _, exists := f.evidence["windsurf_protected_config_change"]; exists {
		return
	}
	change := windsurfConfigChange{GuardError: guardErr != nil, Status: "deadline"}
	if f.ctx == nil || f.ctx.Err() == nil {
		change = windsurfConfigDiagnostic(expected, observed, guardErr, func() ([]byte, error) {
			return projectNativeRuntimeRead(path, windsurfConfigDiagnosticLimit)
		})
	}
	if raw, err := json.Marshal(change); err != nil || len(raw) > 2<<10 {
		change = windsurfConfigChange{GuardError: guardErr != nil, Status: "diagnostic-bound"}
	}
	f.evidence["windsurf_protected_config_change"] = change
}

func windsurfConfigDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func windsurfConfigDigestValid(digest string) bool {
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func windsurfConfigDiagnostic(expected, observed string, guardErr error, read func() ([]byte, error)) windsurfConfigChange {
	change := windsurfConfigChange{GuardError: guardErr != nil, Status: "invalid-digest"}
	if !windsurfConfigDigestValid(expected) {
		return change
	}
	change.ExpectedSHA256, change.Status = expected, "guard-read-refused"
	if guardErr != nil {
		return change
	}
	change.Status = "invalid-digest"
	if !windsurfConfigDigestValid(observed) {
		return change
	}
	change.ObservedSHA256, change.Status = observed, "read-refused"
	body, err := read()
	if err != nil || len(body) > windsurfConfigDiagnosticLimit {
		return change
	}
	change.Status = "observed-digest-changed"
	if windsurfConfigDigest(body) != observed {
		return change
	}
	value, err := projectNativeRuntimeJSON(body)
	valid := err == nil
	change.StrictJSONValid, change.Status = &valid, "invalid-json"
	if err != nil {
		return change
	}
	object, ok := value.(map[string]any)
	change.Status = "non-object"
	if !ok {
		return change
	}
	_, recipe := projectNativeCLIUpdater("windsurf")
	authored, recipeErr := projectNativeRuntimeJSON(recipe)
	baseline, baselineOK := authored.(map[string]any)
	change.Status = "authored-recipe-refused"
	if recipeErr != nil || !baselineOK {
		return change
	}
	count := len(object)
	change.TopLevelFields, change.Status = &count, "captured"
	change.AutoUpdate = windsurfConfigCompareField(object, baseline, "auto_update")
	change.Permissions = windsurfConfigCompareField(object, baseline, "permissions")
	change.AdditionalFields = windsurfConfigClassifyAddedFields(object)
	return change
}

func windsurfConfigClassifyAddedFields(object map[string]any) *windsurfConfigAdditionalFields {
	fields := &windsurfConfigAdditionalFields{Documented: map[string]string{}, UnknownCount: len(object)}
	for _, key := range []string{"auto_update", "permissions"} {
		if _, present := object[key]; present {
			fields.UnknownCount--
		}
	}
	// Exact names from the pinned official config reference, not permission to
	// accept them in the main config or to expose any corresponding value.
	for _, key := range []string{
		"agent", "attribution", "include_gitignored_files", "keymap", "local_tools_bypass_permissions", "mcpServers", "notify", "proxy",
		"read_config_from", "respect_gitignore", "sandbox", "show_hints", "show_path", "subagents_enabled", "theme_mode", "unicode_mode",
	} {
		if value, present := object[key]; present {
			fields.Documented[key] = windsurfConfigValueType(value)
			fields.UnknownCount--
		}
	}
	fields.UnknownObservation = windsurfConfigObserveUnknown(object, fields)
	return fields
}

func windsurfConfigCompareField(object, authored map[string]any, key string) *windsurfConfigField {
	value, present := object[key]
	kind := "absent"
	if present {
		kind = windsurfConfigValueType(value)
	}
	return &windsurfConfigField{Present: present, Type: kind, MatchesAuthored: present && reflect.DeepEqual(value, authored[key])}
}

func windsurfConfigValueType(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "other"
	}
}

func newWindsurfConfigDiagnostic(t *testing.T) (*projectNativeCLIAttempt, string, []byte) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "xdg-config", "devin", "config.json")
	_, body := projectNativeCLIUpdater("windsurf")
	projectNativeRuntimeWrite(t, path, body, 0600)
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "windsurf"}, base: &nativeFixture{Root: root}, ctx: context.Background(),
		used: map[string]bool{"native-attempt": true}, files: map[string][]byte{"native-attempt.command.json": []byte(`{}`), "native-attempt.stdout.log": {}, "native-attempt.stderr.log": {}},
		protected: map[string]string{path: windsurfConfigDigest(body)}, evidence: map[string]any{"runtime_attempt_complete": false, "qualification": map[string]bool{"project-lifecycle": false}}}
	return f, path, body
}

func TestWindsurfConfigDiagnosticCapturesOnlyFiniteAuthoredFieldComparisons(t *testing.T) {
	for _, kind := range []string{"authored", "whitespace", "changed-auto", "wrong-type-auto", "changed-permissions", "private-unknown", "duplicate", "invalid", "non-object", "oversized"} {
		t.Run(kind, func(t *testing.T) {
			f, path, original := newWindsurfConfigDiagnostic(t)
			body, status := windsurfConfigDiagnosticBody(t, original, kind)
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			guard := f.protected[path]
			observed, err := projectNativeRuntimeDigest(path)
			if err != nil {
				t.Fatal(err)
			}
			f.captureWindsurfConfigChange(path, guard, observed, nil)
			change := windsurfConfigDiagnosticResult(t, f, path, guard, status)
			if status == "captured" {
				wantAuto := kind != "changed-auto" && kind != "wrong-type-auto"
				if change.AutoUpdate.MatchesAuthored != wantAuto || change.Permissions.MatchesAuthored != (kind != "changed-permissions") {
					t.Fatal("authored-field equality differs")
				}
			}
			after, err := os.Lstat(path)
			got, readErr := os.ReadFile(path)
			if err != nil || readErr != nil || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) || !bytes.Equal(got, body) {
				t.Fatal("diagnostic changed protected identity or bytes")
			}
		})
	}
}

func windsurfConfigDiagnosticBody(t *testing.T, original []byte, kind string) ([]byte, string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(original, &value); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "authored":
		return original, "captured"
	case "whitespace":
		return append(bytes.Clone(original), '\n'), "captured"
	case "changed-auto":
		value["auto_update"] = true
	case "wrong-type-auto":
		value["auto_update"] = "PRIVATE-VALUE"
	case "changed-permissions":
		value["permissions"].(map[string]any)["allow"] = []any{"read"}
	case "private-unknown":
		value["PRIVATE-KEY"] = "PRIVATE-VALUE https://private.invalid/token"
	case "duplicate":
		return []byte(`{"auto_update":false,"auto_update":true}`), "invalid-json"
	case "invalid":
		return []byte(`{"PRIVATE-KEY":"PRIVATE-VALUE"`), "invalid-json"
	case "non-object":
		return []byte(`[]`), "non-object"
	case "oversized":
		return bytes.Repeat([]byte{' '}, windsurfConfigDiagnosticLimit+1), "read-refused"
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body, "captured"
}

func windsurfConfigDiagnosticResult(t *testing.T, f *projectNativeCLIAttempt, path, guard, status string) windsurfConfigChange {
	t.Helper()
	change, ok := f.evidence["windsurf_protected_config_change"].(windsurfConfigChange)
	if !ok || change.Status != status || f.protected[path] != guard || f.evidence["runtime_attempt_complete"] != false || !reflect.DeepEqual(f.evidence["qualification"], map[string]bool{"project-lifecycle": false}) {
		t.Fatalf("diagnostic altered authority, failure or qualification: %+v", change)
	}
	body, err := json.Marshal(change)
	if err != nil || len(body) > 2<<10 || bytes.ContainsAny(body, "\r\n") {
		t.Fatal("diagnostic serialization bound")
	}
	for _, forbidden := range []string{f.base.Root, "PRIVATE-KEY", "PRIVATE-VALUE", "private.invalid", "token", "private error"} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatal("private profile text escaped diagnostic")
		}
	}
	return change
}

func TestWindsurfConfigDiagnosticRefusesUnsafeOrChangedInputsWithoutRebaseline(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "changed-observed", "guard-error", "bad-expected", "bad-observed", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			f, path, original := newWindsurfConfigDiagnostic(t)
			guard, observed := f.protected[path], windsurfConfigDigest(original)
			target := path
			var guardErr error
			status := "read-refused"
			switch kind {
			case "symlink":
				target = filepath.Join(t.TempDir(), "PRIVATE-VALUE")
				projectNativeRuntimeWrite(t, target, original, 0600)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				target = filepath.Join(t.TempDir(), "PRIVATE-VALUE")
				if err := os.Link(path, target); err != nil {
					t.Fatal(err)
				}
			case "changed-observed":
				if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
				status = "observed-digest-changed"
			case "guard-error":
				guardErr, status = errors.New("private error"), "guard-read-refused"
			case "bad-expected":
				guard, status = "PRIVATE-VALUE", "invalid-digest"
			case "bad-observed":
				observed, status = "PRIVATE-VALUE", "invalid-digest"
			case "deadline":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				f.ctx, status = ctx, "deadline"
			}
			unchanged := windsurfConfigDiagnosticUnchanged(t, path, target)
			f.captureWindsurfConfigChange(path, guard, observed, guardErr)
			windsurfConfigDiagnosticResult(t, f, path, f.protected[path], status)
			unchanged()
		})
	}
}

func windsurfConfigDiagnosticUnchanged(t *testing.T, path, target string) func() {
	t.Helper()
	before := map[string]os.FileInfo{}
	for _, name := range []string{path, target} {
		info, err := os.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		before[name] = info
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		for name, info := range before {
			after, err := os.Lstat(name)
			if err != nil || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) {
				t.Fatal("diagnostic changed original entry or outside target identity")
			}
		}
		got, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatal("diagnostic changed target contents")
		}
	}
}

func TestWindsurfConfigDiagnosticRequiresExactPostNativeOriginalPath(t *testing.T) {
	for _, kind := range []string{"client", "path", "nil-fixture", "unused", "missing-command", "missing-stdout", "missing-stderr"} {
		t.Run(kind, func(t *testing.T) {
			f, path, original := newWindsurfConfigDiagnostic(t)
			switch kind {
			case "client":
				f.spec.client = "qoder"
			case "path":
				path += ".foreign"
			case "nil-fixture":
				f.base = nil
			case "unused":
				f.used["native-attempt"] = false
			case "missing-command":
				delete(f.files, "native-attempt.command.json")
			case "missing-stdout":
				delete(f.files, "native-attempt.stdout.log")
			case "missing-stderr":
				delete(f.files, "native-attempt.stderr.log")
			}
			f.captureWindsurfConfigChange(path, windsurfConfigDigest(original), windsurfConfigDigest(original), nil)
			if _, exists := f.evidence["windsurf_protected_config_change"]; exists {
				t.Fatal("unrelated or incomplete native capture admitted diagnostic")
			}
		})
	}
}
