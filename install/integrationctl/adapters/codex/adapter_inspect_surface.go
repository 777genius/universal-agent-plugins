package codex

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

type codexObservedSurface struct {
	Restrictions []domain.EnvironmentRestrictionCode
	Observed     []domain.NativeObjectRef
	CatalogFound bool
	PluginFound  bool
}

type codexInspectDetails struct {
	CatalogPolicy *domain.CatalogPolicySnapshot
	Warnings      []string
	Observed      []domain.NativeObjectRef
	CacheExists   bool
	ConfigState   pluginConfigState
	EntryFound    bool
}
