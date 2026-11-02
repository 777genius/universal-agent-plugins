//go:build linux

package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func testProof() string {
	body, _ := json.Marshal(isolationProof{PID: 123, UID: 1001, GID: 1001, EUID: 1001, EGID: 1001, Namespace: 2, ParentNamespace: 1, Groups: []int{}, Interfaces: []string{"lo"}, CapabilitiesZero: true, NoNewPrivileges: true})
	return proofPrefix + string(body) + "\n"
}

type inertDuplexRunner struct {
	body    string
	calls   int
	command ports.Command
}

func (r *inertDuplexRunner) RunDuplexWithPlannedShutdown(_ context.Context, command ports.Command, exchange func(io.Writer, io.Reader) error) error {
	r.calls++
	r.command = command
	return exchange(io.Discard, strings.NewReader(r.body))
}
func testController(t *testing.T, runner strictRunner) *controller {
	t.Helper()
	work, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &controller{request: sealedRequest{Base: "/tmp/uap-vscode-sealed-inert", Input: inputRequest{UID: 1001, GID: 1001}}, parentNS: 1, work: work, cancelWork: cancel, runner: runner, files: map[string][]byte{}}
}
func TestInertStrictExchangeKeepsVerifierFirstLiteralIsolationAndRedaction(t *testing.T) {
	runner := &inertDuplexRunner{body: testProof() + "cookie-value\n"}
	c := testController(t, runner)
	c.cookie = "cookie-value"
	record, err := c.strict(t.Context(), "A", []string{"/inert/native-never-executed"}, "/", map[string]string{}, func(_ context.Context, proof isolationProof) error {
		if proof.UID != 1001 || proof.Namespace != 2 {
			t.Fatal(proof)
		}
		return nil
	})
	if err != nil || runner.calls != 1 || !record.TreeReaped || !record.PlannedStop {
		t.Fatalf("fake exchange failed %+v %v", record, err)
	}
	joined := strings.Join(runner.command.Argv, "\n")
	for _, part := range []string{"/usr/bin/unshare", "--net", "--no-new-privs", "--clear-groups", "-I"} {
		if !strings.Contains(joined, part) {
			t.Fatal("literal isolation part missing", part)
		}
	}
	body := string(c.files["A.merged.log"])
	if strings.Contains(body, "cookie-value") || !strings.Contains(body, "[task-cookie-redacted]") {
		t.Fatal("cookie retained", body)
	}
}
func TestInertInvalidProofAndCancellationCannotBecomePlannedSuccess(t *testing.T) {
	for _, body := range []string{"native-before-proof\n", strings.Repeat("x", 4097), strings.Replace(testProof(), `"network_namespace":2`, `"network_namespace":1`, 1), strings.Replace(testProof(), `"interfaces":["lo"]`, `"interfaces":["eth0"]`, 1)} {
		runner := &inertDuplexRunner{body: body}
		c := testController(t, runner)
		called := false
		record, err := c.strict(t.Context(), "A", []string{"/inert"}, "/", map[string]string{}, func(context.Context, isolationProof) error { called = true; return nil })
		if err == nil || called || record.PlannedStop || record.TreeReaped {
			t.Fatalf("proof admitted %+v %v", record, err)
		}
	}
	runner := &inertDuplexRunner{}
	c := testController(t, runner)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.strict(ctx, "A", nil, "/", nil, func(context.Context, isolationProof) error { return nil }); err == nil || runner.calls != 0 {
		t.Fatal("canceled exchange dispatched")
	}
}
func TestDuplicateAmbiguousTrailingAndOversizedJSONRefuse(t *testing.T) {
	for _, body := range []string{`{"a":1,"a":2}`, `{"a":1,"A":2}`, `{} {}`, strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34), "[" + strings.Repeat("0,", 256) + "0]"} {
		if _, err := uniqueJSON([]byte(body)); err == nil {
			t.Fatal("ambiguous/bounded JSON admitted", body[:min(len(body), 80)])
		}
	}
	if _, err := uniqueJSON([]byte{0xff}); err == nil {
		t.Fatal("invalid UTF-8 admitted")
	}
	if _, err := uniqueJSON([]byte(`{"a":[1,true,null]}`)); err != nil {
		t.Fatal(err)
	}
}
func TestOrdinaryFileRejectsAliasesHardlinksAndSparseOverlimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("raw"), 0600); err != nil {
		t.Fatal(err)
	}
	if body, err := readOrdinary(path, 3); err != nil || string(body) != "raw" {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := readOrdinary(alias, 3); err == nil {
		t.Fatal("alias admitted")
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := readOrdinary(path, 3); err == nil {
		t.Fatal("hardlink admitted")
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, 1<<30); err != nil {
		t.Fatal(err)
	}
	if _, err := readOrdinary(path, 3); err == nil {
		t.Fatal("sparse overlimit admitted")
	}
}
func markerSkill(path, name, description string) map[string]any {
	return map[string]any{"uri": map[string]any{"value": "file://" + path, "scheme": "file", "authority": "", "path": path}, "name": name, "description": description, "source": "local", "disableModelInvocation": true}
}
func validObservation(base, folder string) map[string]any {
	return map[string]any{"schema_version": 1, "collected": true, "version": publisherVersion, "app_root": filepath.Join(base, "publisher", publisherPrefix, "resources", "app"), "app_host": "desktop", "observer": map[string]any{"id": "uap-research.workspace-observer", "mode": 3, "proposals": []string{"mcpServerDefinitions", "chatPromptFiles"}}, "folder": map[string]any{"value": "file://" + folder, "scheme": "file", "path": folder, "authority": ""}, "trusted": false, "policy": map[string]any{"skills_enabled": true, "mcp_access": "all", "mcp_autostart": "never"}, "mcp": []any{}, "skills": []any{}, "native_project_qualified": false, "native_mcp_initialization_qualified": false, "native_runtime_qualified": false}
}
func TestObservedContextPolicyShapesAndQualificationAreClosed(t *testing.T) {
	base := "/tmp/uap-vscode-sealed-inert"
	folder := filepath.Join(base, "projects", "A")
	good := validObservation(base, folder)
	body, _ := json.Marshal(good)
	m, err := validateObservation(body, base, folder)
	if err != nil || m == nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"trusted", "qualification", "policy", "mode", "unknown", "skills", "source", "collected", "stage-error", "stage-error-empty"} {
		value := validObservation(base, folder)
		switch kind {
		case "trusted":
			value["trusted"] = true
		case "qualification":
			value["native_project_qualified"] = true
		case "policy":
			value["policy"].(map[string]any)["mcp_autostart"] = "newAndOutdated"
		case "mode":
			value["observer"].(map[string]any)["mode"] = 1
		case "unknown":
			value["unreviewed"] = true
		case "skills":
			value["skills"] = nil
		case "source":
			skill := markerSkill("/tmp/A", "x", "x")
			skill["source"] = "future"
			value["skills"] = []any{skill}
		case "stage-error":
			value["stage_error"] = "contradictory error"
		case "stage-error-empty":
			value["stage_error"] = ""
		case "collected":
			value["collected"] = false
			value["stage_error"] = "API refused"
		}
		body, _ = json.Marshal(value)
		if _, err := validateObservation(body, base, folder); err == nil {
			t.Fatal("invalid native metadata admitted", kind)
		}
	}
}
func TestAllPhysicalMarkerURIAppearancesBlockContradictoryDuplicates(t *testing.T) {
	base := "/tmp/uap-vscode-sealed-inert"
	folder := filepath.Join(base, "projects", "A")
	owned := filepath.Join(folder, ".github", "skills", "uap-a", "SKILL.md")
	home := filepath.Join(base, "home", ".copilot", "skills", "uap-user", "SKILL.md")
	for _, kind := range []string{"valid", "owned-name", "owned-description", "user-name", "identical", "sibling"} {
		rows := []any{markerSkill(owned, "uap-a", "A"), markerSkill(home, "uap-user", "USER")}
		switch kind {
		case "owned-name":
			rows = append(rows, markerSkill(owned, "wrong", "A"))
		case "owned-description":
			rows = append(rows, markerSkill(owned, "uap-a", "wrong"))
		case "user-name":
			rows = append(rows, markerSkill(home, "wrong", "USER"))
		case "identical":
			rows = append(rows, markerSkill(owned, "uap-a", "A"))
		case "sibling":
			rows = append(rows, markerSkill(filepath.Join(base, "projects", "B", "foreign", "SKILL.md"), "foreign", "B"))
		}
		result := metadataAttribution(map[string]any{"skills": rows, "mcp": []any{}}, folder, false)
		if (result["skills"] == "passed") != (kind == "valid") {
			t.Fatalf("kind%s attribution%v", kind, result)
		}
		if result["native_project_qualified"] != false {
			t.Fatal("metadata became runtime proof")
		}
	}
}
func TestMonotonicBudgetAndLocalHostedAdmissionFailClosed(t *testing.T) {
	for _, start := range []int64{0, -1, 1 << 62} {
		if _, err := deadlineAt(start, time.Second); err == nil {
			t.Fatal("invalid task clock admitted", start)
		}
	}
	t.Setenv("GITHUB_ACTIONS", "")
	if hostedRoot() == nil {
		t.Fatal("local host guard admitted")
	}
}

func TestZeroByteArtifactsStillHaveExclusiveNames(t *testing.T) {
	c := testController(t, &inertDuplexRunner{})
	if err := c.keep("empty.log", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.keep("empty.log", []byte{}); err == nil {
		t.Fatal("zero-byte artifact name reused")
	}
	if len(c.files) != 1 || c.bytes != 0 {
		t.Fatal("duplicate empty artifact changed envelope")
	}
}
func TestSnapshotCapacityAdmits256ThenRefuses257WithoutInsertion(t *testing.T) {
	entries := map[string]string{}
	for i := range 256 {
		if err := snapshotCapacity(entries); err != nil {
			t.Fatalf("entry%d refused: %v", i+1, err)
		}
		entries[strings.Repeat("x", i+1)] = "finite fixture entry"
	}
	if err := snapshotCapacity(entries); err == nil {
		t.Fatal("257th entry admitted")
	}
	if len(entries) != 256 {
		t.Fatal("capacity check changed recorded entries")
	}
}

func TestMCPEnvironmentFiniteNumbersAndNullSurviveWireAdmission(t *testing.T) {
	base := "/tmp/uap-vscode-sealed-inert"
	folder := filepath.Join(base, "projects", "A")
	value := validObservation(base, folder)
	value["mcp"] = []any{map[string]any{"label": "inert", "kind": "stdio", "command": "/never-run", "args": []any{}, "env": map[string]any{"TEXT": "value", "PORT": json.Number("42"), "FRACTION": json.Number("0.5"), "UNSET": nil}}}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := validateObservation(body, base, folder)
	if err != nil {
		t.Fatal(err)
	}
	row := observed["mcp"].([]any)[0].(map[string]any)
	env := row["env"].(map[string]any)
	unset, present := env["UNSET"]
	if env["PORT"] != json.Number("42") || env["FRACTION"] != json.Number("0.5") || !present || unset != nil || env["TEXT"] != "value" {
		t.Fatal("environment values changed during metadata admission", env)
	}
}

func TestMCPMalformedContainersNumbersAndMixedTransportsRefuse(t *testing.T) {
	stdio := func() map[string]any {
		return map[string]any{"label": "inert", "kind": "stdio", "command": "/never-run", "args": []any{}, "env": map[string]any{}}
	}
	http := func() map[string]any {
		return map[string]any{"label": "inert", "kind": "http", "uri": map[string]any{"value": "http://127.0.0.1:9/inert", "scheme": "http", "path": "/inert", "authority": "127.0.0.1:9"}, "headers": map[string]any{}}
	}
	for _, number := range []json.Number{"NaN", "Infinity", "-Infinity", "1e1000"} {
		row := stdio()
		row["env"] = map[string]any{"KEY": number}
		if metadataMCP(row) == nil {
			t.Fatal("nonfinite environment number admitted", number)
		}
	}
	for _, key := range []string{"args", "env"} {
		row := stdio()
		row[key] = nil
		if metadataMCP(row) == nil {
			t.Fatal("null metadata container admitted", key)
		}
	}
	for _, headers := range []any{nil, map[string]any{"KEY": json.Number("42")}, map[string]any{"KEY": nil}} {
		row := http()
		row["headers"] = headers
		if metadataMCP(row) == nil {
			t.Fatal("invalid HTTP header container/value admitted", headers)
		}
	}
	for _, key := range []string{"command", "args", "env", "cwd"} {
		row := http()
		row[key] = nil
		if metadataMCP(row) == nil {
			t.Fatal("mixed HTTP/stdio metadata admitted", key)
		}
	}
	row := stdio()
	row["headers"] = map[string]any{}
	if metadataMCP(row) == nil {
		t.Fatal("HTTP headers admitted on stdio metadata")
	}
}
