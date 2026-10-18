package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"

func dependencyRootKey(requirements environment.Requirements, lock environment.Lock, alias string) string {
	pin, ok := lock.Package(alias)
	if !ok {
		return alias
	}
	for key, requirement := range requirements.Dependencies {
		if mustSourceKey(requirement) == pin.SourceKey {
			return key
		}
	}
	return alias
}

func dependencyNodeAlias(requirements environment.Requirements, lock environment.Lock, key string) string {
	requirement, ok := requirements.Dependencies[key]
	if !ok {
		return key
	}
	source := mustSourceKey(requirement)
	for _, pin := range lock.Packages {
		if pin.SourceKey == source {
			return pin.Alias
		}
	}
	return key
}
