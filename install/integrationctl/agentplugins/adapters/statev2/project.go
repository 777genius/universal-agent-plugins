package statev2

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Machine receipts are valid only under the selected physical project. Portable
// transfer uses uap.toml/uap.lock; moving .uap is never an authority transfer.
func (store Store) validateProject(state domain.StateFileV2) error {
	if store.ProjectRoot == "" {
		return nil
	}
	if err := store.validateProjectPaths(); err != nil {
		return err
	}
	for _, receipt := range state.TransactionReceipts {
		if err := store.validateProjectReceipt(receipt); err != nil {
			return err
		}
	}
	for _, installation := range state.Installations {
		if err := store.validateProjectInstallation(installation); err != nil {
			return err
		}
	}
	return nil
}

func (store Store) validateProjectInstallation(installation domain.Installation) error {
	for _, data := range installation.DataReceipts {
		if data.Scope != string(domain.ScopeProject) {
			return fmt.Errorf("project data receipt has non-project scope")
		}
		if err := checkProjectLocator(filepath.Join(store.ProjectRoot, ".uap", "plugin-data"), data.Locator); err != nil {
			return err
		}
	}
	for _, binding := range installation.Clients {
		if err := store.validateProjectBinding(binding); err != nil {
			return err
		}
	}
	return nil
}

func (store Store) validateProjectBinding(binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) {
		return fmt.Errorf("project state contains a non-project binding")
	}
	if err := checkProjectLocator(store.ProjectRoot, binding.TargetLocator); err != nil {
		return err
	}
	nativeBase := store.ProjectRoot
	if binding.NativeProfileRoot == store.ProjectRoot {
		// Root-native registries (for example a workspace opencode.json) use
		// the project directory itself. Only this field permits exact equality;
		// validate its spelling and no-follow authority through its parent.
		if _, err := domain.NewTargetContext(domain.ScopeProject, store.ProjectRoot, ""); err != nil {
			return err
		}
		nativeBase = filepath.Dir(store.ProjectRoot)
	}
	if err := checkProjectLocator(nativeBase, binding.NativeProfileRoot); err != nil {
		return err
	}
	if binding.ProfileNamespace != "" && binding.ProfileNamespace != filepath.Join(store.ProjectRoot, ".uap") {
		return fmt.Errorf("project binding belongs to a different namespace")
	}
	for _, object := range binding.NativeObjects {
		if err := checkProjectLocator(store.ProjectRoot, object.Path); err != nil {
			return err
		}
	}
	for _, receipt := range binding.Receipts {
		if err := store.validateProjectReceipt(receipt); err != nil {
			return err
		}
	}
	return nil
}

func (store Store) validateProjectReceipt(receipt domain.MutationReceipt) error {
	for _, path := range []string{receipt.ActivePath, receipt.StagingPath, receipt.BackupPath} {
		if err := checkProjectLocator(store.ProjectRoot, path); err != nil {
			return err
		}
	}
	for _, owner := range receipt.ProfileOwners {
		if owner.Namespace != filepath.Join(store.ProjectRoot, ".uap") {
			return fmt.Errorf("project receipt belongs to a different namespace")
		}
	}
	return nil
}

func checkProjectLocator(base, path string) error {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("project machine receipt locator must be absolute and clean")
	}
	if err := pathpolicy.RequireContainedChild(base, path); err != nil {
		return fmt.Errorf("project machine receipt points outside selected namespace: %w", err)
	}
	return nil
}

func (store Store) validateProjectPaths() error {
	if store.ProjectRoot == "" {
		return nil
	}
	namespace := filepath.Join(store.ProjectRoot, ".uap")
	if err := pathpolicy.RequireContainedChild(store.ProjectRoot, namespace); err != nil {
		return fmt.Errorf("unsafe project namespace: %w", err)
	}
	if err := pathpolicy.RequireContainedChild(namespace, store.Path); err != nil {
		return fmt.Errorf("unsafe project state path: %w", err)
	}
	return nil
}
