package agentpluginscli

import (
	"context"
	"errors"

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
		return management.SkillCatalog{}, errors.New("catalog session limit reached; close this session before opening another")
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
	session := b.catalogs[input.CatalogId]
	website, webOK := b.webCatalogs[input.CatalogId]
	b.mu.Unlock()
	if webOK {
		return b.prepareWebsiteSkills(ctx, input, clients, operationID, website)
	}
	if session == nil {
		return nil, errors.New("skill catalog is missing or expired; discover the source again")
	}
	selections := []SkillsSelection{}
	seen := map[string]bool{}
	session.mu.Lock()
	for _, id := range input.CandidateIds {
		candidate, ok := session.candidates[id]
		if !ok || seen[id] || candidate.Status != "available" {
			session.mu.Unlock()
			return nil, errors.New("select unique available skill candidates")
		}
		seen[id] = true
		selections = append(selections, session.selection(candidate))
	}
	session.mu.Unlock()
	return b.prepareSelectedSkills(ctx, input, clients, operationID, selections)
}
