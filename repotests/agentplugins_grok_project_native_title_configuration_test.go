//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestGrokProjectTitleConfigurationFreezesBothHeldRoutesInOriginalHome(t *testing.T) {
	for key, value := range map[string]string{
		"UAP_GROK_TITLE_PROVIDER_KEY": "ambient-title-private", "UAP_GROK_TEST_PROVIDER_KEY": "ambient-main-private",
		"GROK_SESSION_SUMMARY_MODEL": "ambient-model", "GROK_TITLE_REFRESH": "false", "HOME": "/ambient-user-home",
	} {
		t.Setenv(key, value)
	}
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "grok"}, ctx: ctx, cancel: cancel, started: time.Now(),
		base: &nativeFixture{Root: root, Home: filepath.Join(root, "original-home"), Project: filepath.Join(root, "project"),
			CodexHome: filepath.Join(root, "original-codex")}, protected: map[string]string{}, evidence: map[string]any{}}
	for _, path := range []string{f.base.Home, f.base.Project} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if f.grok != nil {
			if f.grok.title != nil {
				f.grok.title.transport.stop()
			}
			f.grok.stop()
		}
		cancel()
	})
	f.configureGrokTitleProvider(t)
	main, title := f.grok, f.grok.title.transport
	if main.key == title.key || main.listener.Addr().String() == title.listener.Addr().String() {
		t.Fatal("title and primary must hold distinct listener/key authority")
	}
	model := func(name, address, key string) map[string]any {
		return map[string]any{"model": name, "base_url": "http://" + address + "/v1", "env_key": key,
			"api_backend": "chat_completions", "supports_backend_search": false, "supports_reasoning_effort": false, "max_retries": int64(0)}
	}
	want := map[string]any{
		"models": map[string]any{"default": "uap-grok-fixture", "session_summary": "uap-grok-title-fixture"},
		"model": map[string]any{
			"uap-grok-fixture":       model("uap-grok-fixture", main.listener.Addr().String(), "UAP_GROK_TEST_PROVIDER_KEY"),
			"uap-grok-title-fixture": model("uap-grok-title-fixture", title.listener.Addr().String(), "UAP_GROK_TITLE_PROVIDER_KEY"),
		},
		"marketplace": map[string]any{"default_skills_installs_purged": true},
		"features":    map[string]any{"turn_summary": false},
		"plugins":     map[string]any{"enabled": []any{"uap-grok-attempt"}},
	}
	parsed, status := grokConfigParse(main.authoredConfig)
	if status != "captured" || !reflect.DeepEqual(parsed, want) {
		t.Fatal("independently stated two-model original HOME configuration differs")
	}
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, main.authoredConfig) || f.protected[path] != nativeSHA(t, path) ||
		bytes.Contains(body, []byte(main.key)) || bytes.Contains(body, []byte(title.key)) {
		t.Fatal("config was not frozen once from the complete independently authored recipe")
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".grok", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("project replaced original HOME configuration")
	}
	env := grokContractEnvironment(t, f.env())
	if env["HOME"] != f.base.Home || env["UAP_GROK_TEST_PROVIDER_KEY"] != main.key || env["UAP_GROK_TITLE_PROVIDER_KEY"] != title.key || env["GROK_SAMPLER_SHARED_CLIENT"] != "0" {
		t.Fatal("held original HOME, two keys or existing client optout differs")
	}
	for _, key := range []string{"GROK_SESSION_SUMMARY_MODEL", "GROK_TITLE_REFRESH"} {
		if _, exists := env[key]; exists {
			t.Fatal("ambient auxiliary model/refresh setting entered the child")
		}
	}
	for _, client := range []string{"qoder", "kiro", "windsurf"} {
		f.spec.client = client
		other := grokContractEnvironment(t, f.env())
		if _, exists := other["UAP_GROK_TITLE_PROVIDER_KEY"]; exists {
			t.Fatal("another client inherited title fixture authority")
		}
	}
	if os.Getenv("UAP_GROK_TITLE_PROVIDER_KEY") != "ambient-title-private" || os.Getenv("GROK_SESSION_SUMMARY_MODEL") != "ambient-model" {
		t.Fatal("constructing fixture configuration changed ambient environment")
	}
}
