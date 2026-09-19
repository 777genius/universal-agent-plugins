package shared

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectedOpenAIManifest is the Codex / ChatGPT plugin.json skeleton: a
// preserved upstream OpenAI manifest when the envelope still carries one,
// otherwise the portable envelope projected with an author object.
func ProjectedOpenAIManifest(envelope domain.PackageEnvelope) (map[string]any, error) {
	preserved, ok, err := PreservedOpenAIManifest(envelope)
	if err != nil {
		return nil, err
	}
	if ok {
		return preserved, nil
	}
	return ManifestFromEnvelope(envelope, WithAuthorObject()), nil
}

// ProjectOpenAIMCP writes the OpenAI-dialect MCP document Codex and ChatGPT
// both consume.
func ProjectOpenAIMCP(root string, envelope domain.PackageEnvelope, serverNames []string, hints domain.CompatibilityHints, pluginRoot, dataPath string) error {
	return ProjectMCPServers(MCPProjection{
		Root:       root,
		Envelope:   envelope,
		Names:      serverNames,
		Dialect:    MCPDialectOpenAI,
		PluginRoot: pluginRoot,
		DataPath:   dataPath,
		Hints:      hints,
	})
}
