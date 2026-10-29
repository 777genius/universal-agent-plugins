package agentpluginscli

import (
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// loadSaved removes environment-owned installations from imperative checking.
// Declared roots remain checkable even before their first materialized binding.
func (session *outdatedSession) loadSaved(selector string) (bool, error) {
	manifest, _ := session.app.dependencyPaths()
	if _, err := os.Stat(manifest); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	snapshot, err := snapshotEnvironmentContext(session.ctx, session.app, false)
	if err != nil {
		return false, err
	}
	aliases := []string{}
	selected := false
	if strings.TrimSpace(selector) != "" {
		alias, _, err := resolveSavedAlias(session.state, snapshot.Requirements, selector, session.app.environmentID())
		if err != nil {
			return false, err
		}
		if alias == "" {
			return false, nil
		}
		selected = true
		key := dependencyRootKey(snapshot.Requirements, snapshot.Lock, alias)
		if _, exists := snapshot.Requirements.Dependencies[key]; !exists {
			return false, ambiguousSavedSelector(selector)
		}
		aliases = append(aliases, key)
	} else {
		for alias := range snapshot.Requirements.Dependencies {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	check, err := checkSavedUpdates(session.ctx, session.app, snapshot, aliases)
	if err != nil {
		return false, err
	}
	session.result.Saved = &check
	session.savedOwned = map[string]bool{}
	for _, receipt := range session.state.EnvironmentReceipts {
		if receipt.EnvironmentID == session.app.environmentID() {
			session.savedOwned[receipt.InstallationID] = true
		}
	}
	for _, fact := range check.Entries {
		item := outdatedInstallation{Name: fact.Alias, Alias: fact.Alias, Source: fact.Source, Requested: &fact.Requested, Status: fact.Status, Reason: fact.Reason, Installed: fact.Installed, Available: fact.Available, RequiredBy: fact.RequiredBy}
		if item.Status == "available" {
			item.Status = "outdated"
		}
		for _, receipt := range session.state.EnvironmentReceipts {
			if receipt.EnvironmentID == session.app.environmentID() && receipt.Alias == fact.Alias {
				item.InstallationID = receipt.InstallationID
				if !slices.Contains(item.Targets, domain.ClientID(receipt.ClientID)) {
					item.Targets = append(item.Targets, domain.ClientID(receipt.ClientID))
				}
			}
		}
		session.result.Installations = append(session.result.Installations, item)
		session.countStatus(item.Status)
	}
	return selected, nil
}
