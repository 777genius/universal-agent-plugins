package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func importedOpenCodeInlineCommandArtifacts(raw map[string]any, configPath string) ([]pluginmodel.Artifact, map[string]any, []pluginmodel.Warning, error) {
	return importedOpenCodeInlineMarkdownArtifacts("command", raw, configPath, "commands", normalizeInlineOpenCodeCommand)
}

func importedOpenCodeInlineAgentArtifacts(raw map[string]any, configPath string) ([]pluginmodel.Artifact, map[string]any, []pluginmodel.Warning, error) {
	return importedOpenCodeInlineMarkdownArtifacts("agent", raw, configPath, "agents", normalizeInlineOpenCodeAgent)
}

type openCodeInlineNormalizer func(name string, spec map[string]any) (map[string]any, string, bool)
