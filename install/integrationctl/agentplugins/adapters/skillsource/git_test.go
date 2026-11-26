package skillsource

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

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

type skillRefRunner struct{ remote string }

func (runner skillRefRunner) Run(_ context.Context, command sourceacquisition.Command) ([]byte, error) {
	args := command.Args
	if len(args) > 2 && args[0] == "-C" {
		args = args[2:]
	}
	if len(args) > 0 && args[0] == "ls-remote" {
		return []byte(runner.remote), nil
	}
	return nil, nil
}

func TestSkillURLKeepsExactAnonymousAndNativeRefNames(t *testing.T) {
	commit := strings.Repeat("1", 40)
	for _, native := range []bool{false, true} {
		for _, ref := range []string{"main", "refs/tags/topic"} {
			fullRef := "refs/heads/" + ref
			a := Acquirer{Acquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir(), Runner: skillRefRunner{commit + "\t" + fullRef + "\n"}}, NativeGit: native}
			input := "https://github.com/acme/tools/tree/" + ref + "/skills/pdf"
			source, err := a.Resolve(t.Context(), input, fullRef)
			if err != nil || source.RequestedSelector != fullRef || source.RequestedPath != "skills/pdf" || source.Commit != commit {
				t.Fatalf("exact ref changed native=%v: %+v %v", native, source, err)
			}
			if native {
				source, err = a.Resolve(t.Context(), input, "")
				if err != nil || source.RequestedSelector != fullRef || source.RequestedPath != "skills/pdf" {
					t.Fatalf("native branch namespace stripped twice: %+v %v", source, err)
				}
			}
		}
	}
}
