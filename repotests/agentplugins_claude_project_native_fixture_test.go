package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
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

func (f *claudeProjectNativeFixture) command(t *testing.T, project, label string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), claudeNativeCommandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, f.client, args...)
	command.Dir, command.Env = project, f.base.env(filepath.Dir(f.client))
	body, err := command.CombinedOutput()
	f.base.record(t, label, body)
	if err != nil {
		t.Fatalf("Claude project read-only command %q: %v\n%s", args, err, body)
	}
	f.requireForeign(t)
	return body
}

func (f *claudeProjectNativeFixture) listedPath(t *testing.T, project, label string) string {
	t.Helper()
	body := f.command(t, project, label, "plugin", "list", "--json")
	var entries []claudePluginListEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		t.Fatal(err)
	}
	path := ""
	for _, entry := range entries {
		if entry.ID != claudeCommandsPluginID {
			continue
		}
		if path != "" || entry.Scope != "project" || !entry.Enabled || entry.InstallPath != filepath.Clean(entry.InstallPath) || filepath.Dir(entry.InstallPath) != filepath.Join(project, ".claude", "skills") {
			t.Fatalf("nonexact project listing=%+v", entry)
		}
		nativeRequireContained(t, project, entry.InstallPath)
		path = entry.InstallPath
	}
	return path
}

func (f *claudeProjectNativeFixture) skills(t *testing.T, label string) map[string]string {
	t.Helper()
	path := f.listedPath(t, f.project, label+"-project-A-list")
	if other := f.listedPath(t, f.otherProject, label+"-project-B-list"); other != "" {
		t.Fatalf("selected project plugin leaked into project B: %s", other)
	}
	if path == "" {
		return nil
	}
	details := f.command(t, f.project, label+"-project-A-details", "plugin", "details", claudeCommandsPluginID)
	found, err := claudeDiscoveredSkills(string(details), path)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func (f *claudeProjectNativeFixture) run(t *testing.T, label string, args ...string) (map[string]any, error) {
	t.Helper()
	if len(args) > 0 && args[0] == "add" {
		args = append(args, "--no-save")
	}
	args = append(args, "--scope", "project", "--project-root", f.project)
	result, err := commandsRunInstaller(t, f.base.Root, f.project, f.base.env(filepath.Dir(f.client)), f.installer, label, args...)
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
