//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cursorApprovalConfig = `{"mcpServers":{"uapcursora":{"args":["--internal-stdio-v1","/owned/managed","/owned/data","/owned/managed","plugin","--","./bin/probe","cursor-component"],"command":"/owned/managed/agentplugins","env":{"PLUGIN_DATA":"/owned/data","PLUGIN_ROOT":"/owned/managed","UAP_TEST_EVENTS":"/owned/events.jsonl","UAP_TEST_NONCE":"0123456789abcdef0123456789abcdef","UAP_TEST_REVISION":"cursor-a-one","UAP_TEST_ROOT":"/owned"}}}}`

func TestCursorApprovalUsesPinnedParserOrderAndExactProject(t *testing.T) {
	path, body, err := cursorApprovalRecipe("/owned/home", "/owned/A", []byte(cursorApprovalConfig))
	if err != nil || path != "/owned/home/.cursor/projects/owned-A/mcp-approvals.json" || string(body) != "[\n  \"uapcursora-78ea85b1ebb83201\"\n]" {
		t.Fatal("source-derived approval fingerprint/path/format differs", err)
	}
	var object map[string]any
	if json.Unmarshal([]byte(cursorApprovalConfig), &object) != nil {
		t.Fatal("invalid literal contract")
	}
	server := object["mcpServers"].(map[string]any)["uapcursora"].(map[string]any)
	ordered, _ := json.Marshal(struct {
		Command any `json:"command"`
		Env     any `json:"env"`
		Args    any `json:"args"`
	}{server["command"], server["env"], server["args"]})
	reordered := []byte(`{"mcpServers":{"uapcursora":` + string(ordered) + `}}`)
	_, actual, err := cursorApprovalRecipe("/owned/home", "/owned/A", reordered)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatal("raw config property order changed parsed fingerprint", err)
	}
	_, foreign, err := cursorApprovalRecipe("/owned/home", "/owned/B", reordered)
	if err != nil || bytes.Equal(foreign, body) {
		t.Fatal("approval was not bound to the exact selected project")
	}
	for _, raw := range []string{
		strings.Replace(cursorApprovalConfig, `"mcpServers":`, `"unknown":true,"mcpServers":`, 1),
		strings.Replace(cursorApprovalConfig, `"args":`, `"type":"stdio","args":`, 1),
		strings.Replace(cursorApprovalConfig, `"command":"/owned/managed/agentplugins"`, `"command":false`, 1),
		strings.ReplaceAll(cursorApprovalConfig, "/owned/data", "${PLUGIN_DATA}"),
		strings.ReplaceAll(cursorApprovalConfig, "/owned/data", "/owned/é"),
		strings.Replace(cursorApprovalConfig, `"env":`, `"cwd":"/owned","env":`, 1),
	} {
		if _, _, err := cursorApprovalRecipe("/owned/home", "/owned/A", []byte(raw)); err == nil {
			t.Fatal("unreviewed approval schema/encoding accepted")
		}
	}
}

func cursorApprovalFixture(t *testing.T) (*cursorProjectAttempt, string, []byte) {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	f := &cursorProjectAttempt{base: &nativeFixture{Root: root, Home: home, Project: filepath.Join(root, "A"), Events: filepath.Join(root, "events.jsonl")}, held: map[string]cursorProjectHeld{}, evidence: map[string]any{"installed_config": cursorApprovalConfig}}
	path, body, err := cursorApprovalRecipe(home, f.base.Project, []byte(cursorApprovalConfig))
	if err != nil || os.MkdirAll(filepath.Dir(path), 0755) != nil {
		t.Fatal("inert approval fixture", err)
	}
	f.absent = []string{path, filepath.Join(filepath.Dir(path), "mcp-disabled.json")}
	return f, path, body
}

func TestCursorApprovalAcceptsOnlyNativePhaseExactBytesAndAuthority(t *testing.T) {
	for _, fault := range []string{"none", "wrong-fingerprint", "newline", "mode", "hardlink", "alias", "missing-phase", "unexpected-probe"} {
		t.Run(fault, func(t *testing.T) {
			f, path, body := cursorApprovalFixture(t)
			if f.check() != nil {
				t.Fatal("initial approval absence differs")
			}
			projectNativeRuntimeWrite(t, path, body, 0644)
			if f.check() == nil {
				t.Fatal("approval was accepted before the registered native phase")
			}
			switch fault {
			case "wrong-fingerprint", "newline":
				if fault == "wrong-fingerprint" {
					body = []byte("[\"uapcursora-0000000000000000\"]")
				} else {
					body = append(body, '\n')
				}
				if err := os.WriteFile(path, body, 0644); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, path+"-alias"); err != nil {
					t.Fatal(err)
				}
			case "alias":
				if err := os.Rename(path, path+"-target"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+"-target", path); err != nil {
					t.Fatal(err)
				}
			case "missing-phase":
				f.absent = nil
			case "unexpected-probe":
				projectNativeRuntimeWrite(t, f.base.Events, []byte("unexpected"), 0600)
			}
			accepted := f.acceptApproval()
			if (accepted == nil) != (fault == "none") {
				t.Fatal("native approval authority differs", accepted)
			}
			if accepted == nil && f.check() != nil {
				t.Fatal("accepted native approval failed its held check")
			}
		})
	}
}

func TestCursorApprovalRemainsHeldAcrossLaterChildren(t *testing.T) {
	for _, fault := range []string{"bytes", "inode", "parent-mode", "parent-inode", "disabled-store"} {
		t.Run(fault, func(t *testing.T) {
			f, path, body := cursorApprovalFixture(t)
			projectNativeRuntimeWrite(t, path, body, 0644)
			if err := f.acceptApproval(); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "bytes":
				if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
					t.Fatal(err)
				}
			case "inode":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, body, 0644)
			case "parent-mode":
				if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
			case "parent-inode":
				if err := os.Rename(filepath.Dir(path), filepath.Dir(path)+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, body, 0644)
			case "disabled-store":
				projectNativeRuntimeWrite(t, f.absent[0], []byte("[]"), 0644)
			}
			if f.check() == nil {
				t.Fatal("late native approval drift accepted")
			}
		})
	}
}
