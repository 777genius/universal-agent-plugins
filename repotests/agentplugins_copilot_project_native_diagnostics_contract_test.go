//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func copilotProjectConfigFixture(t *testing.T, baseline []byte) *copilotProjectAttempt {
	t.Helper()
	path := filepath.Join(t.TempDir(), "home", ".copilot", "config.json")
	projectNativeRuntimeWrite(t, path, baseline, 0600)
	return &copilotProjectAttempt{ctx: context.Background(), originalConfigPath: path, originalConfigBaseline: bytes.Clone(baseline), protected: map[string]string{path: copilotProjectConfigHash(baseline)}, files: map[string][]byte{}, evidence: map[string]any{}}
}

func copilotProjectWriteConfig(t *testing.T, f *copilotProjectAttempt, body []byte) {
	t.Helper()
	if err := os.WriteFile(f.originalConfigPath, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func copilotProjectRecordedChange(t *testing.T, f *copilotProjectAttempt) copilotProjectConfigChange {
	t.Helper()
	change, ok := f.evidence["original_config_change"].(copilotProjectConfigChange)
	if !ok {
		t.Fatal("missing typed original-config diagnostic")
	}
	if len(copilotProjectContractBody(t, change)) > copilotProjectConfigDiagnosticLimit {
		t.Fatal("diagnostic exceeds its fixed byte bound")
	}
	return change
}

func TestCopilotProjectNativeInstalledSnapshotsUseExistingClosedNames(t *testing.T) {
	root := t.TempDir()
	mcp, skill := filepath.Join(root, ".github", "mcp.json"), filepath.Join(root, ".github", "skills", "uapcopilota", "SKILL.md")
	projectNativeRuntimeWrite(t, mcp, []byte(`{"mcpServers":{}}`), 0600)
	projectNativeRuntimeWrite(t, skill, []byte("installed marker\n"), 0600)
	f := &copilotProjectAttempt{files: map[string][]byte{}}
	f.captureInstalled(t, mcp, skill)
	want := map[string][]byte{"mcp.json.installed": []byte(`{"mcpServers":{}}`), "uapcopilota-SKILL.md.installed": []byte("installed marker\n")}
	if !reflect.DeepEqual(f.files, want) {
		t.Fatalf("actual held snapshot names/bytes differ from the existing frontend contract: %v", f.files)
	}
}

func TestCopilotProjectNativeConfigFormatChangeDoesNotAdoptBaseline(t *testing.T) {
	baseline := []byte("{\"trustedFolders\":[\"/private/A\",\"/private/B\"]}\n")
	f := copilotProjectConfigFixture(t, baseline)
	want := f.protected[f.originalConfigPath]
	if got, err := f.protectedDigest(f.originalConfigPath, want); err != nil || got != want || len(f.evidence) != 0 {
		t.Fatal("unchanged original config produced failure evidence")
	}
	copilotProjectWriteConfig(t, f, []byte("{\n \"trustedFolders\": [\"/private/A\", \"/private/B\"]\n}\n"))
	got, err := f.protectedDigest(f.originalConfigPath, want)
	change := copilotProjectRecordedChange(t, f)
	if err != nil || got == want || change.Status != "captured" || change.SemanticallyEqual == nil || !*change.SemanticallyEqual || change.TrustedFoldersMatch == nil || !*change.TrustedFoldersMatch || len(change.ChangedMembers) != 0 {
		t.Fatalf("format-only change lost its failed digest or exact semantic facts: %+v, %v", change, err)
	}
	if f.protected[f.originalConfigPath] != want || !bytes.Equal(f.originalConfigBaseline, baseline) {
		t.Fatal("post-launch config was adopted as a baseline")
	}
}

func TestCopilotProjectNativeConfigMismatchStillFailsActualGuard(t *testing.T) {
	const marker = "UAP_COPILOT_CONFIG_GUARD_INERT_CHILD"
	if os.Getenv(marker) == "1" {
		f := copilotProjectConfigFixture(t, []byte(`{"trustedFolders":["A","B"]}`))
		copilotProjectWriteConfig(t, f, []byte("{ \"trustedFolders\": [\"A\", \"B\"] }\n"))
		f.check(t)
		return
	}
	// Only this Go contract is executed in the child, never an acquired client.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCopilotProjectNativeConfigMismatchStillFailsActualGuard$", "-test.count=1")
	cmd.Env = append(os.Environ(), marker+"=1")
	output, err := cmd.CombinedOutput()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 1 || !bytes.Contains(output, []byte("original authority changed:")) {
		t.Fatalf("original config mismatch did not fail the actual guard: %v, %.1024s", err, output)
	}
}

func TestCopilotProjectNativeConfigChangeExportsOnlyHashedTypes(t *testing.T) {
	baseline := []byte(`{"trustedFolders":["/private/A","/private/B"],"removed-key-canary":"removed-value-secret"}`)
	f := copilotProjectConfigFixture(t, baseline)
	copilotProjectWriteConfig(t, f, []byte(`{"trustedFolders":["/private/A","/private/B"],"auth-key-canary":"credential-secret","bool-key-canary":true,"number-key-canary":7,"null-key-canary":null,"array-key-canary":["nested-value-secret"],"object-key-canary":{"nested-key-canary":"nested-secret"}}`))
	want := f.protected[f.originalConfigPath]
	got, err := f.protectedDigest(f.originalConfigPath, want)
	change := copilotProjectRecordedChange(t, f)
	if err != nil || got == want || change.Status != "captured" || change.SemanticallyEqual == nil || *change.SemanticallyEqual || change.TrustedFoldersMatch == nil || !*change.TrustedFoldersMatch || len(change.ChangedMembers) != 7 {
		t.Fatalf("structural failure facts differ: %+v, %v", change, err)
	}
	wantTypes := map[string]string{"removed-key-canary": "absent", "auth-key-canary": "string", "bool-key-canary": "boolean", "number-key-canary": "number", "null-key-canary": "null", "array-key-canary": "array", "object-key-canary": "object"}
	for key, kind := range wantTypes {
		found := false
		beforeType := "absent"
		if key == "removed-key-canary" {
			beforeType = "string"
		}
		for _, member := range change.ChangedMembers {
			found = found || (member.KeySHA256 == copilotProjectConfigHash([]byte(key)) && member.BeforeType == beforeType && member.AfterType == kind)
		}
		if !found {
			t.Fatalf("missing hashed type for %s", kind)
		}
	}
	body := copilotProjectContractBody(t, change)
	for _, forbidden := range []string{"/private/", "canary", "secret", "trustedFolders", f.originalConfigPath} {
		if bytes.Contains(body, []byte(forbidden)) {
			t.Fatalf("raw private field or value escaped into the diagnostic: %s", forbidden)
		}
	}
}

func TestCopilotProjectNativeConfigTrustChangeAndHeldCommandEvidence(t *testing.T) {
	for _, captured := range []bool{false, true} {
		f := copilotProjectConfigFixture(t, []byte(`{"trustedFolders":["A","B"]}`))
		before := map[string][]byte{}
		if captured {
			before = map[string][]byte{
				"native-attempt.stdout.log": []byte("held stdout\n"), "native-attempt.stderr.log": {},
				"native-attempt.command.json": copilotProjectContractBody(t, map[string]any{"argv": []string{"inert-never-executed"}, "cwd": "/fixture/project", "exit": 0, "elapsed_ms": 1, "error": false, "context_error": false, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"}),
			}
			for name, body := range before {
				f.keep(t, name, body)
			}
		}
		copilotProjectWriteConfig(t, f, []byte(`{"trustedFolders":["A","outside-private-folder"]}`))
		got, err := f.protectedDigest(f.originalConfigPath, f.protected[f.originalConfigPath])
		change := copilotProjectRecordedChange(t, f)
		if err != nil || got == f.protected[f.originalConfigPath] || change.TrustedFoldersMatch == nil || *change.TrustedFoldersMatch || len(change.ChangedMembers) != 1 || !reflect.DeepEqual(f.files, before) {
			t.Fatal("trust change was accepted or pre/post command evidence was changed")
		}
	}
}

func TestCopilotProjectNativeConfigDiagnosticBoundsAndRefusals(t *testing.T) {
	for _, kind := range []string{"missing", "alias", "oversized", "malformed", "duplicate", "non-object", "changes-32", "changes-33", "deadline"} {
		t.Run(kind, func(t *testing.T) {
			f := copilotProjectConfigFixture(t, []byte(`{"trustedFolders":["A","B"]}`))
			wantStatus := copilotProjectConfigDiagnosticMutation(t, f, kind)
			got, err := f.protectedDigest(f.originalConfigPath, f.protected[f.originalConfigPath])
			if got == f.protected[f.originalConfigPath] && err == nil {
				t.Fatal("unsafe or changed original config was accepted")
			}
			change := copilotProjectRecordedChange(t, f)
			if change.Status != wantStatus || (kind != "changes-32" && (len(change.ChangedMembers) != 0 || change.SemanticallyEqual != nil || change.TrustedFoldersMatch != nil)) {
				t.Fatalf("unsafe diagnostic did not remain bounded/status-only: %+v", change)
			}
			if kind == "changes-32" && len(change.ChangedMembers) != 32 {
				t.Fatal("exact changed-key boundary was not retained")
			}
		})
	}
}

func copilotProjectConfigDiagnosticMutation(t *testing.T, f *copilotProjectAttempt, kind string) string {
	t.Helper()
	switch kind {
	case "missing", "alias":
		if err := os.Remove(f.originalConfigPath); err != nil {
			t.Fatal(err)
		}
		if kind == "alias" {
			if err := os.Symlink("missing-private-target", f.originalConfigPath); err != nil {
				t.Fatal(err)
			}
		}
		return "guard-read-refused"
	case "oversized":
		copilotProjectWriteConfig(t, f, bytes.Repeat([]byte("x"), copilotProjectConfigDiagnosticLimit+1))
		return "read-refused"
	case "malformed":
		copilotProjectWriteConfig(t, f, []byte(`{"secret-canary":`))
		return "invalid-json"
	case "duplicate":
		copilotProjectWriteConfig(t, f, []byte(`{"trustedFolders":[],"trustedFolders":[]}`))
		return "invalid-json"
	case "non-object":
		copilotProjectWriteConfig(t, f, []byte(`[]`))
		return "non-object"
	case "deadline":
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		f.ctx = ctx
		copilotProjectWriteConfig(t, f, []byte(`{}`))
		return "deadline"
	default:
		count := 32
		if kind == "changes-33" {
			count = 33
		}
		var fields strings.Builder
		fields.WriteString(`{"trustedFolders":["A","B"]`)
		for i := range count {
			_, _ = fmt.Fprintf(&fields, `,"private-key-%d":"private-value-secret"`, i)
		}
		fields.WriteByte('}')
		copilotProjectWriteConfig(t, f, []byte(fields.String()))
		if count == 32 {
			return "captured"
		}
		return "change-limit"
	}
}

func TestCopilotProjectNativeConfigDiagnosticCorrelatesExactGuardSnapshot(t *testing.T) {
	baseline, observed, replaced := []byte(`{"trustedFolders":["A","B"]}`), []byte(`{"trustedFolders":["A"]}`), []byte(`{"private-key":"credential-secret"}`)
	change := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), copilotProjectConfigHash(observed), nil, func() ([]byte, error) { return replaced, nil })
	if change.Status != "unstable" || change.ObservedBytes != nil || change.SemanticallyEqual != nil || change.TrustedFoldersMatch != nil || len(change.ChangedMembers) != 0 {
		t.Fatalf("uncorrelated read produced JSON conclusions: %+v", change)
	}
	refused := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), copilotProjectConfigHash(observed), nil, func() ([]byte, error) { return nil, errors.New("private-read-error-secret") })
	if refused.Status != "read-refused" || bytes.Contains(copilotProjectContractBody(t, refused), []byte("private-read-error")) {
		t.Fatal("private read error was reflected")
	}
}
