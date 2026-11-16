package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestClaudeForeignSnapshotDiagnosticPreservesHashesAndBounds(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".claude", ".claude.json")
	nativeWrite(t, path, []byte(`{"foreign":"original"}`), 0600)
	before := nativeSHA(t, path)
	nativeWrite(t, path, []byte(`{"foreign":"changed"}`), 0600)
	after := nativeSHA(t, path)
	message := claudeForeignSnapshotChange(root, path, before, after)
	for _, expected := range []string{`path=".claude/.claude.json"`, "before_sha256=" + before, "after_sha256=" + after} {
		if !strings.Contains(message, expected) {
			t.Fatalf("missing exact file/hash diagnostic %q: %s", expected, message)
		}
	}
	if before == after || strings.Contains(message, "original") || strings.Contains(message, "changed\"") {
		t.Fatal("diagnostic exposed file contents or lost the actual byte change")
	}
	unsafePath := filepath.Join(root, "line\n\x1b"+strings.Repeat("\U0010ffff", 1024))
	message = claudeForeignSnapshotChange(root, unsafePath, before, after)
	if len(message) > 3072 || strings.ContainsAny(message, "\n\r\x1b") || !strings.Contains(message, "\\n\\x1b") {
		t.Fatalf("path diagnostic was not bounded and escaped: bytes=%d", len(message))
	}
	if !strings.Contains(message, "before_sha256="+before) || !strings.Contains(message, "after_sha256="+after) {
		t.Fatal("bounded path diagnostic truncated the exact hashes")
	}
}

func TestClaudeRemovalProfileDiagnosticsReportOnlyChangedFieldHashes(t *testing.T) {
	before := []byte(`{"projects":{"/A":{"hasTrustDialogAccepted":true,"lastContext":"hidden-before"},"/B":{"hasTrustDialogAccepted":true}},"pluginUsage":{"fixture":1},"foreign":"unchanged"}`)
	after := []byte(`{"projects":{"/A":{"hasTrustDialogAccepted":true,"lastContext":"hidden-after"},"/B":{"hasTrustDialogAccepted":true}},"pluginUsage":{"fixture":2},"foreign":"unchanged"}`)
	lines, err := claudeRemovalProfileChanges(before, after, "/A", "/B")
	if err != nil || len(lines) != 3 {
		t.Fatalf("wrong scoped diagnostic: %v, %v", lines, err)
	}
	joined := strings.Join(lines, "\n")
	for _, expected := range []string{`scope="profile" key="pluginUsage"`, `scope="profile" key="projects"`, `scope="project /A" key="lastContext"`,
		fmt.Sprintf("before_sha256=%x", sha256.Sum256([]byte(`"hidden-before"`))), fmt.Sprintf("after_sha256=%x", sha256.Sum256([]byte(`"hidden-after"`)))} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing scoped changed-key/hash %q: %s", expected, joined)
		}
	}
	for _, forbidden := range []string{"hidden-before", "hidden-after", `key="foreign"`, `scope="project /B"`} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("profile value or unchanged scope leaked: %q", forbidden)
		}
	}
}

func TestClaudeRemovalProfileDiagnosticsBoundOutputAndRejectMalformedJSON(t *testing.T) {
	valid := []byte(`{"projects":{"/A":{},"/B":{}}}`)
	for _, body := range []string{`[]`, `{"projects":{},"projects":{}}`, `{"projects":{"/A":null}}`, string(valid) + `{}`} {
		if _, err := claudeRemovalProfileChanges(valid, []byte(body), "/A", "/B"); err == nil {
			t.Fatalf("malformed diagnostic object accepted: %s", body)
		}
	}
	if _, err := claudeProfileDiagnosticObject(bytes.Repeat([]byte(" "), claudeDiagnosticLimit+1)); err == nil {
		t.Fatal("profile diagnostic byte bound was removed")
	}
	changed := map[string]json.RawMessage{}
	for index := range 32 {
		key := fmt.Sprintf("%03d\n\x1b%s", index, strings.Repeat("\U0010ffff", 256))
		changed[key] = json.RawMessage(`"hidden-value"`)
	}
	lines := claudeProfileChangedFields(strings.Repeat("\U0010ffff", 256), nil, changed)
	if len(lines) != 17 || !strings.Contains(lines[16], "additional changed keys omitted") {
		t.Fatalf("changed-key count was not bounded: %d", len(lines))
	}
	for _, line := range lines {
		if len(line) > 3072 || strings.ContainsAny(line, "\n\r\x1b") || strings.Contains(line, "hidden-value") {
			t.Fatal("changed-key diagnostic was not bounded, escaped and value-free")
		}
	}
}
