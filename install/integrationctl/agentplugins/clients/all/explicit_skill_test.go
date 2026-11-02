package all

import (
	"context"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var explicitCommand = domain.Command{Name: "wizard", Description: "Run the wizard", ExplicitOnly: true, Body: []byte("Reply with the token.\n")}

// Every shipped decorator only adds to the neutral skill and leaves the
// shared explicit-invocation field in the written header.
func TestExplicitSkillDecoratorsOnlyAdd(t *testing.T) {
	t.Parallel()
	for _, adapter := range Default().All() {
		decorator, ok := adapter.(clients.ExplicitSkillDecorator)
		if !ok {
			continue
		}
		t.Run(string(adapter.ID()), func(t *testing.T) {
			t.Parallel()
			neutral := commandext.Render(explicitCommand, "demo")
			decorated, err := decorator.DecorateExplicitSkill(neutral)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(neutral, commandext.Render(explicitCommand, "demo")) {
				t.Fatal("decorator mutated its input")
			}
			if !reflect.DeepEqual(decorated.Frontmatter[:2], neutral.Frontmatter) || string(decorated.Body) != string(neutral.Body) {
				t.Fatalf("decorator changed name, description or body: %+v", decorated)
			}
			if _, err := commandext.Files(context.Background(), explicitCommand, "demo", decorator.DecorateExplicitSkill); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The decorated bytes are what the client reads, so they are pinned: the flag
// must stay a YAML boolean, and Codex also gets its policy file.
func TestExplicitSkillGolden(t *testing.T) {
	t.Parallel()
	header := "---\nname: \"demo-wizard\"\ndescription: \"Run the wizard\"\ndisable-model-invocation: true\n---\nReply with the token.\n"
	want := map[domain.ClientID]map[string]string{
		domain.ClientClaude: {commandext.SkillFile: header},
		domain.ClientCodex:  {commandext.SkillFile: header, "agents/openai.yaml": "policy:\n  allow_implicit_invocation: false\n"},
	}
	for id, files := range want {
		decorator, ok := clients.As[clients.ExplicitSkillDecorator](Default(), id)
		if !ok {
			t.Fatalf("%s has no explicit skill decorator", id)
		}
		rendered, err := commandext.Files(context.Background(), explicitCommand, "demo", decorator.DecorateExplicitSkill)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for name, body := range rendered {
			got[name] = string(body)
		}
		if !reflect.DeepEqual(got, files) {
			t.Fatalf("%s rendered %q, want %q", id, got, files)
		}
	}
}
