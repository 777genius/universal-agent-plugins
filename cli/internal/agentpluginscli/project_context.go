package agentpluginscli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func (app App) scope() domain.InstallScope {
	if app.Scope == domain.ScopeProject || app.ProjectRoot != "" {
		return domain.ScopeProject
	}
	return domain.ScopeUser
}

func (app App) environmentID() string {
	if app.scope() == domain.ScopeProject {
		return environment.EnvironmentProjectDefault
	}
	return environment.EnvironmentUserDefault
}

func (app App) forOptions(opts *options) (App, error) {
	if err := validateCommonOptions(opts); err != nil {
		return App{}, err
	}
	scope := domain.InstallScope(opts.scope)
	if scope == domain.ScopeUser && app.scope() == domain.ScopeUser {
		return app, nil
	}
	root := ""
	if scope == domain.ScopeProject {
		var err error
		root, err = canonicalProjectRoot(opts.projectRoot)
		if err != nil {
			return App{}, err
		}
	}
	selected, err := app.composeScope(scope, root)
	if err != nil {
		return App{}, err
	}
	if selected.scope() != scope || selected.ProjectRoot != root {
		return App{}, fmt.Errorf("selected runtime does not match requested scope and project root")
	}
	if scope == domain.ScopeProject {
		if err := selected.checkProjectNamespace(); err != nil {
			return App{}, err
		}
		if err := selected.checkProjectTargets(opts.target); err != nil {
			return App{}, err
		}
	}
	return selected, nil
}

func canonicalProjectRoot(root string) (string, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("select existing project root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project root must be an existing directory")
	}
	if _, err := domain.NewTargetContext(domain.ScopeProject, root, ""); err != nil {
		return "", err
	}
	return root, nil
}

func (app App) composeScope(scope domain.InstallScope, root string) (App, error) {
	if app.scope() == scope && app.ProjectRoot == root {
		return app, nil
	}
	if app.ForScope == nil {
		return App{}, fmt.Errorf("this composition supports user scope only; project scope runtime is unavailable")
	}
	selected, err := app.ForScope(scope, root)
	if err != nil {
		return App{}, err
	}
	selected.Input, selected.Output, selected.ErrorOutput = app.Input, app.Output, app.ErrorOutput
	selected.Prompter, selected.PromptFactory, selected.Terminal = app.Prompter, app.PromptFactory, app.Terminal
	return selected, nil
}

func (app App) checkProjectTargets(targetOption string) error {
	targets, err := parseTargetOption(targetOption)
	if err != nil {
		return err
	}
	for _, target := range targets {
		if !clients.SupportsScope(app.ClientRegistry, target, domain.ScopeProject) {
			return fmt.Errorf("%s does not support project scope", target)
		}
	}
	return nil
}

// Reject unsupported project declarations before a lock creates mutable state.
// The locked operation still reads and validates the authoritative manifest.
func (app App) preflightProjectDependencyTargets() error {
	if app.scope() != domain.ScopeProject {
		return nil
	}
	manifest, _ := app.dependencyPaths()
	requirements, err := readDependencyManifest(manifest)
	if err != nil {
		return err
	}
	return validateDependencyTargets(app, requirements)
}

func rejectProjectCommand(app App, command string) error {
	if app.scope() == domain.ScopeProject {
		return fmt.Errorf("%s is not supported for project scope", command)
	}
	return nil
}

func selectedEnvironmentID(ids []string) string {
	if len(ids) > 0 && ids[0] != "" {
		return ids[0]
	}
	return environment.EnvironmentUserDefault
}

func selectedScope(scopes []domain.InstallScope) domain.InstallScope {
	if len(scopes) > 0 && scopes[0] != "" {
		return scopes[0]
	}
	return domain.ScopeUser
}
func scopeFromProjectRoot(root string) domain.InstallScope {
	if root != "" {
		return domain.ScopeProject
	}
	return domain.ScopeUser
}

func (app App) checkProjectNamespace() error {
	if app.scope() != domain.ScopeProject {
		return nil
	}
	if app.Installer == nil || app.StateStore == nil || app.Lifecycle.Paths == nil {
		return fmt.Errorf("project runtime requires an installer, scoped state store and path policy")
	}
	if err := app.Installer.CheckProjectNamespace(); err != nil {
		return err
	}
	if err := app.Lifecycle.Paths.RequireContainedChild(filepath.Join(app.ProjectRoot, ".uap"), app.EnvironmentLockPath); err != nil {
		return fmt.Errorf("unsafe project environment lock: %w", err)
	}
	_, err := app.StateStore.Load()
	return err
}
