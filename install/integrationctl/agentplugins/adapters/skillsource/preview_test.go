package skillsource

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type previewGitProbe struct {
	repository, asset string
	assetSizeQueries  int
}

func (p *previewGitProbe) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	if len(command.Args) >= 3 && command.Args[0] == "init" {
		p.repository = command.Args[2]
	}
	if len(command.Args) == 3 && command.Args[0] == "cat-file" && command.Args[1] == "-s" && command.Args[2] == p.asset {
		p.assetSizeQueries++
	}
	return (sourceacquisition.OSRunner{}).Run(ctx, command)
}
func missingObject(t *testing.T, repository, object string) bool {
	t.Helper()
	objects := "\n" + git(t, repository, "rev-list", "--objects", "--missing=print", "FETCH_HEAD") + "\n"
	return strings.Contains(objects, "\n?"+object+"\n")
}

// A size query also fetches a promised blob. The actual filtered transport must
// leave resource objects absent after Preview, regardless of which Git command
// an implementation uses to build the resource inventory.
func TestPreviewDoesNotFetchPromisedResources(t *testing.T) {
	root, a := fixture(t)
	put(t, root, "skills/review/SKILL.md", document("review"), 0600)
	put(t, root, "skills/review/assets/copied-instructions.md", document("review"), 0600)
	put(t, root, "skills/review/assets/payload.bin", strings.Repeat("fixture asset\x00", 1<<16), 0600)
	sha := commitFixture(t, root)
	asset := git(t, root, "rev-parse", "HEAD:skills/review/assets/payload.bin")
	documentObject := git(t, root, "rev-parse", "HEAD:skills/review/SKILL.md")
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, root, "clone", "--quiet", "--bare", root, remote)
	git(t, remote, "config", "uploadpack.allowFilter", "true")
	remoteURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(remote)}).String()
	a.URLForRepo = func(string) string { return remoteURL }
	probe := &previewGitProbe{asset: asset}
	a.Runner = probe
	session, err := a.Open(t.Context(), Source{Repository: "example/fixture", Commit: sha})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	if !missingObject(t, probe.repository, asset) || !missingObject(t, probe.repository, documentObject) {
		t.Fatal("fixture did not create promised missing blobs")
	}
	preview, err := session.Preview(t.Context(), "skills/review")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Resources) != 2 || preview.Resources[1].Path != "assets/payload.bin" || preview.Resources[1].Size != nil {
		t.Fatal("resource inventory must retain unknown sizes", preview.Resources)
	}
	copied := preview.Resources[0]
	if copied.Path != "assets/copied-instructions.md" || copied.Size == nil || *copied.Size != int64(len(document("review"))) {
		t.Fatal("cached object size was lost", copied)
	}
	if !missingObject(t, probe.repository, asset) || probe.assetSizeQueries != 0 {
		t.Fatalf("preview fetched resource object: missing=%v sizeQueries=%d", missingObject(t, probe.repository, asset), probe.assetSizeQueries)
	}
	if missingObject(t, probe.repository, documentObject) {
		t.Fatal("preview did not read the selected document")
	}
}
