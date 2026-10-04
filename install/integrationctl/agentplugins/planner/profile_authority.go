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
		token, err := a.CaptureProfileAuthority(ctx, client)
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
