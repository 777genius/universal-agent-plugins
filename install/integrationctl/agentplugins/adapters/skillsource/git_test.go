package skillsource

import "testing"

func TestNativeSkillSourceGrammar(t *testing.T) {
	for _, input := range []string{
		"https://git.example/team/tools.git#refs/heads/main//skills/pdf",
		"git+https://git.example/team/tools#main//skills/pdf",
		"ssh://git@git.example/team/tools.git#main//skills/pdf",
		"git@git.example:team/tools.git#main//skills/pdf",
		"git+https://github.com/acme/tools.git#main//skills/pdf",
	} {
		request, err := parseSourceInput(input)
		if err != nil || request.git == nil || request.git.Path != "skills/pdf" {
			t.Fatalf("native Skill clone input %s: %+v %v", input, request, err)
		}
	}
	for _, input := range []string{
		"https://user:secret@git.example/team/tools.git",
		"ssh://git@git.example/team/tools.git#main//../pdf",
		"file:///private/repository.git",
		"git@git.example:../tools.git",
		"https://git.example/team/tools.git?token=secret",
	} {
		if _, err := parseSourceInput(input); err == nil {
			t.Fatalf("unsafe native Skill source accepted: %s", input)
		}
	}
}

func TestNativeRootSkillNamesAndLocations(t *testing.T) {
	for _, source := range []Source{
		{Repository: "acme/pdf"},
		{URL: "https://git.example/team/pdf.git"},
		{URL: "ssh://git@git.example/team/pdf.git"},
		{URL: "git@git.example:team/pdf.git"},
	} {
		if sourceRootName(source) != "pdf" || source.Location() == "" {
			t.Fatalf("root Skill name or display identity lost: %+v", source)
		}
	}
}
