package agentpluginscli

import "testing"

// A publisher URL that is not in canonical form must still derive the Skill
// name as the default alias, the same as the canonical URL does.
func TestWebsiteDefaultAliasUsesCanonicalPublisherURL(t *testing.T) {
	for _, website := range []string{"https://publisher.example/docs", "https://Publisher.Example/docs/"} {
		if got := defaultSkillAlias(SkillsSelection{Website: website, Name: "pdf"}); got != "pdf" {
			t.Fatalf("alias for %q = %q, want the Skill name", website, got)
		}
	}
}
