package observercontract_test

import (
	"strings"
	"testing"

	gh "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/geminihooks"
)

// Variables are confined to this on-disk fixture: Gemini interpolates the
// generated settings command, not script contents. Use only PS 5.1/7 APIs.
const consoleScript = `[IO.File]::WriteAllText($args[0], (ConvertTo-Json -InputObject ([string[]]$args[1..($args.Count-1)]) -Compress))
[Console]::Out.WriteLine('direct console stdout')
[Console]::Error.WriteLine('direct console stderr')
Write-Output 'pipeline output'
Write-Error 'pipeline error'
Write-Warning 'pipeline warning'
$VerbosePreference = 'Continue'
Write-Verbose 'pipeline verbose'
$DebugPreference = 'Continue'
Write-Debug 'pipeline debug'
Write-Information 'pipeline information' -InformationAction Continue
`

func assertConsoleInvocation(t *testing.T, shell nativeShell, executable string, failure string) {
	t.Helper()
	ending := ""
	if failure == "throw" {
		ending = "throw 'console fixture thrown failure'"
	}
	if failure == "terminating error" {
		ending = "Write-Error 'console fixture terminating failure' -ErrorAction Stop"
	}
	path, record := scriptFixture(t, ".ps1", consoleScript+ending)
	args := []string{"space inside", "apostrophe's", "literal$", "$", "$(never-run)", "café 日本語", "&;|><`!%", "-leading"}
	hook := gh.HookSpec{Event: "Notification", Name: "fixture.console", Argv: append([]string{path, record}, args...)}
	t.Run("naked command", func(t *testing.T) {
		clearRecordedArgv(t, record)
		stdout, stderr, err := runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
		if !strings.Contains(stdout, "direct console stdout") || !strings.Contains(stderr, "direct console stderr") ||
			(failure != "success" && !strings.Contains(stderr, "console fixture")) {
			t.Errorf("direct console/failure not executed: stdout=%q stderr=%q status=%v", stdout, stderr, err)
		}
		assertRecordedArgv(t, record, args)
	})
	t.Run("pipeline redirection control", func(t *testing.T) {
		clearRecordedArgv(t, record)
		invocation, err := gh.RenderArgv(shell.kind, hook.Argv)
		if err != nil {
			t.Fatal(err)
		}
		// All-stream redirection alone cannot capture direct Console writes.
		guard := "try { " + invocation + " *>&1 | Out-Null } catch {}; [Console]::Out.Write(\"{}`n\"); exit 0"
		stdout, stderr, err := runNative(t, shell, executable, guard)
		if err != nil || !strings.Contains(stdout, "direct console stdout") || !strings.Contains(stderr, "direct console stderr") {
			t.Errorf("pipeline-only redirection failed to reproduce Console leak: stdout=%q stderr=%q status=%v", stdout, stderr, err)
		}
		assertRecordedArgv(t, record, args)
	})
	t.Run("observer", func(t *testing.T) {
		clearRecordedArgv(t, record)
		hook.Observer = true
		stdout, stderr, err := runNative(t, shell, executable, plannedCommand(t, shell.kind, hook))
		assertNeutral(t, stdout, stderr, err)
		assertRecordedArgv(t, record, args)
	})
}
