package managedstdio

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func TestEncodedArgumentsCanonicalRoundTripAndRefusal(t *testing.T) {
	opaque := []string{"", ".", "./relative", "./../opaque", "~", "~/relative", "${UNKNOWN}", "雪", "a b", "\r\n", "--flag"}
	wire := EncodedArguments("/plugin", "/data", "/plugin", pathcontract.Plugin, "./child", opaque)
	if !reflect.DeepEqual(wire[:6], []string{Mode, "/plugin", "/data", "/plugin", "plugin", "--argv-base64-v1"}) || wire[6] != "Li9jaGlsZA==" {
		t.Fatal("encoded v1 fields differ")
	}
	got, err := DecodeCommandArguments(wire[5], wire[6:])
	if err != nil || !reflect.DeepEqual(got, append([]string{"./child"}, opaque...)) {
		t.Fatal("opaque child vector changed", err)
	}
	for _, bad := range [][]string{nil, {""}, {"?"}, {"Li9jaGlsZA==\n"}, {"Li9jaGlsZA==\r\n"}, {"Zh=="}, {"Li9jaGlsZA==", "ZgA="}} {
		if _, err := DecodeCommandArguments("--argv-base64-v1", bad); err == nil {
			t.Fatal("malformed/noncanonical/NUL encoded vector accepted")
		}
	}
	if _, err := DecodeCommandArguments("--unknown", []string{"child"}); err == nil {
		t.Fatal("unknown protocol separator accepted")
	}
	raw, err := DecodeCommandArguments("--", append([]string{"./child"}, opaque...))
	if err != nil || !reflect.DeepEqual(raw, append([]string{"./child"}, opaque...)) {
		t.Fatal("legacy raw vector changed")
	}
}

// Literal source model of pinned Cursor index.js Oa, not a vendor process.
func cursorNormalizedArguments(args []string, cwd, home string) []string {
	result := append([]string(nil), args...)
	for i, value := range result {
		switch {
		case value == "~" || strings.HasPrefix(value, "~/"):
			result[i] = filepath.Join(home, strings.TrimPrefix(value, "~"))
		case value == "." || strings.HasPrefix(value, "./"):
			result[i] = filepath.Join(cwd, value)
		}
	}
	return result
}

func TestEncodedExecPreservesCursorNormalizedOpaqueArguments(t *testing.T) {
	if !Supported() {
		t.Skip("unsupported native lifecycle")
	}
	for _, anchor := range []pathcontract.Anchor{pathcontract.Plugin, pathcontract.Data} {
		t.Run(string(anchor), func(t *testing.T) {
			plugin, data, clientCWD, home := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			root := plugin
			if anchor == pathcontract.Data {
				root = data
			}
			if err := os.WriteFile(filepath.Join(root, "relative.txt"), []byte("relative-ok"), 0600); err != nil {
				t.Fatal(err)
			}
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(exe)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(plugin, "child"), body, 0700); err != nil {
				t.Fatal(err)
			}
			opaque := []string{"", ".", "./relative", "./../opaque", "~", "~/relative", "${UNKNOWN}", "雪", "a b", "\r\n", "$(touch forbidden)", "--flag"}
			childArgs := append([]string{"-test.run=^TestSubprocess$", "fixture", "child"}, opaque...)
			raw := cursorNormalizedArguments(Arguments(plugin, data, root, anchor, "./child", childArgs), clientCWD, home)
			failed := fixtureCommand(t, raw)
			if output, err := failed.CombinedOutput(); err == nil || !bytes.Contains(output, []byte("command must be a bare executable or bundled path")) {
				t.Fatal("unencoded Cursor-normalized bundled command did not reproduce refusal", err)
			}
			wire := EncodedArguments(plugin, data, root, anchor, "./child", childArgs)
			transported := cursorNormalizedArguments(wire, clientCWD, home)
			if !reflect.DeepEqual(transported, wire) {
				t.Fatal("Cursor normalization altered encoded fields")
			}
			cmd := fixtureCommand(t, transported)
			cmd.Stdin = strings.NewReader("roundtrip\n")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err = cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 23 {
				t.Fatal("encoded child exit differs", err, stderr.String())
			}
			var got struct {
				CWD, File, Stdin, Env string
				Args                  []string
			}
			canonical, err := filepath.EvalSymlinks(root)
			if err != nil || json.Unmarshal(stdout.Bytes(), &got) != nil || got.CWD != canonical || got.File != "relative-ok" || got.Stdin != "roundtrip\n" || !reflect.DeepEqual(got.Args, opaque) || got.Env != "${PLUGIN_ROOT};$(untouched)" || stderr.String() != "child-stderr" {
				t.Fatal("encoded process changed cwd/argv/streams/environment")
			}
		})
	}
}

func TestEncodedArgumentsCannotRelaxCommandContainment(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	for _, command := range []string{"/bin/sh", "../escape", "./../escape", "./missing"} {
		cmd := fixtureCommand(t, EncodedArguments(root, data, root, pathcontract.Plugin, command, nil))
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 126 || stdout.Len() != 0 || stderr.Len() == 0 {
			t.Fatal("encoded command escaped unchanged path guard", err)
		}
	}
}
