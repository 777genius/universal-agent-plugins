package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type hermesColdCommand struct {
	command legacyports.Command
	result  legacyports.CommandResult
	err     error
}

type hermesColdRecorder struct {
	original            legacyports.ProcessRunner
	project, executable string
	outer, retained     context.Context
	calls               []hermesColdCommand
	invalid             error
	ordinaryStarts      int
}

func (recorder *hermesColdRecorder) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	if err := recorder.validate(ctx, command); err != nil {
		recorder.invalid = err
		return legacyports.CommandResult{}, err
	}
	if recorder.retained == nil {
		recorder.retained = ctx
	}
	captured := command
	captured.Argv, captured.Env = slices.Clone(command.Argv), slices.Clone(command.Env)
	result, err := recorder.original.Run(ctx, command)
	record := hermesColdCommand{command: captured, result: result, err: err}
	record.result.Stdout, record.result.Stderr = slices.Clone(result.Stdout), slices.Clone(result.Stderr)
	recorder.calls = append(recorder.calls, record)
	if contextErr := ctx.Err(); contextErr != nil {
		recorder.invalid = contextErr
	}
	// Observation must not replace the original delegate's result or error.
	return result, err
}

func (recorder *hermesColdRecorder) validate(ctx context.Context, command legacyports.Command) error {
	if recorder.invalid != nil {
		return recorder.invalid
	}
	if recorder.original == nil || ctx == nil || recorder.outer == nil || len(recorder.calls) >= 4 {
		return fmt.Errorf("native cold observation lacks one original runner or exceeds four commands")
	}
	if err := errors.Join(recorder.outer.Err(), ctx.Err()); err != nil {
		return err
	}
	deadline, bounded := ctx.Deadline()
	outerDeadline, outerBounded := recorder.outer.Deadline()
	if !bounded || !outerBounded || deadline.After(outerDeadline) || recorder.retained != nil && ctx != recorder.retained {
		return fmt.Errorf("native cold command changed its original retained context")
	}
	wanted := [][]string{{recorder.executable, "--version"}, {recorder.executable, "config", "path"},
		{recorder.executable, "config", "get", "mcp_servers", "--json"}, {recorder.executable, "config", "get", "mcp_servers", "--json"}}
	selectedRoot, roots := hermesColdSelectedRoot(command.Env)
	if !slices.Equal(command.Argv, wanted[len(recorder.calls)]) || command.Dir != recorder.project || roots != 1 ||
		selectedRoot != filepath.Join(recorder.project, ".hermes", "uap-session") || command.DiscardStdout || command.StdoutLimitBytes != 1<<20 {
		return fmt.Errorf("native cold command or environment differs from its exact Project selection")
	}
	return nil
}

func hermesColdSelectedRoot(environment []string) (string, int) {
	var root string
	count := 0
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "HERMES_HOME") {
			root, count = value, count+1
		}
	}
	return root, count
}

func (recorder *hermesColdRecorder) refuseOrdinary(context.Context, clients.SessionPlan, []string) error {
	recorder.ordinaryStarts++
	return fmt.Errorf("native cold retained inspection reached the ordinary session callback")
}

func (recorder *hermesColdRecorder) completed(output, diagnostics []byte) (hermesColdCommand, error) {
	if recorder.invalid != nil || recorder.outer == nil || recorder.outer.Err() != nil || len(recorder.calls) != 4 || recorder.ordinaryStarts != 0 {
		return hermesColdCommand{}, fmt.Errorf("native cold retained inspection did not complete four original commands")
	}
	for _, observed := range recorder.calls {
		result := observed.result
		if observed.err != nil || result.ExitCode != 0 || !result.CleanupComplete || result.NoChildStarted ||
			len(result.Stdout) >= 1<<20 || len(result.Stderr) > 32768 {
			return hermesColdCommand{}, fmt.Errorf("native cold command did not complete within its original bounds")
		}
	}
	last := recorder.calls[3]
	if !bytes.Equal(output, last.result.Stdout) || !bytes.Equal(diagnostics, last.result.Stderr) {
		return hermesColdCommand{}, fmt.Errorf("public cold inspection output differs from the actual fourth command")
	}
	return last, nil
}

func (input *hermesColdInputs) inspectCommands(ctx context.Context, t *testing.T, journey projectJourney, publicArgs []string) hermesColdCommand {
	t.Helper()
	recorder := &hermesColdRecorder{project: journey.a, executable: input.bundle, outer: ctx}
	app := journey.app
	app.StartNativeSession = recorder.refuseOrdinary
	compose := app.ForScope
	if compose == nil {
		t.Fatal("native cold inspection lacks its original scope composer")
	}
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		if scope != domain.ScopeProject || root != journey.a || recorder.original != nil || selected.SessionEnv.Runner == nil {
			return selected, fmt.Errorf("native cold inspection selected another runner or Project")
		}
		recorder.original = selected.SessionEnv.Runner
		selected.SessionEnv.Runner, selected.StartNativeSession = recorder, recorder.refuseOrdinary
		return selected, nil
	}
	var output, diagnostics bytes.Buffer
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &diagnostics, false
	command := agentpluginscli.NewRoot(app)
	command.SetOut(&output)
	command.SetErr(&diagnostics)
	command.SetArgs(publicArgs)
	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatal("actual public Cobra/native Hermes inspect did not complete; caller retains the fixture", err)
	}
	observed, err := recorder.completed(output.Bytes(), diagnostics.Bytes())
	if err != nil {
		t.Fatal("actual public Cobra/native Hermes inspect did not settle; caller retains the fixture", err)
	}
	return observed
}

func (input *hermesColdInputs) recordInspection(label string, publicArgs []string, observed hermesColdCommand) {
	result, command := observed.result, observed.command
	selectedRoot, _ := hermesColdSelectedRoot(command.Env)
	stdoutDigest, stderrDigest := sha256.Sum256(result.Stdout), sha256.Sum256(result.Stderr)
	input.commands = append(input.commands, map[string]any{
		"step": label, "argv": command.Argv, "public_argv": publicArgs, "directory": command.Dir, "selected_root": selectedRoot, "exit_code": result.ExitCode,
		"stdout_base64": result.Stdout, "stderr_base64": result.Stderr,
		"stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr),
		"stdout_sha256": hex.EncodeToString(stdoutDigest[:]), "stderr_sha256": hex.EncodeToString(stderrDigest[:]),
		"cleanup_complete": result.CleanupComplete, "no_child_started": result.NoChildStarted,
	})
}

// These delegate fixtures test the recorder only; the cold journey above always
// retains the selected production runner and requires hosted native evidence.
type hermesColdRunFunc func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (run hermesColdRunFunc) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	return run(ctx, command)
}

func coldRecorderFixture(t *testing.T) (*hermesColdRecorder, context.Context, []legacyports.Command) {
	t.Helper()
	outer, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	ctx, release := context.WithCancel(outer)
	t.Cleanup(release)
	project := t.TempDir()
	recorder := &hermesColdRecorder{project: project, executable: "inert-hermes", outer: outer}
	commands := make([]legacyports.Command, 0, 4)
	for _, args := range [][]string{{"--version"}, {"config", "path"}, {"config", "get", "mcp_servers", "--json"}, {"config", "get", "mcp_servers", "--json"}} {
		commands = append(commands, legacyports.Command{Argv: append([]string{recorder.executable}, args...), Dir: project,
			Env: []string{"HERMES_HOME=" + filepath.Join(project, ".hermes", "uap-session"), "RETAINED=original"}, StdoutLimitBytes: 1 << 20})
	}
	return recorder, ctx, commands
}

func TestProjectHermesColdRecorderRetainsDelegateAndFourthCommand(t *testing.T) {
	recorder, ctx, commands := coldRecorderFixture(t)
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	calls := 0
	var returned legacyports.CommandResult
	recorder.original = hermesColdRunFunc(func(actual context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
		if actual != ctx || !reflect.DeepEqual(command, commands[calls]) {
			t.Fatal("recorder changed its original delegate input")
		}
		calls++
		returned = legacyports.CommandResult{CleanupComplete: true, Stdout: []byte(fmt.Sprintf("owned-%d\n", calls)), Stderr: []byte("diagnostic\n")}
		return returned, nil
	})
	for index, command := range commands {
		result, err := recorder.Run(ctx, command)
		if err != nil || !reflect.DeepEqual(result, returned) {
			t.Fatal("recorder replaced its delegate result", err)
		}
		command.Argv[0], command.Env[0], result.Stdout[0], result.Stderr[0] = "changed", "HERMES_HOME=changed", 'X', 'X'
		root, count := hermesColdSelectedRoot(recorder.calls[index].command.Env)
		if recorder.calls[index].command.Argv[0] != "inert-hermes" || root != filepath.Join(recorder.project, ".hermes", "uap-session") || count != 1 ||
			recorder.calls[index].result.Stdout[0] != 'o' || recorder.calls[index].result.Stderr[0] != 'd' {
			t.Fatal("recorder retained mutable delegate/caller slices")
		}
	}
	// Production Close cancels this retained context after all four commands.
	cancel()
	last, err := recorder.completed([]byte("owned-4\n"), []byte("diagnostic\n"))
	if err != nil || calls != 4 || !bytes.Equal(last.result.Stdout, []byte("owned-4\n")) {
		t.Fatal("recorder did not join actual fourth output", err)
	}
	if _, err := recorder.completed([]byte("owned-3\n"), []byte("diagnostic\n")); err == nil {
		t.Fatal("verifier output replaced the final command")
	}
}

func TestProjectHermesColdRecorderRejectsSelectionBeforeDelegation(t *testing.T) {
	for _, change := range []string{"order", "directory", "root", "duplicate-root", "discard", "bound", "context", "fifth"} {
		t.Run(change, func(t *testing.T) {
			recorder, ctx, commands := coldRecorderFixture(t)
			calls := 0
			recorder.original = hermesColdRunFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				return legacyports.CommandResult{CleanupComplete: true}, nil
			})
			command := commands[0]
			switch change {
			case "order":
				command = commands[1]
			case "directory":
				command.Dir = filepath.Dir(command.Dir)
			case "root":
				command.Env[0] = "HERMES_HOME=wrong"
			case "duplicate-root":
				command.Env = append(command.Env, "hermes_home=wrong")
			case "discard":
				command.DiscardStdout = true
			case "bound":
				command.StdoutLimitBytes = 0
			case "context":
				alternate, cancel := context.WithCancel(ctx)
				t.Cleanup(cancel)
				recorder.retained = alternate
			case "fifth":
				for _, current := range commands {
					if _, err := recorder.Run(ctx, current); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := calls
			if _, err := recorder.Run(ctx, command); err == nil || calls != before {
				t.Fatal("invalid selected command reached the native delegate", err)
			}
		})
	}
}

func TestProjectHermesColdRecorderPreservesFailureAndRequiresSettlement(t *testing.T) {
	recorder, ctx, commands := coldRecorderFixture(t)
	original := errors.New("original delegate refusal")
	result := legacyports.CommandResult{ExitCode: 7, Stdout: []byte("partial"), Stderr: []byte("original")}
	recorder.original = hermesColdRunFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) { return result, original })
	actual, err := recorder.Run(ctx, commands[0])
	if reflect.ValueOf(err) != reflect.ValueOf(original) || !reflect.DeepEqual(actual, result) {
		t.Fatal("recorder replaced the original delegate failure")
	}
	if _, err := recorder.completed(result.Stdout, result.Stderr); err == nil {
		t.Fatal("partial native failure produced a completed cold record")
	}
	recorder, ctx, commands = coldRecorderFixture(t)
	recorder.original = hermesColdRunFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
		return legacyports.CommandResult{CleanupComplete: true}, nil
	})
	for _, command := range commands {
		if _, err := recorder.Run(ctx, command); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.refuseOrdinary(ctx, clients.SessionPlan{}, nil); err == nil {
		t.Fatal("ordinary callback trap did not refuse")
	}
	if _, err := recorder.completed(nil, nil); err == nil {
		t.Fatal("ordinary callback reached a successful retained cold report")
	}
}

func TestProjectHermesColdRecorderRefusesCancellationWithoutReplacingResult(t *testing.T) {
	for _, when := range []string{"before", "during"} {
		t.Run(when, func(t *testing.T) {
			recorder, ctx, commands := coldRecorderFixture(t)
			ctx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			calls := 0
			original := legacyports.CommandResult{CleanupComplete: true, Stdout: []byte("owned")}
			recorder.original = hermesColdRunFunc(func(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
				calls++
				cancel()
				return original, nil
			})
			if when == "before" {
				cancel()
			}
			actual, err := recorder.Run(ctx, commands[0])
			if when == "before" && (!errors.Is(err, context.Canceled) || calls != 0) {
				t.Fatal("canceled original context reached the delegate", err)
			}
			if when == "during" && (err != nil || calls != 1 || !reflect.DeepEqual(actual, original)) {
				t.Fatal("observation replaced the actual canceled delegate result", err)
			}
			if !errors.Is(recorder.invalid, context.Canceled) {
				t.Fatal("observation lost cancellation of the original retained context")
			}
		})
	}
}
