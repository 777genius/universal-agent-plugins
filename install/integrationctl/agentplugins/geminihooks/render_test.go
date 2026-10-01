package geminihooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

// Red condition: a shell-quoted fixed argv changes under Gemini settings
// expansion, or a native child sees split/expanded/missing literal arguments.
// The test executable is only an argv probe, never an agent/runtime/project.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--geminihooks-argv-probe" {
		_ = json.NewEncoder(os.Stdout).Encode(os.Args[2:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func expanded(t *testing.T, command string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node >=22.18 required for native resolver fixture:", err)
	}
	input, _ := json.Marshal(map[string]string{"command": command})
	cmd := exec.Command(node, "testdata/expand.mjs")
	cmd.Stdin = bytes.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native resolver: %v, %s", err, out)
	}
	var result string
	if err = json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func probe(t *testing.T) string {
	t.Helper()
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Executable path itself exercises spaces, Unicode, apostrophe and shell
	// punctuation. Each platform's filesystem supports these chosen characters.
	dir := filepath.Join(t.TempDir(), "native café ' & ! % `")
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	name := "argv probe"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	dst := filepath.Join(dir, name)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(dst, b, 0700); err != nil {
		t.Fatal(err)
	}
	return dst
}

func assertArgv(t *testing.T, shell gh.Shell, executable string, prefix []string, args []string) {
	t.Helper()
	argv := append([]string{probe(t), "--geminihooks-argv-probe"}, args...)
	command, err := gh.RenderArgv(shell, argv)
	if err != nil {
		t.Fatal(err)
	}
	effective := expanded(t, command)
	if effective != command {
		t.Fatal("settings expansion changed supported invocation")
	}
	cmd := exec.Command(executable, append(prefix, effective)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native shell: %v, %s", err, out)
	}
	var actual []string
	if err = json.Unmarshal(out, &actual); err != nil {
		t.Fatalf("probe output %q: %v", out, err)
	}
	if !reflect.DeepEqual(actual, args) {
		t.Fatalf("actual argv %q; want %q", actual, args)
	}
	t.Logf("actual %s child argv: %q", executable, actual)
}

func literalArgs() []string {
	return []string{"space inside", "café 日本語", "apostrophe's", "back`tick", "& ampersand", "bang!", "percent%PATH%", ";|><*?()[]{}\\", "-leading", "$", "$(never-run)"}
}

func TestBashNativeExpansionThenArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix native shell case")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	args := append(literalArgs(), "", `double"quote`)
	assertArgv(t, gh.Bash, bash, []string{"-c"}, args)
}

func TestPowerShellNativeExpansionThenArgv(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows native CI only; Linux bash is not Windows proof")
	}
	tested := false
	for _, name := range []string{"pwsh.exe", "powershell.exe"} {
		t.Run(name, func(t *testing.T) {
			executable, err := exec.LookPath(name)
			if err != nil {
				t.Skip("native executable unavailable")
			}
			tested = true
			assertArgv(t, gh.PowerShell, executable, []string{"-NoProfile", "-NonInteractive", "-Command"}, literalArgs())
		})
	}
	if !tested {
		t.Fatal("Windows host has no Gemini-supported PowerShell")
	}
}

func TestNativeExpansionNegativeControlAndUnsupportedTokens(t *testing.T) {
	// Naive bash quoting protects the shell but cannot stop the actual native
	// settings resolver. This is a behavioral red control, not a code snapshot.
	if runtime.GOOS != "windows" {
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
		command, err := gh.RenderArgv(gh.Bash, []string{probe(t), "--geminihooks-argv-probe"})
		if err != nil {
			t.Fatal(err)
		}
		unsafe := command + " '$U2_RENDER_VAR' '${U2_MISSING:-native-default}'"
		cmd := exec.Command(bash, "-c", expanded(t, unsafe))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err)
		}
		var actual []string
		if err = json.Unmarshal(out, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, []string{"native-expanded", "native-default"}) {
			t.Fatalf("native expansion red control %q", actual)
		}
	}
	for _, shell := range []gh.Shell{gh.Bash, gh.PowerShell} {
		for _, token := range []string{"$U2_RENDER_VAR", "${U2_RENDER_VAR}", "${U2_MISSING:-default}", "${arbitrary name}", "x\x00y", "x\ny", "x\ry", string([]byte{0xff})} {
			t.Run(string(shell)+token, func(t *testing.T) {
				command, err := gh.RenderArgv(shell, []string{"probe", token})
				if command != "" || !errors.Is(err, gh.ErrConflict) {
					t.Fatalf("unsupported token returned usable command %q, %v", command, err)
				}
			})
		}
	}
	for _, argv := range [][]string{nil, {""}, {"probe", ""}, {"probe", `embedded"quote`}} {
		if _, err := gh.RenderArgv(gh.PowerShell, argv); !errors.Is(err, gh.ErrConflict) {
			t.Fatal("unsafe PowerShell legacy argument accepted")
		}
	}
	if _, err := gh.RenderArgv("cmd", []string{"probe"}); !errors.Is(err, gh.ErrConflict) {
		t.Fatal("cmd is not native Gemini hook shell")
	}
}

func TestSettingsInterpolationConflictDoesNotReturnMutation(t *testing.T) {
	installed := plan(t, []byte(foreign), gh.Install, specs(), nil)
	h := specs()
	h[0].Argv[0] = "/opt/$U2_RENDER_VAR/helper"
	req := gh.Request{Settings: installed.Desired, Shell: gh.Bash, Operation: gh.Update, Hooks: h, Previous: installed.Receipt}
	r, err := gh.Plan(req)
	if !errors.Is(err, gh.ErrConflict) || !strings.Contains(err.Error(), "interpolation") || !bytes.Equal(r.Desired, req.Settings) || r.Receipt != nil {
		t.Fatal("interpolation conflict mutated/adopted")
	}
}
