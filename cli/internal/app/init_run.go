package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

type resolvedInitConfig struct {
	TemplateName          string
	Platform              string
	Runtime               string
	Targets               []string
	RuntimePackageVersion string
}

// Run validates options, writes scaffold files, and returns the absolute output directory.
func (InitRunner) Run(opts InitOptions) (outDir string, err error) {
	name := strings.TrimSpace(opts.ProjectName)
	if err := scaffold.ValidateProjectName(name); err != nil {
		return "", err
	}
	config, err := resolveInitConfig(name, opts)
	if err != nil {
		return "", err
	}
	out, err := resolveInitOutputDir(name, opts.OutputDir)
	if err != nil {
		return "", err
	}
	if err := writeInitProject(out, buildInitScaffoldData(name, opts, config), opts.Force); err != nil {
		return "", err
	}
	if err := generateInitArtifacts(out); err != nil {
		return "", err
	}
	return out, nil
}
