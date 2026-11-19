package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Ordinary machine problems must be explained in plain words, from the typed
// cause that survives wrapping, and must never leak technical detail.
func TestEnvironmentFailuresGetPlainCauses(t *testing.T) {
	t.Parallel()
	secretPath := "/Users/someone/project/.claude/skills"
	cases := map[string]struct {
		err   error
		kind  failureKind
		words string
	}{
		"permission":  {&fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EACCES}, failurePermission, "not allowed to write"},
		"os-sentinel": {fmt.Errorf("create client target root: %w", os.ErrPermission), failurePermission, "not allowed to write"},
		"disk full":   {fmt.Errorf("write: %w", &fs.PathError{Op: "write", Path: secretPath, Err: syscall.ENOSPC}), failureDiskFull, "free disk space"},
		"read only":   {fmt.Errorf("write: %w", &fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EROFS}), failureReadOnly, "read-only"},
		"network":     {fmt.Errorf("fetch: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("no route to host")}), failureNetwork, "network is unavailable"},
		"no program":  {fmt.Errorf("probe: %w", &exec.Error{Name: "claude", Err: exec.ErrNotFound}), failureAgentMissing, "not found"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failure := describeBindingFailure(environment.BindingOutcome{Status: environment.BindingFailed, Err: tc.err})
			if failure.Kind != tc.kind || !failure.Retryable || !strings.Contains(failure.Summary, tc.words) {
				t.Fatalf("got %+v", failure)
			}
			if strings.Contains(failure.Summary+failure.Next, secretPath) {
				t.Fatalf("the terminal wording must not carry technical detail: %+v", failure)
			}
		})
	}
}

// A real cancellation or deadline keeps its own wording even when the wrapped
// chain also contains a network error.
func TestEnvironmentClassificationDoesNotOverrideTimeoutOrCancel(t *testing.T) {
	t.Parallel()
	timeoutDial := &net.OpError{Op: "dial", Err: context.DeadlineExceeded}
	if got := failureKindOf(fmt.Errorf("x: %w", timeoutDial)); got != failureTimeout {
		t.Fatalf("deadline must win over network: %s", got)
	}
	if got := failureKindOf(fmt.Errorf("x: %w", context.Canceled)); got != failureCanceled {
		t.Fatalf("cancel must win: %s", got)
	}
	if got := failureKindOf(errors.New("something else")); got != failureUnknown {
		t.Fatalf("unrelated errors stay generic: %s", got)
	}
}

// A group refused before any client changed because of a fixable machine
// problem keeps the technical chain in the log and shows only the plain cause,
// without offering an automatic retry (nothing was applied).
func TestRefusedGroupExplainsFixableCauseAndLogsDetail(t *testing.T) {
	secret := "/Users/someone/project/.claude/skills"
	cause := fmt.Errorf("create client target root: %w", &fs.PathError{Op: "mkdir", Path: secret, Err: syscall.EACCES})
	for _, status := range []string{"planned", "preparation_failed"} {
		t.Run(status, func(t *testing.T) {
			var offered []string
			app, _ := retryAppAndOptions(t, &offered)
			app.InstallLogDir = filepath.Join(t.TempDir(), "logs")
			opts := &options{format: "human"}
			cmd, _, errOut := retryTestCommand(t)
			combined := addMultiResult{Status: status, Plugin: "demo", Targets: []addTargetResult{{Target: "claude"}, {Target: "gemini"}}}
			got := app.recordGroupOutcome(cmd, opts, &combined, cause)
			if !errors.Is(got, cause) || opts.installResult != nil {
				t.Fatalf("the cause must survive and no retry may be offered: %v %+v", got, opts.installResult)
			}
			if strings.Contains(got.Error(), secret) || !strings.Contains(got.Error(), "not allowed to write") {
				t.Fatalf("public error must be plain: %q", got.Error())
			}
			files, err := filepath.Glob(filepath.Join(app.InstallLogDir, "install-*.jsonl"))
			if err != nil || len(files) != 1 || !strings.Contains(errOut.String(), "Technical details: "+files[0]) {
				t.Fatalf("the log must exist and be announced: %v %v %q", err, files, errOut.String())
			}
			log, err := os.ReadFile(files[0])
			if err != nil || !strings.Contains(string(log), secret) || !strings.Contains(string(log), string(failurePermission)) {
				t.Fatalf("the log must hold the code and the full detail: %v %s", err, log)
			}
		})
	}
}
