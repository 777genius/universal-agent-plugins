package shared

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func rawSkillAuthenticationFixture(web bool) (domain.PackageEnvelope, domain.DeliveryPlan) {
	hash := strings.Repeat("a", 64)
	source := domain.SourceIdentity{StableSourceKey: "github-skill:example/skills:docs:skills/docs", SourceBindingHint: "uap-dependency-skill",
		Skill: &domain.SkillSourceFacts{Name: "docs", ResolvedPath: "skills/docs", RawTreeDigest: "sha256:" + hash, Normalizer: "agent-skill-package/v1"}}
	if web {
		source.Skill, source.SourceBindingHint = nil, "uap-dependency-web-skill"
		source.StableSourceKey = "https-skill:https://skills.example:docs"
		source.WebSkill = &domain.WebSkillSourceFacts{URL: "https://skills.example", Skill: "docs",
			IndexURL: "https://skills.example/.well-known/agent-skills/index.json", IndexSHA256: hash,
			Normalizer: "agent-skill-package/v1", RawTreeDigest: "sha256:" + hash,
			Files: []domain.WebSkillFile{{Path: "SKILL.md", URL: "https://skills.example/.well-known/agent-skills/docs/SKILL.md", SHA256: hash}}}
	}
	envelope := domain.PackageEnvelope{Source: source, Skills: map[string]domain.Skill{"docs": {Name: "docs"}}}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, Status: domain.PlanReady, Authentication: domain.AuthenticationNotChecked,
		Components:  []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}},
		UserActions: []string{"verify the plugin's authentication requirements before using it", "preserve native discovery step"},
		Warnings:    []string{"authentication_not_catalog_verified", "preserve independent warning"}}
	return envelope, plan
}

func TestRawSkillAuthenticationQualifiesOnlyNativeInstallation(t *testing.T) {
	for _, web := range []bool{false, true} {
		envelope, plan := rawSkillAuthenticationFixture(web)
		QualifyRawSkillInstallationAuthentication(envelope, &plan)
		if plan.Authentication != domain.AuthenticationNotRequired || plan.Activation != "" || plan.Verification != "" ||
			!slices.Equal(plan.UserActions, []string{"preserve native discovery step"}) ||
			!slices.Equal(plan.Warnings, []string{"preserve independent warning"}) || envelope.CatalogEvidence != nil {
			t.Fatalf("raw Skill installation authentication qualified unrelated evidence: %+v", plan)
		}
		before := plan
		QualifyRawSkillInstallationAuthentication(envelope, &plan)
		if !reflect.DeepEqual(before, plan) {
			t.Fatal("qualification is not idempotent")
		}
	}
}

func TestRawSkillAuthenticationPreservesUnqualifiedAndExplicitPolicy(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.PackageEnvelope, *domain.DeliveryPlan)
	}{
		{"general-plugin", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.Skill = nil }},
		{"missing-source-key", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.StableSourceKey = "" }},
		{"wrong-source-kind", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			e.Source.SourceBindingHint = "uap-dependency-git"
		}},
		{"wrong-normalizer", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.Skill.Normalizer = "unknown" }},
		{"missing-raw-digest", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.Skill.RawTreeDigest = "" }},
		{"missing-resolved-path", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.Skill.ResolvedPath = "" }},
		{"different-source-skill", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Source.Skill.Name = "other" }},
		{"ambiguous-source", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			web, _ := rawSkillAuthenticationFixture(true)
			e.Source.WebSkill = web.Source.WebSkill
		}},
		{"catalog-unknown", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.CatalogEvidence = &domain.CatalogEvidence{} }},
		{"catalog-required", func(e *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			e.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{"codex": {Authentication: domain.AuthenticationRequirementRequired}}}
			p.Authentication = domain.AuthenticationPending
		}},
		{"authentication-pending", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			p.Authentication = domain.AuthenticationPending
		}},
		{"authentication-complete", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			p.Authentication = domain.AuthenticationComplete
		}},
		{"unsupported-plan", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) { p.Status = domain.PlanUnsupported }},
		{"unsupported-skill", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			p.Components[0].Support = domain.SupportUnsupported
		}},
		{"unqualified-component", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) { p.Components[0].Support = "" }},
		{"different-plan-skill", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) { p.Components[0].Name = "other" }},
		{"extra-selected-component", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			p.Components = append(p.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "server", Support: domain.SupportNative})
		}},
		{"mcp-file", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.MCP.Present = true }},
		{"mcp-server", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			e.MCP.Servers = map[string]domain.MCPServer{"remote": {Name: "remote"}}
		}},
		{"app-binding", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			e.App.Bindings = map[string]domain.AppBinding{"remote": {ID: "provider"}}
		}},
		{"authored-command", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			e.Commands = map[string]domain.Command{"command": {Name: "command"}}
		}},
		{"projected-command", func(_ *domain.PackageEnvelope, p *domain.DeliveryPlan) {
			p.Components[0].Source = domain.ComponentSourceCommand
		}},
		{"extension", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Inventory.Extensions = []string{"provider"} }},
		{"invalid-skill", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) { e.Inventory.InvalidSkills = []string{"docs"} }},
		{"extra-skill", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			e.Skills["other"] = domain.Skill{Name: "other"}
		}},
		{"invalid-web-source", func(e *domain.PackageEnvelope, _ *domain.DeliveryPlan) {
			web, _ := rawSkillAuthenticationFixture(true)
			e.Source = web.Source
			e.Source.WebSkill.Files[0].SHA256 = "invalid"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			envelope, plan := rawSkillAuthenticationFixture(false)
			test.change(&envelope, &plan)
			before := plan
			QualifyRawSkillInstallationAuthentication(envelope, &plan)
			if !reflect.DeepEqual(before, plan) {
				t.Fatalf("unqualified source or explicit authentication policy changed: before=%+v after=%+v", before, plan)
			}
		})
	}
}
