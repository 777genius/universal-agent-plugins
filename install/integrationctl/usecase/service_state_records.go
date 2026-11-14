package usecase

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func findInstallation(items []domain.InstallationRecord, name string) (domain.InstallationRecord, bool) {
	name = strings.TrimSpace(name)
	for _, item := range items {
		if item.IntegrationID == name {
			return item, true
		}
	}
	return domain.InstallationRecord{}, false
}

func findInstallationMutable(items []domain.InstallationRecord, name string) (domain.InstallationRecord, bool) {
	return findInstallation(items, name)
}

func upsertInstallation(items []domain.InstallationRecord, next domain.InstallationRecord) []domain.InstallationRecord {
	for i := range items {
		if items[i].IntegrationID == next.IntegrationID {
			items[i] = next
			return items
		}
	}
	return append(items, next)
}

func removeInstallation(items []domain.InstallationRecord, name string) []domain.InstallationRecord {
	out := items[:0]
	for _, item := range items {
		if item.IntegrationID != name {
			out = append(out, item)
		}
	}
	return out
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneInstallationRecord(in domain.InstallationRecord) domain.InstallationRecord {
	out := in
	if len(in.Targets) == 0 {
		out.Targets = map[domain.TargetID]domain.TargetInstallation{}
		return out
	}
	out.Targets = make(map[domain.TargetID]domain.TargetInstallation, len(in.Targets))
	for key, value := range in.Targets {
		out.Targets[key] = cloneTargetInstallation(value)
	}
	return out
}

func cloneTargetInstallation(in domain.TargetInstallation) domain.TargetInstallation {
	out := in
	out.CapabilitySurface = append([]string(nil), in.CapabilitySurface...)
	out.CatalogPolicy = cloneCatalogPolicy(in.CatalogPolicy)
	out.EnvironmentRestrictions = append([]domain.EnvironmentRestrictionCode(nil), in.EnvironmentRestrictions...)
	out.OwnedNativeObjects = append([]domain.NativeObjectRef(nil), in.OwnedNativeObjects...)
	out.AdapterMetadata = cloneMetadata(in.AdapterMetadata)
	return out
}

func cloneCatalogPolicy(in *domain.CatalogPolicySnapshot) *domain.CatalogPolicySnapshot {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func firstNonNilCatalogPolicy(values ...*domain.CatalogPolicySnapshot) *domain.CatalogPolicySnapshot {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
