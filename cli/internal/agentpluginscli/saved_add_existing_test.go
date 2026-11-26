package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestSavedAddExistingStateIsExplainedBeforeSelection(t *testing.T) {
	for _, scenario := range []string{"git-unmanaged", "directory-unmanaged", "directory-managed", "managed", "different-source-same-name"} {
		t.Run(scenario, func(t *testing.T) {
			var fixture cliFixture
			var source, target string
			if strings.HasPrefix(scenario, "directory-") {
				rollout := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCursor, domain.ClientGemini}, nil)
				clients, err := rollout.cli.app.Detector.Detect(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				fixture, _ = newDependencyHarness(t, t.TempDir(), clients, installer.Config{Runner: &cliRunOnlyRunner{}})
				fixture.app.DirectoryClient, fixture.app.SourceAcquirer = rollout.directory, rollout.acquirer
				source, target = "rollout-demo", "cursor"
			} else {
				repo, commit := savedJourneyRepository(t)
				fixture, source, target = newSavedJourney(t, repo).cli, "github:owner/repo@"+commit, "codex"
			}
			args := []string{"add", source, "--target", target, "--yes", "--format", "json"}
			if scenario != "managed" && scenario != "directory-managed" {
				args = append(args, "--no-save")
			}
			if out, stderr, err := fixture.execute(false, args...); err != nil {
				t.Fatalf("initial install: %v\n%s\n%s", err, out, stderr)
			}
			before, err := os.ReadFile(fixture.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "different-source-same-name" {
				source = strings.Replace(source, "owner/repo", "other/repo", 1)
			}
			stopped := errors.New("stop at selection")
			selected := false
			var menu prompt.TargetSelectionRequest
			fixture.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
				return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					selected, menu = true, request
					return prompt.TargetSelectionResult{}, stopped
				}}, io.Discard, nil
			}
			_, stderr, err := fixture.execute(true, "add", source)
			if scenario == "managed" || scenario == "directory-managed" || scenario == "different-source-same-name" {
				if !selected || !errors.Is(err, stopped) {
					t.Fatalf("valid selection blocked: selected=%v err=%v", selected, err)
				}
				for _, choice := range menu.Choices {
					wantManaged := (scenario == "managed" || scenario == "directory-managed") && string(choice.ID) == target
					if strings.Contains(choice.Note, "already in your list") != wantManaged || strings.Contains(choice.Info, "kept even if unchecked") != wantManaged {
						t.Fatalf("inaccurate saved-state annotation: %+v", choice)
					}
				}
			} else {
				var foreign *environment.ForeignBindingError
				if selected || !errors.As(err, &foreign) || len(foreign.Bindings) != 1 || foreign.Bindings[0].ClientID != target {
					t.Fatalf("ownership refusal arrived after selection or lost identity: selected=%v err=%v", selected, err)
				}
				if (scenario != "git-unmanaged" && !strings.Contains(stderr, "Checking plugin compatibility")) || !strings.Contains(err.Error(), "existing package ownership") {
					t.Fatalf("missing progress or explanation: %v\n%s", err, stderr)
				}
			}
			after, err := os.ReadFile(fixture.store.Path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("selection/refusal mutated installed state: %v", err)
			}
		})
	}
}

// When every candidate was filtered before package acquisition, still explain
// an exact retained Directory installation using its verified identity.
func TestSavedAddUnloadedDirectoryPreservesOwnershipDiagnostic(t *testing.T) {
	rollout := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCursor, domain.ClientGemini}, nil)
	clients, err := rollout.cli.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	fixture, _ := newDependencyHarness(t, t.TempDir(), clients, installer.Config{Runner: &cliRunOnlyRunner{}})
	fixture.app.DirectoryClient, fixture.app.SourceAcquirer = rollout.directory, rollout.acquirer
	if out, stderr, err := fixture.execute(false, "add", "rollout-demo", "--target", "cursor", "--yes", "--no-save", "--format", "json"); err != nil {
		t.Fatalf("seed imperative install: %v\\n%s\\n%s", err, out, stderr)
	}
	_, err = inspectSavedAddSelection(t.Context(), fixture.app, &options{format: "human"}, "rollout-demo", nil)
	var foreign *environment.ForeignBindingError
	if !errors.As(err, &foreign) || len(foreign.Bindings) != 1 || foreign.Bindings[0].ClientID != "cursor" || !strings.Contains(err.Error(), "existing package ownership") {
		t.Fatalf("missing retained Directory diagnosis: %v", err)
	}
	// An explicit alias must not lose the resolver's more specific diagnostic.
	if _, err := inspectSavedAddSelection(t.Context(), fixture.app, &options{alias: "chosen", format: "human"}, "rollout-demo", nil); err != nil {
		t.Fatalf("explicit alias handled too early: %v", err)
	}
}

// Non-direct Git sources obtain the immutable source identity from the loaded
// package. An empty preview key would silently drop ownership annotations.
func TestSavedAddSelectionCanonicalSourceKeyAfterAcquisition(t *testing.T) {
	identity := domain.SourceIdentity{
		Repository:       "acme/example",
		PackageSubpath:   "tools",
		ResolvedRevision: strings.Repeat("a", 40),
	}
	loaded := loadedPackage{envelope: domain.PackageEnvelope{Source: identity}}
	got, err := savedSelectionSourceKey("source-not-directly-parseable", loaded)
	if err != nil {
		t.Fatal(err)
	}
	requirement := environment.Requirement{Alias: "candidate", Git: &environment.GitSource{
		Repository: identity.Repository,
		Path:       identity.PackageSubpath,
		Selector:   environment.Selector{Kind: environment.SelectorCommit, Value: identity.ResolvedRevision},
	}}
	want, err := requirement.SourceKey()
	if err != nil {
		t.Fatal(err)
	}
	if got == "" || got != want {
		t.Fatalf("loaded source identity lost: got %q, want %q", got, want)
	}
}
