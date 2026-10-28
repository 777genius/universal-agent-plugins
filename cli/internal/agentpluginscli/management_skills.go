package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func (b *ManagementBackend) SkillsCatalog(ctx context.Context, input management.SourceRequest) (management.SkillCatalog, error) {
	if isWebsiteSkillInput(input.Source) {
		return b.websiteSkillsCatalog(ctx, input)
	}
	session, err := b.app.openSkillsCatalog(ctx, SkillsPickerRequest{Input: input.Source, Ref: input.Ref, Path: input.Path, PathSet: input.Path != ""})
	if err != nil {
		return management.SkillCatalog{}, err
	}
	catalog, err := session.scan(ctx, session.source.RequestedPath)
	if err != nil {
		session.cleanup()
		return management.SkillCatalog{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || len(b.catalogs)+len(b.webCatalogs) >= 16 {
		session.cleanup()
		return management.SkillCatalog{}, managementRefusal("CATALOG_LIMIT", "Discard an unused source catalog before discovering more repositories")
	}
	b.catalogs[catalog.CatalogId] = session
	out := management.SkillCatalog{CatalogId: catalog.CatalogId, Source: input.Source, SourceRevision: catalog.SourceRevision, Candidates: []management.SkillCandidate{}, Diagnostics: []management.Diagnostic{}}
	for _, candidate := range catalog.Candidates {
		out.Candidates = append(out.Candidates, management.SkillCandidate{Id: candidate.Id, Name: candidate.Name, Description: candidate.Description, Path: candidate.Path, Available: candidate.Status == "available"})
	}
	for _, diagnostic := range catalog.Diagnostics {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic(diagnostic.Code, diagnostic.Message))
	}
	return out, nil
}

func (b *ManagementBackend) prepareSkills(ctx context.Context, input management.ChangeRequest, clients []string, operationID string) (management.Prepared, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, managementRefusal("CATALOG_EXPIRED", "Discover the Skill source in an open management session")
	}
	session := b.catalogs[input.CatalogId]
	website, webOK := b.webCatalogs[input.CatalogId]
	b.mu.Unlock()
	if webOK {
		return b.prepareWebsiteSkills(ctx, input, clients, operationID, website)
	}
	if session == nil {
		return nil, managementRefusal("CATALOG_EXPIRED", "Discover the Skill source again before selecting candidates")
	}
	selections := []SkillsSelection{}
	seen := map[string]bool{}
	session.mu.Lock()
	for _, id := range input.CandidateIds {
		candidate, ok := session.candidates[id]
		if !ok || seen[id] || candidate.Status != "available" {
			session.mu.Unlock()
			return nil, invalidManagementRequest("Select unique available Skill candidates from the current catalog")
		}
		seen[id] = true
		selections = append(selections, session.selection(candidate))
	}
	session.mu.Unlock()
	return b.prepareSelectedSkills(ctx, input, clients, operationID, selections)
}

// DiscardSkillsCatalog releases only discovery resources. Prepared selected-Skill
// plans own independently retained source snapshots and remain valid.
func (b *ManagementBackend) DiscardSkillsCatalog(ctx context.Context, catalogID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if session := b.catalogs[catalogID]; session != nil {
		delete(b.catalogs, catalogID)
		session.cleanup()
	}
	delete(b.webCatalogs, catalogID)
	return nil
}
