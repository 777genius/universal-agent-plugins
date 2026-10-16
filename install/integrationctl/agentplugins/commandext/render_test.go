package commandext_test

import (
	"bytes"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The rendered bytes are part of the installed tree digest, so the output is
// pinned exactly: a change here is a change users must apply with update.
func TestRenderGolden(t *testing.T) {
	t.Parallel()
	command := domain.Command{Name: "hello", Description: `Say "hi" in one line`, ExplicitOnly: true, Body: []byte("Reply with hi.\n")}
	files, err := commandext.Marshal(commandext.Render(command, "my.plug"))
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: \"my-plug-hello\"\ndescription: \"Say \\\"hi\\\" in one line\"\n---\nReply with hi.\n"
	if len(files) != 1 || string(files[commandext.SkillFile]) != want {
		t.Fatalf("rendered = %q", files)
	}
	skill, err := conformance.ParseInstallerSkill("my-plug-hello", files[commandext.SkillFile])
	if err != nil {
		t.Fatalf("undecorated output must be an installable skill: %v", err)
	}
	if skill.Description != command.Description {
		t.Fatalf("description = %q", skill.Description)
	}
}

// A bool must come back as a bool and a string that looks like a YAML
// keyword must come back as a string, otherwise a flag is silently lost.
func TestMarshalKeepsScalarTypes(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 300) + "end"
	files, err := commandext.Marshal(domain.RenderedSkill{
		Frontmatter: []domain.FrontmatterField{{Key: "name", Value: "yes"}, {Key: "description", Value: long}, {Key: "disable-model-invocation", Value: true}},
		Body:        []byte("body\n"),
		Files:       map[string][]byte{"agents/openai.yaml": []byte("policy: {}\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	front, _, ok := bytes.Cut(bytes.TrimPrefix(files[commandext.SkillFile], []byte("---\n")), []byte("---\n"))
	if !ok {
		t.Fatalf("no closing delimiter in %q", files[commandext.SkillFile])
	}
	var decoded map[string]any
	if err := yaml.Unmarshal(front, &decoded); err != nil {
		t.Fatal(err)
	}
	if name, ok := decoded["name"].(string); !ok || name != "yes" {
		t.Fatalf("name = %#v", decoded["name"])
	}
	if flag, ok := decoded["disable-model-invocation"].(bool); !ok || !flag {
		t.Fatalf("flag = %#v", decoded["disable-model-invocation"])
	}
	if decoded["description"] != long || bytes.Count(front, []byte("\n")) != 3 {
		t.Fatalf("description was folded: %q", front)
	}
	if string(files["agents/openai.yaml"]) != "policy: {}\n" {
		t.Fatalf("files = %q", files)
	}
	for _, bad := range []domain.RenderedSkill{
		{Frontmatter: []domain.FrontmatterField{{Key: "n", Value: 1}}},
		{Frontmatter: []domain.FrontmatterField{{Key: "n", Value: "a"}, {Key: "n", Value: "b"}}},
		{Files: map[string][]byte{"../escape": nil}},
		{Files: map[string][]byte{"skill.md": nil}},
		{Files: map[string][]byte{`agents\openai.yaml`: nil}},
		{Files: map[string][]byte{"agents/openai.yaml:stream": nil}},
	} {
		if _, err := commandext.Marshal(bad); err == nil {
			t.Fatalf("Marshal(%+v) succeeded", bad)
		}
	}
}
