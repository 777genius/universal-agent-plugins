package sourceacquisition

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Historical APM #2368 dropped Git hardening when auth configuration replaced
// an existing GIT_CONFIG_COUNT. UAP's native path must enforce both settings
// as command-scoped arguments, not merge or trust inherited config indices.
func TestNativeGitOptionsRetainMandatoryHardening(t *testing.T) {
	options := nativeGitOptions(t.TempDir())
	values := map[string]string{}
	for i := 0; i < len(options); i += 2 {
		if i+1 >= len(options) || options[i] != "-c" {
			t.Fatalf("native Git option at position %d is not a -c pair: %v", i, options)
		}
		key, value, ok := strings.Cut(options[i+1], "=")
		if !ok || key == "" {
			t.Fatalf("invalid Git config argument %q", options[i+1])
		}
		if _, duplicate := values[key]; duplicate {
			t.Fatalf("duplicated Git option %q", key)
		}
		values[key] = value
	}
	for key, expected := range map[string]string{
		"safe.bareRepository": "explicit",
		"credential.interactive": "never",
		"core.fsmonitor": "false",
		"fetch.recurseSubmodules": "false",
		"protocol.allow": "never",
		"http.sslVerify": "true",
	} {
		if values[key] != expected {
			t.Fatalf("native Git %s=%q, want %q", key, values[key], expected)
		}
	}
}

// Execute the actual Git binary against a disposable workspace: later UAP
// command-scoped policy wins over unsafe earlier commandline defaults and
// inherited GIT_CONFIG_COUNT injection. This covers both Unix and Windows.
func TestNativeGitHardeningWinsOverInheritedGitSettings(t *testing.T) {
	root := t.TempDir()
	for key, expected := range map[string]string{
		"safe.bareRepository": "explicit",
		"credential.interactive": "never",
	} {
		t.Run(key, func(t *testing.T) {
			args := append([]string{
				"-c", "safe.bareRepository=all",
				"-c", "credential.interactive=always",
			}, nativeGitOptions(root)...)
			args = append(args, "config", "--get", key)
			cmd := exec.CommandContext(t.Context(), "git", args...)
			cmd.Dir = root
			cmd.Env = nativeGitEnvironment(append(os.Environ(),
				"GIT_CONFIG_COUNT=1",
				"GIT_CONFIG_KEY_0=safe.bareRepository",
				"GIT_CONFIG_VALUE_0=all",
			))
			output, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(output)) != expected {
				t.Fatalf("native Git effective %s: output=%q error=%v", key, output, err)
			}
		})
	}
}
