package shared

import (
	"encoding/hex"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// QualifyRawSkillInstallationAuthentication is an explicit adapter opt-in for
// native installation of one normalized raw Skill. Codex and Claude can install
// and discover that inert package without a login or model call. This says
// nothing about credentials a Skill's future tools or model use may require.
// Catalog policy and general plugins remain authoritative and unchanged.
func QualifyRawSkillInstallationAuthentication(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan) {
	if plan.Status == domain.PlanUnsupported || plan.Authentication != domain.AuthenticationNotChecked || envelope.CatalogEvidence != nil {
		return
	}
	name := normalizedRawSkillName(envelope.Source)
	if !domain.ValidSkillName(name) || !onlyNormalizedRawSkill(envelope, *plan, name) {
		return
	}
	plan.Authentication = domain.AuthenticationNotRequired
	plan.UserActions = slices.DeleteFunc(plan.UserActions, func(action string) bool {
		return action == "verify the plugin's authentication requirements before using it"
	})
	plan.Warnings = slices.DeleteFunc(plan.Warnings, func(warning string) bool {
		return warning == "authentication_not_catalog_verified"
	})
}

func normalizedRawSkillName(source domain.SourceIdentity) string {
	if source.StableSourceKey == "" {
		return ""
	}
	if facts := source.WebSkill; facts != nil && source.Skill == nil && source.SourceBindingHint == "uap-dependency-web-skill" {
		if domain.ValidateWebSkillSourceFacts(*facts) == nil {
			return facts.Skill
		}
	}
	if facts := source.Skill; facts != nil && source.WebSkill == nil && source.SourceBindingHint == "uap-dependency-skill" &&
		facts.Normalizer == "agent-skill-package/v1" && facts.ResolvedPath != "" {
		digest, prefixed := strings.CutPrefix(facts.RawTreeDigest, "sha256:")
		decoded, err := hex.DecodeString(digest)
		if prefixed && err == nil && len(decoded) == 32 {
			return facts.Name
		}
	}
	return ""
}

func onlyNormalizedRawSkill(envelope domain.PackageEnvelope, plan domain.DeliveryPlan, name string) bool {
	if len(envelope.Skills) != 1 || envelope.Skills[name].Name != name || len(plan.Components) != 1 ||
		envelope.MCP.Present || len(envelope.MCP.Servers)+len(envelope.MCP.InvalidServer) != 0 ||
		envelope.App.Present || envelope.App.Declared || len(envelope.App.Bindings) != 0 ||
		len(envelope.Commands)+len(envelope.InvalidCommands)+len(envelope.Inventory.Extensions)+len(envelope.Manifest.Extensions) != 0 ||
		len(envelope.Inventory.InvalidSkills) != 0 || envelope.Inventory.InvalidSkillsRoot {
		return false
	}
	component := plan.Components[0]
	return component.Kind == domain.ComponentSkill && component.Name == name && component.Source == "" &&
		component.Support != "" && component.Support != domain.SupportUnsupported
}
