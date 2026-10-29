//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func grokPreparedInspectFixture() (map[string]any, string, string, string) {
	project, name := "/owned/project-a", "uap-grok-research-a"
	active := filepath.Join(project, ".grok", "plugins", "selected-artifact")
	// The measured 1.0.70 identity subset; extension data is deliberately opaque.
	object := map[string]any{"grokVersion": "1.0.70", "cwd": project, "projectRoot": project + "/", "projectTrusted": false, "skills": []any{}, "channel": "unknown", "plugins": []any{
		map[string]any{"name": name, "scope": "project", "path": active, "enabled": false, "provides": map[string]any{"skills": 1, "agents": 0, "hooks": false, "mcpServers": 0}},
		map[string]any{"name": "uap-grok-research-user", "scope": "user", "path": "/owned/home/.grok/plugins/user", "enabled": true},
	}}
	return object, project, name, active
}

func TestGrokPreparedInspectRequiresExactMeasuredDisabledProjectCandidate(t *testing.T) {
	for _, trailing := range []string{"", "/"} {
		object, project, name, active := grokPreparedInspectFixture()
		object["projectRoot"] = project + trailing
		body, _ := json.Marshal(object)
		if err := projectGrokPreparedInspect(body, project, name, active); err != nil {
			t.Fatal(err)
		}
	}
	mutations := []string{"version", "cwd", "root", "root-extra-separator", "trusted", "trust-type", "skills", "plugins", "missing", "duplicate-name", "folded-name", "name", "scope", "path", "noncanonical-path", "enabled", "enabled-type", "folded-key", "missing-key", "plugin-type", "foreign-type"}
	for _, mutation := range mutations {
		t.Run(mutation, func(t *testing.T) {
			object, project, name, active := grokPreparedInspectFixture()
			plugins := object["plugins"].([]any)
			entry := plugins[0].(map[string]any)
			switch mutation {
			case "version":
				object["grokVersion"] = "1.0.67"
			case "cwd":
				object["cwd"] = "/other/project"
			case "root":
				object["projectRoot"] = "/other/project"
			case "root-extra-separator":
				object["projectRoot"] = project + "//"
			case "trusted":
				object["projectTrusted"] = true
			case "trust-type":
				object["projectTrusted"] = "false"
			case "skills":
				object["skills"] = map[string]any{}
			case "plugins":
				object["plugins"] = nil
			case "missing":
				object["plugins"] = plugins[1:]
			case "duplicate-name":
				object["plugins"] = append(plugins, entry)
			case "folded-name":
				entry["name"] = strings.ToUpper(name)
			case "name":
				entry["name"] = "other"
			case "scope":
				entry["scope"] = "user"
			case "path":
				entry["path"] = "/other/package"
			case "noncanonical-path":
				entry["path"] = active + "/../selected-artifact"
			case "enabled":
				entry["enabled"] = true
			case "enabled-type":
				entry["enabled"] = 0
			case "folded-key":
				entry["Scope"] = entry["scope"]
				delete(entry, "scope")
			case "missing-key":
				delete(entry, "enabled")
			case "plugin-type":
				plugins[0] = "unknown"
			case "foreign-type":
				plugins[1].(map[string]any)["path"] = "relative"
			}
			body, _ := json.Marshal(object)
			if err := projectGrokPreparedInspect(body, project, name, active); err == nil {
				t.Fatal("changed native inspection admitted")
			}
		})
	}
	object, project, name, active := grokPreparedInspectFixture()
	body, _ := json.Marshal(object)
	for _, bad := range [][]byte{append(body, body...), []byte(`{"cwd":"a","cwd":"b"}`), {0xff}, []byte(`[]`), []byte(`{"x":NaN}`)} {
		if err := projectGrokPreparedInspect(bad, project, name, active); err == nil {
			t.Fatal("unsafe JSON admitted")
		}
	}
	if projectGrokPreparedInspect(body, project, name, "/foreign/selected-artifact") == nil {
		t.Fatal("unanchored expected package admitted")
	}
}

func grokPreparedAuthorityFixture(t *testing.T) *projectNativeCLIAttempt {
	t.Helper()
	root := t.TempDir()
	spec, _ := projectNativeCLISpecification("grok")
	project, source, name := filepath.Join(root, "project-a"), filepath.Join(root, "source"), "uap-grok-attempt"
	active := filepath.Join(project, ".grok", "plugins", "selected-artifact")
	f := &projectNativeCLIAttempt{spec: spec, base: &nativeFixture{Project: project, Package: source, Nonce: "inert-nonce"}, name: name, client: "/owned/grok", skill: filepath.Join(active, "skills", name, "SKILL.md"), binding: domain.ClientBinding{Scope: "project", NativeProfileRoot: filepath.Join(project, ".grok"), PhysicalArtifact: "selected-artifact", TargetLocator: active}}
	skill := []byte("---\nname: " + name + "\ndescription: Harmless installed startup marker\n---\nUAP_NATIVE_INSTALLED_inert-nonce\n")
	for _, path := range []string{f.skill, filepath.Join(source, "skills", name, "SKILL.md")} {
		projectNativeRuntimeWrite(t, path, skill, 0600)
	}
	manifest, _ := json.Marshal(map[string]any{"name": name, "version": "1.0.0", "description": "Disposable native project startup observation", "skills": "./skills/"})
	projectNativeRuntimeWrite(t, filepath.Join(active, ".grok-plugin", "plugin.json"), manifest, 0644)
	return f
}

func TestGrokPreparedAuthorityRefusesSubstitutionBeforeFirstBaselineAndAfterInspection(t *testing.T) {
	for _, after := range []bool{false, true} {
		for _, mutation := range []string{"skill-content", "source-content", "manifest", "manifest-alias", "skill-hardlink", "parent-alias", "directory-replacement", "mode"} {
			t.Run(mutation+"-after-"+map[bool]string{false: "no", true: "yes"}[after], func(t *testing.T) {
				f := grokPreparedAuthorityFixture(t)
				verify, err := projectGrokPreparedAuthority(f)
				if err != nil || verify() != nil {
					t.Fatalf("ordinary original authority failed: %v", err)
				}
				active := f.binding.TargetLocator
				manifest := filepath.Join(active, ".grok-plugin", "plugin.json")
				switch mutation {
				case "skill-content":
					err = os.WriteFile(f.skill, []byte("foreign skill"), 0600)
				case "source-content":
					err = os.WriteFile(filepath.Join(f.base.Package, "skills", f.name, "SKILL.md"), []byte("foreign source"), 0600)
				case "manifest":
					err = os.WriteFile(manifest, []byte(`{"name":"foreign"}`), 0644)
				case "manifest-alias":
					if err = os.Remove(manifest); err == nil {
						err = os.Symlink(f.skill, manifest)
					}
				case "skill-hardlink":
					err = os.Link(f.skill, filepath.Join(active, "extra-link"))
				case "parent-alias":
					if err = os.Rename(filepath.Dir(manifest), filepath.Dir(manifest)+"-original"); err == nil {
						err = os.Symlink(filepath.Dir(manifest)+"-original", filepath.Dir(manifest))
					}
				case "directory-replacement":
					if err = os.Rename(active, active+"-original"); err == nil {
						err = os.Mkdir(active, 0700)
					}
				case "mode":
					err = os.Chmod(f.skill, 0700)
				}
				if err != nil {
					t.Fatal(err)
				}
				if after {
					err = verify()
				} else {
					_, err = projectGrokPreparedAuthority(f)
				}
				// Before the first snapshot the authored byte recipe, ordinary
				// paths and single-link rule refuse corruption. Mode is frozen
				// across the inspection, without inventing a projection mode.
				if mutation != "mode" || after {
					if err == nil {
						t.Fatal("selected path substitution admitted")
					}
				}
			})
		}
	}
	f := grokPreparedAuthorityFixture(t)
	f.binding.TargetLocator = filepath.Join(f.base.Project, "foreign", f.binding.PhysicalArtifact)
	if _, err := projectGrokPreparedAuthority(f); err == nil {
		t.Fatal("binding selected a foreign package anchor")
	}
}

func TestGrokPreparedInspectCommandCannotAuthorizeOtherClientsArgvScopeOrDeadline(t *testing.T) {
	f := grokPreparedAuthorityFixture(t)
	good := []string{f.client, "inspect", "--json"}
	if err := f.selectedCommand("native-inspect", f.base.Project, good, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	for _, client := range []string{"qoder", "kiro", "windsurf"} {
		f.spec.client = client
		if f.selectedCommand("native-inspect", f.base.Project, good, 30*time.Second) == nil {
			t.Fatal("another client gained native inspection")
		}
	}
	f.spec.client = "grok"
	for _, changed := range []struct {
		cwd     string
		argv    []string
		timeout time.Duration
	}{{"/foreign/project", good, 30 * time.Second}, {f.base.Project, []string{f.client, "plugin", "enable"}, 30 * time.Second}, {f.base.Project, good, 60 * time.Second}} {
		if f.selectedCommand("native-inspect", changed.cwd, changed.argv, changed.timeout) == nil {
			t.Fatal("unreviewed command/cwd/deadline admitted")
		}
	}
}
