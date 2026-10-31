//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGrokSelectionFreezesOnlyOriginalHomeExactProjectTrust(t *testing.T) {
	f := grokTurnSummaryContractFixture(t)
	path := filepath.Join(f.base.Home, ".grok", "trusted_folders.toml")
	body, err := projectNativeRuntimeRead(path, 4096)
	info, statErr := os.Lstat(path)
	want := []byte("[folders.\"" + f.base.Project + "\"]\ntrusted = true\n")
	parsed, status := grokConfigParse(body)
	if err != nil || statErr != nil || info.Mode().Perm() != 0600 || !bytes.Equal(body, want) ||
		status != "captured" || !reflect.DeepEqual(parsed, map[string]any{"folders": map[string]any{f.base.Project: map[string]any{"trusted": true}}}) ||
		f.protected[path] != grokConfigSHA(want) || len(f.protected) != 2 {
		t.Fatal("exact original HOME owned trust recipe was not frozen")
	}
	config, status := grokConfigParse(f.grok.authoredConfig)
	if status != "captured" || !reflect.DeepEqual(config["plugins"], map[string]any{"enabled": []any{"uap-grok-attempt"}}) {
		t.Fatal("supported name selector changed or gained alternate plugin paths")
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".grok")); !os.IsNotExist(err) {
		t.Fatal("fixture trust acquired project configuration authority")
	}
	f.check(t)
}

func grokSelectionRootFixture(t *testing.T) *projectNativeCLIAttempt {
	t.Helper()
	root := t.TempDir()
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "grok"}, other: filepath.Join(root, "project-b"),
		base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "project")}}
	for _, path := range []string{f.base.Home, f.base.Project, f.other} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestGrokSelectionTrustRefusesForeignRootsAliasesAndExistingAuthority(t *testing.T) {
	for _, kind := range []string{"home", "root", "b", "relative", "noncanonical", "alias", "other-client", "trust", "config", "dangling-trust"} {
		t.Run(kind, func(t *testing.T) {
			f := grokSelectionRootFixture(t)
			if _, err := grokProjectSelectionTrust(f); err != nil {
				t.Fatal(err)
			}
			mutateGrokSelectionRoot(t, f, kind)
			if _, err := grokProjectSelectionTrust(f); err == nil {
				t.Fatal("foreign/alias/existing trust authority was adopted")
			}
		})
	}
}

func mutateGrokSelectionRoot(t *testing.T, f *projectNativeCLIAttempt, kind string) {
	t.Helper()
	switch kind {
	case "home":
		f.base.Project = f.base.Home
	case "root":
		f.base.Project = f.base.Root
	case "b":
		f.base.Project = f.other
	case "relative":
		f.base.Project = "project"
	case "noncanonical":
		f.base.Home += "/../home"
	case "alias":
		if err := os.Rename(f.base.Project, f.base.Project+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(f.base.Project+"-original", f.base.Project); err != nil {
			t.Fatal(err)
		}
	case "other-client":
		f.spec.client = "qoder"
	default:
		name := "trusted_folders.toml"
		if kind == "config" {
			name = "config.toml"
		}
		path := filepath.Join(f.base.Home, ".grok", name)
		if kind == "dangling-trust" {
			if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(f.base.Root, "absent"), path); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, path, []byte("original authority\n"), 0600)
		}
	}
}

func grokSelectedInspectFixture() (map[string]any, string, string, string) {
	object, project, name, active := grokPreparedInspectFixture()
	object["projectTrusted"] = true
	object["plugins"].([]any)[0].(map[string]any)["enabled"] = true
	object["skills"] = []any{map[string]any{"name": name, "description": "Harmless installed startup marker", "userInvocable": true,
		"source": map[string]any{"type": "plugin", "plugin_name": name, "path": filepath.Join(active, "skills", name, "SKILL.md")}}}
	return object, project, name, active
}

func TestGrokSelectedInspectRequiresSeparateExactLoadedSkill(t *testing.T) {
	for _, kind := range []string{"ordinary", "false-disabled", "trust-only", "untrusted", "disabled-plugin", "duplicate", "folded-name", "description", "user-invocable", "disabled", "disabled-type", "source-type", "source-name", "source-path", "source-alias", "folded-key"} {
		t.Run(kind, func(t *testing.T) {
			object, project, name, active := grokSelectedInspectFixture()
			mutateGrokSelectedInspect(object, name, project, kind)
			body, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			valid := kind == "ordinary" || kind == "false-disabled"
			if (projectGrokInspect(body, project, name, active, true) == nil) != valid {
				t.Fatal("selected loaded skill identity control differs")
			}
			if projectGrokPreparedInspect(body, project, name, active) == nil {
				t.Fatal("selected observation changed the disabled Prepared contract")
			}
		})
	}
}

func mutateGrokSelectedInspect(object map[string]any, name, project, kind string) {
	entry := object["skills"].([]any)[0].(map[string]any)
	source := entry["source"].(map[string]any)
	switch kind {
	case "false-disabled":
		entry["disabled"] = false
	case "trust-only":
		object["skills"] = []any{}
	case "untrusted":
		object["projectTrusted"] = false
	case "disabled-plugin":
		object["plugins"].([]any)[0].(map[string]any)["enabled"] = false
	case "duplicate":
		object["skills"] = append(object["skills"].([]any), entry)
	case "folded-name":
		entry["name"] = strings.ToUpper(name)
	case "description":
		entry["description"] = "foreign marker"
	case "user-invocable":
		entry["userInvocable"] = false
	case "disabled":
		entry["disabled"] = true
	case "disabled-type":
		entry["disabled"] = "false"
	case "source-type":
		source["type"] = "user"
	case "source-name":
		source["plugin_name"] = "foreign-plugin"
	case "source-path":
		source["path"] = filepath.Join(project, "foreign", "SKILL.md")
	case "source-alias":
		source["plugin_Name"] = name
	case "folded-key":
		entry["Name"] = name
	}
}
