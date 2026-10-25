package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeTrustedProfileRequiresExactAuthoredTrust(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project-A")
	key, _ := json.Marshal(filepath.ToSlash(project))
	valid := `{"projects":{` + string(key) + `:{"hasTrustDialogAccepted":true}},"machineID":"native-initialization"}`
	if err := validateClaudeTrustedProfile([]byte(valid), project); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"projects":null}`, `{"projects":{}}`, valid + `{}`,
		`{"projects":{` + string(key) + `:{"hasTrustDialogAccepted":false}}}`,
		`{"projects":{` + string(key) + `:{"hasTrustDialogAccepted":"true"}}}`,
		`{"projects":{` + string(key) + `:{"hasTrustDialogAccepted":true,"extra":1}}}`,
		`{"projects":{` + string(key) + `:{"hasTrustDialogAccepted":true,"hasTrustDialogAccepted":false}}}`,
	} {
		if err := validateClaudeTrustedProfile([]byte(body), project); err == nil {
			t.Fatalf("changed/malformed authored trust accepted: %s", body)
		}
	}
	if err := validateClaudeTrustedProfile([]byte(valid), project, filepath.Join(filepath.Dir(project), "project-B")); err == nil {
		t.Fatal("missing second project trust was accepted")
	}
}

func TestClaudeTrustedEmptyControlRejectsSuppressionAndManagedIdentity(t *testing.T) {
	if err := validateClaudeTrustedListing([]byte(`[]`), "/A", "", false); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		`null`, `{}`, `[] []`, `[false]`, `[{"id":"x","id":"y"}]`,
		`[{"id":"(suppressed)@skills-dir"}]`, `[{"id":"` + claudeCommandsPluginID + `"}]`,
	} {
		if err := validateClaudeTrustedListing([]byte(body), "/A", "", false); err == nil {
			t.Fatalf("invalid empty native control accepted: %s", body)
		}
	}
}

func TestClaudeTrustedProfileReadIsBoundedAndOrdinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude.json")
	body := bytes.Repeat([]byte(" "), claudeDiagnosticLimit)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readClaudeTrustedProfile(path)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatal("exact profile byte boundary changed")
	}
	if err := os.WriteFile(path, append(body, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readClaudeTrustedProfile(path); err == nil {
		t.Fatal("profile byte limit was not enforced while reading")
	}
	if _, err := readClaudeTrustedProfile(filepath.Dir(path)); err == nil {
		t.Fatal("directory was admitted as profile")
	}
}
