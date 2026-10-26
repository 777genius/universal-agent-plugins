package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Regression: the default saved add used to prompt for targets without a
// prompter and failed with "prompt unavailable" in a real terminal.
func TestSavedAddPromptsForTargetsInTerminal(t *testing.T) {
	j := newSavedJourney(t, t.TempDir())
	stop := errors.New("stop after target prompt")
	prompted := false
	j.cli.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		return fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			prompted = true
			return prompt.TargetSelectionResult{}, stop
		}}, io.Discard, nil
	}
	_, _, err := j.cli.execute(true, "add", "github:owner/repo@"+strings.Repeat("a", 40))
	if !prompted || !errors.Is(err, stop) {
		t.Fatalf("prompted=%v err=%v", prompted, err)
	}
}

func TestSavedAddDefaultsOnlySafelyDetectedTargets(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			detected := []domain.DetectedClient{fixtureClient(t, domain.ClientClaude), fixtureClient(t, domain.ClientCodex)}[:count]
			absent := fixtureClient(t, domain.ClientCopilot)
			absent.Status = domain.DetectionNotDetected
			unsafe := fixtureClient(t, domain.ClientCline)
			unsafe.DetectionError = errors.New("private inspection details")
			unsupported := domain.DetectedClient{ClientID: "unsupported-test", Status: domain.DetectionDetected}
			journey := newSavedJourney(t, t.TempDir())
			app := journey.cli.app
			app.Detector = staticDetector{clients: append(detected, absent, unsafe, unsupported)}
			app.Terminal = true
			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			prompted := 0
			var menu prompt.TargetSelectionRequest
			app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
				return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					prompted++
					menu = request
					return prompt.TargetSelectionResult{IDs: request.DefaultIDs}, nil
				}}, &output, nil
			}
			targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: "user"}, "context7", nil)
			if count == 0 {
				if err == nil || !strings.Contains(err.Error(), "no eligible detected client") {
					t.Fatalf("no detected target: targets=%v err=%v", targets, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			want := []domain.ClientID{}
			if count > 0 {
				want = append(want, domain.ClientClaude)
			}
			if count > 1 {
				want = []domain.ClientID{domain.ClientCodex, domain.ClientClaude}
			}
			if len(targets) != len(want) || (count > 0 && !reflect.DeepEqual(targets, want)) {
				t.Fatalf("default targets=%v, want only safely detected %v", targets, want)
			}
			if count > 1 {
				if prompted != 1 || !reflect.DeepEqual(menu.DefaultIDs, want) || len(menu.Choices) != count {
					t.Fatalf("menu=%+v prompted=%d", menu, prompted)
				}
			} else if prompted != 0 {
				t.Fatalf("%d detected clients unexpectedly prompted", count)
			}
			diagnostics := output.String() + strings.Join(menu.SkippedLabels, "\n")
			for _, reason := range []string{"not detected", "could not be inspected safely", "use --no-save"} {
				if !strings.Contains(diagnostics, reason) {
					t.Fatalf("missing %q in %q", reason, diagnostics)
				}
			}
			if strings.Contains(diagnostics, "private inspection details") {
				t.Fatal("private detection error exposed")
			}
			if journey.runner.calls != 0 || journey.source.downloads != 0 {
				t.Fatal("target selection attempted native commands or source acquisition")
			}
		})
	}
}

func TestSavedAddExplicitTargetKeepsInstallerAdmission(t *testing.T) {
	journey := newSavedJourney(t, t.TempDir())
	app := journey.cli.app
	app.Terminal = true
	absent := fixtureClient(t, domain.ClientCopilot)
	absent.Status = domain.DetectionNotDetected
	app.Detector = staticDetector{clients: []domain.DetectedClient{absent}}
	app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		t.Fatal("explicit target unexpectedly prompted")
		return nil, nil, errors.New("unexpected prompt")
	}
	targets, err := savedAddTargets(&cobra.Command{}, app, &options{format: "human", scope: "user", target: "copilot"}, "context7", []domain.ClientID{domain.ClientCopilot})
	if err != nil || !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientCopilot}) {
		t.Fatalf("explicit target was changed before installer admission: %v %v", targets, err)
	}
}
