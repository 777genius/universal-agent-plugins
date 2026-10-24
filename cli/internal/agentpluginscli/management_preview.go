package agentpluginscli

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func (b *ManagementBackend) SkillPreview(ctx context.Context, catalogID, candidateID string) (management.SkillPreview, error) {
	b.mu.Lock()
	git := b.catalogs[catalogID]
	website, webOK := b.webCatalogs[catalogID]
	candidate, available := website.candidates[candidateID]
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return management.SkillPreview{}, errors.New("management session closed")
	}
	if git != nil {
		preview, err := git.preview(ctx, candidateID)
		if err != nil {
			return management.SkillPreview{}, err
		}
		return management.SkillPreview{CatalogId: catalogID, CandidateId: candidateID, Markdown: preview.Content, Diagnostics: []management.Diagnostic{}}, nil
	}
	if !webOK || !available || !candidate.available {
		return management.SkillPreview{}, errors.New("skill catalog candidate is missing or unavailable")
	}
	return b.websiteSkillPreview(ctx, catalogID, candidateID, website, candidate)
}

func (b *ManagementBackend) websiteSkillPreview(ctx context.Context, catalogID, candidateID string, website managementWebCatalog, candidate managementWebCandidate) (management.SkillPreview, error) {
	app, cleanup, err := dependencyPreviewApp(b.app, &options{dryRun: true})
	if err != nil {
		return management.SkillPreview{}, err
	}
	defer cleanup()
	acquirer, err := app.webSkills()
	if err != nil {
		return management.SkillPreview{}, err
	}
	snapshot, facts, err := acquirer.ResolveHTTPSkill(ctx, website.source, candidate.name)
	if err != nil {
		return management.SkillPreview{}, err
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if facts.IndexSHA256 != website.indexDigest {
		return management.SkillPreview{}, management.ErrPlanStale
	}
	markdown, err := managementReadSkillPreview(snapshot.Root)
	if err != nil {
		return management.SkillPreview{}, err
	}
	b.mu.Lock()
	if !b.closed {
		candidate.previewDigest = snapshot.TreeDigest
		website.candidates[candidateID] = candidate
	}
	b.mu.Unlock()
	return management.SkillPreview{CatalogId: catalogID, CandidateId: candidateID, Markdown: markdown, Diagnostics: []management.Diagnostic{}}, nil
}

func managementReadSkillPreview(path string) (string, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	file, err := root.Open("SKILL.md")
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	const limit = 256 << 10
	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if len(body) > limit {
		return "", errors.New("Skill preview exceeds 256 KiB")
	}
	return string(body), nil
}
