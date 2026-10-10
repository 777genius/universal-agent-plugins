//go:build darwin && arm64

package vscode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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
	return a.captureLocalProfileAuthority(ctx, client)
}

func (*LocalAdapter) RevalidateProfileAuthority(ctx context.Context, id domain.ClientID, token domain.ProfileAuthority) error {
	return revalidateLocalProfileAuthority(ctx, id, token)
}
