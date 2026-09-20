package platformexec

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/codexconfig"
	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
)

type codexRuntimeMeta struct {
	ModelHint string `yaml:"model_hint,omitempty"`
}

type codexPackageMeta = codexmanifest.PackageMeta
type importedCodexPluginManifest = codexmanifest.ImportedPluginManifest

type importedCodexNativeConfig = codexconfig.ImportedConfig

func readImportedCodexConfig(root string) (importedCodexNativeConfig, []byte, error) {
	return codexconfig.ReadImportedConfig(root)
}

func readImportedCodexPluginManifest(root string) (codexmanifest.ImportedPluginManifest, []byte, error) {
	return codexmanifest.ReadImportedPluginManifest(root)
}
