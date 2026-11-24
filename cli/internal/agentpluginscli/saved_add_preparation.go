package agentpluginscli

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Only fresh installations may fall back from automatic to preparation. A
// same-name installation is conservatively treated as existing; restoring a
// preparation preference additionally requires the same source identity.
func savedHostedPreparationIntents(app App, loaded loadedPackage) (map[domain.ClientID]domain.InstallIntent, bool, error) {
	intents := map[domain.ClientID]domain.InstallIntent{}
	if app.StateStore == nil {
		return intents, true, nil
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, false, err
	}
	fresh := true
	for _, installation := range state.Installations {
		sameSource := sameSavedPreparationSource(installation, loaded)
		if !sameSource && installation.Package.DeclaredName != loaded.envelope.Manifest.Name {
			continue
		}
		fresh = false
		if !sameSource {
			continue
		}
		for _, binding := range installation.Clients {
			intents[domain.ClientID(binding.ClientID)] = binding.InstallIntent
		}
		for _, preference := range installation.InstallPreferences {
			intents[preference.ClientID] = preference.InstallIntent
		}
	}
	return intents, fresh, nil
}

func sameSavedPreparationSource(installation domain.Installation, loaded loadedPackage) bool {
	source := loaded.envelope.Source
	if loaded.directory != nil || installation.Directory != nil {
		return loaded.directory != nil && installation.Directory != nil && loaded.directory.ProductID != "" && loaded.directory.ProductID == installation.Directory.ProductID
	}
	if source.Repository != "" && installation.Source.Repository != "" {
		return strings.EqualFold(source.Repository, installation.Source.Repository) && source.PackageSubpath == installation.Source.PackageSubpath
	}
	return installation.Source.SourceBindingID == domain.ComputeSourceBindingID(source)
}
