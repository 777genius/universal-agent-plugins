package agentpluginscli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
)

type managementWebCandidate struct {
	name, previewDigest string
	available           bool
}
type managementWebCatalog struct {
	source, indexDigest string
	candidates          map[string]managementWebCandidate
}

func (b *ManagementBackend) websiteSkillsCatalog(ctx context.Context, input management.SourceRequest) (management.SkillCatalog, error) {
	if input.Ref != "" || input.Path != "" {
		return management.SkillCatalog{}, errors.New("HTTPS publisher catalogs do not accept Git ref or path")
	}
	lister, ok := b.app.SourceAcquirer.(interface {
		ListHTTPSkills(context.Context, string) (sourceacquisition.HTTPSkillCatalog, error)
	})
	if !ok {
		return management.SkillCatalog{}, errors.New("HTTPS publisher catalog unavailable")
	}
	discovered, err := lister.ListHTTPSkills(ctx, input.Source)
	if err != nil {
		return management.SkillCatalog{}, err
	}
	id := newSkillCatalogID()
	retained := managementWebCatalog{source: discovered.URL, indexDigest: discovered.IndexSHA256, candidates: map[string]managementWebCandidate{}}
	result := management.SkillCatalog{CatalogId: id, Source: discovered.URL, SourceRevision: "sha256:" + discovered.IndexSHA256, Candidates: []management.SkillCandidate{}, Diagnostics: []management.Diagnostic{}}
	for _, candidate := range discovered.Candidates {
		digest := sha256.Sum256([]byte(id + "\x00" + candidate.Name))
		candidateID := "candidate-" + hex.EncodeToString(digest[:])
		retained.candidates[candidateID] = managementWebCandidate{name: candidate.Name, available: candidate.Available}
		result.Candidates = append(result.Candidates, management.SkillCandidate{Id: candidateID, Name: candidate.Name, Description: candidate.Description, Path: candidate.Name, Available: candidate.Available, Reason: candidate.Reason})
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || len(b.catalogs)+len(b.webCatalogs) >= 16 {
		return management.SkillCatalog{}, errors.New("catalog session limit reached; open a new session")
	}
	b.webCatalogs[id] = retained
	return result, nil
}

func (b *ManagementBackend) prepareWebsiteSkills(ctx context.Context, input management.ChangeRequest, clients []string, operationID string, website managementWebCatalog) (management.Prepared, error) {
	b.mu.Lock()
	selections := []SkillsSelection{}
	seen := map[string]bool{}
	for _, id := range input.CandidateIds {
		candidate, ok := website.candidates[id]
		if b.closed || !ok || seen[id] || !candidate.available {
			b.mu.Unlock()
			return nil, errors.New("select unique available skill candidates")
		}
		seen[id] = true
		selections = append(selections, SkillsSelection{Website: website.source, Name: candidate.name, CandidateID: id, ExpectedIndexSHA256: website.indexDigest, ExpectedRawTreeDigest: candidate.previewDigest})
	}
	b.mu.Unlock()
	return b.prepareSelectedSkills(ctx, input, clients, operationID, selections)
}

func (b *ManagementBackend) prepareSelectedSkills(ctx context.Context, input management.ChangeRequest, clients []string, operationID string, selections []SkillsSelection) (management.Prepared, error) {
	prepared, err := b.app.PrepareSkills(ctx, SkillsBatchRequest{Selections: selections, TargetIDs: clients})
	if err != nil {
		return nil, err
	}
	p, err := b.finishManagementPreparation(ctx, prepared.preview, prepared.Close, input, clients, prepared.request, operationID)
	if err != nil {
		prepared.Close()
		return nil, err
	}
	for _, warning := range prepared.Warnings() {
		p.review.Warnings = append(p.review.Warnings, managementDiagnostic("INSTALLER_WARNING", warning))
	}
	return p, nil
}
