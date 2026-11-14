package dependencymanifest

import "testing"

func TestExplicitSkillSourceDecoder(t *testing.T) {
	good := []byte("schema_version=1\n[dependencies.pdf]\nkind='skill'\nskill='pdf'\ngit='https://github.com/acme/tools.git'\npath='skills/pdf'\nref='main'\n[targets]\ncodex=['pdf']\n")
	doc, err := Decode(good, "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	req, err := doc.Root()
	if err != nil {
		t.Fatal(err)
	}
	if req.Dependencies["pdf"].Git.Skill != "pdf" {
		t.Fatal(req)
	}
	for _, body := range []string{"schema_version=1\n[dependencies.pdf]\nkind='skill'\nskill='pdf'\nregistry='uap'\ndistribution='acme/pdf'\nrelease=1\n", "schema_version=1\n[dependencies.pdf]\nskill='pdf'\ngit='acme/tools'\n"} {
		if _, err := Decode([]byte(body), "uap.toml"); err == nil {
			t.Fatal("accepted ambiguous Skill source")
		}
	}
}
