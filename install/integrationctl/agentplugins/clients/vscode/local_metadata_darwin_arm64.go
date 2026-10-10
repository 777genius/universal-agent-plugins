//go:build darwin && arm64

package vscode

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// No pathname metadata grant is issued here. Apply on this same plain exact
// transaction authorizes mutation from its held descriptors, or returns the
// backend's metadata/custom-IO refusal. Locked planning/no-op remains available.
func checkLocalProfileMetadata(_ *nativeconfig.ExactFile, _ string) error { return nil }

// Opt in only the newly qualified Darwin consumer; Linux/Windows retain their
// existing Local preparation and historical binding contracts.
var _ clients.PhysicalProfileAuthority = (*LocalAdapter)(nil)

func (a *LocalAdapter) CaptureProfileAuthority(ctx context.Context, client domain.DetectedClient) (domain.ProfileAuthority, error) {
	if err := ctx.Err(); err != nil {
		return domain.ProfileAuthority{}, err
	}
	if client.ClientID != domain.ClientVSCode {
		return domain.ProfileAuthority{}, fmt.Errorf("local physical authority requires VS Code")
	}
	root, err := a.ResolveProfileRoot(client.ConfigRoot)
	if err != nil {
		return domain.ProfileAuthority{}, err
	}
	return profileauthority.Capture(ctx, root)
}

// Revalidation consumes only persisted authority, never current constructor inputs.
func (*LocalAdapter) RevalidateProfileAuthority(ctx context.Context, id domain.ClientID, token domain.ProfileAuthority) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id != domain.ClientVSCode || token.IsZero() {
		return fmt.Errorf("local recorded physical authority required")
	}
	return profileauthority.Revalidate(ctx, token)
}
