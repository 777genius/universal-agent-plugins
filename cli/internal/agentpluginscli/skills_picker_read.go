package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (s *skillsCatalogSession) preview(ctx context.Context, id string) (skillspicker.Preview, error) {
	s.mu.Lock()
	candidate, ok := s.candidates[id]
	catalogID := s.catalog.CatalogId
	s.mu.Unlock()
	if !ok {
		return skillspicker.Preview{}, errors.New("unknown catalog candidate")
	}
	result, err := s.session.Preview(ctx, candidate.Path)
	if err != nil {
		return skillspicker.Preview{}, err
	}
	preview := skillspicker.Preview{CatalogId: catalogID, CandidateId: id, Path: result.Path, Content: result.Markdown, RawContent: &result.RawContent, Format: skillspicker.Markdown, Resources: []skillspicker.Resource{}, Diagnostics: []skillspicker.Diagnostic{}}
	for _, resource := range result.Resources {
		preview.Resources = append(preview.Resources, wireSkillResource(resource))
	}
	return preview, nil
}

func (app App) skillsTargets(ctx context.Context) ([]skillspicker.Target, error) {
	if err := app.checkSkillsContext(nil); err != nil {
		return nil, err
	}
	if app.Detector == nil {
		return nil, errors.New("client detector unavailable")
	}
	detected, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[domain.ClientID]domain.DetectedClient{}
	for _, client := range detected {
		byID[client.ClientID] = client
	}
	targets := []skillspicker.Target{}
	for _, id := range domain.SupportedClientIDs() {
		supported := supportsDependencyTarget(app, string(id))
		if !supported && app.scope() != domain.ScopeProject {
			continue
		}
		definition, _ := domain.ClientDefinitionFor(id)
		client, ok := byID[id]
		available := supported && ok && client.DetectionError == nil && client.Status == domain.DetectionDetected
		target := skillspicker.Target{Id: string(id), Label: definition.DisplayName, Scope: string(app.scope()), Available: available}
		if app.scope() == domain.ScopeUser {
			target.DisplayPath = client.ConfigRoot
		}
		if !supported {
			reason := "This client does not support project installation"
			target.Reason = &reason
		} else if !available {
			reason := "This client is not detected; install or configure it before selecting it"
			if client.DetectionError != nil {
				reason = "Client configuration could not be inspected safely"
			}
			target.Reason = &reason
		} else {
			path, err := app.skillsTargetPath(client)
			if err != nil {
				reason := "Client project configuration could not be inspected safely"
				target.Available, target.Reason = false, &reason
			} else {
				target.DisplayPath = path
			}
		}
		targets = append(targets, target)
	}
	return targets, nil
}
