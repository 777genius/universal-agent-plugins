package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

// Platform defaults belong to the executable composition, not language SDKs.
func managementAppFactory(home, dataRoot string) func(management.Startup) (agentpluginscli.App, error) {
	return func(cfg management.Startup) (agentpluginscli.App, error) {
		if cfg.StateRoot == "" {
			cfg.StateRoot = dataRoot
		}
		if cfg.DependenciesRoot == "" {
			cfg.DependenciesRoot = cfg.StateRoot
		}
		captured, err := agentpluginscli.NormalizeManagementStartup(cfg)
		if err != nil {
			return agentpluginscli.App{}, err
		}
		configured, err := composeAgentpluginsApp(home, captured.StateRoot)
		if err != nil {
			return agentpluginscli.App{}, err
		}
		return agentpluginscli.ConfigureManagementApp(configured, captured)
	}
}
