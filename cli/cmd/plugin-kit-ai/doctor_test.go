package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

type fakeDoctorRunner struct {
	result app.PluginDoctorResult
	err    error
}

func (f fakeDoctorRunner) Doctor(app.PluginDoctorOptions) (app.PluginDoctorResult, error) {
	return f.result, f.err
}

func TestDoctorHelpIncludesReadOnlyReadinessCheck(t *testing.T) {
	t.Parallel()
	cmd := newDoctorCmd(fakeDoctorRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"Read-only readiness check",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestDoctorReturnsExitCodeOneWhenProjectIsNotReady(t *testing.T) {
	t.Parallel()
	cmd := newDoctorCmd(fakeDoctorRunner{
		result: app.PluginDoctorResult{
			Ready: false,
			Lines: []string{
				"Project: lane=codex-runtime runtime=node manager=npm",
				"Status: needs_bootstrap (npm install state is missing)",
				"Next:",
				"  plugin-kit-ai bootstrap .",
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	output := buf.String()
	for _, want := range []string{"Project:", "Status:", "Next:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}
