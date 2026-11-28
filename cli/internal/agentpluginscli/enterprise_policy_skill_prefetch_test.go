package agentpluginscli

import (
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestEnterprisePolicySkillPrefetchChecksSelectedTargets(t *testing.T) {
	journey, source, commit := multiSkillJourney(t, []string{"alpha", "beta"})
	gate := sourcePolicyApp(t, "schema_version: 2\ninstall:\n  allowed_targets: [codex]\n").InstallationPolicy
	app := journey.cli.app.withEnterprisePolicyGate(gate)
	plan, err := app.PrepareSkills(t.Context(), SkillsBatchRequest{
		Selections: skillSelections("acme/tools", commit, "alpha", "beta"),
		TargetIDs:  []string{"claude"},
	})
	if plan != nil {
		defer plan.Close()
	}
	var denied *domain.InstallPolicyError
	if !errors.As(err, &denied) {
		t.Fatalf("expected target policy refusal, got %v", err)
	}
	if calls := source.keys(); len(calls) != 0 {
		t.Fatalf("denied target acquired raw Skill trees: %v", calls)
	}
}

type enterprisePrefetchSource struct {
	SourceAcquirer
	*prefetchSkillAcquirer
}

func TestEnterprisePolicySkillPrefetchAdmitsAllowedTarget(t *testing.T) {
	plentyOfPrefetchDisk(t)
	raw := &prefetchSkillAcquirer{dir: t.TempDir()}
	app := sourcePolicyApp(t, "schema_version: 2\ninstall:\n  allowed_targets: [codex]\n")
	app.SourceAcquirer = enterprisePrefetchSource{prefetchSkillAcquirer: raw}
	candidate := newSkillsBatchCandidate(environmentSnapshot{})
	commit := strings.Repeat("a", 40)
	prefetch := candidate.prefetchRawSkills(t.Context(), app, skillSelections("acme/tools", commit, "alpha", "beta"), []string{"codex"})
	if prefetch == nil {
		t.Fatal("allowed batch did not start prefetch")
	}
	defer prefetch.close()
	waitFor(t, "both admitted Skill acquisitions", func() bool { return len(raw.startedPaths()) == 2 })
	if calls := raw.calls.Load(); calls != 2 {
		t.Fatalf("raw Skill acquisitions = %d, want both selections", calls)
	}
}
