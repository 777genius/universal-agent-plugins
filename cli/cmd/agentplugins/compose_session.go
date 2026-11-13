package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func configureNativeSession(app *agentpluginscli.App, facade *installer.Engine, kernel nativeconfig.Kernel) {
	app.Installer = facade
	app.SessionEnv = clients.Env{NativeConfig: kernel, Runner: processadapter.OS{}, LookupExecutable: exec.LookPath}
	app.StartNativeSession = func(ctx context.Context, plan clients.SessionPlan) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return managedstdio.StartSessionContext(ctx, plan.Executable, append([]string{plan.Executable}, plan.Arguments...),
			plan.Directory, plan.ChildEnvironment(os.Environ()))
	}
}
