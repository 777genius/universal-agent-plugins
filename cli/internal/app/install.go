package app

import (
	"context"

	"github.com/777genius/plugin-kit-ai/plugininstall"
)

// PluginInstaller is the install use case boundary for tests (production uses plugininstall.Install).
type PluginInstaller interface {
	Install(ctx context.Context, p plugininstall.Params) (plugininstall.Result, error)
}

type plugininstallFacade struct{}

func (plugininstallFacade) Install(ctx context.Context, p plugininstall.Params) (plugininstall.Result, error) {
	return plugininstall.Install(ctx, p)
}

// InstallRunner runs plugin-kit-ai install behind the CLI.
type InstallRunner struct {
	Installer PluginInstaller
}

// NewInstallRunner returns a runner. If inst is nil, plugininstall.Install is used.
func NewInstallRunner(inst PluginInstaller) *InstallRunner {
	if inst == nil {
		inst = plugininstallFacade{}
	}
	return &InstallRunner{Installer: inst}
}

// Install executes installation with the given params.
func (r *InstallRunner) Install(ctx context.Context, p plugininstall.Params) (plugininstall.Result, error) {
	return r.Installer.Install(ctx, p)
}
