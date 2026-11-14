package codex

import (
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) inspectDetails(inputs codexInspectInputs) codexInspectDetails {
	catalogPolicy, entryFound, warnings := inspectCodexCatalogPolicy(inputs.Paths.CatalogPath, inputs.IntegrationID)
	marketplaceName := inspectCodexMarketplaceName(inputs)
	observed, cacheExists := a.inspectCodexCache(inputs.IntegrationID, marketplaceName)
	configObserved, configState, configWarning := inspectCodexConfig(inputs.Paths.ConfigPath, inputs.IntegrationID, marketplaceName)
	if configWarning != "" {
		warnings = append(warnings, configWarning)
	}
	observed = append(observed, configObserved...)

	return codexInspectDetails{
		CatalogPolicy: catalogPolicy,
		Warnings:      append([]string{}, warnings...),
		Observed:      observed,
		CacheExists:   cacheExists,
		ConfigState:   configState,
		EntryFound:    entryFound,
	}
}

func inspectCodexCatalogPolicy(catalogPath string, integrationID string) (*domain.CatalogPolicySnapshot, bool, []string) {
	if entry, found, err := readMarketplaceEntry(catalogPath, integrationID); err == nil && found {
		return policyFromEntry(entry), true, nil
	} else if err != nil {
		return nil, false, []string{err.Error()}
	}
	return nil, false, nil
}

func inspectCodexMarketplaceName(inputs codexInspectInputs) string {
	marketplaceName := marketplaceNameFromRecord(inputs.Record)
	if strings.TrimSpace(marketplaceName) == "" {
		if doc, err := readMarketplace(inputs.Paths.CatalogPath); err == nil {
			marketplaceName = strings.TrimSpace(doc.Name)
		}
	}
	return marketplaceName
}

func (a Adapter) inspectCodexCache(integrationID string, marketplaceName string) ([]domain.NativeObjectRef, bool) {
	if integrationID == "" || marketplaceName == "" {
		return nil, false
	}
	cachePath := a.cachePath(marketplaceName, integrationID)
	if cacheInfo, cacheErr := os.Stat(cachePath); cacheErr == nil && cacheInfo != nil && cacheInfo.IsDir() {
		return []domain.NativeObjectRef{{
			Kind:            "installed_cache_bundle",
			Name:            integrationID,
			Path:            cachePath,
			ProtectionClass: domain.ProtectionUserMutable,
		}}, true
	}
	return nil, false
}

func inspectCodexConfig(configPath string, integrationID string, marketplaceName string) ([]domain.NativeObjectRef, pluginConfigState, string) {
	pluginRef := ""
	if integrationID != "" && marketplaceName != "" {
		pluginRef = integrationID + "@" + marketplaceName
	}
	configState, configWarning := readPluginConfigState(configPath, pluginRef)
	if !configState.Present {
		return nil, configState, configWarning
	}
	return []domain.NativeObjectRef{{
		Kind:            "plugin_toggle",
		Name:            pluginRef,
		Path:            configPath,
		ProtectionClass: domain.ProtectionUserMutable,
	}}, configState, configWarning
}
