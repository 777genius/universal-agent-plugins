package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func resolvePackageOnlyInitConfig(name string, opts InitOptions, config resolvedInitConfig) (resolvedInitConfig, error) {
	if opts.ClaudeExtendedHooks {
		return resolvedInitConfig{}, fmt.Errorf("--claude-extended-hooks is only supported with --template custom-logic and --platform claude")
	}
	if opts.TypeScript {
		return resolvedInitConfig{}, fmt.Errorf("--typescript is not supported with --template %s; use --template custom-logic when you need runtime code", config.TemplateName)
	}
	if opts.RuntimePackage {
		return resolvedInitConfig{}, fmt.Errorf("--runtime-package is not supported with --template %s; use --template custom-logic when you need a shared runtime package", config.TemplateName)
	}
	if config.RuntimePackageVersion != "" {
		return resolvedInitConfig{}, fmt.Errorf("--runtime-package-version requires --template custom-logic with --runtime-package")
	}
	if opts.RuntimeExplicit && config.Runtime != "" {
		return resolvedInitConfig{}, fmt.Errorf("--runtime is not supported with --template %s; use --template custom-logic when you need launcher-backed code", config.TemplateName)
	}
	targets, err := resolvePackageOnlyTargets(config.TemplateName, opts, config.Platform)
	if err != nil {
		return resolvedInitConfig{}, err
	}
	if err := validatePackageOnlyGeminiTargets(name, config.TemplateName, targets); err != nil {
		return resolvedInitConfig{}, err
	}
	config.Platform = ""
	config.Runtime = ""
	config.Targets = targets
	return config, nil
}

func resolvePackageOnlyTargets(templateName string, opts InitOptions, platform string) ([]string, error) {
	if !opts.PlatformExplicit {
		return scaffold.DefaultJobTemplateTargets(templateName), nil
	}
	meta, ok := scaffold.LookupPlatform(platform)
	if !ok {
		return nil, errUnknownPlatform(opts.Platform)
	}
	switch meta.Name {
	case "claude", "codex-package", "gemini", "opencode", "cursor", "cursor-workspace":
		return []string{meta.Name}, nil
	default:
		return nil, fmt.Errorf("--template %s only supports package and workspace outputs; use --template custom-logic for %s", templateName, meta.Name)
	}
}

func validatePackageOnlyGeminiTargets(name, templateName string, targets []string) error {
	for _, target := range targets {
		if target != "gemini" {
			continue
		}
		if err := pluginmanifest.ValidateGeminiExtensionName(name); err != nil {
			return fmt.Errorf("project name %q must be lowercase kebab-case when --template %s includes gemini output: %w", name, templateName, err)
		}
	}
	return nil
}
