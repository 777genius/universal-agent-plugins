package planner

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func commandEnvelope() domain.PackageEnvelope {
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "demo", Version: "1.0.0"},
		Commands: map[string]domain.Command{
			"echo":   {Name: "echo", Description: "Echo"},
			"wizard": {Name: "wizard", Description: "Wizard", ExplicitOnly: true},
		},
	}
}

func commandComponents(plan domain.DeliveryPlan) []string {
	var names []string
	for _, component := range plan.Components {
		if component.Source == domain.ComponentSourceCommand {
			names = append(names, component.SourceName+"="+component.Name)
		}
	}
	return names
}

func hasCommandDiagnostic(diagnostics []domain.Diagnostic, item, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Boundary == domain.BoundaryCommand && diagnostic.Item == item && diagnostic.Code == code {
			return true
		}
	}
	return false
}

// The planner is the only place that decides whether an explicit-only
// command reaches a client, and it decides by the declared trait alone. A
// client without the trait must never get the command as an ordinary skill.
func TestCommandDeliveryFollowsTheExplicitInvocationTrait(t *testing.T) {
	t.Parallel()
	root := goldenRoot(t)
	planner := testPlanner(Planner{ManagedRoot: filepath.Join(root, "managed")})
	for _, definition := range domain.ClientDefinitions() {
		t.Run(string(definition.ID), func(t *testing.T) {
			plan, err := planner.Plan(context.Background(), commandEnvelope(), goldenClient(definition.ID, root), domain.ScopeUser, "demo-0123456789ab")
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"echo=demo-echo"}
			if definition.Traits.EnforcesExplicitSkillInvocation {
				want = append(want, "wizard=demo-wizard")
			} else if !hasCommandDiagnostic(plan.Diagnostics, "wizard", explicitInvocationNotEnforceable) {
				t.Fatalf("withheld command has no diagnostic: %+v", plan.Diagnostics)
			}
			if got := commandComponents(plan); !reflect.DeepEqual(got, want) {
				t.Fatalf("command components = %v, want %v", got, want)
			}
		})
	}
	enforcing, _ := domain.ClientDefinitionFor(domain.ClientClaude)
	enforcing.Traits.EnforcesExplicitSkillInvocation = true
	delivered, withheld := commandDecisions(commandEnvelope(), enforcing)
	if len(delivered) != 2 || len(withheld) != 0 || delivered[1].SourceName != "wizard" || delivered[1].Support != enforcing.Capabilities.SkillSupport {
		t.Fatalf("enforcing client: delivered = %+v, withheld = %+v", delivered, withheld)
	}
	enforcing.Capabilities.SkillSupport = domain.SupportUnsupported
	if delivered, withheld = commandDecisions(commandEnvelope(), enforcing); len(delivered) != 0 || len(withheld) != 2 || withheld[0].Code != componentNotSupported {
		t.Fatalf("skill-less client: delivered = %+v, withheld = %+v", delivered, withheld)
	}
}

func TestCommandVerdicts(t *testing.T) {
	t.Parallel()
	root := goldenRoot(t)
	planner := testPlanner(Planner{ManagedRoot: filepath.Join(root, "managed")})
	plan := func(envelope domain.PackageEnvelope) domain.DeliveryPlan {
		t.Helper()
		result, err := planner.Plan(context.Background(), envelope, goldenClient(domain.ClientClaude, root), domain.ScopeUser, "demo-0123456789ab")
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	onlyExplicit := commandEnvelope()
	delete(onlyExplicit.Commands, "echo")
	if result := plan(onlyExplicit); result.Status != domain.PlanUnsupported || !compatContains(result.Warnings, "no_supported_components") || len(result.Components) != 0 {
		t.Fatalf("explicit-only package = %+v", result)
	}
	withSkill := onlyExplicit
	withSkill.Skills = map[string]domain.Skill{"guide": {Name: "guide"}}
	if result := plan(withSkill); result.Status == domain.PlanUnsupported || !compatContains(result.Warnings, explicitInvocationNotEnforceable) {
		t.Fatalf("skill plus withheld command = %+v", result)
	}
	invalid := domain.PackageEnvelope{
		Manifest:        domain.PluginManifest{Name: "demo"},
		InvalidCommands: []string{"bad"},
		Diagnostics:     []domain.Diagnostic{{Severity: domain.SeverityError, Boundary: domain.BoundaryCommand, Code: "command_unknown_field", Item: "bad"}},
	}
	if result := plan(invalid); result.Status != domain.PlanUnsupported || !compatContains(result.Warnings, "no_valid_components") {
		t.Fatalf("invalid-only package = %+v", result)
	}
}

func TestCommandCompatibilityRows(t *testing.T) {
	t.Parallel()
	envelope := commandEnvelope()
	envelope.Skills = map[string]domain.Skill{"zeta": {Name: "zeta"}}
	envelope.Inventory.InvalidSkillsRoot = true
	envelope.InvalidCommands = []string{"bad"}
	envelope.Diagnostics = []domain.Diagnostic{
		{Severity: domain.SeverityError, Boundary: domain.BoundaryCommand, Code: "command_unknown_field", Item: "bad"},
		{Severity: domain.SeverityError, Boundary: domain.BoundaryCommand, Code: "command_body_empty", Item: "bad"},
	}
	result, err := testCompatibility(envelope, []domain.ClientID{domain.ClientClaude})
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Index       int
		Source      string
		Support     domain.SupportLevel
		Limitations []string
	}
	got := make([]row, 0, len(result[0].Components))
	for _, component := range result[0].Components {
		got = append(got, row{component.Index, component.Source, component.Support, component.Limitations})
	}
	want := []row{
		{1, "", domain.SupportUnsupported, []string{invalidComponent}},
		{2, domain.ComponentSourceCommand, domain.SupportUnsupported, []string{invalidComponent, "command_body_empty", "command_unknown_field"}},
		{3, domain.ComponentSourceCommand, domain.SupportProjected, nil},
		{4, domain.ComponentSourceCommand, domain.SupportUnsupported, []string{explicitInvocationNotEnforceable}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %+v, want %+v", got, want)
	}
}

type preparedHost struct{ skills bool }

func (host preparedHost) ValidateNative(skills bool, _ []string) error {
	if skills && !host.skills {
		return errors.New("skill effects were not prepared")
	}
	return nil
}

// OpenCode prepares its host from the package before planning, and RefinePlan
// rejects a plan that needs more than was prepared. A package whose only
// skills come from commands must still prepare skill support.
func TestOpenCodeHostPreparesSkillsForCommandsOnly(t *testing.T) {
	t.Parallel()
	envelope := commandEnvelope()
	envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"remote": {Name: "remote", Type: "streamable-http", Decoded: map[string]any{"url": "https://example.test/mcp"}},
	}}
	definition, _ := domain.ClientDefinitionFor(domain.ClientOpenCode)
	skills, _ := clients.OpenCodeNativeRequirements(envelope, definition)
	if !skills {
		t.Fatal("a delivered command did not count as a skill requirement")
	}
	root := goldenRoot(t)
	client := goldenClient(domain.ClientOpenCode, root)
	client.OpenCodeHost = preparedHost{skills: skills}
	plan, err := testPlanner(Planner{ManagedRoot: filepath.Join(root, "managed")}).Plan(context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil || !reflect.DeepEqual(commandComponents(plan), []string{"echo=demo-echo"}) {
		t.Fatalf("plan = %+v, err = %v", plan, err)
	}
}
