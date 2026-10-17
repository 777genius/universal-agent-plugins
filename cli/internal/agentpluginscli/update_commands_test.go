package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// A tree rendered under older command rules matches the package revision, so
// a package-only comparison calls it current. update --all must still run the
// update, which applies the current rules.
func TestUpdateAllRefreshesAnOutdatedCommandProjection(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	plugin := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	writeCommandFixtureFile(t, filepath.Join(plugin, "skills", "guide", "SKILL.md"), "---\nname: guide\ndescription: Guide\n---\nRead the guide.\n")
	writeCommandFixtureFile(t, filepath.Join(plugin, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Echo the text\ndisable-model-invocation: false\n---\nReply with UAP-ECHO.\n")
	if _, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
		t.Fatal(err)
	}
	skill := rewindCommandProjection(t, fixture, "alpha-echo")

	stdout, _, err := fixture.execute(false, "update", "--all", "--format", "json")
	if err != nil {
		t.Fatalf("update --all: %v\n%s", err, stdout)
	}
	for _, want := range []string{`"planned":1`, `"updated":1`, `"skipped":0`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("update --all omitted %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(skill, commandext.SkillFile)); err != nil {
		t.Fatalf("update --all left the command skill out: %v", err)
	}
}

// rewindCommandProjection makes the only installed tree look as if an
// installer with stricter command rules had produced it: the command skill is
// gone and every recorded digest matches what is left. It returns the removed
// skill directory.
func rewindCommandProjection(t *testing.T, fixture cliFixture, skill string) string {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("want one installed binding, got %+v", state.Installations)
	}
	var binding domain.ClientBinding
	for _, candidate := range state.Installations[0].Clients {
		binding = candidate
	}
	recorded := ""
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" {
			recorded = object.ManagedDigest
		}
	}
	directory := filepath.Join(binding.TargetLocator, "skills", skill)
	if _, err := os.Stat(directory); err != nil || recorded == "" {
		t.Fatalf("installed command skill %s (digest %q): %v", directory, recorded, err)
	}
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	var mismatch *ports.VerificationError
	if err := providerstest.NewStager(providers.Stager{}).Verify(context.Background(), binding.TargetLocator, recorded); !errors.As(err, &mismatch) || mismatch.ActualDigest == "" {
		t.Fatalf("digest of the rewound tree: %v", err)
	}
	body, err := os.ReadFile(fixture.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.store.Path, []byte(strings.ReplaceAll(string(body), recorded, mismatch.ActualDigest)), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func writeCommandFixtureFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
