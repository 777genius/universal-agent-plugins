package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type claudeProjectNativeFixture struct {
	base                                     *claudeNativeFixture
	client, installer, project, otherProject string
	foreign                                  map[string]string
	diagnosticAttempted                      bool
}

func newClaudeProjectNativeFixture(t *testing.T) *claudeProjectNativeFixture {
	t.Helper()
	client := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(want) != 64 || claudeSHA256(t, client) != want {
		t.Fatal("Claude binary pin mismatch")
	}
	f := &claudeProjectNativeFixture{base: newClaudeNativeFixture(t), client: client, installer: installer, foreign: map[string]string{}}
	f.project, f.otherProject = filepath.Join(f.base.Root, "project-A"), filepath.Join(f.base.Root, "project-B")
	for _, root := range []string{f.base.ConfigDir, filepath.Join(f.project, ".claude"), filepath.Join(f.otherProject, ".claude")} {
		f.keep(t, filepath.Join(root, "settings.json"), `{"enabledPlugins":{"foreign-project-fixture@skills-dir":false}}`)
		f.keep(t, filepath.Join(root, "skills", "foreign-project-fixture", "SKILL.md"), "---\nname: foreign-project-fixture\ndescription: Preserve foreign project fixture\n---\nInert foreign bytes.\n")
	}
	f.keep(t, filepath.Join(f.base.ConfigDir, "plugins", "cache", "foreign", "keep.txt"), "foreign user plugin cache\n")
	return f
}

func (f *claudeProjectNativeFixture) keep(t *testing.T, path, body string) {
	t.Helper()
	nativeWrite(t, path, []byte(body), 0600)
	f.foreign[path] = nativeSHA(t, path)
}

func (f *claudeProjectNativeFixture) requireForeign(t *testing.T) {
	t.Helper()
	for path, want := range f.foreign {
		if nativeSHA(t, path) != want {
			t.Fatalf("foreign user/project configuration changed: %s", path)
		}
	}
}

func (f *claudeProjectNativeFixture) run(t *testing.T, label string, args ...string) (map[string]any, error) {
	t.Helper()
	if len(args) > 0 && args[0] == "add" {
		args = append(args, "--no-save")
	}
	args = append(args, "--scope", "project", "--project-root", f.project)
	result, err := commandsRunInstaller(t, f.base.Root, f.project, f.base.env(filepath.Dir(f.client)), f.installer, label, args...)
	if err != nil && !f.diagnosticAttempted {
		f.diagnosticAttempted = true
		records, logs := claudeFailureListingRecords(f.client, f.project, f.otherProject, f.base.env(filepath.Dir(f.client)), captureClaudeDiagnostic)
		for name, body := range logs {
			nativeWrite(t, filepath.Join(f.base.Root, name), body, 0600)
		}
		nativeJSON(t, filepath.Join(f.base.Root, "failure-listing-diagnostics.json"), map[string]any{"installer_label": label, "original_installer_error": boundedClaudeDiagnosticError(err), "diagnostics": records, "original_failure_preserved": true, "project_qualified": false})
	}
	f.requireForeign(t)
	return result, err
}

func (f *claudeProjectNativeFixture) writeEvidence(t *testing.T, evidence map[string]any, stages map[string]nativeStage) {
	t.Helper()
	hashes := map[string]string{}
	entries, err := os.ReadDir(f.base.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".json") || strings.HasSuffix(entry.Name(), ".jsonl") || strings.HasSuffix(entry.Name(), ".log")) {
			hashes[entry.Name()] = nativeSHA(t, filepath.Join(f.base.Root, entry.Name()))
		}
	}
	evidence["stages"], evidence["artifact_sha256"], evidence["finished_utc"] = stages, hashes, time.Now().UTC().Format(time.RFC3339Nano)
	nativeJSON(t, filepath.Join(f.base.Root, "claude-project-plugin-evidence.json"), evidence)
	if evidence["project_contract"] == "untrusted_prepared_v1" {
		if err := validateClaudePreparedStages(stages); err != nil {
			t.Error(err)
		}
	}
	if err := nativeStageFailures(stages); err != nil {
		t.Error(err)
	}
}

func claudeProjectNativeIdentity(t *testing.T, f *claudeProjectNativeFixture) map[string]any {
	t.Helper()
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	version := claudeVersionString(t, f.base, f.client)
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"); want == "" || strings.TrimSpace(version) != want {
		t.Fatalf("wrong native Claude version=%q,want=%q", version, want)
	}
	return map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "os": runtime.GOOS, "arch": runtime.GOARCH, "source_identity": identity, "client": "claude", "client_version_measured": version, "client_sha256": nativeSHA(t, f.client), "installer_sha256": nativeSHA(t, f.installer), "native_surface": "fresh exact project plugin listing and details; no session/model/trust/MCP initialization", "scanner": nativeProvisionScanner(t, &nativeFixture{Root: f.base.Root})}
}
