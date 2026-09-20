package app

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func buildInitScaffoldData(name string, opts InitOptions, config resolvedInitConfig) scaffold.Data {
	data := scaffold.Data{
		ProjectName:           name,
		ModulePath:            scaffold.DefaultModulePath(name),
		Description:           initDescription(config.TemplateName),
		Version:               "0.1.0",
		GoSDKReplacePath:      defaultGoSDKReplacePath(),
		Platform:              config.Platform,
		Runtime:               config.Runtime,
		TypeScript:            opts.TypeScript,
		SharedRuntimePackage:  opts.RuntimePackage,
		RuntimePackageVersion: config.RuntimePackageVersion,
		HasSkills:             opts.Extras,
		HasCommands:           opts.Extras,
		WithExtras:            opts.Extras,
		ClaudeExtendedHooks:   opts.ClaudeExtendedHooks,
		JobTemplate:           config.TemplateName,
		Targets:               config.Targets,
		AuthoredRoot:          pluginmodel.SourceDirName,
		AuthoredReadmePath:    filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "README.md")),
	}
	if config.Platform == "codex-runtime" {
		data.CodexModel = scaffold.DefaultCodexModel
	}
	return data
}

func writeInitProject(out string, data scaffold.Data, force bool) error {
	return scaffold.Write(out, data, force)
}
