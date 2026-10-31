package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This POSIX-only check replays captured argv through the real public CLI.
// The shell used for presentation runs printf, never agentplugins/purge.
func TestProjectRetainedDataGuidancePreservesSelectedRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell argv replay is not qualified on Windows, even when Git supplies sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("POSIX argument-presentation check requires sh; no Windows shell qualification claimed")
	}
	transport := newProjectJourneyTransport(t, "zip")
	for _, route := range []string{"imperative-json", "imperative-human", "saved-human"} {
		t.Run(route, func(t *testing.T) {
			journey := newProjectJourney(t, transport)
			root := filepath.Join(t.TempDir(), "project A 'literal' $cash")
			if err := os.Rename(journey.a, root); err != nil {
				t.Fatal(err)
			}
			var err error
			journey.a, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			installation := prepareRetainedGuidanceJourney(t, journey, route == "saved-human")
			selector, format := installation.InstallationID, "human"
			switch route {
			case "saved-human":
				selector = "docs"
			case "imperative-json":
				format = "json"
			}
			output := executeRetainedGuidanceCLI(t, journey, journey.a, format, "remove", selector, "--target", "gemini")
			command := retainedGuidanceCommand(t, output, route)
			args := captureRetainedGuidanceArguments(t, command)
			want := []string{"agentplugins", "remove", installation.InstallationID, "--purge-data", "--scope", "project", "--project-root", journey.a}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("follow-up lost selected argv: got %q want %q", args, want)
			}
			replayRetainedGuidance(t, journey, installation, args[1:])
		})
	}
}

func prepareRetainedGuidanceJourney(t *testing.T, journey projectJourney, saved bool) domain.Installation {
	t.Helper()
	var selected domain.Installation
	for _, root := range []string{"", journey.a, journey.b} {
		args := []string{"add", journey.transport.endpoint, "--alias", "docs", "--target", "gemini"}
		if !saved {
			args = []string{"add", journey.transport.repo, "--no-save", "--target", "gemini"}
		}
		journey.run(t, root, args...)
		state := journey.state(t, root)
		if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
			t.Fatalf("expected one installed fixture in %q: %+v", root, state)
		}
		projectJourneyDataMarker(t, journey, root, state.Installations[0])
		if root == journey.a {
			selected = state.Installations[0]
		}
	}
	return selected
}

func executeRetainedGuidanceCLI(t *testing.T, journey projectJourney, project, format string, args ...string) string {
	t.Helper()
	var output, stderr bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), &output, &stderr, false
	root := agentpluginscli.NewRoot(app)
	root.SetOut(&output)
	root.SetErr(&stderr)
	args = append(args, "--format", format)
	if project != "" {
		args = append(args, "--scope", "project", "--project-root", project)
	}
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("public CLI %q: %v\n%s\n%s", args, err, output.String(), stderr.String())
	}
	return output.String()
}

func retainedGuidanceCommand(t *testing.T, output, route string) string {
	t.Helper()
	if route == "imperative-json" {
		var response struct {
			Data struct {
				DataRetained bool   `json:"data_retained"`
				Action       string `json:"retained_data_action"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(output), &response); err != nil || !response.Data.DataRetained {
			t.Fatalf("expected retained project JSON: %v\n%s", err, output)
		}
		output = response.Data.Action
	}
	prefix, suffix := "run `", "` to delete retained plugin data"
	if route == "saved-human" {
		prefix, suffix = "PLUGIN_DATA preserved; after removal completes, run ", " to delete retained data"
	}
	for _, line := range strings.Split(output, "\n") {
		if start := strings.Index(line, prefix); start >= 0 && strings.HasSuffix(line, suffix) {
			return strings.TrimSuffix(line[start+len(prefix):], suffix)
		}
	}
	t.Fatalf("missing retained-data follow-up in %s: %s", route, output)
	return ""
}

func captureRetainedGuidanceArguments(t *testing.T, command string) []string {
	t.Helper()
	if !strings.HasPrefix(command, "agentplugins remove ") || strings.ContainsAny(command, "\n\r") {
		t.Fatalf("unexpected command presentation: %q", command)
	}
	// All values are test-owned. set captures shell word boundaries; the
	// generated agentplugins command is never executed by this shell.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, "sh", "-c", "set -- "+command+"; printf '%s\\0' \"$@\"")
	process.Env = []string{"PATH=/usr/bin:/bin"}
	process.Stdin = strings.NewReader("")
	body, err := process.Output()
	if err != nil || len(body) == 0 || body[len(body)-1] != 0 {
		t.Fatalf("inert shell argument capture: %v %q", err, body)
	}
	return strings.Split(string(body[:len(body)-1]), "\x00")
}

func replayRetainedGuidance(t *testing.T, journey projectJourney, installation domain.Installation, args []string) {
	t.Helper()
	retained := journey.state(t, journey.a)
	if len(retained.Installations) != 1 || !retained.Installations[0].DataRetained || len(retained.Installations[0].Clients) != 0 {
		t.Fatalf("normal removal did not retain data: %+v", retained)
	}
	userBefore, bBefore := securityFixtureTree(t, journey.home), securityFixtureTree(t, journey.b)
	t.Chdir(t.TempDir())
	output := executeRetainedGuidanceCLI(t, journey, "", "json", args...)
	if !strings.Contains(output, `"status":"purged"`) || len(journey.state(t, journey.a).Installations) != 0 {
		t.Fatalf("emitted follow-up did not purge selected project: %s", output)
	}
	for _, receipt := range installation.DataReceipts {
		projectJourneyAbsent(t, receipt.Locator)
	}
	if !reflect.DeepEqual(userBefore, securityFixtureTree(t, journey.home)) || !reflect.DeepEqual(bBefore, securityFixtureTree(t, journey.b)) {
		t.Fatal("project follow-up changed user/B native state, retained data or shared cache")
	}
}
