package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeUntrustedProfileGuardSurvivesAbsentInitialProfile(t *testing.T) {
	f := &claudeProjectNativeFixture{base: &claudeNativeFixture{ConfigDir: t.TempDir()}, foreign: map[string]string{}, untrustedProfile: true}
	if err := f.checkUntrustedProfile(); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(f.base.ConfigDir, ".claude.json")
	for _, test := range []struct {
		name, body string
		invalid    bool
	}{
		{"native metadata", `{"numStartups":1,"projects":{"A":{"hasTrustDialogAccepted":false}}}`, false},
		{"trust created later", `{"projects":{"A":{"hasTrustDialogAccepted":true}}}`, true},
		{"other project trusted", `{"projects":{"A":{},"B":{"hasTrustDialogAccepted":true}}}`, true},
		{"duplicate trust masks true", `{"projects":{"A":{"hasTrustDialogAccepted":true,"hasTrustDialogAccepted":false}}}`, true},
		{"duplicate projects masks trust", `{"projects":{"A":{"hasTrustDialogAccepted":true}},"projects":{}}`, true},
		{"malformed trust", `{"projects":{"A":{"hasTrustDialogAccepted":"false"}}}`, true},
		{"malformed project", `{"projects":{"A":null}}`, true},
		{"malformed projects", `{"projects":[]}`, true},
		{"trailing JSON", `{"projects":{}} {"projects":{}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(profile, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := f.checkUntrustedProfile(); (err != nil) != test.invalid {
				t.Fatalf("untrusted profile qualification error=%v, invalid=%v", err, test.invalid)
			}
			if len(f.foreign) != 0 {
				t.Fatal("later native profile silently became a new foreign baseline")
			}
		})
	}
}

func TestClaudeUntrustedProfileGuardPreservesExistingBaseline(t *testing.T) {
	f := &claudeProjectNativeFixture{base: &claudeNativeFixture{ConfigDir: t.TempDir()}, foreign: map[string]string{}, untrustedProfile: true}
	profile := filepath.Join(f.base.ConfigDir, ".claude.json")
	nativeWrite(t, profile, []byte(`{"projects":{"A":{"hasTrustDialogAccepted":false}},"foreign":"original"}`), 0600)
	f.foreign[profile] = nativeSHA(t, profile)
	original := f.foreign[profile]
	nativeWrite(t, profile, []byte(`{"projects":{"A":{"hasTrustDialogAccepted":false}},"foreign":"changed"}`), 0600)
	if err := f.checkUntrustedProfile(); err != nil {
		t.Fatal(err)
	}
	if f.foreign[profile] != original || f.foreign[profile] == nativeSHA(t, profile) {
		t.Fatal("untrusted-profile check waived the original whole-byte foreign baseline")
	}
}

func TestClaudeUntrustedNativeProfileAllowsOnlyUsageChanges(t *testing.T) {
	const original = `{"userID":"native-generated","projects":{"A":{"hasTrustDialogAccepted":false}}}`
	f := &claudeProjectNativeFixture{base: &claudeNativeFixture{ConfigDir: t.TempDir()}, foreign: map[string]string{},
		untrustedProfile: true, untrustedProfileBaseline: []byte(original)}
	profile := filepath.Join(f.base.ConfigDir, ".claude.json")
	for _, test := range []struct {
		name, body string
		invalid    bool
	}{
		{"unchanged", original, false},
		{"native usage", `{"pluginUsage":{"fixture@marketplace":{"usageCount":0,"lastUsedAt":123,"lastUsedNumStartups":0}},"projects":{"A":{"hasTrustDialogAccepted":false}},"userID":"native-generated"}`, false},
		{"other native field", `{"userID":"changed","projects":{"A":{"hasTrustDialogAccepted":false}},"pluginUsage":{}}`, true},
		{"foreign field added", `{"userID":"native-generated","projects":{"A":{"hasTrustDialogAccepted":false}},"foreign":"added","pluginUsage":{}}`, true},
		{"trust with usage", `{"userID":"native-generated","projects":{"A":{"hasTrustDialogAccepted":true}},"pluginUsage":{"fixture@marketplace":{"usageCount":0}}}`, true},
		{"malformed usage", `{"userID":"native-generated","projects":{"A":{"hasTrustDialogAccepted":false}},"pluginUsage":[]}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			nativeWrite(t, profile, []byte(test.body), 0600)
			if err := f.checkUntrustedProfile(); (err != nil) != test.invalid {
				t.Fatalf("native profile preservation error=%v, invalid=%v", err, test.invalid)
			}
			if string(f.untrustedProfileBaseline) != original || len(f.foreign) != 0 {
				t.Fatal("native observation changed the original profile baseline")
			}
		})
	}
	if err := os.Remove(profile); err != nil {
		t.Fatal(err)
	}
	if err := f.checkUntrustedProfile(); err == nil {
		t.Fatal("native baseline profile disappeared without refusal")
	}
}

func TestClaudeNativeProfileAllowsOnlyExactSelectedTrustGrant(t *testing.T) {
	for _, test := range []struct {
		name, before, after string
		invalid             bool
	}{
		{"existing false", `{"projects":{"/A":{"hasTrustDialogAccepted":false},"/B":{"hasTrustDialogAccepted":false}},"foreign":1}`, `{"projects":{"/A":{"hasTrustDialogAccepted":true},"/B":{"hasTrustDialogAccepted":false}},"foreign":1}`, false},
		{"new entry", `{"userID":"native"}`, `{"userID":"native","projects":{"/A":{"hasTrustDialogAccepted":true}}}`, false},
		{"existing metadata", `{"projects":{"/A":{"metadata":1}}}`, `{"projects":{"/A":{"metadata":1,"hasTrustDialogAccepted":true}}}`, false},
		{"wrong root", `{}`, `{"projects":{"/A-other":{"hasTrustDialogAccepted":true}}}`, true},
		{"other project", `{"projects":{"/B":{"hasTrustDialogAccepted":false}}}`, `{"projects":{"/A":{"hasTrustDialogAccepted":true},"/B":{"hasTrustDialogAccepted":true}}}`, true},
		{"extra entry", `{}`, `{"projects":{"/A":{"hasTrustDialogAccepted":true},"/B":{}}}`, true},
		{"extra selected field", `{}`, `{"projects":{"/A":{"hasTrustDialogAccepted":true,"extra":1}}}`, true},
		{"foreign mutation", `{"foreign":1}`, `{"foreign":2,"projects":{"/A":{"hasTrustDialogAccepted":true}}}`, true},
		{"malformed trust", `{}`, `{"projects":{"/A":{"hasTrustDialogAccepted":"true"}}}`, true},
		{"duplicate trust", `{}`, `{"projects":{"/A":{"hasTrustDialogAccepted":false,"hasTrustDialogAccepted":true}}}`, true},
		{"revoked existing false", `{"projects":{"/A":{"hasTrustDialogAccepted":false}}}`, `{"projects":{"/A":{}}}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := &claudeProjectNativeFixture{base: &claudeNativeFixture{ConfigDir: t.TempDir()}, project: "/A", untrustedProfile: true, untrustedProfileBaseline: []byte(test.before)}
			nativeWrite(t, filepath.Join(f.base.ConfigDir, ".claude.json"), []byte(test.after), 0600)
			if err := f.checkUntrustedProfile(); (err != nil) != test.invalid {
				t.Fatalf("exact selected trust validation error=%v, invalid=%v", err, test.invalid)
			}
			if string(f.untrustedProfileBaseline) != test.before {
				t.Fatal("original native baseline changed")
			}
		})
	}
}
