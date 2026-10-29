package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

func TestSavedAddAutomaticTargetsRespectPackageHostPreflight(t *testing.T) {
	for _, mcp := range []bool{false, true} {
		t.Run(map[bool]string{false: "skills", true: "mcp"}[mcp], func(t *testing.T) {
			source := writeCLIPlugin(t)
			skillRoot := filepath.Join(source, "skills", "docs")
			if err := os.MkdirAll(skillRoot, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("---\nname: docs\ndescription: Inert eligibility fixture\n---\nText only.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if mcp {
				writeCLIMCP(t, source)
			}
			journey := newSavedJourney(t, t.TempDir())
			app := journey.cli.app
			app.Terminal = true
			kiro, cursor := fixtureClient(t, domain.ClientKiro), fixtureClient(t, domain.ClientCursor)
			kiro.ExecutablePath = "/test/bin/kiro-cli"
			app.Detector = staticDetector{clients: []domain.DetectedClient{kiro, cursor}}
			var previewRoot string
			app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
				previewRoot = root
				return app.SourceAcquirer
			}
			// A run-only host cannot qualify native MCP duplex containment. Skills-only
			// packages must remain eligible for the same detected client.
			app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: &cliRunOnlyRunner{}})
			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			prompted := false
			app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
				return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					prompted = true
					return prompt.TargetSelectionResult{IDs: request.DefaultIDs}, nil
				}}, &output, nil
			}
			targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: "user"}, source, nil)
			want := []domain.ClientID{domain.ClientCursor, domain.ClientKiro}
			if mcp {
				want = []domain.ClientID{domain.ClientCursor}
			}
			if err != nil || !reflect.DeepEqual(targets, want) {
				t.Fatalf("targets=%v want=%v err=%v", targets, want, err)
			}
			if prompted == mcp {
				t.Fatalf("prompted=%v mcp=%v", prompted, mcp)
			}
			if previewRoot == "" {
				t.Fatal("target admission did not use a disposable preview")
			}
			if _, err := os.Stat(previewRoot); !os.IsNotExist(err) {
				t.Fatalf("preview retained: %v", err)
			}
			for _, profile := range []string{kiro.ConfigRoot, cursor.ConfigRoot} {
				if _, err := os.Stat(profile); !os.IsNotExist(err) {
					t.Fatalf("profile changed before confirmation: %s %v", profile, err)
				}
			}
			if mcp && !strings.Contains(output.String(), "Nothing was installed in Kiro") {
				t.Fatalf("missing preflight guidance: %s", output.String())
			}
		})
	}
}

func TestSavedAddAutomaticTargetsRetainQualifiedOwnedOpenCode(t *testing.T) {
	repo := dependencyRepo(t)
	fixture, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientOpenCode, executable: true})
	source := githubIdentity("acme/demo", savedJourneyGit(t, repo, "rev-parse", "HEAD"), "")
	dependencyRun(t, fixture, "add", source, "--target", "opencode", "--format", "json")
	dependencyBinding(t, fixture)
	app := fixture.app
	app.Terminal = true
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: "user"}, source, nil)
	if err != nil || !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientOpenCode}) {
		t.Fatalf("qualified owned OpenCode was excluded: targets=%v err=%v", targets, err)
	}
}

func TestSavedAddAutomaticTargetsDeferUnqualifiedHostWithoutProbing(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			for _, dryRun := range []bool{false, true} {
				t.Run(map[bool]string{false: "normal", true: "dry-run"}[dryRun], func(t *testing.T) {
					fixture, preparer, source, _ := openCodePreparationCLI(t, true)
					journey := newSavedJourney(t, t.TempDir())
					app := journey.cli.app
					app.Detector = fixture.app.Detector
					app.Scope = scope
					if scope == domain.ScopeProject {
						project, err := filepath.EvalSymlinks(t.TempDir())
						if err != nil {
							t.Fatal(err)
						}
						app.ProjectRoot = project
						app.Planner = clientplanner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: app.ClientRegistry}
					}
					app.Lifecycle.ClientPreparation = preparer
					app.Terminal = true
					prompted := false
					app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
						return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
							prompted = true
							return prompt.TargetSelectionResult{IDs: request.DefaultIDs}, nil
						}}, io.Discard, nil
					}
					cmd := &cobra.Command{}
					cmd.SetContext(t.Context())
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
					targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: string(scope), dryRun: dryRun}, source, nil)
					want := []domain.ClientID{domain.ClientCursor, domain.ClientOpenCode}
					if err != nil || !reflect.DeepEqual(targets, want) {
						t.Fatalf("targets=%v want=%v err=%v", targets, want, err)
					}
					if !prompted {
						t.Fatalf("deferred host was omitted from menu: dryRun=%v", dryRun)
					}
					if dryRun && preparer.probes != 0 {
						t.Fatalf("dry-run probed %d times", preparer.probes)
					}
					if preparer.calls != 0 || preparer.baselines != 0 {
						t.Fatalf("selection prepared host before target consent: calls=%d baselines=%d", preparer.calls, preparer.baselines)
					}
				})
			}
		})
	}
}

func TestSavedAddDeferredHostCancelOrDeselectNeverProbes(t *testing.T) {
	for _, cancelMenu := range []bool{false, true} {
		t.Run(map[bool]string{false: "deselect", true: "cancel"}[cancelMenu], func(t *testing.T) {
			fixture, preparer, source, _ := openCodePreparationCLI(t, true)
			journey := newSavedJourney(t, t.TempDir())
			app := journey.cli.app
			app.Terminal = true
			app.Detector = fixture.app.Detector
			app.Lifecycle.ClientPreparation = preparer
			canceled := errors.New("selection canceled")
			app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
				return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					if preparer.calls != 0 {
						t.Fatal("native preparation happened before selection")
					}
					if cancelMenu {
						return prompt.TargetSelectionResult{}, canceled
					}
					return prompt.TargetSelectionResult{IDs: []domain.ClientID{domain.ClientCursor}}, nil
				}}, io.Discard, nil
			}
			cmd := &cobra.Command{}
			cmd.SetContext(t.Context())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			targets, err := savedAddTargets(cmd, app, &options{format: "human", scope: "user"}, source, nil)
			if cancelMenu {
				if !errors.Is(err, canceled) {
					t.Fatalf("cancel error=%v", err)
				}
			} else if err != nil || !reflect.DeepEqual(targets, []domain.ClientID{domain.ClientCursor}) {
				t.Fatalf("deselected targets=%v err=%v", targets, err)
			}
			if preparer.calls != 0 || preparer.probes != 0 {
				t.Fatalf("unselected host inspected: calls=%d probes=%d", preparer.calls, preparer.probes)
			}
		})
	}
}

func TestSavedAddDeferredHostQualifiesOnlyAfterSelection(t *testing.T) {
	repo := dependencyRepo(t)
	fixture, _, runner := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientOpenCode, executable: true})
	clients, err := fixture.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	clients[0].OpenCodeHost = nil
	fixture.app.Detector = staticDetector{clients: append(clients, fixtureClient(t, domain.ClientCursor))}
	preparer := &cliPreparationFixture{}
	fixture.app.Lifecycle.ClientPreparation = preparer
	selected := false
	fixture.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			if runner.probes != 0 || preparer.calls != 0 {
				t.Fatal("native host executed before target selection")
			}
			selected = true
			return prompt.TargetSelectionResult{IDs: []domain.ClientID{domain.ClientOpenCode}}, nil
		}, confirmFn: func() (prompt.ConfirmationResult, error) {
			return prompt.ConfirmationResult{Accepted: true}, nil
		}}, io.Discard, nil
	}
	source := githubIdentity("acme/demo", savedJourneyGit(t, repo, "rev-parse", "HEAD"), "")
	_, stderr, err := fixture.executeInput(true, "y\ny\n", "add", source)
	if err != nil {
		t.Fatalf("selected saved add failed: %v %s", err, stderr)
	}
	if !selected || runner.probes == 0 {
		t.Fatalf("selection=%v authoritative host probes=%d", selected, runner.probes)
	}
	if preparer.calls != 0 {
		t.Fatalf("selection preparation escaped into authoritative operation: %d", preparer.calls)
	}
	dependencyBinding(t, fixture)
}

func TestSavedAddDeferredOwnedHostOfflineNoOpDoesNotProbe(t *testing.T) {
	repo := dependencyRepo(t)
	fixture, _, runner := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientOpenCode, executable: true})
	source := githubIdentity("acme/demo", savedJourneyGit(t, repo, "rev-parse", "HEAD"), "")
	dependencyRun(t, fixture, "add", source, "--target", "opencode", "--format", "json")
	before := runner.probes
	detected, err := fixture.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	detected[0].OpenCodeHost = nil
	fixture.app.Detector = staticDetector{clients: detected}
	preparer := &cliPreparationFixture{}
	fixture.app.Lifecycle.ClientPreparation = preparer
	output, stderr, err := fixture.executeInput(true, "y\n", "add", source, "--dry-run")
	if err != nil {
		t.Fatalf("offline owned default re-add failed: %v %s %s", err, output, stderr)
	}
	if runner.probes != before || preparer.calls != 0 {
		t.Fatalf("offline unchanged action probed host: before=%d after=%d preparation=%d", before, runner.probes, preparer.calls)
	}
	dependencyBinding(t, fixture)
}
