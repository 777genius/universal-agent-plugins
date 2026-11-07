package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectChatGPTCLIRefusesAllTargetedIntentsBeforeEffects(t *testing.T) {
	app, user, a, b := projectCompositionFixture(t)
	spy := &projectChatGPTBoundaries{}
	app = projectChatGPTApp(app, spy)
	source := filepath.Join(t.TempDir(), "package")
	projectChatGPTWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"chatgpt-scope-proof"}`)
	for _, root := range []string{app.UserHome, a, b} {
		projectChatGPTWrite(t, filepath.Join(root, "foreign.txt"), "preserve bytes and mode\n")
	}
	before := []map[string]string{securityFixtureTree(t, app.UserHome), securityFixtureTree(t, a), securityFixtureTree(t, b)}
	for _, args := range [][]string{
		{"add", "acme/docs", "--target", "chatgptweb"},
		{"add", source, "--no-save", "--target", "chatgptweb"},
		{"add", "context7", "--target", "chatgptweb", "--chatgpt-app-id", "plugin_asdk_app_0123456789abcdef0123456789abcdef"},
		{"add", source, "--no-save", "--target", "chatgptweb", "--activation-complete", "--auth-complete"},
		{"add", source, "--no-save", "--target", "gemini,chatgptweb"},
		{"install", source, "--no-save", "--target", "chatgptweb"},
		{"update", "00000000-0000-4000-8000-000000000001", "--target", "chatgptweb"},
		{"repair", "00000000-0000-4000-8000-000000000001", "--target", "chatgptweb"},
		{"remove", "00000000-0000-4000-8000-000000000001", "--target", "chatgptweb", "--external-uninstalled"},
	} {
		output, err := projectChatGPTExecute(app, a, args...)
		if err == nil || err.Error() != "chatgpt does not support project scope" || strings.Contains(output, "success") {
			t.Fatalf("unsupported project request %v: %v\n%s", args, err, output)
		}
	}
	t.Chdir(a)
	if _, err := projectChatGPTExecute(app, "", "add", source, "--target", "chatgptweb", "--no-save"); err == nil || err.Error() != "chatgpt does not support project scope" {
		t.Fatalf("invocation-directory default bypassed project refusal: %v", err)
	}
	for index, root := range []string{app.UserHome, a, b} {
		if !reflect.DeepEqual(before[index], securityFixtureTree(t, root)) {
			t.Fatalf("unsupported project command changed user/A/B filesystem: %s", root)
		}
	}
	projectChatGPTAssertNoEffects(t, spy, user, a)
}

// The public chatgpt spelling means Desktop; unlike canonical chatgpt Web,
// it must fail before project acquisition rather than fall through to Codex.
func TestPublicChatGPTDesktopProjectAliasRefusesBeforeEffects(t *testing.T) {
	app, user, project, _ := projectCompositionFixture(t)
	spy := &projectChatGPTBoundaries{}
	app = projectChatGPTApp(app, spy)
	source := filepath.Join(t.TempDir(), "package")
	projectChatGPTWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"chatgpt-desktop-scope-proof"}`)
	before := securityFixtureTree(t, project)
	for _, args := range [][]string{
		{"add", source, "--no-save", "--target", "chatgpt"},
		{"install", source, "--no-save", "--target", "chatgpt"},
		{"update", "missing", "--target", "chatgpt"},
		{"repair", "missing", "--target", "chatgpt"},
		{"remove", "missing", "--target", "chatgpt"},
		{"skills", "add", "https://invalid.example/no-acquisition", "--skill", "docs", "--target", "chatgpt", "--yes"},
	} {
		out, err := projectChatGPTExecute(app, project, args...)
		if err == nil || !strings.Contains(err.Error(), "chatgpt (Desktop) supports only --scope user") || strings.Contains(out, "success") {
			t.Fatalf("Desktop project alias must refuse before effects %v: %s %v", args, out, err)
		}
	}
	if !reflect.DeepEqual(before, securityFixtureTree(t, project)) {
		t.Fatal("Desktop project alias changed project files")
	}
	projectChatGPTAssertNoEffects(t, spy, user, project)
}

func TestProjectChatGPTSavedTargetRefusesBeforeLockAndAcquisition(t *testing.T) {
	for _, args := range [][]string{{"deps", "lock"}, {"deps", "update", "docs"}, {"deps", "sync", "--yes"}, {"install", "--yes"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, user, a, _ := projectCompositionFixture(t)
			spy := &projectChatGPTBoundaries{}
			app = projectChatGPTApp(app, spy)
			projectChatGPTWrite(t, filepath.Join(a, "uap.toml"), "schema_version = 1\n[dependencies.docs]\ngit = \"https://github.com/acme/docs.git\"\nref = \"main\"\n[targets]\nchatgpt = [\"docs\"]\n")
			before := securityFixtureTree(t, a)
			output, err := projectChatGPTExecute(app, a, args...)
			if err == nil {
				t.Fatalf("saved unsupported project target succeeded: %s", output)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, a)) {
				t.Fatal("unsupported saved target created a namespace/lock or changed intent")
			}
			if err.Error() != "chatgpt does not support project scope" || strings.Contains(output, "success") {
				t.Fatalf("saved unsupported project target guidance: %v\n%s", err, output)
			}
			projectChatGPTAssertNoEffects(t, spy, user, a)
		})
	}
}

func projectChatGPTAssertNoEffects(t *testing.T, spy *projectChatGPTBoundaries, user, project string) {
	t.Helper()
	if spy.acquisition != 0 || spy.detection != 0 || spy.security != 0 {
		t.Fatalf("unsupported project crossed host boundaries: %+v", spy)
	}
	for _, path := range []string{user, filepath.Join(project, ".uap"), filepath.Join(project, "uap.lock")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unsupported project request created %s: %v", path, err)
		}
	}
}
