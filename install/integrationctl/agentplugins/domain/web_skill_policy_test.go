package domain

import "testing"

func TestWebSkillEndpointPreservesPrefixAndRejectsAmbiguousCoordinates(t *testing.T) {
	for input, expected := range map[string]string{"https://Skills.Example/": "https://skills.example", "https://Skills.Example/prefix/": "https://skills.example/prefix"} {
		if actual, err := NormalizeWebSkillURL(input); err != nil || actual != expected {
			t.Fatalf("endpoint %q: %q %v", input, actual, err)
		}
	}
	for _, input := range []string{"http://skills.example", "https://user@skills.example", "https://skills.example?", "https://skills.example#", "https://skills.example/a/../b", "https://skills.example/a/%2e%2e/b", "https://skills.example/a%2fb", "https://skills.example/a%5cb", "https://skills.example/a//b", "https://skills.example/%0a"} {
		if _, err := NormalizeWebSkillURL(input); err == nil {
			t.Fatalf("unsafe endpoint accepted: %q", input)
		}
	}
}

func TestWebSkillBindingIdentityDependsOnPublisherAndName(t *testing.T) {
	first := SourceIdentity{WebSkill: &WebSkillSourceFacts{URL: "https://skills.example/prefix", Skill: "pdf"}, SourceBindingHint: "uap-dependency-web-skill"}
	id := ComputeSourceBindingID(first)
	second := first
	second.WebSkill = CloneWebSkillSourceFacts(first.WebSkill)
	second.WebSkill.RawTreeDigest = "new revision"
	second.StableSourceKey = "caller override"
	if ComputeSourceBindingID(second) != id {
		t.Fatal("mutable revision or caller key changed HTTPS Skill identity")
	}
	second.WebSkill.Skill = "other"
	if ComputeSourceBindingID(second) == id {
		t.Fatal("different Skills share a binding")
	}
	second.WebSkill.Skill = "pdf"
	second.WebSkill.URL += "/other"
	if ComputeSourceBindingID(second) == id {
		t.Fatal("different publisher prefixes share a binding")
	}
}
