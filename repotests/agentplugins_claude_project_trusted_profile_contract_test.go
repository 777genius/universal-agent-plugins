package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
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

func TestClaudeOwnedUsageRemovalPreservesExactForeignData(t *testing.T) {
	const id = "uap-commands-fixture@agentplugins-6919c38313b8"
	const owned = `"` + id + `":{"usageCount":0,"lastUsedAt":1794794500941,"lastUsedNumStartups":0}`
	const sibling = `"foreign@market": { "usageCount" : 7 }`
	const tail = `,"foreign":{ "large":9007199254740993 },"projects":{"/A":{"hasTrustDialogAccepted":true}}}`
	for _, tc := range []struct{ name, before, after string }{
		{"absent-unchanged", `{"foreign":"exact"}`, `{"foreign":"exact"}`},
		{"owned-unchanged", `{"pluginUsage":{` + owned + `}}`, `{"pluginUsage":{` + owned + `}}`},
		{"observed-owned-singleton", `{"pluginUsage":{` + owned + `}}`, `{"pluginUsage":{}}`},
		{"foreign-sibling", `{"pluginUsage":{` + owned + `,` + sibling + `}` + tail, `{"pluginUsage":{` + sibling + `}` + tail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, after := []byte(tc.before), []byte(tc.after)
			if err := validateClaudeOwnedUsageRemoval(before, after, id); err != nil {
				t.Fatal(err)
			}
			if string(before) != tc.before || string(after) != tc.after {
				t.Fatal("profile validator rewrote its original inputs")
			}
		})
	}
}

func TestClaudeOwnedUsageRemovalRejectsForeignChanges(t *testing.T) {
	const id = "uap-commands-fixture@agentplugins-6919c38313b8"
	const owned = `"` + id + `":{"usageCount":0}`
	const sibling = `"foreign@market": { "usageCount" : 7 }`
	const tail = `,"foreign":{ "large":9007199254740993 },"projects":{"/A":{"hasTrustDialogAccepted":true}}}`
	const before = `{"pluginUsage":{` + owned + `,` + sibling + `}` + tail
	const after = `{"pluginUsage":{` + sibling + `}` + tail
	for _, tc := range []struct{ name, left, right string }{
		{"foreign-name", before, strings.Replace(after, `foreign@market`, `other@market`, 1)},
		{"foreign-removed", before, strings.Replace(after, sibling, "", 1)},
		{"foreign-modified", before, strings.Replace(after, `: 7`, `: 8`, 1)},
		{"foreign-value-reformatted", before, strings.Replace(after, `{ "usageCount" : 7 }`, `{"usageCount":7}`, 1)},
		{"foreign-key-reencoded", before, strings.Replace(after, `foreign@market`, `foreign\u0040market`, 1)},
		{"foreign-added", before, strings.Replace(after, sibling, sibling+`,"extra":{}`, 1)},
		{"foreign-reordered", strings.Replace(before, sibling, sibling+`,"second":{}`, 1), strings.Replace(after, sibling, `"second":{},`+sibling, 1)},
		{"metadata-modified", before, strings.Replace(after, `9007199254740993`, `9007199254740992`, 1)},
		{"trust-modified", before, strings.Replace(after, `true`, `false`, 1)},
		{"outer-bytes-reformatted", before, " " + after},
		{"whole-usage-removed", before, `{` + strings.TrimPrefix(tail, ",")},
		{"owned-modified", before, strings.Replace(before, `"usageCount":0`, `"usageCount":1`, 1)},
		{"owned-scalar", strings.Replace(before, `{"usageCount":0}`, `0`, 1), after},
		{"unowned-before", strings.Replace(before, id, "another@market", 1), after},
		{"duplicate-owned", strings.Replace(before, owned, owned+","+owned, 1), after},
		{"duplicate-profile", before, strings.Replace(after, `"pluginUsage":`, `"pluginUsage":{},"pluginUsage":`, 1)},
		{"null-usage", before, `{"pluginUsage":null}`},
		{"array-usage", before, `{"pluginUsage":[]}`},
		{"trailing-output", before, after + `{}`},
		{"oversize", before, after + strings.Repeat(" ", claudeDiagnosticLimit)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateClaudeOwnedUsageRemoval([]byte(tc.left), []byte(tc.right), id); err == nil {
				t.Fatal("foreign or malformed profile change was accepted")
			}
		})
	}
	if err := validateClaudeOwnedUsageRemoval([]byte(before), []byte(after), ""); err == nil {
		t.Fatal("empty owned identity was accepted")
	}
}

func TestClaudeOwnedUsageRemovalKeepsOriginalControllerAuthority(t *testing.T) {
	f, before, after, physical := claudeOwnedUsageContractFixture(t)
	originalHashes := maps.Clone(f.foreign)
	originalBefore := bytes.Clone(before)
	removal, err := claudeTrustedRemovalFixture(f, before, physical)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.base.ConfigDir, ".claude.json")
	nativeWrite(t, path, after, 0600)
	removal.requireForeign(t)
	got := nativeSHA(t, path)
	if !removal.allowsOwnedUsageRemoval(path, f.foreign[path], got) || f.allowsOwnedUsageRemoval(path, f.foreign[path], got) {
		t.Fatal("owned removal escaped its private controller copy")
	}
	if f.ownedUsageRemoval != nil || !maps.Equal(f.foreign, originalHashes) || !bytes.Equal(before, originalBefore) {
		t.Fatal("controller replaced the original profile baseline")
	}
	before[0] = '['
	if !bytes.Equal(removal.ownedUsageRemoval.before, originalBefore) {
		t.Fatal("controller retained a mutable caller baseline")
	}
	delete(removal.foreign, path)
	if !maps.Equal(f.foreign, originalHashes) {
		t.Fatal("controller reused the original foreign hash map")
	}
}

func TestClaudeOwnedUsageRemovalRequiresExactBaselineAndIdentity(t *testing.T) {
	f, before, after, physical := claudeOwnedUsageContractFixture(t)
	for _, selected := range []string{"", "../foreign"} {
		if _, err := claudeTrustedRemovalFixture(f, before, selected); err == nil {
			t.Fatal("invalid physical identity was accepted")
		}
	}
	if _, err := claudeTrustedRemovalFixture(f, append(bytes.Clone(before), ' '), physical); err == nil {
		t.Fatal("different original profile bytes were adopted")
	}
	removal, err := claudeTrustedRemovalFixture(f, before, physical)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := claudeTrustedRemovalFixture(removal, before, physical); err == nil {
		t.Fatal("nested removal authorization was accepted")
	}
	path := filepath.Join(f.base.ConfigDir, ".claude.json")
	nativeWrite(t, path, after, 0600)
	got := nativeSHA(t, path)
	for _, args := range [][3]string{{path, "wrong-baseline", got}, {path, f.foreign[path], "wrong-observation"}, {path + ".other", f.foreign[path], got}} {
		if removal.allowsOwnedUsageRemoval(args[0], args[1], args[2]) {
			t.Fatal("controller accepted a different original or observed authority")
		}
	}
	foreign, err := claudeTrustedRemovalFixture(f, before, physical+"-foreign")
	if err != nil {
		t.Fatal(err)
	}
	if foreign.allowsOwnedUsageRemoval(path, f.foreign[path], got) {
		t.Fatal("controller allowed deletion under another physical plugin identity")
	}
}

func claudeOwnedUsageContractFixture(t *testing.T) (*claudeProjectNativeFixture, []byte, []byte, string) {
	t.Helper()
	root := t.TempDir()
	f := &claudeProjectNativeFixture{
		base:    &claudeNativeFixture{Root: root, ConfigDir: filepath.Join(root, "config")},
		project: filepath.Join(root, "project-A"), foreign: map[string]string{},
	}
	const physical = "uap-commands-fixture-owned"
	id := commandsPluginName + "@" + claude.ProjectMarketplaceName(physical, f.project)
	before := []byte(fmt.Sprintf(`{"foreign":{"bytes":"retained"},"pluginUsage":{%q:{"usageCount":0}}}`, id))
	after := []byte(`{"foreign":{"bytes":"retained"},"pluginUsage":{}}`)
	f.keep(t, filepath.Join(f.base.ConfigDir, ".claude.json"), string(before))
	f.keep(t, filepath.Join(f.project, ".claude", "settings.json"), `{"enabledPlugins":{"foreign-project-fixture@skills-dir":false}}`)
	f.keep(t, filepath.Join(f.base.ConfigDir, "foreign.json"), `{"foreign":"exact"}`)
	return f, before, after, physical
}
