package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

// Native first-start metadata belongs to fixture setup. There is exactly one
// baseline, before public UAP; every later command keeps the whole-byte guard.
func claudeTrustedInitializeProfile(t *testing.T, f *claudeProjectNativeFixture, authored []byte) {
	t.Helper()
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
