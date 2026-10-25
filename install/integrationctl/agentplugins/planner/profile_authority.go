package planner

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ ports.PhysicalProfileAuthority = Planner{}

func (p Planner) CaptureProfileAuthority(ctx context.Context, client domain.DetectedClient) (domain.ProfileAuthority, error) {
	if a, ok := clients.As[clients.PhysicalProfileAuthority](p.Registry, client.ClientID); ok {
		token, err := p.captureProfileAuthority(ctx, client, a)
		if err != nil {
			return domain.ProfileAuthority{}, err
		}
		if token.IsZero() {
			return token, fmt.Errorf("opted profile authority is missing")
		}
		return token, a.RevalidateProfileAuthority(ctx, client.ClientID, token)
	}
	return domain.ProfileAuthority{}, nil
}

func (p Planner) captureProfileAuthority(ctx context.Context, client domain.DetectedClient, user clients.PhysicalProfileAuthority) (domain.ProfileAuthority, error) {
	if p.ProjectRoot == "" {
		return user.CaptureProfileAuthority(ctx, client)
	}
	project, ok := clients.As[clients.ProjectPhysicalAuthority](p.Registry, client.ClientID)
	if !ok {
		return domain.ProfileAuthority{}, fmt.Errorf("project physical authority capture is required")
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, p.ProjectRoot, client.ConfigRoot)
	if err != nil {
		return domain.ProfileAuthority{}, err
	}
	layout, ok := clients.As[clients.NativeRegistryLayout](p.Registry, client.ClientID)
	if !ok {
		return domain.ProfileAuthority{}, fmt.Errorf("project native registry layout is required")
	}
	root, _ := layout.NativeRegistry(clients.PlanInput{Client: client, TargetContext: target})
	token, err := project.CaptureProjectAuthority(ctx, client, target, root)
	if err == nil && token.Facts().CanonicalRoot != root {
		return token, fmt.Errorf("project authority differs from native registry")
	}
	return token, err
}
func (p Planner) RevalidateProfileAuthority(ctx context.Context, id domain.ClientID, token domain.ProfileAuthority) error {
	if a, ok := clients.As[clients.PhysicalProfileAuthority](p.Registry, id); ok {
		if token.IsZero() {
			return fmt.Errorf("opted profile authority is missing")
		}
		return a.RevalidateProfileAuthority(ctx, id, token)
	}
	if !token.IsZero() {
		return fmt.Errorf("profile authority verifier is missing for %s", id)
	}
	return nil
}
