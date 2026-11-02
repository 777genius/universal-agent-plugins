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
	if change.BaselineJSONValid != nil || change.ObservedJSONValid != nil || change.StrictJSONFailure != "" || change.BaselineJSONReason != "" || change.ObservedJSONReason != "" {
		t.Fatal("JSON refusal fields escaped the invalid-json diagnostic branch")
	}
}

func TestCopilotProjectNativeConfigJSONRefusalsExposeOnlyFiniteFacts(t *testing.T) {
	valid := `{"private-key":"private-value"}`
	syntax := `{"private-key":@}`
	for _, tc := range []struct {
		name, baseline, observed, side, beforeReason, afterReason string
		beforeValid, afterValid                                   bool
	}{
		{"syntax", valid, syntax, "observed", "valid", "syntax", true, false},
		{"duplicate", valid, `{"private-key":"private-value","private-key":null}`, "observed", "valid", "duplicate-key", true, true},
		{"baseline", syntax, valid, "baseline", "syntax", "valid", false, true},
		{"both", syntax, `{"private-key":`, "both", "syntax", "unexpected-eof", false, false},
		{"nonfinite", valid, `{"private-key":1e10000}`, "observed", "valid", "nonfinite-number", true, true},
		{"structure", valid, strings.Repeat("[", 34) + strings.Repeat("]", 34), "observed", "valid", "structure-limit", true, true},
		{"trailing", valid, `{} {}`, "observed", "valid", "trailing-data", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := copilotProjectConfigFixture(t, []byte(tc.baseline))
			want := f.protected[f.originalConfigPath]
			copilotProjectWriteConfig(t, f, []byte(tc.observed))
			got, err := f.protectedDigest(f.originalConfigPath, want)
			change := copilotProjectRecordedChange(t, f)
			if err != nil || got == want || change.Status != "invalid-json" || change.StrictJSONFailure != tc.side || change.BaselineJSONReason != tc.beforeReason || change.ObservedJSONReason != tc.afterReason {
				t.Fatalf("strict refusal or finite category differs: %+v, %v", change, err)
			}
			if change.BaselineJSONValid == nil || *change.BaselineJSONValid != tc.beforeValid || change.ObservedJSONValid == nil || *change.ObservedJSONValid != tc.afterValid {
				t.Fatal("standard JSON syntax facts differ")
			}
			encoded := string(copilotProjectContractBody(t, change))
			if strings.Contains(encoded, "private-key") || strings.Contains(encoded, "private-value") || len(change.ChangedMembers) != 0 || change.SemanticallyEqual != nil || change.TrustedFoldersMatch != nil {
				t.Fatal("private content or unsupported interpretation escaped JSON refusal")
			}
			if f.protected[f.originalConfigPath] != want || !bytes.Equal(f.originalConfigBaseline, []byte(tc.baseline)) {
				t.Fatal("invalid original config was adopted")
			}
			body, err := os.ReadFile(f.originalConfigPath)
			if err != nil || string(body) != tc.observed {
				t.Fatal("read-only diagnostic changed config bytes", err)
			}
		})
	}
	if copilotProjectJSONErrorKind(errors.New("private-key private-value")) != "other" {
		t.Fatal("unrecognized private error was classified dynamically")
	}
}

func TestCopilotProjectNativeConfigMismatchStillFailsActualGuard(t *testing.T) {
	const marker = "UAP_COPILOT_CONFIG_GUARD_INERT_CHILD"
	if mode := os.Getenv(marker); mode == "1" || mode == "prefix" {
		f := copilotProjectConfigFixture(t, []byte(`{"trustedFolders":["A","B"]}`))
		body := "{ \"trustedFolders\": [\"A\", \"B\"] }\n"
		if mode == "prefix" {
			body = "// inert leading comment\n" + body
		}
		copilotProjectWriteConfig(t, f, []byte(body))
		f.check(t)
		return
	}
	// Only this Go contract is executed in the child, never an acquired client.
	for _, mode := range []string{"1", "prefix"} {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCopilotProjectNativeConfigMismatchStillFailsActualGuard$", "-test.count=1")
		cmd.Env = append(os.Environ(), marker+"="+mode)
		output, err := cmd.CombinedOutput()
		cancel()
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 1 || !bytes.Contains(output, []byte("original authority changed:")) {
			t.Fatalf("original config mismatch did not fail the actual guard: %v, %.1024s", err, output)
		}
	}
}

func TestCopilotProjectNativeConfigPrefixMetadataNeverAdoptsOriginal(t *testing.T) {
	baseline := []byte(`{"trustedFolders":["/private/A","/private/B"],"private-key":"private-value"}`)
	const prefix = "// private-comment-canary\n// User settings belong in settings.json\n"
	f := copilotProjectConfigFixture(t, baseline)
	want := f.protected[f.originalConfigPath]
	body := append([]byte(prefix), baseline...)
	copilotProjectWriteConfig(t, f, body)
	got, err := f.protectedDigest(f.originalConfigPath, want)
	change := copilotProjectRecordedChange(t, f)
	p := change.LeadingCommentProbe
	if err != nil || got == want || change.Status != "invalid-json" || change.ObservedJSONReason != "syntax" || p == nil || p.Status != "captured" {
		t.Fatal("comment projection replaced original failure")
	}
	if p.PrefixBytes != 67 || p.CommentLines != 2 || p.PrefixSHA256 != "3a3761e2eaa39ca22d590c1650b43c80031bddaac196b4a940c8c86ad9a00c25" || p.SuffixBytes != len(baseline) || p.SuffixJSONReason != "valid" || p.SemanticallyEqual == nil || !*p.SemanticallyEqual || p.TrustedFoldersMatch == nil || !*p.TrustedFoldersMatch {
		t.Fatal("bounded prefix or exact private semantic comparison differs")
	}
	if change.SemanticallyEqual != nil || change.TrustedFoldersMatch != nil || len(change.ChangedMembers) != 0 || f.protected[f.originalConfigPath] != want || !bytes.Equal(f.originalConfigBaseline, baseline) {
		t.Fatal("diagnostic promoted or renewed original authority")
	}
	encoded := copilotProjectContractBody(t, change)
	for _, private := range []string{"private-comment-canary", "User settings", "settings.json", "private-key", "private-value", "trustedFolders", "/private/", f.originalConfigPath} {
		if bytes.Contains(encoded, []byte(private)) {
			t.Fatal("private comment, path, key or value escaped projection")
		}
	}
	actual, err := os.ReadFile(f.originalConfigPath)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("diagnostic modified the held original config")
	}
}

func TestCopilotProjectNativeConfigPrefixKeepsExactSemanticAndTrustFacts(t *testing.T) {
	baseline := []byte(`{"trustedFolders":["A","B"]}`)
	for _, tc := range []struct {
		body           string
		equal, trusted bool
	}{
		{" \t\r\n// fixture\n \n{ \"trustedFolders\": [\"A\", \"B\"] }\n", true, true},
		{"// fixture\n{\"trustedFolders\":[\"B\",\"A\"]}", false, false},
		{"// fixture\n{\"trustedFolders\":[\"A\",\"B\"],\"private\":\"canary\"}", false, true},
		{"// fixture\n{\"trustedFolders\":null}", false, false},
		{"// fixture\n{}", false, false},
	} {
		change := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), copilotProjectConfigHash([]byte(tc.body)), nil, func() ([]byte, error) { return []byte(tc.body), nil })
		p := change.LeadingCommentProbe
		if change.Status != "invalid-json" || p == nil || p.Status != "captured" || p.SemanticallyEqual == nil || *p.SemanticallyEqual != tc.equal || p.TrustedFoldersMatch == nil || *p.TrustedFoldersMatch != tc.trusted {
			t.Fatal("prefix projection changed exact object/trust semantics")
		}
	}
}

func TestCopilotProjectNativeConfigPrefixRefusesOtherSyntaxAndBounds(t *testing.T) {
	baseline := []byte(`{"trustedFolders":["A","B"]}`)
	for index, tc := range []struct{ body, status, reason string }{
		{"/* private */{}", "absent", ""}, {"\xef\xbb\xbf{}", "absent", ""},
		{"{\"x\":1,//private\n\"y\":2}", "absent", ""},
		{"// no terminating LF{}", "prefix-refused", ""}, {"// private\n[]", "prefix-refused", ""},
		{"// private\x00\n{}", "prefix-refused", ""}, {"// private\x7f\n{}", "prefix-refused", ""},
		{"// private\u0085\n{}", "prefix-refused", ""},
		{strings.Repeat("//x\n", 17) + "{}", "prefix-refused", ""},
		{"//" + strings.Repeat("x", 4094) + "\n{}", "prefix-refused", ""},
		{"// private\n{\"x\":@}", "suffix-refused", "syntax"},
		{"// private\n{\"x\":1,\"x\":2}", "suffix-refused", "duplicate-key"},
		{"// private\n{\"x\":1e10000}", "suffix-refused", "nonfinite-number"},
		{"// private\n{\"x\":" + strings.Repeat("[", 34) + strings.Repeat("]", 34) + "}", "suffix-refused", "structure-limit"},
		{"// private\n{\"x\":1,}", "suffix-refused", "syntax"},
		{"// private\n{} {}", "suffix-refused", "trailing-data"},
		{"// private\n{\"url\":\"https://inert.invalid/a//b\",/*private*/\"x\":1}", "suffix-refused", "syntax"},
	} {
		change := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), copilotProjectConfigHash([]byte(tc.body)), nil, func() ([]byte, error) { return []byte(tc.body), nil })
		p := change.LeadingCommentProbe
		if change.Status != "invalid-json" || p == nil || p.Status != tc.status || p.SuffixJSONReason != tc.reason || p.SemanticallyEqual != nil || p.TrustedFoldersMatch != nil {
			t.Fatal("unadmitted prefix/suffix syntax escaped finite refusal", index)
		}
	}
	for _, prefix := range []string{strings.Repeat("//x\n", 16), "//" + strings.Repeat("x", 4093) + "\n"} {
		p := copilotProjectConfigLeadingComments(map[string]any{}, []byte(prefix+"{}"))
		if p.Status != "captured" || p.PrefixBytes != len(prefix) || p.PrefixBytes > 4096 || p.CommentLines > 16 {
			t.Fatal("exact finite prefix boundary refused")
		}
	}
}

func TestCopilotProjectNativeConfigPrefixRequiresOriginalEligibleBranch(t *testing.T) {
	invalidUTF8 := []byte("// private\xff\n{}")
	invalid := copilotProjectConfigDiagnostic([]byte(`{}`), copilotProjectConfigHash([]byte(`{}`)), copilotProjectConfigHash(invalidUTF8), nil, func() ([]byte, error) { return invalidUTF8, nil })
	if invalid.Status != "invalid-json" || invalid.ObservedJSONReason != "body-limit" || invalid.LeadingCommentProbe != nil {
		t.Fatal("non-UTF8 body escaped the original strict-parser refusal")
	}
	if _, _, status := copilotProjectConfigCommentEnd(invalidUTF8); status != "prefix-refused" {
		t.Fatal("private prefix scanner admitted non-UTF8 bytes")
	}
	for _, baseline := range [][]byte{[]byte(`{`), []byte(`[]`)} {
		body := []byte("// private\n{}")
		change := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), copilotProjectConfigHash(body), nil, func() ([]byte, error) { return body, nil })
		if change.LeadingCommentProbe != nil {
			t.Fatal("prefix projection used invalid or non-object original authority")
		}
	}
	baseline, body := []byte(`{}`), []byte("// private\n{}")
	for _, observed := range []string{"different-held-digest", copilotProjectConfigHash(body)} {
		guardErr := errors.New("private guard failure")
		change := copilotProjectConfigDiagnostic(baseline, copilotProjectConfigHash(baseline), observed, guardErr, func() ([]byte, error) {
			t.Fatal("guard failure attempted prefix read")
			return nil, nil
		})
		if change.LeadingCommentProbe != nil {
			t.Fatal("guard failure produced prefix evidence")
		}
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

func TestCopilotProjectNativeConfigSuffixMembersPreserveTypesAndPrivateHashes(t *testing.T) {
	const nullHash = "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"
	for _, tc := range []struct{ before, after, oldType, newType, oldHash, newHash string }{
		{"", "null", "absent", "null", "", nullHash},
		{"null", "", "null", "absent", nullHash, ""},
		{"true", "false", "boolean", "boolean", "b5bea41b6c623f7c09f1bf24dcae58ebab3c0cdd90ad966bc43a45b44867e12b", "fcbcf165908dd18a9e49f7ff27810176db8e9f63b4352213741664245224f8aa"},
		{"1", "1.0", "number", "number", "6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b", "d0ff5974b6aa52cf562bea5921840c032a860a91a3512f7fe8f768f6bbe005f6"},
		{`"private-before-value"`, `"private-after-value"`, "string", "string", "4756854de2207db20c77d3914d9aefe77c0d6695742ce2243b06a8332f6d1798", "ccbe7324a86d730a1f2c5d22bb7dac6475692499fcce939ae32221d77f18a274"},
		{`["private-array-value"]`, `{"nested":"private-object-value"}`, "array", "object", "aeedd015ae0418c83e87694ac2a8ff6e4ff443854643bbc90d954b26ebb04cb3", "c0be3fc965ed828dd024d04cad67ffdaab2be39d0cf9d1086b131038da39c83e"},
	} {
		object := func(value string) []byte {
			body := `{"trustedFolders":["A","B"]`
			if value != "" {
				body += `,"private-one-key":` + value
			}
			return []byte(body + "}")
		}
		baseline, body := object(tc.before), append([]byte("// private-comment-canary\n"), object(tc.after)...)
		f := copilotProjectConfigFixture(t, baseline)
		want := f.protected[f.originalConfigPath]
		copilotProjectWriteConfig(t, f, body)
		got, err := f.protectedDigest(f.originalConfigPath, want)
		change := copilotProjectRecordedChange(t, f)
		p := change.LeadingCommentProbe
		if err != nil || got == want || change.Status != "invalid-json" || p == nil || p.MemberDelta == nil || p.SemanticallyEqual == nil || *p.SemanticallyEqual || p.TrustedFoldersMatch == nil || !*p.TrustedFoldersMatch {
			t.Fatal("suffix member observation replaced original failure or trust facts")
		}
		d := p.MemberDelta
		oldFields, newFields := 2, 2
		if tc.before == "" {
			oldFields = 1
		}
		if tc.after == "" {
			newFields = 1
		}
		member := copilotProjectConfigSuffixMember{"b4ff3321122d762f828581a2a4c29dbcdf7f60a1ac622a59d8f16b3c387de37c", tc.oldType, tc.newType, tc.oldHash, tc.newHash}
		if d.Status != "captured" || d.BeforeFields != oldFields || d.AfterFields != newFields || d.ChangedFields != 1 || d.OmittedCount != 0 || !reflect.DeepEqual(d.Members, []copilotProjectConfigSuffixMember{member}) {
			t.Fatal("presence-aware type or independently expected value hash differs")
		}
		encoded := copilotProjectContractBody(t, change)
		if len(encoded) > 2<<10 || bytes.Contains(encoded, []byte("private-")) || bytes.Contains(encoded, []byte("trustedFolders")) || bytes.Contains(encoded, []byte("nested")) {
			t.Fatal("suffix diagnostic exceeded its complete bound or exposed private bytes")
		}
		actual, err := os.ReadFile(f.originalConfigPath)
		if err != nil || !bytes.Equal(actual, body) || !bytes.Equal(f.originalConfigBaseline, baseline) || f.protected[f.originalConfigPath] != want {
			t.Fatal("suffix diagnostic rewrote or adopted original authority")
		}
	}
}

func TestCopilotProjectNativeConfigSuffixMemberCountsOrderAndCompleteBound(t *testing.T) {
	var previous []copilotProjectConfigSuffixMember
	for _, reverse := range []bool{false, true} {
		before, after := `{"trustedFolders":["A","B"]`, `{"trustedFolders":["A","B"]`
		for i := range 100 {
			index := i
			if reverse {
				index = 99 - i
			}
			before += fmt.Sprintf(`,"private-key-%03d":true`, index)
			after += fmt.Sprintf(`,"private-key-%03d":false`, index)
		}
		baseline, body := []byte(before+"}"), []byte("//"+strings.Repeat("x", 4093)+"\n"+after+"}")
		f := copilotProjectConfigFixture(t, baseline)
		want := f.protected[f.originalConfigPath]
		copilotProjectWriteConfig(t, f, body)
		got, err := f.protectedDigest(f.originalConfigPath, want)
		change := copilotProjectRecordedChange(t, f)
		if err != nil || got == want || change.LeadingCommentProbe == nil || change.LeadingCommentProbe.MemberDelta == nil {
			t.Fatal("maximum prefix/changed-map diagnostic missed original failure")
		}
		d := change.LeadingCommentProbe.MemberDelta
		if d.Status != "entry-limit" || d.BeforeFields != 101 || d.AfterFields != 101 || d.ChangedFields != 100 || len(d.Members) != 3 || d.OmittedCount != 97 || len(copilotProjectContractBody(t, change)) > 2<<10 {
			t.Fatal("complete diagnostic cap, deterministic entry limit or exact counts differ")
		}
		for i, digest := range []string{"00fcf287dade8c3029d2e30771902e1c351d2112af3fdb95933b5049cc93aaf0", "0401377f50697597d8fb543117c0c2667402c1113df49fe1c136b17e3be5e213", "06c1237ab15604a07fdac6acc726f72db6a62ba06e77dd027e91ab749958c82e"} {
			if d.Members[i].KeySHA256 != digest || d.Members[i].BeforeType != "boolean" || d.Members[i].AfterType != "boolean" || d.Members[i].BeforeValueSHA256 != "b5bea41b6c623f7c09f1bf24dcae58ebab3c0cdd90ad966bc43a45b44867e12b" || d.Members[i].AfterValueSHA256 != "fcbcf165908dd18a9e49f7ff27810176db8e9f63b4352213741664245224f8aa" {
				t.Fatal("selected member order or independent hash differs")
			}
		}
		if reverse && !reflect.DeepEqual(previous, d.Members) {
			t.Fatal("object ordering changed deterministic diagnostic selection")
		}
		previous = d.Members
	}
}
