package planner

import (
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	explicitInvocationNotEnforceable = "explicit_invocation_not_enforceable"
	componentNotSupported            = "component_not_supported_by_client"
	invalidComponent                 = "invalid_component"
)

// commandDecisions turns the package's valid commands into the skills this
// client receives. The decision is the client's declared trait alone. A
// command the client cannot receive is withheld: it never becomes a
// component, only a warning on the plan, so no later stage can deliver it.
func commandDecisions(envelope domain.PackageEnvelope, definition domain.ClientDefinition) ([]domain.ComponentDecision, []domain.Diagnostic) {
	var delivered []domain.ComponentDecision
	var withheld []domain.Diagnostic
	for _, name := range sortedKeys(envelope.Commands) {
		command := envelope.Commands[name]
		if domain.DeliversCommand(command, definition) {
			delivered = append(delivered, domain.ComponentDecision{
				Kind: domain.ComponentSkill, Name: commandext.SkillName(envelope.Manifest.Name, name),
				Support: definition.Capabilities.SkillSupport, Source: domain.ComponentSourceCommand, SourceName: name,
			})
			continue
		}
		code, message := componentNotSupported, "this client does not take skills, so the command is not delivered"
		if command.ExplicitOnly {
			code, message = explicitInvocationNotEnforceable, "this client cannot keep the command out of automatic invocation, so it is not delivered"
		}
		withheld = append(withheld, domain.Diagnostic{
			Severity: domain.SeverityWarning, Boundary: domain.BoundaryCommand, Code: code,
			Path: command.RelativePath, Item: name, Message: message,
		})
	}
	return delivered, withheld
}

// applyCommandDecisions adds the delivered commands to the plan's components
// and the withheld ones to its warnings, and reports whether any was withheld.
func applyCommandDecisions(plan *domain.DeliveryPlan, envelope domain.PackageEnvelope, definition domain.ClientDefinition) bool {
	delivered, withheld := commandDecisions(envelope, definition)
	plan.Components = append(plan.Components, delivered...)
	for _, diagnostic := range withheld {
		plan.Diagnostics = append(plan.Diagnostics, diagnostic)
		plan.Warnings = appendUnique(plan.Warnings, diagnostic.Code)
	}
	return len(withheld) > 0
}

// commandCompatibility is one row per command entry: delivered, withheld or
// invalid. Name holds only the sort key (the projected skill, or the file
// stem of an invalid entry); rows never expose it.
func commandCompatibility(envelope domain.PackageEnvelope, definition domain.ClientDefinition) []domain.ComponentDecision {
	rows, withheld := commandDecisions(envelope, definition)
	for _, diagnostic := range withheld {
		rows = append(rows, domain.ComponentDecision{
			Kind: domain.ComponentSkill, Name: commandext.SkillName(envelope.Manifest.Name, diagnostic.Item),
			Support: domain.SupportUnsupported, Reason: diagnostic.Code,
			Source: domain.ComponentSourceCommand, SourceName: diagnostic.Item,
		})
	}
	for _, key := range envelope.InvalidCommands {
		rows = append(rows, domain.ComponentDecision{
			Kind: domain.ComponentSkill, Name: key, Support: domain.SupportUnsupported, Reason: invalidComponent,
			Source: domain.ComponentSourceCommand, SourceName: key,
		})
	}
	return rows
}

// commandErrorCodes are the fixed error codes reported for one invalid
// command entry, sorted.
func commandErrorCodes(envelope domain.PackageEnvelope, key string) []string {
	var codes []string
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Boundary == domain.BoundaryCommand && diagnostic.Severity == domain.SeverityError && diagnostic.Item == key {
			codes = appendUnique(codes, diagnostic.Code)
		}
	}
	sort.Strings(codes)
	return codes
}
