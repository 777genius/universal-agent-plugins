package grok

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectGrokOwnedSkillsReadyWithUnrelatedNativeSources(t *testing.T) {
	for _, origin := range []string{"project", "user", "bundled", "configToml", "server"} {
		t.Run(origin, func(t *testing.T) {
			fixture := newProjectSkillFixture(t)
			skill := map[string]any{"name": "foreign-" + origin, "description": "Retained native Skill", "userInvocable": true,
				"source": map[string]any{"type": origin, "path": filepath.Join(fixture.request.Client.ConfigRoot, origin, "SKILL.md")}}
			if origin == "project" {
				skill["collidesWith"] = "foreign"
				skill["invocableAs"] = "foreign:project"
			}
			if origin == "user" {
				skill["vendor"] = "claude"
				skill["compatibilityStatus"] = "disabled"
				skill["disabled"] = true
			}
			fixture.runner.document["skills"] = append(fixture.runner.document["skills"].([]any), skill)
			unchanged := projectSkillFilesUnchanged(t,
				filepath.Join(fixture.request.Client.ConfigRoot, "config.toml"),
				filepath.Join(fixture.request.Client.ConfigRoot, "trusted_folders.toml"))
			outcome, err := New().Activate(t.Context(), fixture.env, fixture.request)
			if err != nil || outcome.Activation != domain.ActivationActive ||
				outcome.Verification != domain.VerificationInstalled ||
				outcome.NativeEffect != domain.NativeEffectUnchanged || fixture.runner.calls != 1 {
				t.Fatalf("unrelated native %s Skill hid installed owned Skill: %+v %v", origin, outcome, err)
			}
			unchanged()
		})
	}
}

func TestProjectGrokUnrelatedNativeSourcesCannotForgeOwnedSkillProof(t *testing.T) {
	cases := map[string]func(*projectSkillFixture){
		"folded-owner-alias": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"source": map[string]any{"type": "plugin", "plugin_name": "DEMO", "path": "/tmp/foreign/SKILL.md"}})
		},
		"unknown-source": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"source": map[string]any{"type": "unknown", "path": "/tmp/foreign/SKILL.md"}})
		},
		"native-claims-plugin": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"source": map[string]any{"type": "project", "plugin_name": "demo", "path": "/tmp/foreign/SKILL.md"}})
		},
		"selected-name-alias": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "Docs", "description": "Other", "userInvocable": true,
				"source": map[string]any{"type": "user", "path": "/tmp/foreign/SKILL.md"}})
		},
		"invalid-status": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"compatibilityStatus": "some-future-status",
				"source": map[string]any{"type": "user", "path": "/tmp/foreign/SKILL.md"}})
		},
		"unsupported-config-alias": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"source": map[string]any{"type": "config", "path": "/tmp/foreign/SKILL.md"}})
		},
		"invalid-extra-field": func(f *projectSkillFixture) {
			f.runner.document["skills"] = append(f.runner.document["skills"].([]any), map[string]any{
				"name": "another", "description": "Other", "userInvocable": true,
				"collidesWith": 1,
				"source": map[string]any{"type": "user", "path": "/tmp/foreign/SKILL.md"}})
		},
	}
	for name, alter := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectSkillFixture(t)
			alter(f)
			outcome, err := New().Activate(t.Context(), f.env, f.request)
			if err != nil || outcome.Activation != domain.ActivationPrepared ||
				outcome.Verification != domain.VerificationPackageValid || f.runner.calls != 1 {
				t.Fatalf("unqualified source fabricated Grok Ready: %+v %v", outcome, err)
			}
		})
	}
}
