package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func hermesConflictPackage(t *testing.T, base string) (string, string) {
	t.Helper()
	user, pkg := filepath.Join(base, "user"), filepath.Join(base, "package")
	claudeSDKWrite(t, filepath.Join(user, "config.yaml"), "model: original\n")
	claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-hermes","version":"1.0.0"}`)
	claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert Hermes conflict\n---\nOwned.\n")
	claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"http://127.0.0.1:9/owned"}}}`)
	return user, pkg
}

func TestProjectHermesSDKRejectsPreexistingUnownedRootBeforeEffects(t *testing.T) {
	for _, kind := range []string{"empty", "same-marker", "foreign-config"} {
		t.Run(kind, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			user, pkg := hermesConflictPackage(t, base)
			fixture := newHermesSDKFixture(t, filepath.Join(base, "project"), user, pkg)
			root := filepath.Join(fixture.project, ".hermes", "uap-session")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "same-marker":
				claudeSDKWrite(t, filepath.Join(root, ".uap-owner"), "uap-hermes-project-session-v1\n"+fixture.project+"\n")
			case "foreign-config":
				claudeSDKWrite(t, filepath.Join(root, "config.yaml"), "mcp_servers:\n  docs:\n    command: foreign\n")
			}
			before := fixture.state(t)
			fixture.request.Operation, fixture.request.OperationID = installer.OpInstall, "refuse-foreign"
			handle, err := fixture.engine.Prepare(t.Context(), fixture.request)
			if handle != nil {
				if closeErr := handle.Close(); closeErr != nil {
					t.Error(closeErr)
				}
			}
			if err == nil || !reflect.DeepEqual(before, fixture.state(t)) || fixture.runner.calls != 0 {
				t.Fatal("unowned selected root was adopted or refusal had effects")
			}
		})
	}
}

func TestProjectHermesSDKEditedEntryRefusesUpdateAndRemoval(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg := hermesConflictPackage(t, base)
	fixture := newHermesSDKFixture(t, filepath.Join(base, "project"), user, pkg)
	fixture.apply(t, installer.OpInstall, "install")
	config := filepath.Join(fixture.project, ".hermes", "uap-session", "config.yaml")
	changed := strings.ReplaceAll(claudeSDKBytes(t, config), "127.0.0.1:9/owned", "foreign.test/changed")
	claudeSDKWrite(t, config, changed)
	before := fixture.state(t)
	for _, operation := range []installer.Operation{installer.OpUpdate, installer.OpRemove} {
		fixture.request.Operation, fixture.request.OperationID = operation, "refuse-edited-"+string(operation)
		handle, err := fixture.engine.Prepare(t.Context(), fixture.request)
		if handle != nil {
			if closeErr := handle.Close(); closeErr != nil {
				t.Error(closeErr)
			}
		}
		if err == nil || !reflect.DeepEqual(before, fixture.state(t)) || claudeSDKBytes(t, config) != changed || fixture.runner.calls != 0 {
			t.Fatalf("%s adopted or changed edited entry: %v", operation, err)
		}
	}
}
