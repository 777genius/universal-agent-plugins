package dependencymanifest

import (
	"strings"
	"testing"
)

func TestLocalSkillManifestIsRootOnly(t *testing.T) {
	body := "schema_version=1\n[dependencies.review]\nlocal='/skills/review'\nkind='skill'\nskill='review'\n[targets]\ncodex=['review']\n"
	doc, err := Decode([]byte(body), "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := doc.Root()
	if err != nil || reqs.Dependencies["review"].Local == nil {
		t.Fatalf("root local source: %+v %v", reqs, err)
	}
	if _, err := doc.Package(); err == nil || !strings.Contains(err.Error(), "root-only") {
		t.Fatalf("published package accepted local source: %v", err)
	}
	for _, edit := range []string{"git='acme/tools'\n", "ref='main'\n", "commit='1234'\n", "path='review'\n", "sha256='1234'\n"} {
		bad := strings.Replace(body, "[targets]", edit+"[targets]", 1)
		if _, err := Decode([]byte(bad), "uap.toml"); err == nil {
			t.Errorf("accepted mixed local fields %q", edit)
		}
	}
	cloned := doc.clone()
	cloned.Dependencies["review"].Local.Path = "/other/review"
	if doc.requirements.Dependencies["review"].Local.Path != "/skills/review" {
		t.Fatal("caller mutated captured local source")
	}
}
