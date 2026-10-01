package vscodelocalhooks_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	hooks "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

func linux() hooks.Target { return hooks.Target{Shell: hooks.LinuxSH} }

func windows() hooks.Target {
	return hooks.Target{Shell: hooks.WindowsPowerShell51, SystemRoot: `C:\Windows`, ComSpec: `C:\Windows\System32\cmd.exe`}
}

func stop() hooks.Spec {
	return hooks.Spec{Event: hooks.Stop, Executable: "/TEST/runtime", Args: []string{"local-stop"}, TimeoutSeconds: 5}
}

// Red: milliseconds, CLI/group shape, fallback or ambient cwd/env are emitted.
func TestRenderNativeFile(t *testing.T) {
	spec := stop()
	sub := hooks.Spec{Event: hooks.SubagentStop, Executable: "/TEST/runtime", TimeoutSeconds: 5}
	body, err := hooks.Render(linux(), []hooks.Spec{sub, spec})
	if err != nil {
		t.Fatal(err)
	}
	var got any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal([]byte(`{"hooks":{"Stop":[{"type":"command","linux":"'/TEST/runtime' 'local-stop'","timeout":5}],"SubagentStop":[{"type":"command","linux":"'/TEST/runtime'","timeout":5}]}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || hooks.PluginPath != "com.github.copilot/hooks/hooks.json" {
		t.Fatalf("unexpected native file: %s", body)
	}
	spec.Args[0] = "caller-mutated"
	if strings.Contains(string(body), "caller-mutated") {
		t.Fatal("rendered result retains caller memory")
	}
	if err := hooks.VerifyOwned(body, linux(), []hooks.Spec{sub, stop()}); err != nil {
		t.Fatal(err)
	}
}

// Prepared syntax only. This test deliberately executes no Windows shell.
func TestWindowsPreparedContract(t *testing.T) {
	spec := hooks.Spec{Event: hooks.Stop, Executable: `C:\TEST app\runtime.exe`, Args: []string{"it's $literal; [x] `tick", `back\slash`}, TimeoutSeconds: 5}
	body, err := hooks.Render(windows(), []hooks.Spec{spec})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]map[string][]map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"type": "command", "windows": "& 'C:\\TEST app\\runtime.exe' 'it''s $literal; [x] `tick' 'back\\slash'", "timeout": float64(5)}
	if !reflect.DeepEqual(got["hooks"]["Stop"][0], want) || len(got) != 1 {
		t.Fatalf("unexpected preparation: %s", body)
	}
	if err := hooks.VerifyOwned(body, windows(), []hooks.Spec{spec}); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseUnsupportedInputs(t *testing.T) {
	for _, target := range []hooks.Target{
		{}, {Shell: "pwsh7"}, {Shell: "macos-/bin/sh"},
		{Shell: hooks.LinuxSH, ComSpec: "cmd.exe"},
		{Shell: hooks.WindowsPowerShell51},
		{Shell: hooks.WindowsPowerShell51, SystemRoot: `C:\Windows`, ComSpec: "cmd.exe"},
		{Shell: hooks.WindowsPowerShell51, SystemRoot: `C:\Windows`, ComSpec: `D:\cmd.exe`},
		{Shell: hooks.WindowsPowerShell51, SystemRoot: `C:\Windows`, ComSpec: `C:\Windows\System32\pwsh.exe`},
		{Shell: hooks.WindowsPowerShell51, SystemRoot: `C:\Windows\..`, ComSpec: `C:\Windows\..\System32\cmd.exe`},
	} {
		if _, err := hooks.RenderArgv(target, "/TEST/runtime", nil); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatalf("target accepted or wrong error: %+v, %v", target, err)
		}
	}
	for _, value := range []string{"", "embedded\"quote", `trailing\`, "tab\t", "control\x01", "delete\x7f"} {
		if _, err := hooks.RenderArgv(windows(), `C:\TEST\runtime.exe`, []string{value}); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatalf("Windows argument accepted: %q, %v", value, err)
		}
	}
	for _, executable := range []string{"runtime.exe", `C:runtime.exe`, `\runtime.exe`, `\\server\runtime.exe`, `\\?\C:\runtime.exe`, `C:/runtime.exe`, `C:\TEST\script.cmd`, `C:\NUL.exe`, `C:\NUL .exe`, `C:\COM¹\runtime.exe`, `C:\TEST.\runtime.exe`, `C:\TEST\..\runtime.exe`} {
		if _, err := hooks.RenderArgv(windows(), executable, nil); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatalf("Windows executable accepted: %q, %v", executable, err)
		}
	}
	for _, executable := range []string{"", "runtime", "./runtime", "/", "/TEST/"} {
		if _, err := hooks.RenderArgv(linux(), executable, nil); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatalf("Unix executable accepted: %q, %v", executable, err)
		}
	}
}

// Red: CR/LF in either literal position is accepted, produces output, or leaks
// a supplied value through a classified refusal at any public boundary.
func TestRefuseLineBreakLiterals(t *testing.T) {
	for _, test := range []struct {
		name       string
		target     hooks.Target
		executable string
		args       []string
	}{
		{"LF argument", linux(), "/TEST/runtime", []string{"TEST-private-sentinel\nbreak"}},
		{"CR argument", linux(), "/TEST/runtime", []string{"TEST-private-sentinel\rreturn"}},
		{"LF executable", linux(), "/TEST/private-sentinel\ntime", nil},
		{"CR executable", linux(), "/TEST/private-sentinel\rtime", nil},
		{"Windows LF argument", windows(), `C:\TEST\runtime.exe`, []string{"newline\n"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := hooks.Spec{Event: hooks.Stop, Executable: test.executable, Args: test.args, TimeoutSeconds: 5}
			command, argvErr := hooks.RenderArgv(test.target, test.executable, test.args)
			body, renderErr := hooks.Render(test.target, []hooks.Spec{spec})
			verifyErr := hooks.VerifyOwned([]byte(authored), test.target, []hooks.Spec{spec})
			for _, err := range []error{argvErr, renderErr, verifyErr} {
				if !errors.Is(err, hooks.ErrInvalid) {
					t.Errorf("line break accepted or wrong classification: %v", err)
				} else if strings.Contains(err.Error(), "private-sentinel") || strings.Contains(err.Error(), "newline") {
					t.Error("refusal leaked supplied literal")
				}
			}
			if command != "" || len(body) != 0 {
				t.Error("refused literal produced executable output")
			}
		})
	}
}

func TestLiteralAndCommandBounds(t *testing.T) {
	for _, value := range []string{"nul\x00", string([]byte{0xff}), strings.Repeat("x", hooks.MaxLiteralBytes+1)} {
		if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", []string{value}); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatalf("invalid literal accepted: %q, %v", value[:1], err)
		}
	}
	for _, value := range []string{"${PLUGIN_ROOT}/runtime", "x${PLUGIN_DATA}y", "${CLAUDE_PLUGIN_ROOT}"} {
		if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", []string{value}); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatalf("native placeholder accepted: %q, %v", value, err)
		}
	}
	args := make([]string, hooks.MaxArgs)
	if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", args); err != nil {
		t.Fatal(err)
	}
	if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", append(args, "")); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("argument-count bound not enforced", err)
	}
	if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", []string{strings.Repeat("x", hooks.MaxLiteralBytes)}); err != nil {
		t.Fatal(err)
	}
	for i := range args {
		args[i] = strings.Repeat("x", hooks.MaxLiteralBytes)
	}
	if _, err := hooks.RenderArgv(linux(), "/TEST/runtime", args); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("command bound not enforced", err)
	}
	// Short raw commands can grow sixfold during JSON escaping.
	spec := stop()
	spec.Args = make([]string, 7)
	for i := range spec.Args {
		spec.Args[i] = strings.Repeat("\x01", hooks.MaxLiteralBytes)
	}
	if _, err := hooks.Render(linux(), []hooks.Spec{spec}); !errors.Is(err, hooks.ErrInvalid) {
		t.Fatal("encoded document bound not enforced", err)
	}
}

func TestSpecRefusal(t *testing.T) {
	for _, event := range []hooks.Event{"stop", "agentStop", "PreToolUse", ""} {
		spec := stop()
		spec.Event = event
		if _, err := hooks.Render(linux(), []hooks.Spec{spec}); !errors.Is(err, hooks.ErrUnsupported) {
			t.Fatal("unsupported event accepted", event, err)
		}
	}
	for _, timeout := range []int{0, -1, 4, 5000} {
		spec := stop()
		spec.TimeoutSeconds = timeout
		if _, err := hooks.Render(linux(), []hooks.Spec{spec}); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatal("unsupported timeout accepted", timeout, err)
		}
	}
	for _, specs := range [][]hooks.Spec{nil, {stop(), stop()}, {stop(), stop(), stop()}} {
		if _, err := hooks.Render(linux(), specs); !errors.Is(err, hooks.ErrInvalid) {
			t.Fatal("empty/duplicate/excess events accepted", err)
		}
	}
}
