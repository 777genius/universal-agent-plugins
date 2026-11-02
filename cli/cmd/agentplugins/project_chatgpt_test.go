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
		{"add", "acme/docs", "--target", "chatgpt"},
		{"add", source, "--no-save", "--target", "chatgpt"},
		{"add", "context7", "--target", "chatgpt", "--chatgpt-app-id", "plugin_asdk_app_0123456789abcdef0123456789abcdef"},
		{"add", source, "--no-save", "--target", "chatgpt", "--activation-complete", "--auth-complete"},
		{"add", source, "--no-save", "--target", "gemini,chatgpt"},
		{"install", source, "--no-save", "--target", "chatgpt"},
		{"update", "00000000-0000-4000-8000-000000000001", "--target", "chatgpt"},
		{"repair", "00000000-0000-4000-8000-000000000001", "--target", "chatgpt"},
		{"remove", "00000000-0000-4000-8000-000000000001", "--target", "chatgpt", "--external-uninstalled"},
	} {
		output, err := projectChatGPTExecute(app, a, args...)
		if err == nil || err.Error() != "chatgpt does not support project scope" || strings.Contains(output, "success") {
			t.Fatalf("unsupported project request %v: %v\n%s", args, err, output)
		}
	}
	t.Chdir(a)
	if _, err := projectChatGPTExecute(app, "", "add", source, "--target", "chatgpt", "--no-save"); err == nil || err.Error() != "chatgpt does not support project scope" {
		t.Fatalf("invocation-directory default bypassed project refusal: %v", err)
	}
	for index, root := range []string{app.UserHome, a, b} {
		if !reflect.DeepEqual(before[index], securityFixtureTree(t, root)) {
			t.Fatalf("unsupported project command changed user/A/B filesystem: %s", root)
		}
	}
	projectChatGPTAssertNoEffects(t, spy, user, a)
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
