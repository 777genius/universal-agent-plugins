package agentpluginscli

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
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
		"permission":           {&fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EACCES}, failurePermission, "does not have permission"},
		"os-sentinel":          {fmt.Errorf("create client target root: %w", os.ErrPermission), failurePermission, "does not have permission"},
		"disk full":            {fmt.Errorf("write: %w", &fs.PathError{Op: "write", Path: secretPath, Err: syscall.ENOSPC}), failureDiskFull, "free disk space"},
		"read only":            {fmt.Errorf("write: %w", &fs.PathError{Op: "mkdir", Path: secretPath, Err: syscall.EROFS}), failureReadOnly, "read-only"},
		"network":              {fmt.Errorf("fetch: %w", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("no route to host")}), failureNetwork, "network could not be reached"},
		"unexecutable program": {fmt.Errorf("probe: %w", &exec.Error{Name: "claude", Err: fs.ErrPermission}), failureAgentMissing, "cannot be run"},
		"no program":           {fmt.Errorf("probe: %w", &exec.Error{Name: "claude", Err: exec.ErrNotFound}), failureAgentMissing, "not found or cannot be run"},
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
	for _, status := range []string{"preparation_failed"} {
		t.Run(status, func(t *testing.T) {
			app, _ := retryAppAndOptions(t, new([]string))
			app.InstallLogDir = filepath.Join(t.TempDir(), "logs")
			opts := &options{format: "human"}
			cmd, _, errOut := retryTestCommand(t)
			combined := addMultiResult{Status: status, Plugin: "demo", Targets: []addTargetResult{{Target: "claude"}, {Target: "gemini"}}}
			got := app.recordGroupOutcome(cmd, opts, &combined, cause)
			if !errors.Is(got, cause) || opts.installResult != nil {
				t.Fatalf("the cause must survive and no retry may be offered: %v %+v", got, opts.installResult)
			}
			if strings.Contains(got.Error(), secret) || !strings.Contains(got.Error(), "does not have permission") {
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

// Causes that merely look like network or permission problems must not be
// relabelled: a person would be told something untrue.
func TestEnvironmentClassificationStaysGenericForLookalikes(t *testing.T) {
	t.Parallel()
	cases := map[string]error{
		"url syntax":              &url.Error{Op: "Get", URL: "http://bad host", Err: errors.New("invalid character in host name")},
		"tls authority":           &url.Error{Op: "Get", URL: "https://x", Err: x509.UnknownAuthorityError{}},
		"tls alert from the peer": &net.OpError{Op: "remote error", Err: errors.New("tls: bad certificate")},
		"unrelated":               errors.New("boom"),
		"wrapped unknown":         fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New("c"))),
	}
	for name, err := range cases {
		if got := failureKindOf(err); got != failureUnknown {
			t.Errorf("%s was relabelled as %s", name, got)
		}
	}
	timeout := &net.OpError{Op: "read", Err: os.ErrDeadlineExceeded}
	if got := failureKindOf(fmt.Errorf("fetch: %w", timeout)); got != failureTimeout {
		t.Errorf("a network timeout is a timeout, got %s", got)
	}
	canceled := &url.Error{Op: "Get", URL: "https://x", Err: context.Canceled}
	if got := failureKindOf(canceled); got != failureCanceled {
		t.Errorf("cancel inside a url error must stay canceled, got %s", got)
	}
	both := fmt.Errorf("%w: %w", installer.ErrRecoveryRequired, &fs.PathError{Op: "mkdir", Path: "/x", Err: syscall.EACCES})
	if got := failureKindOf(both); got != failureRecovery {
		t.Errorf("recovery must win over permission, got %s", got)
	}
}

// A refusal only gets the plain public error when the person can fix the cause
// and the technical chain really landed in a log; every other case keeps the
// original error and writes nothing.
func TestRefusedGroupKeepsOriginalErrorWhenPlainWordingWouldLoseDetail(t *testing.T) {
	permission := fmt.Errorf("create client target root: %w", &fs.PathError{Op: "mkdir", Path: "/p", Err: syscall.EACCES})
	cases := map[string]struct {
		status string
		format string
		logDir bool
		cause  error
	}{
		"planned phase":         {"planned", "human", true, permission},
		"json output":           {"preparation_failed", "json", true, permission},
		"no log directory":      {"preparation_failed", "human", false, permission},
		"cancel in url error":   {"preparation_failed", "human", true, &url.Error{Op: "Get", URL: "https://x", Err: context.Canceled}},
		"recovery and denied":   {"preparation_failed", "human", true, fmt.Errorf("%w: %w", installer.ErrRecoveryRequired, permission)},
		"tls authority failure": {"preparation_failed", "human", true, &url.Error{Op: "Get", URL: "https://x", Err: x509.UnknownAuthorityError{}}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			app, _ := retryAppAndOptions(t, new([]string))
			app.InstallLogDir = ""
			if tc.logDir {
				app.InstallLogDir = filepath.Join(t.TempDir(), "logs")
			}
			opts := &options{format: tc.format}
			cmd, _, errOut := retryTestCommand(t)
			combined := addMultiResult{Status: tc.status, Plugin: "demo", Targets: []addTargetResult{{Target: "claude"}}}
			got := app.recordGroupOutcome(cmd, opts, &combined, tc.cause)
			if got != tc.cause {
				t.Fatalf("the original error must be returned unchanged: %v", got)
			}
			if errOut.Len() != 0 {
				t.Fatalf("nothing may be announced: %q", errOut.String())
			}
			if tc.logDir {
				if _, err := os.Stat(app.InstallLogDir); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("no log may be written: %v", err)
				}
			}
		})
	}
}
