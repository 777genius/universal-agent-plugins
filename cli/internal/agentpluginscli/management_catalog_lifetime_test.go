package agentpluginscli

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementCatalogDiscardReleasesQuotaWithoutInvalidatingPreparedPlan(t *testing.T) {
	j, _, sha := skillJourney(t)
	backend := managementJourneyBackend(t, j)
	source := "https://github.com/acme/tools/tree/" + sha + "/skills/pdf"
	var retained management.Prepared
	cleanups := 0
	for attempt := 0; attempt < 20; attempt++ {
		catalog, err := backend.SkillsCatalog(t.Context(), management.SourceRequest{Source: source})
		if err != nil || len(catalog.Candidates) != 1 {
			t.Fatal("a discarded source retained the catalog quota", attempt, catalog, err)
		}
		id, candidate := catalog.CatalogId, catalog.Candidates[0].Id
		session := backend.catalogs[id]
		cleanup := session.cleanup
		session.cleanup = func() { cleanups++; cleanup() }
		if attempt == 0 {
			retained = managementPrepare(t, backend, management.ChangeRequest{Action: management.InstallSkills, CatalogId: id, CandidateIds: []string{candidate}, TargetIds: []string{"profile-codex"}})
		}
		if err := backend.DiscardSkillsCatalog(t.Context(), id); err != nil {
			t.Fatal(err)
		}
		if err := backend.DiscardSkillsCatalog(t.Context(), id); err != nil {
			t.Fatal("catalog discard must be idempotent", err)
		}
		if len(backend.catalogs) != 0 || cleanups != attempt+1 {
			t.Fatal("disposal did not close exactly one source session", cleanups)
		}
		if _, err := backend.SkillPreview(t.Context(), id, candidate); err == nil {
			t.Fatal("discarded catalog remained a live preview authority")
		}
	}
	managementApply(t, retained)
	requirements, lock := j.intent(t)
	if requirements.Dependencies["pdf"].Git.Path != "skills/pdf" || lock.Packages[0].Source.Git.Commit != sha {
		t.Fatal("discard changed the independent retained plan's reviewed source", lock)
	}
	backend.Close()
	if cleanups != 20 {
		t.Fatal("shutdown repeated disposed source cleanup", cleanups)
	}
}
