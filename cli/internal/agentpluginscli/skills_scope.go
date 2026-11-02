package agentpluginscli

import (
	"errors"
	"fmt"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// The public picker and batch APIs enforce the same scope boundary as CLI entry.
// In particular, a disposable preview cannot replace the original namespace guard.
func (app App) checkSkillsContext(targets []string) error {
	if app.scope() == domain.ScopeProject {
		root, err := canonicalProjectRoot(app.ProjectRoot)
		if err != nil {
			return err
		}
		if root != app.ProjectRoot {
			return fmt.Errorf("skills project root must be canonical")
		}
	}
	if err := app.checkProjectNamespace(); err != nil {
		return err
	}
	// A first Skills add creates the manifest; an existing declaration still
	// needs scope qualification before any new source is acquired.
	if err := app.preflightProjectDependencyTargets(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	requirements := environment.Requirements{Targets: map[string][]string{}}
	for _, target := range targets {
		requirements.Targets[target] = nil
	}
	return validateDependencyTargets(app, requirements)
}

func (app App) skillsOptions() *options {
	return &options{scope: string(app.scope()), projectRoot: app.ProjectRoot}
}

func (app App) skillsTargetPath(client domain.DetectedClient) (string, error) {
	if app.scope() != domain.ScopeProject {
		return client.ConfigRoot, nil
	}
	target, err := domain.NewTargetContext(domain.ScopeProject, app.ProjectRoot, client.ConfigRoot)
	if err != nil {
		return "", err
	}
	layout, ok := clients.As[clients.ProjectTargetLayout](app.ClientRegistry, client.ClientID)
	if !ok {
		return "", fmt.Errorf("%s does not support project scope", client.ClientID)
	}
	definition, _ := domain.ClientDefinitionFor(client.ClientID)
	_, root, err := layout.ProjectTargetRoot(client, definition.Capabilities.PackageMode, app.ManagedRoot, target)
	return root, err
}
