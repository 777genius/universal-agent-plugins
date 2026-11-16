package shared

import (
	"reflect"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func observedSkillAuthenticationPlan() domain.DeliveryPlan {
	return domain.DeliveryPlan{Status: domain.PlanReady, Authentication: domain.AuthenticationNotChecked,
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "guide", Support: domain.SupportNative}},
		Warnings:   []string{"authentication_not_catalog_verified"}}
}

func TestObservedSkillAuthenticationRequiresKnownSupportedSkills(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.DeliveryPlan)
		allow  bool
	}{
		{"native skill", func(*domain.DeliveryPlan) {}, true},
		{"projected command", func(p *domain.DeliveryPlan) {
			p.Components[0].Support, p.Components[0].Source, p.Components[0].SourceName = domain.SupportProjected, domain.ComponentSourceCommand, "echo"
		}, true},
		{"multiple skills", func(p *domain.DeliveryPlan) {
			p.Components = append(p.Components, domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "other", Support: domain.SupportProjected})
		}, true},
		{"empty plan", func(p *domain.DeliveryPlan) { p.Components = nil }, false},
		{"unsupported plan", func(p *domain.DeliveryPlan) { p.Status = domain.PlanUnsupported }, false},
		{"missing catalog marker", func(p *domain.DeliveryPlan) { p.Warnings = nil }, false},
		{"explicit catalog unknown", func(p *domain.DeliveryPlan) { p.Warnings = append(p.Warnings, "authentication_requirement_unknown") }, false},
		{"prepared skill requires explicit opt-in", func(p *domain.DeliveryPlan) { p.Components[0].Support = domain.SupportPrepared }, false},
		{"unsupported skill", func(p *domain.DeliveryPlan) { p.Components[0].Support = domain.SupportUnsupported }, false},
		{"unknown support", func(p *domain.DeliveryPlan) { p.Components[0].Support = "" }, false},
		{"manual support", func(p *domain.DeliveryPlan) { p.Components[0].Support = "manual" }, false},
		{"unnamed skill", func(p *domain.DeliveryPlan) { p.Components[0].Name = "" }, false},
		{"MCP", func(p *domain.DeliveryPlan) { p.Components[0].Kind = domain.ComponentMCPServer }, false},
		{"app", func(p *domain.DeliveryPlan) { p.Components[0].Kind = domain.ComponentApp }, false},
		{"hook extension", func(p *domain.DeliveryPlan) { p.Components[0].Kind = domain.ComponentExtension }, false},
		{"unprojected command", func(p *domain.DeliveryPlan) { p.Components[0].Kind = "command" }, false},
		{"unknown source", func(p *domain.DeliveryPlan) { p.Components[0].Source = "extension" }, false},
		{"orphan source name", func(p *domain.DeliveryPlan) { p.Components[0].SourceName = "echo" }, false},
		{"unnamed command source", func(p *domain.DeliveryPlan) { p.Components[0].Source = domain.ComponentSourceCommand }, false},
		{"unsupported sibling", func(p *domain.DeliveryPlan) {
			p.Components = append(p.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportUnsupported})
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := observedSkillAuthenticationPlan()
			test.change(&plan)
			before := plan
			before.Components, before.Warnings = slices.Clone(plan.Components), slices.Clone(plan.Warnings)
			want := domain.AuthenticationNotChecked
			if test.allow {
				want = domain.AuthenticationNotRequired
			}
			if got := ObservedSkillInstallationAuthentication(plan); got != want || !reflect.DeepEqual(plan, before) {
				t.Fatalf("installation authentication=%s want=%s or plan evidence changed: %+v", got, want, plan)
			}
		})
	}
}

func TestObservedSkillAuthenticationPreservesExplicitStates(t *testing.T) {
	for _, state := range []domain.AuthenticationState{"", domain.AuthenticationPending, domain.AuthenticationFailed, domain.AuthenticationComplete, domain.AuthenticationNotRequired, "unknown"} {
		plan := observedSkillAuthenticationPlan()
		plan.Authentication = state
		for _, qualify := range []func(domain.DeliveryPlan) domain.AuthenticationState{
			ObservedSkillInstallationAuthentication, ObservedPreparedSkillInstallationAuthentication,
		} {
			if got := qualify(plan); got != state {
				t.Fatalf("explicit authentication=%s changed to %s", state, got)
			}
		}
	}
}

func TestObservedPreparedSkillAuthenticationKeepsCatalogAndComponentGuards(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*domain.DeliveryPlan)
		allow  bool
	}{
		{"observed prepared Skill", func(*domain.DeliveryPlan) {}, true},
		{"observed projected command", func(p *domain.DeliveryPlan) {
			p.Components[0].Source, p.Components[0].SourceName = domain.ComponentSourceCommand, "echo"
		}, true},
		{"missing catalog marker", func(p *domain.DeliveryPlan) { p.Warnings = nil }, false},
		{"explicit unknown", func(p *domain.DeliveryPlan) {
			p.Warnings = append(p.Warnings, "authentication_requirement_unknown")
		}, false},
		{"unsupported plan", func(p *domain.DeliveryPlan) { p.Status = domain.PlanUnsupported }, false},
		{"no components", func(p *domain.DeliveryPlan) { p.Components = nil }, false},
		{"unknown support", func(p *domain.DeliveryPlan) { p.Components[0].Support = "" }, false},
		{"unsupported Skill", func(p *domain.DeliveryPlan) { p.Components[0].Support = domain.SupportUnsupported }, false},
		{"unknown origin", func(p *domain.DeliveryPlan) { p.Components[0].Source = "extension" }, false},
		{"missing command origin", func(p *domain.DeliveryPlan) { p.Components[0].Source = domain.ComponentSourceCommand }, false},
		{"MCP sibling", func(p *domain.DeliveryPlan) {
			p.Components = append(p.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared})
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := observedSkillAuthenticationPlan()
			plan.Components[0].Support = domain.SupportPrepared
			test.change(&plan)
			before := plan
			before.Components, before.Warnings = slices.Clone(plan.Components), slices.Clone(plan.Warnings)
			want := domain.AuthenticationNotChecked
			if test.allow {
				want = domain.AuthenticationNotRequired
			}
			if got := ObservedPreparedSkillInstallationAuthentication(plan); got != want || !reflect.DeepEqual(plan, before) {
				t.Fatalf("prepared installation authentication=%s want=%s or plan changed: %+v", got, want, plan)
			}
			if got := ObservedSkillInstallationAuthentication(plan); got != domain.AuthenticationNotChecked {
				t.Fatalf("default qualifier admitted prepared support: %s", got)
			}
		})
	}
}
