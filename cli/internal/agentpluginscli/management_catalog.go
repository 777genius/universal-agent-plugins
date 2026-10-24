package agentpluginscli

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (b *ManagementBackend) Catalog(ctx context.Context, query string) (management.Catalog, error) {
	out := management.Catalog{Freshness: management.CatalogFreshnessUnavailable, Items: []management.CatalogEntry{}, Diagnostics: []management.Diagnostic{}}
	if b.app.DirectoryClient == nil {
		return out, errors.New("signed Directory is unavailable")
	}
	state, err := b.app.StateStore.Load()
	if err != nil {
		return out, err
	}
	bundle, loadErr := b.app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	return b.managementCatalogOutcome(bundle, loadErr, query), nil
}

// An unavailable remote catalog is an explicit observation, not an empty
// successful refresh. Installed inspection remains usable without the network.
func (b *ManagementBackend) managementCatalogOutcome(bundle directoryv1.VerifiedBundle, loadErr error, query string) management.Catalog {
	out := management.Catalog{Freshness: management.CatalogFreshnessUnavailable, Items: []management.CatalogEntry{}, Diagnostics: []management.Diagnostic{}}
	if loadErr != nil {
		out.Diagnostics = append(out.Diagnostics, managementDiagnostic("CATALOG_UNAVAILABLE", "Could not refresh the verified catalog; installed environment inspection remains available"))
		return out
	}
	out.Freshness = management.CatalogFreshnessCurrent
	if bundle.Source != directoryv1.BundleSourceRemote {
		out.Freshness = management.CatalogFreshnessCached
	}
	for _, product := range bundle.Snapshot.Products {
		if query != "" && !strings.Contains(strings.ToLower(product.DisplayName+" "+product.ID+" "+product.Description), strings.ToLower(query)) {
			continue
		}
		out.Items = append(out.Items, b.catalogEntry(bundle.Snapshot, product))
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Name < out.Items[j].Name })
	return out
}
func (b *ManagementBackend) catalogEntry(snapshot domain.DirectorySnapshot, product domain.DirectoryProduct) management.CatalogEntry {
	item := management.CatalogEntry{Id: product.ID, Name: product.DisplayName, Description: product.Description, Source: product.ID, Kind: management.CatalogEntryKindPlugin, Targets: []string{}}
	host := directoryEnvironment(nil)
	for _, target := range b.config.Targets {
		selection, err := domain.ResolveDirectory(snapshot, domain.DirectoryResolveRequest{Selector: product.ID, Targets: []domain.ClientID{domain.ClientID(target.Client)}, Scope: domain.ScopeUser, Operation: domain.DirectoryInstall, SchemaVersion: "1.0.0", InstallerVersion: b.app.Version, OS: host.OS, Architecture: host.Architecture})
		if err != nil {
			item.Reason = err.Error()
			continue
		}
		item.Available = true
		item.Targets = append(item.Targets, target.Id)
		item.DistributionId = selection.DistributionID
		release := inspectDistribution(snapshot, product, selection.DistributionID, selection.ReleaseSequence)
		item.Version = release.PackageVersion
		item.Homepage = "https://github.com/" + release.Repository
	}
	if item.Available {
		item.Reason = ""
	}
	return item
}
func (b *ManagementBackend) CatalogDetail(ctx context.Context, id string) (management.CatalogEntry, error) {
	catalog, err := b.Catalog(ctx, "")
	if err != nil {
		return management.CatalogEntry{}, err
	}
	for _, item := range catalog.Items {
		if item.Id == id {
			return item, nil
		}
	}
	return management.CatalogEntry{}, errors.New("verified catalog entry is unavailable")
}
