package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Planning/history are inert dispatcher fixtures. The final child and its
// containment are real process.OS, so successful exec cannot hide settlement.
type sessionRetainedProcess struct {
	fixture *sessionExecutionFixture
	actual  bool
	result  legacyports.CommandResult
	err     error
	argv    []string
}

func (runner *sessionRetainedProcess) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.argv = append([]string{}, command.Argv...)
	if ctx != runner.fixture.ctx || runner.fixture.file == nil || runner.fixture.closes != 0 ||
		command.StdoutLimitBytes != 1<<20 || command.Dir != runner.fixture.plan.Directory {
		return legacyports.CommandResult{}, errors.New("final child lost original execution authority")
	}
	if _, err := runner.fixture.file.Stat(); err != nil {
		return legacyports.CommandResult{}, err
	}
	if runner.actual {
		runner.result, runner.err = (processadapter.OS{}).Run(ctx, command)
	}
	return runner.result, runner.err
}

func retainedProcessFixture(t *testing.T, mode string) (App, *sessionExecutionFixture, *sessionRetainedProcess) {
	t.Helper()
	app, fixture := newSessionExecutionFixture(t, "clean")
	physical, err := filepath.EvalSymlinks(fixture.plan.Directory)
	if err != nil {
		t.Fatal(err)
	}
	app.ProjectRoot, fixture.plan.Directory = physical, physical
	fixture.path = filepath.Join(physical, "history")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture.plan.Executable = executable
	fixture.plan.Arguments = []string{"-test.run=^TestSessionRetainedProcessChild$", "-test.count=1"}
	fixture.plan.Environment = map[string]string{
		"UAP_RETAINED_HISTORY": fixture.path, "UAP_RETAINED_MODE": mode,
		"HERMES_HOME": filepath.Join(fixture.plan.Directory, "selected"),
	}
	runner := &sessionRetainedProcess{fixture: fixture, actual: true}
	fixture.finalRunner = runner
	t.Setenv("UAP_RETAINED_INHERITED", "original-base")
	return app, fixture, runner
}

func TestSessionRetainedProcessChild(t *testing.T) {
	path := os.Getenv("UAP_RETAINED_HISTORY")
	if path == "" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil || cwd != filepath.Dir(path) || os.Getenv("HERMES_HOME") != filepath.Join(cwd, "selected") ||
		os.Getenv("UAP_RETAINED_INHERITED") != "original-base" {
		t.Fatal("actual final child received the wrong validated environment/directory", err)
	}
	if err := os.WriteFile(path+".started", []byte("started"), 0600); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UAP_RETAINED_MODE") == "mutation" {
		if err := os.WriteFile(path, []byte("changed!"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("retained-native-output")
	if _, err := fmt.Fprintln(os.Stderr, "retained-native-diagnostic"); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UAP_RETAINED_MODE") == "deadline" {
		time.Sleep(30 * time.Second)
	}
}

func assertRetainedProcessClosed(t *testing.T, fixture *sessionExecutionFixture, runner *sessionRetainedProcess) {
	t.Helper()
	if fixture.calls != 4 || fixture.starts != 1 || fixture.ordinary != 0 || fixture.closes != 1 ||
		fixture.releases != 1 || fixture.plans != 2 || len(runner.argv) != 3 ||
		!runner.result.CleanupComplete || runner.result.NoChildStarted {
		t.Fatalf("actual retained child or original resource did not settle: calls=%d starts=%d ordinary=%d closes=%d cleanup=%t no-child=%t exit=%d error=%v", fixture.calls, fixture.starts, fixture.ordinary, fixture.closes, runner.result.CleanupComplete, runner.result.NoChildStarted, runner.result.ExitCode, runner.err)
	}
	assertRetainedOriginalClosed(t, fixture.original)
	if body, err := os.ReadFile(fixture.path + ".started"); err != nil || string(body) != "started" {
		t.Fatal("actual final child never reached the fixture", err)
	}
}

func assertRetainedOriginalClosed(t *testing.T, original *os.File) {
	t.Helper()
	_, statErr := original.Stat()
	var pathErr *os.PathError
	// Windows Stat exposes GetFileType's invalid-handle result after Close.
	windowsClosed := runtime.GOOS == "windows" && errors.As(statErr, &pathErr) &&
		pathErr.Op == "GetFileType" && pathErr.Path == original.Name() && errors.Is(pathErr.Err, syscall.Errno(6))
	if !errors.Is(statErr, os.ErrClosed) && !windowsClosed {
		t.Fatal("original descriptor was not closed", statErr)
	}
	// Require Go's already-closed state, not just an unusable OS handle.
	if err := original.Close(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("original descriptor was not already closed", err)
	}
}

func TestSessionRetainedActualProcessReturnsBeforeSettlement(t *testing.T) {
	for _, change := range []string{"none", "ambient", "adapter-slice", "mutation"} {
		t.Run(change, func(t *testing.T) {
			app, fixture, runner := retainedProcessFixture(t, change)
			fixture.finalEdit = func(base []string) {
				if change == "ambient" {
					t.Setenv("UAP_RETAINED_INHERITED", "late-ambient")
				}
				if change == "adapter-slice" {
					for index, value := range base {
						if strings.HasPrefix(value, "UAP_RETAINED_INHERITED=") {
							base[index] = "UAP_RETAINED_INHERITED=late-adapter"
						}
					}
				}
			}
			command := &cobra.Command{}
			var out, diagnostic bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&diagnostic)
			err := runClientSession(t.Context(), command, app, &options{target: "hermes", format: "human"}, true, true)
			assertRetainedProcessClosed(t, fixture, runner)
			if change == "mutation" {
				if !errors.Is(err, errSessionHistoryChanged) || out.Len() != 0 || diagnostic.Len() != 0 || runner.result.ExitCode != 0 {
					t.Fatal("successful child mutation escaped post-validation or emitted success", err)
				}
				return
			}
			if err != nil || out.String() != "retained-native-output\nPASS\n" || diagnostic.String() != "retained-native-diagnostic\n" {
				t.Fatalf("actual final child output/environment differs: err=%v out=%q stderr=%q", err, out.String(), diagnostic.String())
			}
		})
	}
}

func TestSessionRetainedActualProcessUsesOriginalDeadline(t *testing.T) {
	app, fixture, runner := retainedProcessFixture(t, "deadline")
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	command := &cobra.Command{}
	var out, diagnostic bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&diagnostic)
	err := runClientSession(ctx, command, app, &options{target: "hermes", format: "human"}, true, true)
	assertRetainedProcessClosed(t, fixture, runner)
	actual, ok := fixture.ctx.Deadline()
	if !errors.Is(err, context.DeadlineExceeded) || !ok || !actual.Equal(deadline) ||
		out.Len() != 0 || diagnostic.Len() != 0 || time.Since(deadline) > 8*time.Second {
		t.Fatal("actual final child renewed the deadline or escaped cancellation/settlement", err)
	}
}

func TestSessionRetainedRejectsIncompleteAuthorityBeforeVerification(t *testing.T) {
	for _, mode := range []string{"nil-context", "nil-runner", "noninspect"} {
		t.Run(mode, func(t *testing.T) {
			app, fixture := newSessionExecutionFixture(t, mode)
			err := runClientSession(t.Context(), &cobra.Command{}, app, &options{target: "hermes", format: "human"}, mode != "noninspect", true)
			if err == nil || fixture.begins != 1 || fixture.closes != 1 || fixture.releases != 1 ||
				fixture.verifies != 0 || fixture.calls != 0 || fixture.starts != 0 || fixture.ordinary != 0 {
				t.Fatal("incomplete/noninspect authority reached a verifier or native child", err)
			}
			assertRetainedOriginalClosed(t, fixture.original)
		})
	}
}

func TestSessionRetainedRejectsUnclosedAndOverboundResults(t *testing.T) {
	for _, mode := range []string{"exit", "stdout", "stderr", "unclosed", "not-started"} {
		t.Run(mode, func(t *testing.T) {
			app, fixture := newSessionExecutionFixture(t, "clean")
			runner := &sessionRetainedProcess{fixture: fixture, result: legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("unadmitted")}}
			switch mode {
			case "exit":
				runner.result.ExitCode = 9
			case "stdout":
				runner.result.Stdout = make([]byte, 1<<20)
			case "stderr":
				runner.result.Stderr = make([]byte, 32769)
			case "unclosed":
				runner.result.CleanupComplete = false
			case "not-started":
				runner.result.NoChildStarted = true
			}
			fixture.finalRunner = runner
			command := &cobra.Command{}
			var out, diagnostic bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&diagnostic)
			err := runClientSession(t.Context(), command, app, &options{target: "hermes", format: "human"}, true, true)
			if err == nil || out.Len() != 0 || diagnostic.Len() != 0 || fixture.closes != 1 || fixture.ordinary != 0 || fixture.calls != 4 {
				t.Fatal("unadmitted native result escaped the retained boundary", err)
			}
		})
	}
}

type sessionSettledOutput struct {
	fixture *sessionExecutionFixture
	count   int
}

func (writer *sessionSettledOutput) Write(body []byte) (int, error) {
	writer.count++
	if writer.fixture.closes != 1 || writer.fixture.file != nil {
		return 0, errors.New("output preceded original settlement")
	}
	return len(body) - 1, nil
}

func TestSessionRetainedPropagatesWriterFailureAfterSettlement(t *testing.T) {
	app, fixture := newSessionExecutionFixture(t, "clean")
	writer := &sessionSettledOutput{fixture: fixture}
	command := &cobra.Command{}
	command.SetOut(writer)
	err := runClientSession(t.Context(), command, app, &options{target: "hermes", format: "human"}, true, true)
	if !errors.Is(err, io.ErrShortWrite) || writer.count != 1 || fixture.closes != 1 || fixture.ordinary != 0 {
		t.Fatal("writer failure or original settlement was lost", err)
	}
}
