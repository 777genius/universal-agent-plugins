package providers

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// The use case discovers these capabilities with a type assertion, so a drifted
// signature would not fail to compile - it would silently stop being found and
// take the fallback path instead. These assertions turn that into a build error.
var (
	_ ports.ActivationPreflighter         = Activator{}
	_ ports.AutomaticActivationClassifier = Activator{}
	_ ports.ActivationVerifierClassifier  = Activator{}
	_ ports.ManagedStdioPreflighter       = Stager{}
	_ ports.PluginDataAwareStager         = Stager{}
	_ ports.DataPathPreflighter           = PluginDataManager{}
)
