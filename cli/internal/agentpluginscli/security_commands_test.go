package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/environmentstore"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type recordingSecurityEvaluator struct {
	inputs []domain.SecurityEvaluationInput
}

func (evaluator *recordingSecurityEvaluator) Evaluate(_ context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	evaluator.inputs = append(evaluator.inputs, input)
	return domain.SecurityAssessment{}, nil
}

// Both gates that scan a package before it is used, install and dependency
// sync, must hand the evaluator the commands it is to render and scan.
func TestSecurityGatesReceiveCommands(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	files := map[string]string{
		"plugin.json": `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`,
		commandext.RelativeDirectory + "/echo.md": "---\ndescription: Echo\ndisable-model-invocation: false\n---\nReply.\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree := "sha256:" + strings.Repeat("a", 64)
	app := formatSelectionApp(t)
	evaluator := &recordingSecurityEvaluator{}
	app.SecurityEvaluator = evaluator
	loaded, err := app.loadSnapshotMode(context.Background(), domain.PackageSnapshot{Root: root, TreeDigest: tree}, domain.OriginModeDirect, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	locked := environment.LockedPackage{TreeDigest: tree, ManifestDigest: loaded.envelope.ManifestDigest, Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "owner/repo", Commit: strings.Repeat("b", 40)}}}
	_, _ = loadStoredDependency(context.Background(), app, environmentstore.Package{Root: root, Metadata: environmentstore.Metadata{TreeDigest: tree}}, locked, nil)
	if len(evaluator.inputs) != 2 {
		t.Fatalf("security gates evaluated %d times", len(evaluator.inputs))
	}
	for _, input := range evaluator.inputs {
		if _, ok := input.Commands["echo"]; !ok || input.PluginName != "demo" {
			t.Fatalf("security gate input = %+v", input)
		}
	}
}
