package dependencymanifest

import (
	"strings"
	"testing"
)

func TestWebsiteSkillIntentDecode(t *testing.T) {
	prefix := "schema_version=1\n[dependencies.pdf]\n"
	doc, err := Decode([]byte(prefix+"website='https://publisher.example/prefix/'\nkind='skill'\nskill='pdf'\n[targets]\ncodex=['pdf']\n"), "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := doc.Root()
	if err != nil {
		t.Fatal(err)
	}
	if reqs.Dependencies["pdf"].Web.URL != "https://publisher.example/prefix" {
		t.Fatal(reqs)
	}
	reqs.Dependencies["pdf"].Web.Skill = "other"
	isolated, err := doc.Package()
	if err != nil || isolated["pdf"].Web.Skill != "pdf" {
		t.Fatal("document source aliases returned view", err)
	}
	for _, fields := range []string{
		"website='https://publisher.example'\nskill='pdf'",
		"website='https://publisher.example'\nkind='skill'",
		"website='https://publisher.example'\nkind='plugin'\nskill='pdf'",
		"website='http://publisher.example'\nkind='skill'\nskill='pdf'",
		"website='https://user@publisher.example'\nkind='skill'\nskill='pdf'",
		"website='https://publisher.example?x=1'\nkind='skill'\nskill='pdf'",
		"website='https://publisher.example#x'\nkind='skill'\nskill='pdf'",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\ngit='acme/tools'",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\narchive='https://publisher.example/a.zip'",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\nregistry='uap'\ndistribution='acme/pdf'\nrelease=1",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\npath='skills/pdf'",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\nref='main'",
		"website='https://publisher.example'\nkind='skill'\nskill='pdf'\nsha256='" + strings.Repeat("a", 64) + "'",
	} {
		if _, err := Decode([]byte(prefix+fields+"\n"), "uap.toml"); err == nil {
			t.Fatalf("accepted malformed website intent: %s", fields)
		}
	}
}
