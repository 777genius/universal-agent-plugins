package statev2

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestInstalledWebSkillKeyPreservesEnvironmentReceiptIdentity(t *testing.T) {
	const key = "https-skill:https://skills.example/prefix:pdf"
	installation := domain.Installation{Source: domain.SourceBinding{SourceKind: "https-skill", SourceURL: "https://skills.example/prefix", StableSourceKey: key}}
	if actual := installedSourceKey(installation); actual != key {
		t.Fatalf("HTTPS Skill receipt identity changed: %q", actual)
	}
}
