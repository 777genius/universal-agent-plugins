package targetcontracts

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

func TestManagedArtifactRulesUsesReadableConditions(t *testing.T) {
	t.Parallel()
	profile := platformmeta.PlatformProfile{
		ManagedArtifacts: []platformmeta.ManagedArtifactSpec{
			{Kind: platformmeta.ManagedArtifactStatic, Path: ".app.json"},
			{Kind: platformmeta.ManagedArtifactPortableSkills, OutputRoot: "skills"},
			{Kind: platformmeta.ManagedArtifactSelectedContext},
		},
	}

	got := managedArtifactStrings(managedArtifactRules(profile))
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		".app.json (when app_manifest is enabled)",
		"skills/** (when portable skills are authored)",
		"GEMINI.md or selected root context (when contexts are authored)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("managed artifacts missing %q:\n%s", want, joined)
		}
	}
}
