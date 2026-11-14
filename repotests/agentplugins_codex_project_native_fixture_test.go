package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type codexProjectNativeFixture struct {
	base              *nativeFixture
	client, installer string
	otherProject      string
	unchanged         map[string]string
}

func newCodexProjectNativeFixture(t *testing.T) *codexProjectNativeFixture {
	t.Helper()
	client := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	installer := nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(expected) != 64 || nativeSHA(t, client) != expected {
		t.Fatal("Codex binary pin mismatch")
	}
	f := &codexProjectNativeFixture{base: newNativeFixture(t, true), client: client, installer: installer, unchanged: map[string]string{}}
	f.base.ClientBinDir, f.otherProject = filepath.Dir(client), filepath.Join(f.base.Root, "project-B")
	for _, project := range []string{f.base.Project, f.otherProject} {
		if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	config := "cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"
	f.foreign(t, filepath.Join(f.base.CodexHome, "config.toml"), config)
	f.foreign(t, filepath.Join(f.base.Project, ".codex", "config.toml"), "# preserve project preferences\nmodel = 'unobserved-model'\n")
	f.foreign(t, filepath.Join(f.otherProject, ".codex", "config.toml"), "# preserve project B\nmodel = 'project-B-model'\n")
	f.foreign(t, filepath.Join(f.base.CodexHome, "plugins", "cache", "foreign", "keep.txt"), "foreign user plugin cache\n")
	for _, root := range []string{f.base.Home, f.base.Project, f.otherProject} {
		f.foreign(t, filepath.Join(root, ".agents", "skills", "foreign-codex-project", "SKILL.md"), "---\nname: foreign-codex-project\ndescription: Inert foreign fixture\n---\nPreserve foreign bytes.\n")
	}
	copyTree(t, commandsFixtureSource(t), f.base.Package)
	return f
}

func (f *codexProjectNativeFixture) foreign(t *testing.T, path, body string) {
	t.Helper()
	nativeWrite(t, path, []byte(body), 0600)
	f.unchanged[path] = nativeSHA(t, path)
}

func (f *codexProjectNativeFixture) requireForeign(t *testing.T) {
	t.Helper()
	for path, digest := range f.unchanged {
		if nativeSHA(t, path) != digest {
			t.Fatalf("foreign user/project-A/project-B bytes changed: %s", path)
		}
	}
}

func (f *codexProjectNativeFixture) run(t *testing.T, label string, args ...string) map[string]any {
	t.Helper()
	args = append(args, "--target", "codex", "--scope", "project", "--project-root", f.base.Project)
	result, err := commandsRunInstaller(t, f.base.Root, f.base.Project, f.base.env(filepath.Dir(f.client)), f.installer, label, args...)
	if err != nil || result["result"] != "success" {
		t.Fatalf("%s failed: %v; %v", label, err, result)
	}
	f.requireForeign(t)
	return result
}

func (f *codexProjectNativeFixture) skills(t *testing.T, label, project string) map[string]string {
	t.Helper()
	selected := *f.base
	selected.Project = project
	r := startNativeRPC(t, &selected, f.client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_project_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	found, err := codexProjectDiscoveredSkills(r.must(t, "skills/list", map[string]any{"cwds": []string{project}, "forceReload": true}), project)
	if err != nil {
		t.Fatal(err)
	}
	f.requireForeign(t)
	return found
}

func (f *codexProjectNativeFixture) evidence(t *testing.T, stages map[string]nativeStage) map[string]any {
	t.Helper()
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := nativeCommand(t, f.base, f.client, "client-version", "--version")
	if want := os.Getenv("AGENTPLUGINS_CODEX_VERSION"); err != nil || want == "" || strings.TrimSpace(string(version)) != "codex-cli "+want {
		t.Fatalf("wrong pinned client: %s %v", version, err)
	}
	return map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"client": "codex", "client_version_measured": strings.TrimSpace(string(version)), "client_sha256": nativeSHA(t, f.client),
		"installer_sha256": nativeSHA(t, f.installer), "source_identity": identity, "acquisition": "local_directory",
		"native_surface": "fresh app-server skills/list with exact selected-project repository paths; no thread/model/trust/MCP initialization",
		"scanner":        nativeProvisionScanner(t, f.base), "stages": stages}
}

func (f *codexProjectNativeFixture) writeEvidence(t *testing.T, evidence map[string]any) {
	t.Helper()
	evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
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
	evidence["artifact_sha256"] = hashes
	nativeJSON(t, filepath.Join(f.base.Root, "codex-project-components-evidence.json"), evidence)
}
