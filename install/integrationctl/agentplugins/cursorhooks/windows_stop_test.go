package cursorhooks_test

import (
	"bytes"
	"errors"
	"testing"

	ch "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
)

func TestWindowsFixedStopGrammarAndOwnership(t *testing.T) {
	spec := ch.HookSpec{Executable: `C:\TEST observer.exe`, Selector: `C:\TEST binding's selector.json`}
	got, err := ch.RenderArgv(ch.WindowsUserPowerShell20261001, []string{spec.Executable, "cursor-event", "stop", "--binding", spec.Selector})
	want := `& 'C:\TEST observer.exe' 'cursor-event' 'stop' '--binding' 'C:\TEST binding''s selector.json'`
	if err != nil || got != want {
		t.Fatalf("fixed Stop command: %q, %v", got, err)
	}
	foreign := []byte(`{"version":1,"hooks":{"stop":[{"command":"TEST foreign"}]}}`)
	req := ch.Request{Document: foreign, Operation: ch.Install, Specs: []ch.HookSpec{spec}, Shell: ch.WindowsUserPowerShell20261001, ExecutableVerified: true}
	installed, err := ch.Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Document = installed.Desired
	req.Operation = ch.Update
	req.Previous = installed.Receipt
	repeated, err := ch.Plan(req)
	if err != nil || !repeated.NoOp {
		t.Fatalf("repeat: %v", err)
	}
	req.Operation = ch.Remove
	removed, err := ch.Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(removed.Desired, []byte(`"command":"TEST foreign"`)) {
		t.Fatal("foreign Stop changed or disappeared")
	}
	// A fresh install after removal must recreate the same ownership digest.
	req.Document = removed.Desired
	req.Operation = ch.Install
	req.Previous = nil
	again, err := ch.Plan(req)
	if err != nil || again.Receipt.EntryDigest != installed.Receipt.EntryDigest {
		t.Fatalf("ownership roundtrip: %v", err)
	}
}

func TestWindowsRejectsAmbiguousPathsAndOtherArgv(t *testing.T) {
	good := []string{`C:\TEST.exe`, "cursor-event", "stop", "--binding", `C:\TEST\binding.json`}
	for _, bad := range []string{`C:relative.exe`, `C:\`, `\\server\share\TEST.exe`, `\\?\C:\TEST.exe`, `C:\TEST\..\other.exe`, `C:\TEST\\other.exe`, `C:\TEST.exe:stream`, `C:\NUL.exe`, `C:\COM1\TEST.exe`, `C:\TEST.\other.exe`, `C:\TEST \other.exe`, `C:/TEST.exe`, `C:\$env.exe`, `C:\TEST` + "`" + `.exe`, `C:\TEST.cmd`} {
		args := append([]string(nil), good...)
		args[0] = bad
		if _, err := ch.RenderArgv(ch.WindowsUserPowerShell20261001, args); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatalf("unsafe executable accepted: %q", bad)
		}
	}
	for _, args := range [][]string{good[:4], append(good, "extra"), {good[0], "other-event", "stop", "--binding", good[4]}} {
		if _, err := ch.RenderArgv(ch.WindowsUserPowerShell20261001, args); !errors.Is(err, ch.ErrUnsupported) {
			t.Fatalf("unqualified argv accepted: %q", args)
		}
	}
	args := append([]string(nil), good...)
	args[4] = `C:\TEST\binding.json:stream`
	if _, err := ch.RenderArgv(ch.WindowsUserPowerShell20261001, args); !errors.Is(err, ch.ErrUnsupported) {
		t.Fatal("selector ADS accepted")
	}
}

func TestWindowsRejectsSmartQuoteCommandDelimiters(t *testing.T) {
	for _, quote := range []string{"‘", "’"} {
		for _, index := range []int{0, 4} {
			args := []string{`C:\TEST.exe`, "cursor-event", "stop", "--binding", `C:\TEST\binding.json`}
			args[index] = `C:\TEST\binding` + quote + ` ; Write-Output TEST; #.exe`
			if _, err := ch.RenderArgv(ch.WindowsUserPowerShell20261001, args); !errors.Is(err, ch.ErrUnsupported) {
				t.Fatalf("PowerShell smart delimiter accepted at argv %d: %q", index, args[index])
			}
		}
	}
}
