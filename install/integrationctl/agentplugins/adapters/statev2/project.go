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
	root := store.ProjectRoot
	namespace := filepath.Join(root, ".uap")
	if err := store.validateProjectPaths(); err != nil {
		return err
	}
	check := func(base, path string) error {
		if path == "" {
			return nil
		}
		if err := pathpolicy.RequireContainedChild(base, path); err != nil {
			return fmt.Errorf("project machine receipt points outside selected namespace: %w", err)
		}
		return nil
	}
	checkReceipt := func(receipt domain.MutationReceipt) error {
		for _, path := range []string{receipt.ActivePath, receipt.StagingPath, receipt.BackupPath} {
			if err := check(root, path); err != nil {
				return err
			}
		}
		for _, owner := range receipt.ProfileOwners {
			if owner.Namespace != namespace {
				return fmt.Errorf("project receipt belongs to a different namespace")
			}
		}
		return nil
	}
	for _, receipt := range state.TransactionReceipts {
		if err := checkReceipt(receipt); err != nil {
			return err
		}
	}
	for _, installation := range state.Installations {
		for _, data := range installation.DataReceipts {
			if data.Scope != string(domain.ScopeProject) {
				return fmt.Errorf("project data receipt has non-project scope")
			}
			if err := check(filepath.Join(namespace, "plugin-data"), data.Locator); err != nil {
				return err
			}
		}
		for _, binding := range installation.Clients {
			if binding.Scope != string(domain.ScopeProject) {
				return fmt.Errorf("project state contains a non-project binding")
			}
			for _, path := range []string{binding.TargetLocator, binding.NativeProfileRoot} {
				if err := check(root, path); err != nil {
					return err
				}
			}
			if binding.ProfileNamespace != "" && binding.ProfileNamespace != namespace {
				return fmt.Errorf("project binding belongs to a different namespace")
			}
			for _, object := range binding.NativeObjects {
				if err := check(root, object.Path); err != nil {
					return err
				}
			}
			for _, receipt := range binding.Receipts {
				if err := checkReceipt(receipt); err != nil {
					return err
				}
			}
		}
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
