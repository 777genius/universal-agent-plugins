package gemini

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestGeminiSkillsInventoryRequiresExactEnabledOrigin(t *testing.T) {
	root := t.TempDir()
	components := make([]domain.ComponentDecision, 1, 2)
	components[0] = domain.ComponentDecision{Kind: domain.ComponentSkill, Name: "audit"}
	good := "Discovered Agent Skills:\n\naudit [Enabled]\n  Description: audit marker\n  Location:    " + filepath.Join(root, "skills", "audit", "SKILL.md") + "\n\n"
	if !geminiSkillsListed([]byte(good), root, components) {
		t.Fatal("exact native discovery refused")
	}
	for _, bad := range []string{
		strings.Replace(good, "[Enabled]", "[Disabled]", 1),
		strings.Replace(good, root, root+"-foreign", 1),
		good + strings.TrimPrefix(good, "Discovered Agent Skills:\n\n"),
		strings.Replace(good, "[Enabled]", "[Enabled] [Built-in]", 1),
		good + "unexpected diagnostics\n", good + "\x1b[0m",
	} {
		if geminiSkillsListed([]byte(bad), root, components) {
			t.Fatalf("accepted uncertain native output %q", bad)
		}
	}
	if geminiSkillsListed([]byte(good), root, append(components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "audit"})) {
		t.Fatal("skills output qualified MCP")
	}
}

func TestGeminiHumanInventoryRejectsDescriptionRecordInjection(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SKILL.md")
	source := "---\nname: audit\ndescription: audit marker\n---\nLocal marker.\n"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	good := "Discovered Agent Skills:\n\naudit [Enabled]\n  Description: audit marker\n  Location:    " + path + "\n\n"
	kernel := nativeconfig.New()
	if !geminiSkillMetadataMatches(context.Background(), kernel, []byte(good)) {
		t.Fatal("native metadata did not match")
	}
	malicious := strings.Replace(source, "description: audit marker", "description: |\n  audit marker\n  Location:    /forged/SKILL.md\n\n  synthetic [Enabled]\n    Description: spoof", 1)
	if err := os.WriteFile(path, []byte(malicious), 0600); err != nil {
		t.Fatal(err)
	}
	if geminiSkillMetadataMatches(context.Background(), kernel, []byte(good)) {
		t.Fatal("accepted description-injected native row")
	}
}
