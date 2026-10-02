package vscodelocalhooks_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	hooks "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

// Red: macOS is refused, writes linux/default/bash, or accepts Windows facts.
// Selection follows frozen hookSchema.ts: macOS uses osx, never linux.
func TestMacNativePlatformContract(t *testing.T) {
	target := hooks.Target{Shell: hooks.MacOSSH}
	specs := []hooks.Spec{stop(), {Event: hooks.SubagentStop, Executable: "/TEST/runtime", TimeoutSeconds: 5}}
	body, err := hooks.Render(target, specs)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string][]map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got["hooks"]) != 2 {
		t.Fatalf("unexpected artifact: %s", body)
	}
	for _, event := range []string{"Stop", "SubagentStop"} {
		entries := got["hooks"][event]
		if len(entries) != 1 {
			t.Fatalf("unexpected entries: %s", body)
		}
		entry := entries[0]
		if len(entry) != 3 || entry["type"] != "command" || entry["timeout"] != float64(5) || entry["osx"] == nil {
			t.Fatalf("incorrect macOS fields: %s", body)
		}
	}
	if err := hooks.VerifyOwned(body, target, specs); err != nil {
		t.Fatal(err)
	}
	for _, other := range []hooks.Target{linux(), windows()} {
		otherSpecs := specs
		if other.Shell == hooks.WindowsPowerShell51 {
			otherSpecs = []hooks.Spec{{Event: hooks.Stop, Executable: `C:\TEST\runtime.exe`, TimeoutSeconds: 5}}
		}
		if err := hooks.VerifyOwned(body, other, otherSpecs); !errors.Is(err, hooks.ErrNotOwned) {
			t.Fatal("cross-platform ownership accepted", err)
		}
	}
	for _, field := range []string{"linux", "windows", "command", "bash"} {
		drift := []byte(strings.ReplaceAll(string(body), `"osx":`, `"`+field+`":`))
		if err := hooks.VerifyOwned(drift, target, specs); !errors.Is(err, hooks.ErrNotOwned) {
			t.Fatal("alternate platform accepted", field, err)
		}
	}
	for _, bad := range []hooks.Target{
		{Shell: target.Shell, ComSpec: `C:\Windows\System32\cmd.exe`},
		{Shell: target.Shell, SystemRoot: `C:\Windows`},
	} {
		command, err := hooks.RenderArgv(bad, "/TEST/runtime", nil)
		if !errors.Is(err, hooks.ErrUnsupported) || command != "" {
			t.Fatal("Windows snapshot accepted on Mac", err)
		}
	}
}

// Red: the newly admitted Mac target bypasses any existing POSIX refusal or
// limit. Exercise its public boundary without introducing another quote engine.
func TestMacFixedLiteralRefusals(t *testing.T) {
	target := hooks.Target{Shell: hooks.MacOSSH}
	for _, executable := range []string{"", "relative", "/", "/TEST/"} {
		if _, err := hooks.RenderArgv(target, executable, nil); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatal("Mac executable accepted", err)
		}
	}
	for _, value := range []string{"nul\x00", "line\n", "return\r", string([]byte{0xff}), strings.Repeat("x", hooks.MaxLiteralBytes+1)} {
		if _, err := hooks.RenderArgv(target, "/TEST/runtime", []string{value}); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatal("Mac literal accepted", err)
		}
	}
	for _, value := range []string{"${PLUGIN_ROOT}", "${PLUGIN_DATA}", "${CLAUDE_PLUGIN_ROOT}"} {
		if _, err := hooks.RenderArgv(target, "/TEST/runtime", []string{value}); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatal("Mac placeholder accepted", err)
		}
	}
	if _, err := hooks.RenderArgv(target, "/TEST/runtime", make([]string, hooks.MaxArgs+1)); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("Mac argv bound bypassed", err)
	}
	args := make([]string, hooks.MaxArgs)
	for i := range args {
		args[i] = strings.Repeat("x", hooks.MaxLiteralBytes)
	}
	if _, err := hooks.RenderArgv(target, "/TEST/runtime", args); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("Mac command bound bypassed", err)
	}
	spec := stop()
	spec.Args = make([]string, 7)
	for i := range spec.Args {
		spec.Args[i] = strings.Repeat("\x01", hooks.MaxLiteralBytes)
	}
	if _, err := hooks.Render(target, []hooks.Spec{spec}); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("Mac document bound bypassed", err)
	}
}
