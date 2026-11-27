package main

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
)

// Policy locations are independent of install/cache roots. In particular, a
// project or user-selected data directory cannot relocate the machine layer.
func configureEnterprisePolicy(app agentpluginscli.App, home, projectRoot string) (agentpluginscli.App, error) {
	machine, err := machinePolicyPath()
	if err != nil {
		return app, err
	}
	invocationRoot := projectRoot
	if invocationRoot == "" {
		invocationRoot, err = os.Getwd()
		if err != nil {
			return app, err
		}
	}
	files := &enterprisepolicy.Files{Paths: []enterprisepolicy.Path{
		{Layer: "machine", Path: machine},
		{Layer: "user", Path: filepath.Join(home, ".config", "agentplugins", "policy.yaml")},
		{Layer: "project", Path: filepath.Join(invocationRoot, "uap-policy.yaml")},
	}}
	if explicit, configured := os.LookupEnv("UAP_POLICY_FILE"); configured {
		if explicit != "" && !filepath.IsAbs(explicit) {
			explicit = filepath.Join(invocationRoot, explicit)
		}
		files.Paths = append(files.Paths, enterprisepolicy.Path{Layer: "deployment", Path: explicit, Required: true})
	}
	app.PolicyFiles, app.InstallationPolicy = files, files
	app.Lifecycle.InstallationPolicy = files
	if app.Installer != nil {
		app.Installer = app.Installer.WithInstallationPolicy(files)
	}
	return app, nil
}
