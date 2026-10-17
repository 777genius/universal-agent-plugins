package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

const takenSkill = "---\nname: demo-taken\ndescription: A real skill\n---\nKeep these exact bytes.\r\n"

// commandPackage loads a package through the real loader: one ordinary and
// one explicit command, a command whose skill name is taken by a real skill,
// and an entry that is not a command at all.
func commandPackage(t *testing.T) domain.PackageEnvelope {
	t.Helper()
	root := filepath.Join(t.TempDir(), "snapshot")
	writeTestFile(t, filepath.Join(root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0"}`)
	writeTestFile(t, filepath.Join(root, "skills", "demo-taken", "SKILL.md"), takenSkill)
	commands := filepath.Join(root, filepath.FromSlash(commandext.RelativeDirectory))
	writeTestFile(t, filepath.Join(commands, "echo.md"), "---\ndescription: Echo the text\ndisable-model-invocation: false\n---\nReply with UAP-ECHO.\n")
	writeTestFile(t, filepath.Join(commands, "wizard.md"), "---\ndescription: Run the wizard\ndisable-model-invocation: true\n---\nReply with UAP-WIZARD.\n")
	writeTestFile(t, filepath.Join(commands, "taken.md"), "---\ndescription: Collides\ndisable-model-invocation: false\n---\nNever delivered.\n")
	writeTestFile(t, filepath.Join(commands, "Bad Name.md"), "not a command\n")
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(envelope.Commands) != 2 || len(envelope.InvalidCommands) != 2 {
		t.Fatalf("fixture commands = %v, invalid = %v", envelope.Commands, envelope.InvalidCommands)
	}
	return envelope
}

// The ordinary command arrives as a skill and the explicit one is nowhere,
// the real skill that owns a projected name keeps its bytes, and invalid
// entries do not fail the stage. Cursor is planned by the real planner;
// Claude, whose plan needs a trusted CLI, gets the same command component.
func TestStagerDeliversOnlyPlannedCommandSkills(t *testing.T) {
	t.Parallel()
	echo := domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "demo-echo", Support: domain.SupportProjected, Source: domain.ComponentSourceCommand, SourceName: "echo"}
	plans := map[domain.ClientID]func(*testing.T, domain.PackageEnvelope) domain.DeliveryPlan{
		domain.ClientCursor: func(t *testing.T, envelope domain.PackageEnvelope) domain.DeliveryPlan {
			root := t.TempDir()
			plan, err := plannertest.NewPlanner(planner.Planner{ManagedRoot: filepath.Join(root, "managed")}).Plan(context.Background(), domain.PlanRequest{
				Envelope:           envelope,
				Client:             domain.DetectedClient{ClientID: domain.ClientCursor, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "config")},
				Scope:              domain.ScopeUser,
				PhysicalArtifactID: "demo-0123456789ab",
			})
			if err != nil {
				t.Fatal(err)
			}
			return plan
		},
		domain.ClientClaude: func(t *testing.T, _ domain.PackageEnvelope) domain.DeliveryPlan {
			plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
			plan.Components = []domain.ComponentDecision{echo}
			return plan
		},
	}
	skillsDirs := map[domain.ClientID]string{domain.ClientCursor: "skills", domain.ClientClaude: filepath.Join(".agentplugins-runtime", "skills")}
	for id, planFor := range plans {
		t.Run(string(id), func(t *testing.T) {
			t.Parallel()
			envelope := commandPackage(t)
			delivery, err := testStager(Stager{}).Stage(context.Background(), envelope, planFor(t, envelope), "operation-commands", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			skills := filepath.Join(delivery.StagingPath, skillsDirs[id])
			body, err := os.ReadFile(filepath.Join(skills, "demo-echo", commandext.SkillFile))
			if err != nil || string(body) != "---\nname: \"demo-echo\"\ndescription: \"Echo the text\"\n---\nReply with UAP-ECHO.\n" {
				t.Fatalf("echo skill = %q, %v", body, err)
			}
			assertMissing(t, filepath.Join(skills, "demo-wizard"))
			if id == domain.ClientClaude {
				// Claude exposes only planned skills, and this plan names none of the package's own.
				assertMissing(t, filepath.Join(skills, "demo-taken"))
				return
			}
			taken, err := os.ReadFile(filepath.Join(skills, "demo-taken", "SKILL.md"))
			if err != nil || string(taken) != takenSkill {
				t.Fatalf("real skill changed: %q, %v", taken, err)
			}
			entries, err := os.ReadDir(skills)
			if err != nil || len(entries) != 2 {
				t.Fatalf("staged skills = %v, %v", entries, err)
			}
		})
	}
}

// A flat native client digests every skill directory it owns. The command
// skill must be owned at stage time and found again by the read-only
// projection of the installed tree.
func TestStagerProjectsCommandSkillsForNativeClients(t *testing.T) {
	t.Parallel()
	envelope := commandPackage(t)
	plan := stagingPlan(t, domain.ClientKiro, domain.PackageNative)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo-echo", Support: domain.SupportNative, Source: domain.ComponentSourceCommand, SourceName: "echo"}}
	stager := testStager(Stager{})
	delivery, err := stager.Stage(context.Background(), envelope, plan, "operation-kiro-commands", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	staged := ownedKiroSkills(delivery.NativeObjects)
	if len(staged) != 1 || staged[0].LogicalName != "demo-echo" {
		t.Fatalf("staged native objects = %+v", delivery.NativeObjects)
	}
	if err := os.MkdirAll(filepath.Dir(plan.ActivePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(delivery.StagingPath, plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	active, err := stager.ProjectActiveNative(context.Background(), envelope, plan, delivery.ArtifactDigest, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := ownedKiroSkills(active); len(got) != 1 || got[0] != staged[0] {
		t.Fatalf("active projection = %+v, staged %+v", got, staged)
	}
}

// An installed tree rendered under other command rules is intact, so only a
// comparison with the current rendering finds it. Every kind of difference
// stops the read-only projection with the typed error instead of a missing
// file or a quietly different native projection.
func TestActiveNativeProjectionRefusesAnOutdatedCommandTree(t *testing.T) {
	t.Parallel()
	envelope := commandPackage(t)
	echo := domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "demo-echo", Support: domain.SupportNative, Source: domain.ComponentSourceCommand, SourceName: "echo"}
	withEcho := stagingPlan(t, domain.ClientKiro, domain.PackageNative)
	withEcho.Components = []domain.ComponentDecision{echo}
	withoutEcho := withEcho
	withoutEcho.Components = nil
	installed := filepath.Join(withEcho.ActivePath, "skills", "demo-echo")
	cases := map[string]struct {
		plan   domain.DeliveryPlan
		change func(*testing.T)
	}{
		"rules now withhold the command": {plan: withoutEcho},
		"rules now deliver the command": {plan: withEcho, change: func(t *testing.T) {
			if err := os.RemoveAll(installed); err != nil {
				t.Fatal(err)
			}
		}},
		"rules now render other bytes": {plan: withEcho, change: func(t *testing.T) {
			writeTestFile(t, filepath.Join(installed, commandext.SkillFile), "---\nname: \"demo-echo\"\ndescription: \"Echo the text\"\n---\nOlder body.\n")
		}},
		"rules now add a file": {plan: withEcho, change: func(t *testing.T) {
			writeTestFile(t, filepath.Join(installed, "agents", "openai.yaml"), "policy: {}\n")
		}},
	}
	for name, tc := range cases {
		stager := testStager(Stager{})
		delivery, err := stager.Stage(context.Background(), envelope, withEcho, "operation-outdated", domain.CompatibilityHints{})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(withEcho.ActivePath); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(withEcho.ActivePath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(delivery.StagingPath, withEcho.ActivePath); err != nil {
			t.Fatal(err)
		}
		digest := delivery.ArtifactDigest
		if tc.change != nil {
			tc.change(t)
			var mismatch *ports.VerificationError
			if err := stager.Verify(context.Background(), withEcho.ActivePath, digest); !errors.As(err, &mismatch) {
				t.Fatalf("%s: changed tree verified: %v", name, err)
			}
			digest = mismatch.ActualDigest
		}
		if _, err := stager.ProjectActiveNative(context.Background(), envelope, tc.plan, digest, ""); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
			t.Errorf("%s: projection error = %v", name, err)
		}
	}
}

// Reading command skills back must look where each client's projection puts
// them, Claude Code's isolated runtime directory included, or an installed
// tree would look outdated forever. For every client that receives commands,
// a freshly staged tree verifies as current, and as outdated against a plan
// that no longer delivers the command.
func TestCommandSkillVerifierFollowsEveryProjector(t *testing.T) {
	t.Parallel()
	envelope := commandPackage(t)
	stager := testStager(Stager{})
	for _, definition := range domain.ClientDefinitions() {
		if !domain.DeliversCommand(envelope.Commands["echo"], definition) {
			continue
		}
		t.Run(string(definition.ID), func(t *testing.T) {
			t.Parallel()
			plan := stagingPlan(t, definition.ID, definition.Capabilities.PackageMode)
			plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "demo-echo", Support: definition.Capabilities.SkillSupport, Source: domain.ComponentSourceCommand, SourceName: "echo"}}
			if definition.ID == domain.ClientOpenCode {
				plan.OpenCodeHost = contracttest.OpenCodeV1Host{}
			}
			delivery, err := stager.Stage(context.Background(), envelope, plan, "operation-verify", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			if err := stager.VerifyCommandSkills(context.Background(), delivery.StagingPath, envelope, plan); err != nil {
				t.Fatalf("staged tree is not current: %v", err)
			}
			withheld := plan
			withheld.Components = nil
			if err := stager.VerifyCommandSkills(context.Background(), delivery.StagingPath, envelope, withheld); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
				t.Fatalf("a withheld command still verified: %v", err)
			}
		})
	}
}

// Rendered files come from a client decorator. Writing them re-checks that
// each one stays inside its skill directory, so a decorator that names a
// path outside it writes nothing there.
func TestWriteCommandSkillsKeepsFilesInTheSkill(t *testing.T) {
	t.Parallel()
	staging := t.TempDir()
	rendered := commandSkills{"demo-echo": {commandext.SkillFile: []byte("skill\n"), "../../escape.txt": []byte("outside\n")}}
	if err := writeCommandSkills(testStager(Stager{}).Paths, staging, rendered); err == nil {
		t.Fatal("a file outside the command skill was written")
	}
	assertMissing(t, filepath.Join(staging, "escape.txt"))
}

func ownedKiroSkills(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	var skills []domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind != "managed_package_directory" && object.Kind != kiro.MCPObjectKind {
			skills = append(skills, object)
		}
	}
	return skills
}

// No client enforces explicit invocation today. A plan that carries an
// explicit command anyway is refused before the target or staging exists.
func TestStagerRefusesExplicitCommandWithoutTheTrait(t *testing.T) {
	t.Parallel()
	envelope := commandPackage(t)
	plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
	plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "demo-wizard", Support: domain.SupportProjected, Source: domain.ComponentSourceCommand, SourceName: "wizard"})
	_, err := testStager(Stager{}).Stage(context.Background(), envelope, plan, "operation-explicit", domain.CompatibilityHints{})
	if err == nil || !strings.Contains(err.Error(), "cannot enforce explicit invocation") {
		t.Fatalf("stage error = %v", err)
	}
	if _, statErr := os.Lstat(plan.TargetAnchor); !os.IsNotExist(statErr) {
		t.Fatalf("refused stage wrote to disk: %v", statErr)
	}
}

type fakeDecorator func(domain.RenderedSkill) (domain.RenderedSkill, error)

func (decorate fakeDecorator) DecorateExplicitSkill(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
	return decorate(skill)
}

// The trait and the decorator are both required, and the decorator's output
// is checked the same way for every client: the written header must still
// say disable-model-invocation: true as a YAML boolean.
func TestRenderCommandSkillsIsFailClosedForExplicitCommands(t *testing.T) {
	t.Parallel()
	envelope := commandPackage(t)
	plan := domain.DeliveryPlan{ClientID: domain.ClientClaude, Components: []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "demo-wizard", Support: domain.SupportProjected, Source: domain.ComponentSourceCommand, SourceName: "wizard"},
	}}
	withFlag := fakeDecorator(func(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
		skill.Frontmatter = append(skill.Frontmatter, domain.FrontmatterField{Key: domain.ExplicitInvocationField, Value: true})
		return skill, nil
	})
	appendFlag := func(value any) fakeDecorator {
		return func(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
			skill.Frontmatter = append(skill.Frontmatter, domain.FrontmatterField{Key: domain.ExplicitInvocationField, Value: value})
			return skill, nil
		}
	}
	cases := map[string]struct {
		enforces  bool
		decorator clients.ExplicitSkillDecorator
	}{
		"trait false":      {false, withFlag},
		"no decorator":     {true, nil},
		"flag dropped":     {true, fakeDecorator(func(skill domain.RenderedSkill) (domain.RenderedSkill, error) { return skill, nil })},
		"flag as a string": {true, appendFlag("true")},
		"flag false":       {true, appendFlag(false)},
		"description moved": {true, fakeDecorator(func(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
			skill.Frontmatter[1].Value = "auto"
			return withFlag(skill)
		})},
	}
	for name, tc := range cases {
		if _, err := renderCommandSkills(context.Background(), envelope, plan, tc.enforces, tc.decorator); err == nil {
			t.Errorf("%s: explicit command rendered", name)
		}
	}
	rendered, err := renderCommandSkills(context.Background(), envelope, plan, true, withFlag)
	if err != nil || !strings.Contains(string(rendered["demo-wizard"][commandext.SkillFile]), "\ndisable-model-invocation: true\n") {
		t.Fatalf("decorated explicit command = %q, %v", rendered, err)
	}
}
