package dependencymanifest

import (
	"strings"
	"testing"
)

func TestGenericGitAndArchiveDeclarations(t *testing.T) {
	body := []byte("schema_version = 1\n[dependencies.git]\ngit = 'ssh://git@example.test/Team/Repo'\nref = 'main'\npath = 'plugins/demo'\n[dependencies.assets]\narchive = 'https://example.test/package'\nsha256 = '" + strings.Repeat("a", 64) + "'\npath = 'plugins/demo'\n[targets]\ncodex = ['git','assets']\n")
	doc, err := Decode(body, "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := doc.Root()
	if err != nil {
		t.Fatal(err)
	}
	if reqs.Dependencies["git"].Git.URL != "ssh://git@example.test/Team/Repo" || reqs.Dependencies["assets"].Archive.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("coordinates lost: %+v", reqs)
	}
	for _, change := range []string{
		"git = 'https://user:TOKEN@example.test/repo.git'\nref='main'",
		"archive = 'https://example.test/pkg.zip'\nref='main'",
		"archive = 'https://example.test/pkg.zip'\nversion='1.2.3'",
		"archive = 'https://example.test/pkg.zip'\ngit='acme/repo'",
		"git='acme/repo'\nsha256='" + strings.Repeat("a", 64) + "'",
		"archive='https://example.test/pkg.zip?token=TOKEN'",
	} {
		_, err := Decode([]byte("schema_version=1\n[dependencies.demo]\n"+change+"\n[targets]\ncodex=['demo']\n"), "uap.toml")
		if err == nil {
			t.Fatalf("invalid mixed source accepted: %s", change)
		}
		if strings.Contains(err.Error(), "TOKEN") {
			t.Fatal("URL credentials leaked")
		}
	}
}
