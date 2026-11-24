package agentpluginscli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestSavedDirectoryDerivedAliasPreservesResolverPriority(t *testing.T) {
	for _, claimed := range []bool{true, false} {
		t.Run(map[bool]string{true: "claimed", false: "unclaimed"}[claimed], func(t *testing.T) {
			directory := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientClaude, domain.ClientCursor}, []domain.ClientID{domain.ClientClaude, domain.ClientCursor})
			repo := writeCLIPlugin(t)
			manifestPath := filepath.Join(repo, "plugin.json")
			body, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			dependencyWrite(t, manifestPath, strings.Replace(string(body), `"name": "demo"`, `"name": "other"`, 1))
			revision := strings.Repeat("3", 40)
			directory.acquirer.revisionRoots[revision] = repo
			j := newSavedJourney(t, repo)
			clients, err := j.cli.app.Detector.Detect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			j.cli.app.Detector = staticDetector{clients: []domain.DetectedClient{clients[1], fixtureClient(t, domain.ClientCursor)}}
			j.cli.app.DirectoryClient = directory.directory
			j.cli.app.SourceAcquirer = directory.acquirer
			alias := "different"
			if claimed {
				alias = "demo"
			}
			j.run(t, "add", githubIdentity("acme/other", revision, ""), "--alias", alias, "--target", "cursor", "--format", "json")
			j.run(t, "add", "rollout-demo", "--no-save", "--target", "cursor", "--yes", "--format", "json")
			beforeState := j.state(t)
			manifest, lock := j.cli.app.dependencyPaths()
			beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
			beforeAcquisitions, beforeNative := directory.acquirer.verifiedCalls, j.runner.calls
			selected := false
			j.cli.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
				return fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
					selected = true
					return prompt.TargetSelectionResult{IDs: []domain.ClientID{domain.ClientCursor}}, nil
				}}, io.Discard, nil
			}
			_, stderr, err := j.cli.execute(true, "add", "rollout-demo")
			var foreign *environment.ForeignBindingError
			if claimed {
				if err == nil || errors.As(err, &foreign) || !strings.Contains(err.Error(), "alias already belongs to another source; use --alias") || !selected {
					t.Fatalf("derived alias refusal lost priority: selected=%v err=%v stderr=%s", selected, err, stderr)
				}
			} else if !errors.As(err, &foreign) || !ShouldWriteAgentFailure(err) || selected {
				t.Fatalf("unclaimed alias lost early ownership refusal: selected=%v err=%v stderr=%s", selected, err, stderr)
			}
			if directory.acquirer.verifiedCalls <= beforeAcquisitions {
				t.Fatal("test did not exercise acquired Directory identity")
			}
			if j.runner.calls != beforeNative || !reflect.DeepEqual(beforeState, j.state(t)) || dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
				t.Fatal("refusal changed saved state or executed a native client")
			}
		})
	}
}
