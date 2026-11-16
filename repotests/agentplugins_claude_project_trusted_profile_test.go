package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Native first-start metadata belongs to fixture setup. There is exactly one
// baseline, before public UAP; every later command keeps the whole-byte guard.
func claudeTrustedInitializeProfile(t *testing.T, f *claudeProjectNativeFixture, authored []byte) {
	t.Helper()
	// Listing alone leaves userID uninitialized. The native SDK creates it
	// without a user/model request, before the one immutable profile baseline.
	claudeProjectInitializedSkills(t, f, "trusted-prime", nil, false)
	claudeTrustedProjectObservation(t, f, "trusted-prime", "", false)
	body := claudeTrustedRecordProfile(t, f, "trust-profile-initialized.json")
	path := filepath.Join(f.base.ConfigDir, ".claude.json")
	f.foreign[path] = fmt.Sprintf("%x", sha256.Sum256(body))
	var before, after map[string]any
	if err := json.Unmarshal(authored, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &after); err != nil {
		t.Fatal(err)
	}
	changed := []string{}
	for key, value := range after {
		if !reflect.DeepEqual(before[key], value) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	nativeJSON(t, filepath.Join(f.base.Root, "trust-profile-initialization.json"), map[string]any{
		"authored_sha256": fmt.Sprintf("%x", sha256.Sum256(authored)), "initialized_sha256": fmt.Sprintf("%x", sha256.Sum256(body)),
		"native_changed_top_level_keys": changed, "baseline_captured_before_public_uap": true,
		"baseline_rewritten": false, "baseline_capture_count": 1,
	})
	claudeTrustedProjectObservation(t, f, "trusted-stable", "", false)
}

func claudeTrustedRecordProfile(t *testing.T, f *claudeProjectNativeFixture, name string) []byte {
	t.Helper()
	body, err := readClaudeTrustedProfile(filepath.Join(f.base.ConfigDir, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, filepath.Join(f.base.Root, name), body, 0600)
	return body
}

func claudeTrustedRunScriptedProfile(t *testing.T, f *claudeProjectNativeFixture, project string, wantCall bool) {
	t.Helper()
	before, after := "trusted-foreign-mcp-native-A-before-profile.json", "trusted-foreign-mcp-native-A-after-profile.json"
	if project == f.otherProject {
		before, after = "trusted-foreign-mcp-native-B-before-profile.json", "trusted-foreign-mcp-native-B-after-profile.json"
	} else if project != f.project {
		t.Fatal("trusted profile capture requires the original project A or B")
	}
	claudeTrustedRecordProfile(t, f, before)
	claudeProjectRunScripted(t, f, project, wantCall)
	claudeTrustedRecordProfile(t, f, after)
}

func claudeTrustedRecordRemoveBaseline(t *testing.T, f *claudeProjectNativeFixture) {
	t.Helper()
	body := claudeTrustedRecordProfile(t, f, "trusted-foreign-mcp-remove-before-profile.json")
	want := f.foreign[filepath.Join(f.base.ConfigDir, ".claude.json")]
	if fmt.Sprintf("%x", sha256.Sum256(body)) != want {
		t.Fatal("trusted foreign profile differs from original pre-remove baseline")
	}
}

func readClaudeTrustedProfile(path string) (body []byte, readErr error) {
	named, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !named.Mode().IsRegular() {
		return nil, fmt.Errorf("trusted profile is not an ordinary regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { readErr = errors.Join(readErr, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(named, opened) {
		return nil, fmt.Errorf("trusted profile changed while opening")
	}
	body, err = io.ReadAll(io.LimitReader(file, claudeDiagnosticLimit+1))
	if err != nil {
		return nil, err
	}
	if len(body) > claudeDiagnosticLimit {
		return nil, fmt.Errorf("trusted profile exceeds fixed byte limit")
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return nil, fmt.Errorf("trusted profile changed while reading")
	}
	return body, nil
}

func validateClaudeTrustedProfile(body []byte, projects ...string) error {
	value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
	if err != nil || !json.Valid(body) {
		return fmt.Errorf("trusted profile is not unique whole JSON")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("trusted profile is not an object")
	}
	entries, ok := object["projects"].(map[string]any)
	if !ok {
		return fmt.Errorf("trusted profile projects are missing or malformed")
	}
	for _, project := range projects {
		entry, ok := entries[filepath.ToSlash(project)].(map[string]any)
		if !ok || len(entry) != 1 || entry["hasTrustDialogAccepted"] != true {
			return fmt.Errorf("authored project trust entry changed")
		}
	}
	return nil
}

func claudeTrustedRemoveWithProfileDiagnostic(t *testing.T, f *claudeProjectNativeFixture, c commandsNativeClient, before []byte, physical string) {
	t.Helper()
	nativeWrite(t, filepath.Join(f.base.Root, "trusted-mcp-profile-before-remove.json"), before, 0600)
	wasFailed := t.Failed()
	defer func() {
		if !wasFailed {
			claudeTrustedRemovalProfileDiagnostic(t, f, before)
		}
	}()
	removal, err := claudeTrustedRemovalFixture(f, before, physical)
	if err != nil {
		t.Fatal(err)
	}
	// Bind only this fixed removal to the scoped copy; f.run keeps its original
	// pre-remove snapshot, captured command, error, and foreign-file checks.
	c.run = removal.run
	c.succeed(t, "trusted-foreign-mcp-remove", "remove", commandsPluginName)
	removal.requireForeign(t)
}

func claudeTrustedRemovalProfileDiagnostic(t *testing.T, f *claudeProjectNativeFixture, before []byte) {
	t.Helper()
	if !t.Failed() {
		return
	}
	path := filepath.Join(f.base.ConfigDir, ".claude.json")
	after, err := readClaudeTrustedProfile(path)
	if err != nil {
		t.Logf("Claude remove profile diagnostic unavailable: %.256q", err.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(f.base.Root, "trusted-mcp-profile-after-remove.json"), after, 0600); err != nil {
		t.Logf("Claude remove profile diagnostic artifact unavailable: %.256q", err.Error())
	}
	beforeHash := fmt.Sprintf("%x", sha256.Sum256(before))
	t.Logf("Claude remove profile failure: baseline_sha256=%s before_read_sha256=%s after_sha256=%x", f.foreign[path], beforeHash, sha256.Sum256(after))
	if beforeHash != f.foreign[path] {
		t.Log("Claude pre-remove profile read differs from the strict baseline; field changes cannot be attributed to removal")
		return
	}
	lines, err := claudeRemovalProfileChanges(before, after, f.project, f.otherProject)
	for _, line := range lines {
		t.Log(line)
	}
	if err != nil {
		t.Logf("Claude remove profile field diagnostic unavailable: %.256q", err.Error())
	}
}

func claudeRemovalProfileChanges(before, after []byte, project, otherProject string) ([]string, error) {
	left, err := claudeProfileDiagnosticObject(before)
	if err != nil {
		return nil, err
	}
	right, err := claudeProfileDiagnosticObject(after)
	if err != nil {
		return nil, err
	}
	lines := claudeProfileChangedFields("profile", left, right)
	leftProjects, err := claudeProfileDiagnosticMember(left, "projects")
	if err != nil {
		return lines, err
	}
	rightProjects, err := claudeProfileDiagnosticMember(right, "projects")
	if err != nil {
		return lines, err
	}
	for _, path := range []string{project, otherProject} {
		key := filepath.ToSlash(path)
		leftEntry, err := claudeProfileDiagnosticMember(leftProjects, key)
		if err != nil {
			return lines, err
		}
		rightEntry, err := claudeProfileDiagnosticMember(rightProjects, key)
		if err != nil {
			return lines, err
		}
		lines = append(lines, claudeProfileChangedFields("project "+key, leftEntry, rightEntry)...)
	}
	return lines, nil
}

func claudeProfileDiagnosticObject(body []byte) (map[string]json.RawMessage, error) {
	if len(body) > claudeDiagnosticLimit || !json.Valid(body) {
		return nil, fmt.Errorf("profile diagnostic JSON is invalid or exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if _, err := shared.DecodeUniqueJSONValue(decoder); err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return nil, fmt.Errorf("profile diagnostic member is not an object")
	}
	return object, nil
}

func claudeProfileDiagnosticMember(object map[string]json.RawMessage, key string) (map[string]json.RawMessage, error) {
	body, exists := object[key]
	if !exists {
		return map[string]json.RawMessage{}, nil
	}
	return claudeProfileDiagnosticObject(body)
}

func claudeProfileChangedFields(scope string, before, after map[string]json.RawMessage) []string {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	lines := []string{}
	for _, key := range ordered {
		left, had := before[key]
		right, has := after[key]
		if had == has && bytes.Equal(left, right) {
			continue
		}
		if len(lines) == 16 {
			return append(lines, fmt.Sprintf("Claude profile scope=%.128q: additional changed keys omitted", scope))
		}
		lines = append(lines, fmt.Sprintf("Claude profile scope=%.128q key=%.128q before_present=%t before_sha256=%x after_present=%t after_sha256=%x", scope, key, had, sha256.Sum256(left), has, sha256.Sum256(right)))
	}
	return lines
}

// Only the fixed successful-install fixture may authorize removal of its exact
// native usage key. The original profile bytes and foreign hash map stay intact.
type claudeOwnedUsageRemoval struct {
	path, pluginID string
	before         []byte
}

func claudeTrustedRemovalFixture(f *claudeProjectNativeFixture, before []byte, physical string) (*claudeProjectNativeFixture, error) {
	path := filepath.Join(f.base.ConfigDir, ".claude.json")
	if physical == "" || filepath.Base(physical) != physical || !filepath.IsAbs(f.project) || f.ownedUsageRemoval != nil {
		return nil, fmt.Errorf("owned usage removal identity is incomplete")
	}
	if fmt.Sprintf("%x", sha256.Sum256(before)) != f.foreign[path] {
		return nil, fmt.Errorf("owned usage removal lost the original profile baseline")
	}
	pluginID := commandsPluginName + "@" + claude.ProjectMarketplaceName(physical, f.project)
	if err := validateClaudeOwnedUsageRemoval(before, before, pluginID); err != nil {
		return nil, err
	}
	removal := *f
	removal.foreign = maps.Clone(f.foreign)
	removal.ownedUsageRemoval = &claudeOwnedUsageRemoval{path: path, pluginID: pluginID, before: bytes.Clone(before)}
	return &removal, nil
}

func (f *claudeProjectNativeFixture) allowsOwnedUsageRemoval(path, want, got string) bool {
	owned := f.ownedUsageRemoval
	if owned == nil || path != owned.path || fmt.Sprintf("%x", sha256.Sum256(owned.before)) != want {
		return false
	}
	after, err := readClaudeTrustedProfile(path)
	return err == nil && fmt.Sprintf("%x", sha256.Sum256(after)) == got &&
		validateClaudeOwnedUsageRemoval(owned.before, after, owned.pluginID) == nil
}

func validateClaudeOwnedUsageRemoval(before, after []byte, pluginID string) error {
	left, err := claudeProfileDiagnosticObject(before)
	if err != nil {
		return err
	}
	right, err := claudeProfileDiagnosticObject(after)
	if err != nil {
		return err
	}
	for _, object := range []map[string]json.RawMessage{left, right} {
		if _, err := claudeProfileDiagnosticMember(object, "pluginUsage"); err != nil {
			return err
		}
	}
	if pluginID == "" {
		return fmt.Errorf("owned usage removal has no exact plugin identity")
	}
	if bytes.Equal(before, after) {
		return nil
	}
	if err := claudeOwnedUsageForeignFields(left["pluginUsage"], right["pluginUsage"], pluginID); err != nil {
		return err
	}
	leftSpans, err := claudeProfileMemberSpans(before)
	if err != nil {
		return err
	}
	rightSpans, err := claudeProfileMemberSpans(after)
	if err != nil {
		return err
	}
	l, r := leftSpans["pluginUsage"], rightSpans["pluginUsage"]
	if !bytes.Equal(before[:l.value], after[:r.value]) || !bytes.Equal(before[l.end:], after[r.end:]) {
		return fmt.Errorf("owned usage removal changed foreign profile bytes")
	}
	return nil
}

func claudeOwnedUsageForeignFields(before, after []byte, pluginID string) error {
	left, err := claudeProfileDiagnosticObject(before)
	if err != nil {
		return err
	}
	right, err := claudeProfileDiagnosticObject(after)
	if err != nil {
		return err
	}
	owned, exists := left[pluginID]
	if !exists || right[pluginID] != nil {
		return fmt.Errorf("owned usage removal did not delete only the selected entry")
	}
	if _, err := claudeProfileDiagnosticObject(owned); err != nil {
		return err
	}
	l, err := claudeProfileMemberSpans(before)
	if err != nil {
		return err
	}
	r, err := claudeProfileMemberSpans(after)
	if err != nil {
		return err
	}
	delete(l, pluginID)
	if !reflect.DeepEqual(claudeProfileOrderedFields(before, l), claudeProfileOrderedFields(after, r)) {
		return fmt.Errorf("owned usage removal changed foreign usage bytes or order")
	}
	return nil
}

type claudeProfileMemberSpan struct{ field, value, end int }

func claudeProfileOrderedFields(body []byte, spans map[string]claudeProfileMemberSpan) []string {
	keys := make([]string, 0, len(spans))
	for key := range spans {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return spans[keys[i]].field < spans[keys[j]].field })
	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		span := spans[key]
		fields = append(fields, string(body[span.field:span.end]))
	}
	return fields
}

// Offsets come from the standard decoder after the existing bounded unique-JSON
// reader has rejected duplicate, trailing, malformed, and non-object input.
func claudeProfileMemberSpans(body []byte) (map[string]claudeProfileMemberSpan, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	result := map[string]claudeProfileMemberSpan{}
	for decoder.More() {
		start := int(decoder.InputOffset())
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		field := start + bytes.IndexByte(body[start:int(decoder.InputOffset())], '"')
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok || field < start {
			return nil, fmt.Errorf("profile member has no exact field span")
		}
		end := int(decoder.InputOffset())
		result[name] = claudeProfileMemberSpan{field: field, value: end - len(value), end: end}
	}
	return result, nil
}
