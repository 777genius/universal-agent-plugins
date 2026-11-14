// Package profileauthority bridges the one neutral directory algorithm to the
// private domain value. It has no selection, discovery, port or effect wiring.
package profileauthority

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func Capture(ctx context.Context, canonicalRoot string) (domain.ProfileAuthority, error) {
	a, err := directoryidentity.Capture(ctx, canonicalRoot)
	if err != nil {
		return domain.ProfileAuthority{}, err
	}
	converted, err := FromNeutral(a)
	if err != nil {
		return domain.ProfileAuthority{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.ProfileAuthority{}, err
	}
	return converted, nil
}

func Revalidate(ctx context.Context, expected domain.ProfileAuthority) error {
	a, err := ToNeutral(expected)
	if err != nil {
		return err
	}
	return directoryidentity.Revalidate(ctx, a)
}

func FromNeutral(a directoryidentity.Authority) (domain.ProfileAuthority, error) {
	f := a.Facts()
	entries := make([]domain.ProfileAuthorityEntry, len(f.Ancestry))
	for i, e := range f.Ancestry {
		entries[i] = domain.ProfileAuthorityEntry{CanonicalPath: e.CanonicalPath, Scheme: e.Scheme, VolumeID: e.VolumeID, ObjectID: e.ObjectID}
	}
	return domain.NewProfileAuthority(domain.ProfileAuthorityFacts{Version: f.Version, CanonicalRoot: f.CanonicalRoot, Ancestry: entries})
}

func ToNeutral(a domain.ProfileAuthority) (directoryidentity.Authority, error) {
	f := a.Facts()
	entries := make([]directoryidentity.Entry, len(f.Ancestry))
	for i, e := range f.Ancestry {
		entries[i] = directoryidentity.Entry{CanonicalPath: e.CanonicalPath, Scheme: e.Scheme, VolumeID: e.VolumeID, ObjectID: e.ObjectID}
	}
	return directoryidentity.NewAuthority(directoryidentity.Facts{Version: f.Version, CanonicalRoot: f.CanonicalRoot, Ancestry: entries})
}
