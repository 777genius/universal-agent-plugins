package statev2

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func TestInstalledGenericGitSkillKeepsStableKey(t *testing.T) {
	endpoint := "ssh://git@git.example/team/tools.git"
	key := sourcespec.GitSkillKey(endpoint, "pdf", "skills/pdf")
	installation := domain.Installation{Source: domain.SourceBinding{SourceKind: "git", SourceURL: endpoint, PackageSubpath: "moved/pdf", StableSourceKey: key}}
	if actual := installedSourceKey(installation); actual != key {
		t.Fatalf("generic Skill ownership key changed: %s", actual)
	}
	installation.Source.StableSourceKey = "arbitrary-caller-key"
	if installedSourceKey(installation) == "arbitrary-caller-key" {
		t.Fatal("generic plugin accepted arbitrary stable ownership key")
	}
}
