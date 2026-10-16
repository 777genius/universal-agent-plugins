package project

import (
	"context"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Authoring must judge commands from the captured bytes with the same rules
// the installer applies, or validate and install would disagree.
func TestReadAppliesInstallerCommandRules(t *testing.T) {
	writableNative(t)
	scratch := t.TempDir()
	root := t.TempDir()
	put(t, root, "plugin.json", core)
	put(t, root, "skills/demo-taken/SKILL.md", "---\nname: demo-taken\ndescription: d\n---\n")
	put(t, root, commandext.RelativeDirectory+"/hello.md", "---\ndescription: Hi\ndisable-model-invocation: true\n---\nHello\n")
	put(t, root, commandext.RelativeDirectory+"/taken.md", "---\ndescription: Hi\ndisable-model-invocation: true\n---\nHello\n")
	put(t, root, commandext.RelativeDirectory+"/.gitkeep", "")
	put(t, root, commandext.RelativeDirectory+"/nested/x.md", "x")
	p, err := Service{Scratch: scratch}.Read(context.Background(), root)
	if err != nil || p.Facts.Package == nil {
		t.Fatalf("read: %+v %v", p.Facts, err)
	}
	pkg := p.Facts.Package
	if command := pkg.Commands["hello"]; len(pkg.Commands) != 1 || !command.ExplicitOnly || string(command.Body) != "Hello\n" {
		t.Fatalf("commands = %+v", pkg.Commands)
	}
	if !reflect.DeepEqual(pkg.InvalidCommands, []string{"nested", "taken"}) || !reflect.DeepEqual(pkg.SkillsDirEntries, []string{"demo-taken"}) {
		t.Fatalf("invalid = %v, skills entries = %v", pkg.InvalidCommands, pkg.SkillsDirEntries)
	}
	var got []string
	for _, diagnostic := range pkg.Diagnostics {
		if diagnostic.Boundary == domain.BoundaryCommand {
			got = append(got, diagnostic.Item+":"+diagnostic.Code)
		}
	}
	if want := []string{"nested:command_name_invalid", "taken:command_skill_name_collision"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("command diagnostics = %v, want %v", got, want)
	}
	empty(t, scratch)
}
