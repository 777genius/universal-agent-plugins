//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func grokContractEnvironment(t *testing.T, entries []string) map[string]string {
	t.Helper()
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, ok := strings.Cut(entry, "=")
		if _, duplicate := values[key]; !ok || key == "" || duplicate {
			t.Fatal("fixture environment contains an invalid or duplicate entry")
		}
		values[key] = value
	}
	return values
}

func assertGrokContractEnvironment(t *testing.T, f *projectNativeCLIAttempt) {
	t.Helper()
	values := grokContractEnvironment(t, f.env())
	for key, want := range map[string]string{
		"HOME": f.base.Home, "CODEX_HOME": f.base.CodexHome,
		"AGENTPLUGINS_HOME": filepath.Join(f.base.Root, "installer-state"),
		"NO_COLOR":          "1", "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.DevNull, "GIT_TERMINAL_PROMPT": "0",
	} {
		if values[key] != want {
			t.Fatalf("held environment field %s changed", key)
		}
	}
	for _, key := range []string{"GROK_DISABLE_AUTOUPDATER", "GROK_SAMPLER_SHARED_CLIENT", "UAP_GROK_TEST_PROVIDER_KEY", "UAP_GROK_AMBIENT_SECRET"} {
		want, exists := "", false
		if key == "GROK_DISABLE_AUTOUPDATER" && f.spec.client == "grok" {
			want, exists = "1", true
		} else if f.spec.client == "grok" && f.grok != nil {
			switch key {
			case "GROK_SAMPLER_SHARED_CLIENT":
				want, exists = "0", true
			case "UAP_GROK_TEST_PROVIDER_KEY":
				want, exists = f.grok.key, true
			}
		}
		if value, present := values[key]; value != want || present != exists {
			t.Fatalf("client/provider routing or ambient isolation changed for %s", key)
		}
	}
}

func TestGrokProjectNativeSharedClientOptOutRequiresHeldFixtureProvider(t *testing.T) {
	for key, value := range map[string]string{
		"GROK_SAMPLER_SHARED_CLIENT": "true", "UAP_GROK_TEST_PROVIDER_KEY": "ambient-provider-key",
		"GROK_DISABLE_AUTOUPDATER": "0", "UAP_GROK_AMBIENT_SECRET": "ambient-private-value", "HOME": "/ambient-user-home",
	} {
		t.Setenv(key, value)
	}
	root := t.TempDir()
	for _, client := range []string{"grok", "qoder", "kiro", "windsurf"} {
		for _, held := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/provider-%t", client, held), func(t *testing.T) {
				f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: client}, bin: filepath.Join(root, "bin"),
					base: &nativeFixture{Root: root, Home: filepath.Join(root, "original-home"),
						Project: filepath.Join(root, "project"), CodexHome: filepath.Join(root, "original-codex")}}
				if held {
					f.grok = &grokProjectProvider{key: "held-fixture-key"}
				}
				assertGrokContractEnvironment(t, f)
			})
		}
	}
	if os.Getenv("GROK_SAMPLER_SHARED_CLIENT") != "true" || os.Getenv("UAP_GROK_TEST_PROVIDER_KEY") != "ambient-provider-key" {
		t.Fatal("constructing a child environment mutated process-global settings")
	}
}
