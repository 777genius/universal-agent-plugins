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

func grokTurnSummaryContractFixture(t *testing.T) *projectNativeCLIAttempt {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "grok"}, ctx: ctx, cancel: cancel, started: time.Now(),
		base: &nativeFixture{Root: root, Home: filepath.Join(root, "original-home"), Project: filepath.Join(root, "project"),
			CodexHome: filepath.Join(root, "original-codex")}, protected: map[string]string{}, evidence: map[string]any{},
		files: map[string][]byte{"native-attempt.stdout.log": {}, "native-attempt.stderr.log": {}, "native-attempt.command.json": []byte(`{"inert":true}`)},
		used:  map[string]bool{"native-attempt": true}}
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
	return f
}

func TestGrokTurnSummarySettingIsFrozenOnceInCompleteOriginalHomeRecipe(t *testing.T) {
	f := grokTurnSummaryContractFixture(t)
	want := append(f.grok.title.configuration(f.grok.configuration()), []byte("\n[features]\nturn_summary = false\n")...)
	parsed, status := grokConfigParse(f.grok.authoredConfig)
	if status != "captured" || !reflect.DeepEqual(parsed["features"], map[string]any{"turn_summary": false}) ||
		!bytes.Equal(f.grok.authoredConfig, want) || bytes.Count(want, []byte("[features]")) != 1 {
		t.Fatal("fixed native boolean suffix changed the complete two-model recipe")
	}
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	body, err := os.ReadFile(path)
	info, statErr := os.Lstat(path)
	if err != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 ||
		!bytes.Equal(body, want) || len(f.protected) != 1 || f.protected[path] != grokConfigSHA(want) {
		t.Fatal("complete recipe was not the original ordinary protected config")
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".grok", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("feature setting acquired project configuration authority")
	}
	f.check(t)
}

func TestGrokTurnSummaryRecipeChangesRemainObservedRefusalsWithoutAdoption(t *testing.T) {
	for _, kind := range []string{"true", "quoted", "duplicate-table", "duplicate-key", "extra-feature", "sampling-table"} {
		t.Run(kind, func(t *testing.T) {
			f := grokTurnSummaryContractFixture(t)
			original := bytes.Clone(f.grok.authoredConfig)
			body := bytes.Clone(original)
			switch kind {
			case "true":
				body = bytes.Replace(body, []byte("turn_summary = false"), []byte("turn_summary = true"), 1)
			case "quoted":
				body = bytes.Replace(body, []byte("turn_summary = false"), []byte(`turn_summary = "false"`), 1)
			case "duplicate-table":
				body = append(body, []byte("\n[features]\nturn_summary = false\n")...)
			case "duplicate-key":
				body = append(body, []byte("turn_summary = true\n")...)
			case "extra-feature":
				body = append(body, []byte("ordinary_extra = true\n")...)
			case "sampling-table":
				body = bytes.Replace(body, []byte("[features]"), []byte("[turn_summary]"), 1)
			}
			originalValue, originalStatus := grokConfigParse(original)
			changedValue, changedStatus := grokConfigParse(body)
			captured := kind != "duplicate-table" && kind != "duplicate-key"
			if originalStatus != "captured" || (changedStatus == "captured") != captured ||
				captured && reflect.DeepEqual(originalValue, changedValue) || !captured && changedValue != nil {
				t.Fatal("changed native boolean recipe escaped complete-map or strict TOML distinction")
			}
			path := filepath.Join(f.base.Home, ".grok", "config.toml")
			before := f.protected[path]
			grokConfigRewrite(t, path, body)
			f.observeGrokConfigMutation(path, before, nativeSHA(t, path))
			observation, ok := f.evidence["grok_config_mutation"].(grokConfigObservation)
			if !ok || bytes.Equal(body, original) || f.protected[path] != before || !bytes.Equal(f.grok.authoredConfig, original) {
				t.Fatal("changed feature data renewed original authority")
			}
			if observation.Status != "recipe-mismatch" || observation.Comparison != nil || bytes.Equal(original, f.grok.configuration()) {
				t.Fatal("changed feature data bypassed the existing finite diagnostic refusal")
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, body) || nativeSHA(t, path) == before {
				t.Fatal("diagnostic rewrote observed bytes or adopted their digest")
			}
		})
	}
}

func TestGrokTurnSummaryAmbientOverridesCannotEnterOriginalFixture(t *testing.T) {
	for key, value := range map[string]string{
		"GROK_TURN_SUMMARY": "true", "GROK_TITLE_REFRESH": "true", "GROK_SESSION_SUMMARY_MODEL": "ambient-model",
		"HOME": "/ambient-home", "UAP_GROK_TEST_PROVIDER_KEY": "ambient-main", "UAP_GROK_TITLE_PROVIDER_KEY": "ambient-title",
	} {
		t.Setenv(key, value)
	}
	f := grokTurnSummaryContractFixture(t)
	env := grokContractEnvironment(t, f.env())
	for _, key := range []string{"GROK_TURN_SUMMARY", "GROK_TITLE_REFRESH", "GROK_SESSION_SUMMARY_MODEL"} {
		if _, exists := env[key]; exists {
			t.Fatal("ambient feature or model setting entered native child environment")
		}
	}
	if env["HOME"] != f.base.Home || env["UAP_GROK_TEST_PROVIDER_KEY"] != f.grok.key ||
		env["UAP_GROK_TITLE_PROVIDER_KEY"] != f.grok.title.transport.key || env["GROK_SAMPLER_SHARED_CLIENT"] != "0" {
		t.Fatal("supported feature setting changed original HOME or held route authority")
	}
	for _, client := range []string{"qoder", "kiro", "windsurf"} {
		f.spec.client = client
		other := grokContractEnvironment(t, f.env())
		for _, key := range []string{"GROK_TURN_SUMMARY", "GROK_TITLE_REFRESH", "UAP_GROK_TEST_PROVIDER_KEY", "UAP_GROK_TITLE_PROVIDER_KEY"} {
			if _, exists := other[key]; exists {
				t.Fatal("another client inherited Grok feature or fixture authority")
			}
		}
	}
	if os.Getenv("GROK_TURN_SUMMARY") != "true" || os.Getenv("HOME") != "/ambient-home" || os.Getenv("GROK_SESSION_SUMMARY_MODEL") != "ambient-model" {
		t.Fatal("constructing fixture configuration changed parent environment")
	}
}
