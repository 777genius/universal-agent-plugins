package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

type PluginGenerateOptions struct {
	Root   string
	Target string
	Check  bool
}

type PluginImportOptions struct {
	Root             string
	Source           string
	From             string
	Force            bool
	IncludeUserScope bool
}

type PluginNormalizeOptions struct {
	Root  string
	Force bool
}

type PluginInspectOptions struct {
	Root   string
	Target string
}

type PluginCompatOptions struct {
	Source           string
	From             string
	Target           string
	IncludeUserScope bool
}

type PluginService struct{}

func (PluginService) Generate(opts PluginGenerateOptions) ([]string, error) {
	if opts.Check {
		return pluginmanifest.Drift(opts.Root, opts.Target)
	}
	result, err := pluginmanifest.Generate(opts.Root, opts.Target)
	if err != nil {
		return nil, err
	}
	if err := pluginmanifest.WriteArtifacts(opts.Root, result.Artifacts); err != nil {
		return nil, err
	}
	if err := pluginmanifest.RemoveArtifacts(opts.Root, result.StalePaths); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		out = append(out, artifact.RelPath)
	}
	return out, nil
}

func (PluginService) Import(opts PluginImportOptions) ([]pluginmanifest.Warning, error) {
	if strings.TrimSpace(opts.Source) != "" {
		_, warnings, err := pluginmanifest.ImportFromSource(opts.Root, opts.Source, opts.From, opts.Force, opts.IncludeUserScope)
		if err != nil {
			return warnings, err
		}
		return warnings, nil
	}
	_, warnings, err := pluginmanifest.Import(opts.Root, opts.From, opts.Force, opts.IncludeUserScope)
	if err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (PluginService) Normalize(opts PluginNormalizeOptions) ([]pluginmanifest.Warning, error) {
	return pluginmanifest.Normalize(opts.Root, opts.Force)
}

func (PluginService) Inspect(opts PluginInspectOptions) (pluginmanifest.Inspection, []pluginmanifest.Warning, error) {
	return pluginmanifest.Inspect(opts.Root, opts.Target)
}

func (PluginService) Compat(opts PluginCompatOptions) (pluginmanifest.SourceInspection, []pluginmanifest.Warning, error) {
	return pluginmanifest.InspectSourceRef(opts.Source, opts.From, opts.Target, opts.IncludeUserScope)
}
