package installer

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func acquiredWebSkillFixture() domain.AcquiredSnapshot {
	key := "https-skill:https://skills.example/prefix:pdf"
	facts := &domain.WebSkillSourceFacts{URL: "https://skills.example/prefix", Skill: "pdf", IndexURL: "https://skills.example/prefix/.well-known/agent-skills/index.json", IndexSHA256: strings.Repeat("a", 64), RawTreeDigest: "sha256:" + strings.Repeat("b", 64), Normalizer: "agent-skill-package/v1", Files: []domain.WebSkillFile{{Path: "SKILL.md", URL: "https://skills.example/prefix/.well-known/agent-skills/pdf/SKILL.md", SHA256: strings.Repeat("c", 64)}}}
	return domain.AcquiredSnapshot{OriginMode: domain.OriginModeDirect, Source: domain.SourceIdentity{SourceKind: "https-skill", SourceURL: facts.URL, WebSkill: facts, StableSourceKey: key, RequestedSource: key, CanonicalSource: key, SourceBindingHint: "uap-dependency-web-skill", ResolvedRevision: facts.RawTreeDigest}}
}

func TestAcquiredWebSkillRequiresExactPinnedAuthority(t *testing.T) {
	valid := acquiredWebSkillFixture()
	if key, err := validateAcquiredSource(&valid, true); err != nil || key != valid.Source.StableSourceKey {
		t.Fatalf("valid HTTPS Skill refused: %q %v", key, err)
	}
	for name, mutate := range map[string]func(*domain.SourceIdentity){
		"override":       func(s *domain.SourceIdentity) { s.StableSourceKey = "https-skill:https://other.example:pdf" },
		"coordinate":     func(s *domain.SourceIdentity) { s.SourceURL = "https://other.example" },
		"revision":       func(s *domain.SourceIdentity) { s.ResolvedRevision = "sha256:" + strings.Repeat("d", 64) },
		"git identity":   func(s *domain.SourceIdentity) { s.Repository = "acme/tools" },
		"missing facts":  func(s *domain.SourceIdentity) { s.WebSkill = nil },
		"normalizer":     func(s *domain.SourceIdentity) { s.WebSkill.Normalizer = "future" },
		"name":           func(s *domain.SourceIdentity) { s.WebSkill.Skill = "../pdf" },
		"raw digest":     func(s *domain.SourceIdentity) { s.WebSkill.RawTreeDigest = "sha256:invalid" },
		"index checksum": func(s *domain.SourceIdentity) { s.WebSkill.IndexSHA256 = strings.Repeat("A", 64) },
		"file checksum":  func(s *domain.SourceIdentity) { s.WebSkill.Files[0].SHA256 = "invalid" },
		"index origin": func(s *domain.SourceIdentity) {
			s.WebSkill.IndexURL = "https://other.example/.well-known/agent-skills/index.json"
		},
		"file origin":        func(s *domain.SourceIdentity) { s.WebSkill.Files[0].URL = "https://other.example/SKILL.md" },
		"file traversal":     func(s *domain.SourceIdentity) { s.WebSkill.Files[0].Path = "../SKILL.md" },
		"file collision":     func(s *domain.SourceIdentity) { s.WebSkill.Files = append(s.WebSkill.Files, s.WebSkill.Files[0]) },
		"missing entrypoint": func(s *domain.SourceIdentity) { s.WebSkill.Files = nil },
	} {
		t.Run(name, func(t *testing.T) {
			bad := acquiredWebSkillFixture()
			mutate(&bad.Source)
			if _, err := validateAcquiredSource(&bad, true); err == nil {
				t.Fatal("tampered HTTPS Skill authority admitted")
			}
		})
	}
	if _, err := validateAcquiredSource(&valid, false); err == nil {
		t.Fatal("HTTPS Skill accepted without portable environment authority")
	}
	valid.OriginMode = domain.OriginModeDirectory
	if _, err := validateAcquiredSource(&valid, true); err == nil {
		t.Fatal("HTTPS Skill accepted as Directory release")
	}
}
