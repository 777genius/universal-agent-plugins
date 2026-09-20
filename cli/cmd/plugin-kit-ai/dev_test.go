package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

type fakeDevRunner struct {
	summary app.PluginDevSummary
	err     error
	updates []app.PluginDevUpdate
}

func (f fakeDevRunner) Dev(ctx context.Context, opts app.PluginDevOptions, emit func(app.PluginDevUpdate)) (app.PluginDevSummary, error) {
	_ = ctx
	_ = opts
	for _, update := range f.updates {
		emit(update)
	}
	return f.summary, f.err
}

func TestDevHelpIncludesWatchLanguage(t *testing.T) {
	t.Parallel()
	cmd := newDevCmd(fakeDevRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{"Watch", "rebuild", "fixture", "Gemini", "production-ready 9-hook Go runtime", "make test-gemini-runtime", "make test-gemini-runtime-live"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestDevOnceReturnsExitCodeOneWhenCycleFails(t *testing.T) {
	t.Parallel()
	cmd := newDevCmd(fakeDevRunner{
		summary: app.PluginDevSummary{Cycles: 1, LastPassed: false},
		updates: []app.PluginDevUpdate{{
			Cycle:  1,
			Passed: false,
			Lines:  []string{"Cycle 1 [initial]", "Validate: 1 failure(s)"},
		}},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--once", "--platform", "claude", "--all"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(buf.String(), "Cycle 1 [initial]") {
		t.Fatalf("output = %s", buf.String())
	}
}

func TestDevOnceWritesRunnerOutput(t *testing.T) {
	t.Parallel()
	cmd := newDevCmd(fakeDevRunner{
		summary: app.PluginDevSummary{Cycles: 1, LastPassed: true},
		updates: []app.PluginDevUpdate{{
			Cycle:  1,
			Passed: true,
			Lines: []string{
				"Cycle 1 [initial]",
				"Generate: wrote 2 artifact(s)",
				"PASS codex-runtime/Notify fixture=/tmp/fixtures/codex-runtime/Notify.json exit=0 golden=matched",
			},
		}},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--once", "--platform", "codex-runtime", "--event", "Notify", "--interval", (50 * time.Millisecond).String()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{"Generate:", "PASS codex-runtime/Notify"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}
