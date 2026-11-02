package commandext_test

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Staging and the envelope a client projector sees follow one rule, so a
// command the plan marks unsupported is neither written nor listed.
func TestWithCommandSkillsListsOnlyDeliveredCommands(t *testing.T) {
	t.Parallel()
	envelope := load(t, "demo", nil, file("echo.md", validHeader+"x\n"), file("wave.md", validHeader+"y\n"))
	delivered := domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "demo-echo", Support: domain.SupportProjected, Source: domain.ComponentSourceCommand, SourceName: "echo"}
	unsupported := domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "demo-wave", Support: domain.SupportUnsupported, Source: domain.ComponentSourceCommand, SourceName: "wave"}
	got, err := commandext.WithCommandSkills(envelope, domain.DeliveryPlan{Components: []domain.ComponentDecision{delivered, unsupported}})
	if err != nil {
		t.Fatal(err)
	}
	if _, listed := got.Skills["demo-wave"]; listed || got.Skills["demo-echo"].RelativePath != "skills/demo-echo/"+commandext.SkillFile {
		t.Fatalf("skills = %+v", got.Skills)
	}
	if !commandext.DeliveredSkill(delivered) || commandext.DeliveredSkill(unsupported) {
		t.Fatal("DeliveredSkill disagrees with the plan")
	}
}

// The names an installed tree can hold for commands leave out the package's
// own skills and entries that could never become a skill name.
func TestProjectedSkillNamesCoverEveryPossibleCommandSkill(t *testing.T) {
	t.Parallel()
	envelope := load(t, "demo", []string{"demo-taken"}, file("echo.md", validHeader+"x\n"), file("broken.md", "no header\n"), file("taken.md", validHeader+"z\n"), file("Bad Name.md", "x\n"))
	if got, want := commandext.ProjectedSkillNames(envelope), []string{"demo-broken", "demo-echo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}
