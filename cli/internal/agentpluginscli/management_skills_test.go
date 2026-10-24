package agentpluginscli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

func TestManagementGitSkillCatalogRetainsExactScopeAndSelection(t *testing.T) {
	j, _, sha := skillJourney(t)
	backend := managementJourneyBackend(t, j)
	source := "https://github.com/acme/tools/tree/" + sha + "/skills/pdf"
	catalog, err := backend.SkillsCatalog(t.Context(), management.SourceRequest{Source: source})
	if err != nil || len(catalog.Candidates) != 1 {
		t.Fatal(catalog, err)
	}
	candidate := catalog.Candidates[0]
	if candidate.Path != "skills/pdf" || !candidate.Available {
		t.Fatal(candidate)
	}
	preview, err := backend.SkillPreview(t.Context(), catalog.CatalogId, candidate.Id)
	if err != nil || !strings.Contains(preview.Markdown, "references/note.txt") {
		t.Fatal(preview, err)
	}
	request := management.ChangeRequest{Action: management.InstallSkills, CatalogId: catalog.CatalogId, CandidateIds: []string{candidate.Id}, TargetIds: []string{"profile-codex"}}
	managementApply(t, managementPrepare(t, backend, request))
	requirements, locked := j.intent(t)
	if requirements.Dependencies["pdf"].Git.Path != "skills/pdf" || locked.Packages[0].Source.Git.Commit != sha {
		t.Fatal("source review identity was not retained", locked)
	}
	request.CandidateIds = []string{"invented-candidate"}
	if _, err := backend.Prepare(t.Context(), request, "unknown-candidate"); err == nil {
		t.Fatal("accepted caller-invented candidate authority")
	}
}

func managementMutableWebsite(t *testing.T) (*ManagementBackend, savedJourney, string, func(string)) {
	t.Helper()
	var mu sync.Mutex
	content := "---\nname: pdf\ndescription: Read PDFs\n---\nFirst reviewed content.\n"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/.well-known/agent-skills/index.json":
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"pdf","description":"Read PDFs","files":["SKILL.md"]}]}`)
		case "/.well-known/agent-skills/pdf/SKILL.md":
			_, _ = fmt.Fprint(w, content)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	j := newSavedJourney(t, t.TempDir())
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	backend := managementJourneyBackend(t, j)
	return backend, j, server.URL, func(next string) {
		mu.Lock()
		defer mu.Unlock()
		content = next
	}
}

func TestManagementWebsitePreviewFreezesOriginalBytes(t *testing.T) {
	backend, j, source, changeContent := managementMutableWebsite(t)
	catalog, err := backend.SkillsCatalog(t.Context(), management.SourceRequest{Source: source})
	if err != nil || len(catalog.Candidates) != 1 {
		t.Fatal(catalog, err)
	}
	candidate := catalog.Candidates[0]
	preview, err := backend.SkillPreview(t.Context(), catalog.CatalogId, candidate.Id)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, err := backend.SkillPreview(t.Context(), catalog.CatalogId, candidate.Id); err != nil || repeated.Markdown != preview.Markdown {
		t.Fatal("unchanged preview must retain its original bytes", repeated, err)
	}
	changeContent(preview.Markdown + "Changed after the preview.\n")
	request := management.ChangeRequest{Action: management.InstallSkills, CatalogId: catalog.CatalogId, CandidateIds: []string{candidate.Id}, TargetIds: []string{"profile-codex"}}
	if _, err := backend.Prepare(t.Context(), request, "stale-website"); !errors.Is(err, ErrSkillsPlanStale) {
		t.Fatal("mutable publisher content bypassed preview binding", err)
	}
	if _, err := backend.SkillPreview(t.Context(), catalog.CatalogId, candidate.Id); !errors.Is(err, management.ErrPlanStale) {
		t.Fatal("a later preview replaced the retained candidate's original authority", err)
	}
	if _, err := backend.Prepare(t.Context(), request, "still-stale-website"); !errors.Is(err, ErrSkillsPlanStale) {
		t.Fatal("refused preview changed the earlier review's authority", err)
	}
	if err := backend.DiscardSkillsCatalog(t.Context(), catalog.CatalogId); err != nil {
		t.Fatal(err)
	}
	catalog, err = backend.SkillsCatalog(t.Context(), management.SourceRequest{Source: source})
	if err != nil || len(catalog.Candidates) != 1 {
		t.Fatal(catalog, err)
	}
	candidate = catalog.Candidates[0]
	if _, err := backend.SkillPreview(t.Context(), catalog.CatalogId, candidate.Id); err != nil {
		t.Fatal("rediscovery must permit reviewing the new publisher bytes", err)
	}
	request.CatalogId, request.CandidateIds = catalog.CatalogId, []string{candidate.Id}
	prepared := managementPrepare(t, backend, request)
	if err := backend.DiscardSkillsCatalog(t.Context(), catalog.CatalogId); err != nil {
		t.Fatal(err)
	}
	if len(backend.webCatalogs) != 0 {
		t.Fatal("discard retained publisher catalog state")
	}
	managementApply(t, prepared)
	_, locked := j.intent(t)
	if locked.Packages[0].Source.Web == nil || locked.Packages[0].Source.Web.IndexSHA256 == "" || locked.Packages[0].Source.Web.RawTreeDigest == "" {
		t.Fatal("publisher provenance missing", locked)
	}
}
