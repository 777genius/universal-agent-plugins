package agentpluginscli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminalactivity"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

func TestPluginActivityStopsBeforeWarningsAndPreservesTheme(t *testing.T) {
	var output bytes.Buffer
	format := "human"
	themed := terminaltheme.Wrap(&output, &terminaltheme.Policy{Mode: "always", Explicit: true}, &format)
	stop := terminalactivity.Start(&output, "Loading plugin", true, nil)
	app := stopPluginActivityOnWrite(App{ErrorOutput: themed}, stop)
	if !terminaltheme.For(app.errorOutput()).Enabled {
		t.Fatal("acquisition warning writer lost its theme")
	}
	const warnings = "WARNING [revoked]: review this source\nWARNING [unreviewed]: static checks only\n"
	for _, line := range strings.SplitAfter(warnings, "\n") {
		if line != "" {
			if _, err := app.errorOutput().Write([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
	}
	stop() // The caller's final cleanup must not erase the second warning.
	_, _ = output.WriteString("acquisition failed\n")
	view := output.String()
	first := strings.Index(view, "WARNING")
	if first < 0 || view[first:] != warnings+"acquisition failed\n" || strings.Contains(view[first:], "\x1b") {
		t.Fatalf("activity cleanup corrupted warnings/error: %q", view)
	}
}

type pluginActivityErrorWriter struct {
	expected    error
	beforeWrite func()
}

func (writer pluginActivityErrorWriter) Write([]byte) (int, error) {
	writer.beforeWrite()
	return 0, writer.expected
}

func TestPluginActivityStopsBeforeForwardingWriterFailure(t *testing.T) {
	expected := errors.New("warning output failed")
	stopped := false
	writer := pluginActivityWriter{Writer: pluginActivityErrorWriter{expected: expected, beforeWrite: func() {
		if !stopped {
			t.Fatal("warning write began while the renderer was live")
		}
	}}, stop: func() { stopped = true }}
	n, err := writer.Write([]byte("warning"))
	if !stopped || n != 0 || !errors.Is(err, expected) {
		t.Fatalf("stop/error forwarding changed: stopped=%v n=%d err=%v", stopped, n, err)
	}
}

func TestAddProgressPreservesLegacyPlainAndAutomatedOutput(t *testing.T) {
	for _, scenario := range []struct {
		name, format    string
		terminal, plain bool
	}{
		{"non-terminal", "human", false, false},
		{"plain", "human", true, true},
		{"json", "json", true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var output bytes.Buffer
			command := &cobra.Command{}
			command.SetErr(&output)
			app := App{Terminal: scenario.terminal, ErrorOutput: &output}
			stop := startAddProgress(command, app, &options{format: scenario.format, plain: scenario.plain}, "Resolving plugin...")
			stop()
			if output.String() != "Resolving plugin...\n" {
				t.Fatalf("legacy progress changed: %q", output.String())
			}
		})
	}
}
