package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

func TestPublicationDoctorInputDefaultsRootToDot(t *testing.T) {
	t.Parallel()

	got := publicationDoctorInput(publicationDoctorFlags{target: "all", format: "text"}, nil)
	if got.root != "." {
		t.Fatalf("root = %q", got.root)
	}
}

func TestNormalizedPublicationDoctorFormatRejectsUnknownValues(t *testing.T) {
	t.Parallel()

	if got := normalizedPublicationDoctorFormat("yaml"); got != "invalid" {
		t.Fatalf("format = %q", got)
	}
}

func TestWritePublicationDoctorWarningsPrefixesMessages(t *testing.T) {
	t.Parallel()

	cmd := newPublicationDoctorCmd(fakeInspectRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	writePublicationDoctorWarnings(cmd, []pluginmanifest.Warning{{Message: "demo warning"}})
	if !strings.Contains(buf.String(), "Warning: demo warning") {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestPublicationDoctorRunEUsesParsedRootAndTarget(t *testing.T) {
	t.Parallel()

	runner := &recordingPublicationDoctorRunner{err: errors.New("stop")}
	flags := publicationDoctorFlags{target: "gemini"}
	cmd := &cobra.Command{}

	err := publicationDoctorRunE(runner, &flags)(cmd, []string{"./demo"})
	if err == nil || err.Error() != "stop" {
		t.Fatalf("error = %v", err)
	}
	if runner.opts.Root != "./demo" || runner.opts.Target != "gemini" {
		t.Fatalf("inspect opts = %+v", runner.opts)
	}
}

func TestNewPublicationDoctorFlagsStartsEmpty(t *testing.T) {
	t.Parallel()

	if got := newPublicationDoctorFlags(); got != (publicationDoctorFlags{}) {
		t.Fatalf("flags = %+v", got)
	}
}

type recordingPublicationDoctorRunner struct {
	opts app.PluginInspectOptions
	err  error
}

func (r *recordingPublicationDoctorRunner) Inspect(opts app.PluginInspectOptions) (pluginmanifest.Inspection, []pluginmanifest.Warning, error) {
	r.opts = opts
	return pluginmanifest.Inspection{}, nil, r.err
}
