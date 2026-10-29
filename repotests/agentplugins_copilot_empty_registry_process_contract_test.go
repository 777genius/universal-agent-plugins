//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func copilotEmptyRegistryInertChild(t *testing.T, mode string) *copilotProjectAttempt {
	t.Helper()
	f := copilotEmptyRegistryContractFixture(t)
	body := "#!/bin/sh\n[ \"$#\" = 2 ] && [ \"$1\" = plugin ] && [ \"$2\" = list ] || exit 99\nprintf 'opaque inert listing\\n'\nprintf 'inert diagnostic\\n' >&2\n"
	switch mode {
	case "failed":
		body += "exit 7\n"
	case "config":
		body += "printf '{}\\n' >\"$HOME/.copilot/config.json\"\n"
	}
	projectNativeRuntimeWrite(t, f.client, []byte(body), 0700)
	f.immutable = map[string]string{f.client: nativeSHA(t, f.client)}
	f.ctx, f.cancel = context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(f.cancel)
	f.evidence["commands"] = append([]string{}, copilotEmptyRegistryCommands[:3]...)
	return f
}

func TestCopilotEmptyRegistryActualInertCommandKeepsOpaqueBytes(t *testing.T) {
	f := copilotEmptyRegistryInertChild(t, "normal")
	baseline := bytes.Clone(f.originalConfigBaseline)
	r := f.command(t, "native-list", f.outside, f.env(), []string{f.client, "plugin", "list"}, 30*time.Second)
	if !bytes.Equal(r.Stdout, []byte("opaque inert listing\n")) || !bytes.Equal(r.Stderr, []byte("inert diagnostic\n")) || r.ExitCode != 0 || len(f.files) != 3 {
		t.Fatal("inert output was parsed, normalized or not captured exactly")
	}
	if !bytes.Equal(f.originalConfigBaseline, baseline) || f.protected[f.originalConfigPath] != copilotProjectConfigHash(baseline) {
		t.Fatal("listing changed or adopted the original config baseline")
	}
	f.check(t)
}

func TestCopilotEmptyRegistryOriginalFatalGuardCannotContinue(t *testing.T) {
	const marker = "UAP_COPILOT_EMPTY_REGISTRY_INERT_CHILD"
	if mode := os.Getenv(marker); mode == "failed" || mode == "config" {
		f := copilotEmptyRegistryInertChild(t, mode)
		f.command(t, "native-list", f.outside, f.env(), []string{f.client, "plugin", "list"}, 30*time.Second)
		fmt.Println("AFTER_EMPTY_REGISTRY_FATAL_GUARD")
		return
	}
	// The child is this same inert Go contract plus an authored local script;
	// no vendor binary, installer, scanner or hosted-eligibility flag is used.
	for _, mode := range []string{"failed", "config"} {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCopilotEmptyRegistryOriginalFatalGuardCannotContinue$", "-test.count=1")
		cmd.Env = append(os.Environ(), marker+"="+mode)
		output, err := cmd.CombinedOutput()
		cancel()
		message := "native-list process/capture/cleanup failed"
		if mode == "config" {
			message = "original authority changed:"
		}
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 1 || !bytes.Contains(output, []byte(message)) || bytes.Contains(output, []byte("AFTER_EMPTY_REGISTRY_FATAL_GUARD")) {
			t.Fatalf("original guard did not stop the actual inert command: %v, %.1024s", err, output)
		}
	}
}
