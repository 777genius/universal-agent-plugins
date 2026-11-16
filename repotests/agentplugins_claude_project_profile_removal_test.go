package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Only the two trusted project removal cases use this boundary. Installation,
// repeat and unrelated operations retain requireForeign's whole-file guard.
func claudeTrustedRemoveOwnedPlugin(t *testing.T, f *claudeProjectNativeFixture, before []byte, managed, label string) {
	t.Helper()
	nativeRequireContained(t, f.project, managed)
	if filepath.Dir(managed) != filepath.Join(f.project, ".claude", "skills") {
		t.Fatal("trusted removal does not identify the installed project package")
	}
	f.requireForeign(t)
	if label == "managed-mcp-remove" {
		claudeTrustedRecordRemoveBaseline(t, f)
	}
	result, err := f.run(t, label, "remove", commandsPluginName, "--target", "claude")
	if label == "managed-mcp-remove" {
		claudeTrustedRecordProfile(t, f, "trusted-foreign-mcp-remove-after-profile.json")
	}
	if err != nil || result["result"] != "success" {
		t.Fatalf("trusted project removal failed: %v; %v", err, result)
	}
	outcome, err := commandsTargetOutcome(result)
	if err != nil {
		t.Fatal(err)
	}
	deactivation, _ := outcome["deactivation"].(map[string]any)
	if outcome["client_id"] != "claude" || outcome["plugin"] != commandsPluginName || deactivation["external_removal_complete"] != true {
		t.Fatal("public removal did not confirm the selected native registration was removed")
	}
	for _, path := range []string{managed, claudeProjectMarketplaceCache(f, managed)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("owned project package or exact current cache survived removal", err)
		}
	}
	claudeRequireNativeRemovedProject(t, f, managed)
	profilePath := filepath.Join(f.base.ConfigDir, ".claude.json")
	after := claudeTrustedRecordProfile(t, f, "trusted-mcp-profile-after-remove.json")
	if fmt.Sprintf("%x", sha256.Sum256(before)) != f.foreign[profilePath] {
		t.Fatal("owned usage comparison lost its original pre-remove baseline")
	}
	if err := validateClaudeOwnedUsageRemoval(before, after, claudeProjectMarketplaceID(f.project, managed)); err != nil {
		t.Fatal(err)
	}
	// Recheck the original scoped baseline after both native absence reads.
	// The profile stays in the original hash map; no replacement is adopted.
	f.requireForeign(t)
	nativeJSON(t, filepath.Join(f.base.Root, "trusted-owned-usage-removal.json"), map[string]any{
		"owned_plugin_id": claudeProjectMarketplaceID(f.project, managed), "native_absence_in_A_and_B": true,
		"before_sha256": fmt.Sprintf("%x", sha256.Sum256(before)), "after_sha256": fmt.Sprintf("%x", sha256.Sum256(after)),
		"owned_usage_removed": !bytes.Equal(before, after), "foreign_metadata_unchanged": true, "baseline_rewritten": false,
	})
}

func claudeRequireNativeRemovedProject(t *testing.T, f *claudeProjectNativeFixture, managed string) {
	t.Helper()
	for _, selected := range []struct{ suffix, project string }{{"A", f.project}, {"B", f.otherProject}} {
		capture := captureClaudeDiagnostic(f.client, selected.project, f.base.env(filepath.Dir(f.client)), []string{"plugin", "list", "--json"})
		prefix := "trusted-remove-native-" + selected.suffix
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stdout.log"), capture.Stdout, 0600)
		nativeWrite(t, filepath.Join(f.base.Root, prefix+".stderr.log"), capture.Stderr, 0600)
		if capture.Exit != 0 || capture.Error != "" || capture.Deadline || capture.StdoutLimit || capture.StderrLimit {
			t.Fatal("native removal absence could not be observed")
		}
		if err := validateClaudeMarketplaceListing(capture.Stdout, f, managed, false, false); err != nil {
			t.Fatal(err)
		}
		f.requireForeign(t)
	}
}

func validateClaudeOwnedUsageEntry(body []byte) error {
	entry, err := claudeProfileDiagnosticObject(body)
	if err != nil || len(entry) != 3 {
		return fmt.Errorf("owned plugin usage is not the captured native metadata object")
	}
	for _, key := range []string{"usageCount", "lastUsedAt", "lastUsedNumStartups"} {
		if _, err := strconv.ParseUint(string(entry[key]), 10, 53); err != nil {
			return fmt.Errorf("owned plugin usage has invalid %s", key)
		}
	}
	return nil
}

func TestClaudeOwnedUsageRemovalPreservesEveryForeignMember(t *testing.T) {
	owned := "owned@exact-project-marketplace"
	before := []byte(`{"projects":{"A":{"hasTrustDialogAccepted":true},"B":{"hasTrustDialogAccepted":false}},"userID":"keep","pluginUsage":{"owned@exact-project-marketplace":{"usageCount":0,"lastUsedAt":123,"lastUsedNumStartups":0},"foreign@other":{"usageCount":8}},"foreign":9007199254740993}`)
	removed := []byte(`{"projects":{"A":{"hasTrustDialogAccepted":true},"B":{"hasTrustDialogAccepted":false}},"userID":"keep","pluginUsage":{"foreign@other":{"usageCount":8}},"foreign":9007199254740993}`)
	for _, after := range [][]byte{before, removed} {
		if err := validateClaudeOwnedUsageRemoval(before, after, owned); err != nil {
			t.Fatal("qualified exact native variant refused", err)
		}
	}
	for name, after := range map[string][]byte{
		"foreign usage removed":  bytes.Replace(removed, []byte(`"foreign@other":{"usageCount":8}`), nil, 1),
		"foreign usage changed":  bytes.Replace(removed, []byte(`"usageCount":8`), []byte(`"usageCount":9`), 1),
		"owned value changed":    bytes.Replace(before, []byte(`"usageCount":0`), []byte(`"usageCount":1`), 1),
		"trust changed":          bytes.Replace(removed, []byte(`"hasTrustDialogAccepted":true`), []byte(`"hasTrustDialogAccepted":false`), 1),
		"other project changed":  bytes.Replace(removed, []byte(`"hasTrustDialogAccepted":false`), []byte(`"hasTrustDialogAccepted":true`), 1),
		"foreign number changed": bytes.Replace(removed, []byte(`9007199254740993`), []byte(`9007199254740992`), 1),
		"usage object removed":   []byte(`{"projects":{"A":{"hasTrustDialogAccepted":true},"B":{"hasTrustDialogAccepted":false}},"userID":"keep","foreign":9007199254740993}`),
		"usage null":             bytes.Replace(removed, []byte(`{"foreign@other":{"usageCount":8}}`), []byte(`null`), 1),
		"duplicate key":          bytes.Replace(removed, []byte(`"userID":"keep"`), []byte(`"userID":"keep","userID":"keep"`), 1),
		"nonobject":              []byte(`null`),
		"trailing JSON":          append(append([]byte(nil), removed...), []byte(`{}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateClaudeOwnedUsageRemoval(before, after, owned); err == nil {
				t.Fatal("unowned profile change accepted")
			}
		})
	}
	for _, unknown := range []string{"", "foreign@other", "owned@other-project-marketplace"} {
		if err := validateClaudeOwnedUsageRemoval(before, removed, unknown); err == nil {
			t.Fatalf("unbound removal identity accepted: %q", unknown)
		}
	}
	for _, malformed := range [][]byte{
		bytes.Replace(before, []byte(`"usageCount":0`), []byte(`"usageCount":-1`), 1),
		bytes.Replace(before, []byte(`"usageCount":0`), []byte(`"usageCount":null`), 1),
		bytes.Replace(before, []byte(`"usageCount":0`), []byte(`"usageCount":9007199254740992`), 1),
		bytes.Replace(before, []byte(`"usageCount":0,`), nil, 1),
	} {
		if err := validateClaudeOwnedUsageRemoval(malformed, removed, owned); err == nil {
			t.Fatal("deleting malformed/unobserved native usage was accepted")
		}
	}
}
