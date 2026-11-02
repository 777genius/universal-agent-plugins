//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

func copilotPostAddContractEncode(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func copilotPostAddContractArguments(f *copilotProjectAttempt, label string) (string, []string) {
	switch label {
	case "git-A":
		return f.base.Project, []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "git-B":
		return f.other, []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		return f.outside, []string{f.client, "--version"}
	case "add-user":
		return f.outside, []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "user", "--target", "copilot", "--format", "json"}
	default:
		return f.outside, []string{f.client, "plugin", "list"}
	}
}

func copilotPostAddContractFiles(t *testing.T, n int, stage string, statuses ...string) *copilotProjectAttempt {
	t.Helper()
	f := copilotEmptyRegistryContractFixture(t)
	f.postAddRegistryEvidence()
	f.evidence["commands"] = append([]string{}, copilotPostAddRegistryCommands[:n]...)
	f.evidence["stage"] = stage
	for _, label := range copilotPostAddRegistryCommands[:n] {
		cwd, argv := copilotPostAddContractArguments(f, label)
		f.files[label+".command.json"] = copilotPostAddContractEncode(t, map[string]any{"argv": argv, "cwd": cwd, "exit": 0, "elapsed_ms": 1, "error": false, "context_error": false, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"})
		f.files[label+".stdout.log"], f.files[label+".stderr.log"] = []byte{}, []byte{}
	}
	if n >= 3 {
		f.files["version.stdout.log"] = []byte("GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n")
	}
	if n >= 4 {
		f.files["add-user.stdout.log"] = []byte(`{"schema_version":1,"command":"add","result":"success","data":{}}`)
	}
	if n == 5 {
		f.files["native-list.stdout.log"], f.files["native-list.stderr.log"] = []byte("opaque inert registry\n"), []byte("opaque diagnostic\n")
	}
	for index, status := range statuses {
		name, label := []string{"user-profile-metadata.json", copilotPostAddListMetadata}[index], []string{"add-user", "native-list"}[index]
		value := map[string]any{"stage": label, "status": status}
		if status == "observed" {
			value = map[string]any{"stage": label, "status": status, "before_entries": 1, "after_entries": 1, "changed_entries": 0, "omitted_count": 0, "entries": []any{}}
		}
		f.files[name] = copilotPostAddContractEncode(t, value)
	}
	return f
}

func TestCopilotPostAddRegistryExactCommandBoundary(t *testing.T) {
	f := copilotEmptyRegistryContractFixture(t)
	f.postAddRegistryEvidence()
	for index, label := range copilotPostAddRegistryCommands {
		f.evidence["commands"] = append([]string{}, copilotPostAddRegistryCommands[:index]...)
		cwd, argv := copilotPostAddContractArguments(f, label)
		if f.postAddRegistryCommand(label, cwd, argv, 30*time.Second) != nil {
			t.Fatal("fixed command refused")
		}
		for _, bad := range []struct {
			label, cwd string
			argv       []string
			timeout    time.Duration
		}{
			{"native-attempt", cwd, argv, 30 * time.Second}, {label, cwd + "/wrong", argv, 30 * time.Second},
			{label, cwd, append(slices.Clone(argv), "--enable"), 30 * time.Second}, {label, cwd, argv, 60 * time.Second},
		} {
			if f.postAddRegistryCommand(bad.label, bad.cwd, bad.argv, bad.timeout) == nil {
				t.Fatal("foreign command accepted")
			}
		}
	}
	f.evidence["commands"] = slices.Clone(copilotPostAddRegistryCommands)
	if f.postAddRegistryCommand("native-list", f.outside, []string{f.client, "plugin", "list"}, 30*time.Second) == nil {
		t.Fatal("sixth command accepted")
	}
}

func TestCopilotPostAddRegistryFiniteStagesAndClosedFiles(t *testing.T) {
	for _, tc := range []struct {
		stage    string
		n        int
		statuses []string
	}{
		{"prepare", 0, nil}, {"prepare", 1, nil}, {"prepare", 2, nil}, {"prepare", 3, nil},
		{"before-user", 3, []string{"refused"}}, {"public-user", 3, []string{"observed"}}, {"public-user", 4, []string{"observed"}},
		{"public-result", 4, []string{"observed"}}, {"after-user-metadata", 4, []string{"refused"}},
		{"before-list", 4, []string{"observed", "refused"}}, {"native-list", 4, []string{"observed", "refused"}},
		{"native-list", 5, []string{"observed", "observed"}}, {"after-list-metadata", 5, []string{"observed", "refused"}}, {"review-barrier", 5, []string{"observed", "observed"}},
	} {
		f := copilotPostAddContractFiles(t, tc.n, tc.stage, tc.statuses...)
		sequence := slices.Clone(copilotPostAddRegistryCommands[:tc.n])
		if !f.postAddRegistryFiles(sequence, tc.stage) {
			t.Fatal("truthful prefix refused", tc.stage, tc.n)
		}
		for _, name := range []string{"probe-events.jsonl", "provider-request-1.json", "native-list-2.stdout.log"} {
			f.files[name] = []byte("unknown")
			if f.postAddRegistryFiles(sequence, tc.stage) {
				t.Fatal("unknown member accepted", name)
			}
			delete(f.files, name)
		}
		for name, body := range f.files {
			delete(f.files, name)
			if f.postAddRegistryFiles(sequence, tc.stage) {
				t.Fatal("missing member accepted", name)
			}
			f.files[name] = body
		}
	}
}

func TestCopilotPostAddRegistryContinuationCannotBorrowFailedReceipts(t *testing.T) {
	for _, label := range copilotPostAddRegistryCommands {
		for _, change := range []map[string]any{{"exit": 7}, {"error": true}, {"context_error": true}, {"elapsed_ms": true}, {"elapsed_ms": 35001}, {"cleanup": "forced close"}} {
			f := copilotPostAddContractFiles(t, 5, "review-barrier", "observed", "observed")
			value, err := projectNativeRuntimeJSON(f.files[label+".command.json"])
			if err != nil {
				t.Fatal(err)
			}
			row := value.(map[string]any)
			for key, value := range change {
				row[key] = value
			}
			f.files[label+".command.json"] = copilotPostAddContractEncode(t, row)
			if f.postAddRegistryFiles(copilotPostAddRegistryCommands, "review-barrier") {
				t.Fatal("false continuation accepted", label, change)
			}
		}
	}
}

func TestCopilotPostAddRegistryMetadataAndPublicBounds(t *testing.T) {
	for _, name := range []string{"user-profile-metadata.json", copilotPostAddListMetadata} {
		for _, body := range [][]byte{[]byte(`{"stage":"unknown","status":"refused"}`), make([]byte, 2049), []byte(`{"stage":"add-user","status":"observed","private":"no"}`)} {
			f := copilotPostAddContractFiles(t, 5, "review-barrier", "observed", "observed")
			f.files[name] = body
			if f.postAddRegistryFiles(copilotPostAddRegistryCommands, "review-barrier") {
				t.Fatal("metadata mismatch accepted")
			}
		}
	}
	for _, body := range [][]byte{[]byte(`{}`), []byte(`{"schema_version":true,"command":"add","result":"success","data":{}}`), []byte(`{"schema_version":1,"command":"remove","result":"success","data":{}}`)} {
		if copilotPostAddPublic(body) {
			t.Fatal("false public progress accepted")
		}
	}
	f := copilotPostAddContractFiles(t, 5, "review-barrier", "observed", "observed")
	if len(f.files) != 17 {
		t.Fatal("pre-summary full closure differs")
	}
	f.files["native-list.stdout.log"] = make([]byte, (1<<20)+1)
	if f.postAddRegistryFiles(copilotPostAddRegistryCommands, "review-barrier") {
		t.Fatal("oversized list accepted")
	}
}
