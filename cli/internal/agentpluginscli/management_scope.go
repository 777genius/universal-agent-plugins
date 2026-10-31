package agentpluginscli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A shared state directory is not permission to mutate another native profile.
// Every planned environment and every recovery observation must remain inside
// the immutable startup context, including when an external CLI changed state.
func (b *ManagementBackend) validateManagementEnvironment(ctx context.Context, requirements environment.Requirements) error {
	if err := b.validateManagementContext(ctx); err != nil {
		return err
	}
	profiles := map[string]string{}
	for _, target := range b.config.Targets {
		profiles[target.Client] = target.ConfigRoot
	}
	for client := range requirements.Targets {
		if profiles[client] == "" {
			return managementRefusal("TARGET_OUTSIDE_CONTEXT", "Reopen the environment with every affected native profile before changing this binding")
		}
	}
	state, err := b.app.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			root := profiles[binding.ClientID]
			if root == "" || binding.Scope != string(domain.ScopeUser) || !managementBindingInProfile(binding, root) {
				return managementRefusal("TARGET_OUTSIDE_CONTEXT", "Reopen the environment with every affected native profile before changing this binding")
			}
		}
	}
	return nil
}

func managementBindingInProfile(binding domain.ClientBinding, root string) bool {
	recorded := binding.NativeProfileRoot
	if binding.ProfileAuthority != nil {
		recorded = binding.ProfileAuthority.Facts().CanonicalRoot
	}
	if recorded != "" {
		normalized, err := managementAbsolute(recorded)
		return err == nil && normalized == root
	}
	// Some native configuration adapters retain a private package artifact plus
	// separately owned projections instead of a profile token. The artifact is
	// not the user's profile; every native projection must prove the same root.
	return managementProjectionInProfile(binding, root)
}

func managementProjectionInProfile(binding domain.ClientBinding, root string) bool {
	proved := managementPathInProfile(binding.TargetLocator, root)
	for _, object := range binding.NativeObjects {
		switch object.ProtectionClass {
		case "managed", "managed_entry":
		default:
			continue
		}
		if object.Kind == "managed_package_directory" && object.Path == binding.TargetLocator {
			continue
		}
		if !managementPathInProfile(object.Path, root) {
			return false
		}
		proved = true
	}
	return proved
}

func managementPathInProfile(path, root string) bool {
	physical, err := managementAbsolute(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, physical)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func (b *ManagementBackend) validateManagementContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := NormalizeManagementStartup(b.config)
	if err != nil || current.ContextId != b.ContextID() {
		return management.ErrPlanStale
	}
	return nil
}

func (b *ManagementBackend) validateManagementActions(ctx context.Context, actions []dependencyAction) error {
	if err := b.validateManagementContext(ctx); err != nil {
		return err
	}
	profiles, installations := map[string]string{}, map[string]bool{}
	for _, target := range b.config.Targets {
		profiles[target.Client] = target.ConfigRoot
	}
	for _, action := range actions {
		if profiles[action.ClientID] == "" {
			return managementRefusal("TARGET_OUTSIDE_CONTEXT", "Reopen the environment with every affected native profile before changing this binding")
		}
		if action.InstallationID != "" {
			installations[action.InstallationID] = true
		}
	}
	state, err := b.app.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		if !installations[installation.InstallationID] {
			continue
		}
		// A shared installation's siblings participate in the installer's identity
		// checks even when the selected action changes only one target.
		for _, binding := range installation.Clients {
			root := profiles[binding.ClientID]
			if root == "" || binding.Scope != string(domain.ScopeUser) || !managementBindingInProfile(binding, root) {
				return managementRefusal("TARGET_OUTSIDE_CONTEXT", "Reopen the environment with every affected native profile before changing this binding")
			}
		}
	}
	return nil
}
