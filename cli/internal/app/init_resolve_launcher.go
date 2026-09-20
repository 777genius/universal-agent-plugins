package app

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

func resolveLauncherInitConfig(name string, opts InitOptions, config resolvedInitConfig) (resolvedInitConfig, error) {
	if config.TemplateName == scaffold.InitTemplateCustomLogic {
		normalizeCustomLogicDefaults(opts, &config)
		switch config.Platform {
		case "codex-runtime", "claude", "gemini":
		default:
			return resolvedInitConfig{}, fmt.Errorf("--template custom-logic supports launcher-backed targets only; choose codex-runtime, claude, or gemini")
		}
	}
	if _, ok := scaffold.LookupPlatform(config.Platform); !ok {
		return resolvedInitConfig{}, errUnknownPlatform(opts.Platform)
	}
	if opts.ClaudeExtendedHooks && config.Platform != "claude" {
		return resolvedInitConfig{}, fmt.Errorf("--claude-extended-hooks is only supported with --platform claude")
	}
	if err := validateInitPlatformOptions(name, opts, config); err != nil {
		return resolvedInitConfig{}, err
	}
	if err := validateInitRuntimeOptions(opts, &config); err != nil {
		return resolvedInitConfig{}, err
	}
	return config, nil
}

func normalizeCustomLogicDefaults(opts InitOptions, config *resolvedInitConfig) {
	if !opts.PlatformExplicit || config.Platform == "" {
		config.Platform = "codex-runtime"
	}
	if !opts.RuntimeExplicit {
		config.Runtime = scaffold.RuntimeGo
	}
}

func validateInitPlatformOptions(name string, opts InitOptions, config resolvedInitConfig) error {
	switch config.Platform {
	case "gemini":
		if opts.TypeScript {
			return fmt.Errorf("--typescript is not supported with --platform %s", config.Platform)
		}
		if err := pluginmanifest.ValidateGeminiExtensionName(name); err != nil {
			return err
		}
		if runtimeFlag := strings.ToLower(strings.TrimSpace(config.Runtime)); runtimeFlag != "" && runtimeFlag != scaffold.RuntimeGo {
			return fmt.Errorf("--runtime is not supported with --platform %s", config.Platform)
		}
	case "opencode", "cursor", "cursor-workspace":
		if opts.TypeScript {
			return fmt.Errorf("--typescript is not supported with --platform %s", config.Platform)
		}
		if strings.TrimSpace(config.Runtime) != "" {
			return fmt.Errorf("--runtime is not supported with --platform %s", config.Platform)
		}
	case "codex-package":
		if strings.TrimSpace(config.Runtime) != "" {
			return fmt.Errorf("--runtime is not supported with --platform %s", config.Platform)
		}
		if opts.TypeScript {
			return fmt.Errorf("--typescript is not supported with --platform %s", config.Platform)
		}
	}
	if opts.RuntimePackage && (config.Platform == "gemini" || config.Platform == "codex-package" || config.Platform == "opencode" || config.Platform == "cursor" || config.Platform == "cursor-workspace") {
		return fmt.Errorf("--runtime-package is not supported with --platform %s", config.Platform)
	}
	return nil
}

func validateInitRuntimeOptions(opts InitOptions, config *resolvedInitConfig) error {
	if config.Platform != "codex-package" && config.Platform != "opencode" && config.Platform != "cursor" && config.Platform != "cursor-workspace" {
		if _, ok := scaffold.LookupRuntime(config.Runtime); !ok {
			return errUnknownRuntime(opts.Runtime)
		}
	}
	if opts.TypeScript && config.Runtime != scaffold.RuntimeNode {
		return fmt.Errorf("--typescript requires --runtime node")
	}
	if opts.RuntimePackage && config.Runtime != scaffold.RuntimePython && config.Runtime != scaffold.RuntimeNode {
		return fmt.Errorf("--runtime-package requires --runtime python or --runtime node")
	}
	if !opts.RuntimePackage && config.RuntimePackageVersion != "" {
		return fmt.Errorf("--runtime-package-version requires --runtime-package")
	}
	if opts.RuntimePackage && config.RuntimePackageVersion == "" {
		config.RuntimePackageVersion = defaultRuntimePackageVersion()
		if config.RuntimePackageVersion == "" {
			return fmt.Errorf("--runtime-package requires --runtime-package-version when the CLI build does not have a stable tagged version")
		}
	}
	return nil
}
