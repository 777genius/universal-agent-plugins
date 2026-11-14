package transaction

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// validateDirectoryGroupItem is used in two distinct checkpoints:
// first for the entire group before any filesystem effect, then per item as
// earlier items update desired state. Do not remove either checkpoint.
func validateDirectoryGroupItem(before, state domain.StateFileV2, mutation DirectoryMutation, index int, operations, paths map[string]struct{}) (int, domain.ClientBinding, error) {
	if mutation.Standalone || mutation.PublicationParentAuthority != nil {
		return 0, domain.ClientBinding{}, fmt.Errorf("standalone publication cannot join a package group")
	}
	if mutation.Sequence < 1 || mutation.OperationID == "" {
		return 0, domain.ClientBinding{}, fmt.Errorf("mutation %d has incomplete identity", index)
	}
	if _, duplicate := operations[mutation.OperationID]; duplicate || operationIDExists(before, mutation.OperationID) {
		return 0, domain.ClientBinding{}, fmt.Errorf("mutation operation id %q was already used", mutation.OperationID)
	}
	operations[mutation.OperationID] = struct{}{}
	if _, duplicate := paths[mutation.ActivePath]; duplicate {
		return 0, domain.ClientBinding{}, fmt.Errorf("physical backend %q occurs more than once in operation group", mutation.ActivePath)
	}
	paths[mutation.ActivePath] = struct{}{}
	return loadClientFromState(state, mutation.InstallationID, mutation.ClientBindingID)
}

// validateDirectoryRemovalGroupItem runs before all effects and then during staging.
// The preflight loop separately verifies every occupied target.
func validateDirectoryRemovalGroupItem(before domain.StateFileV2, removal DirectoryRemoval, index int, operations, paths map[string]struct{}) (int, error) {
	if removal.OperationID == "" || removal.Sequence < 1 {
		return -1, fmt.Errorf("removal %d has incomplete identity", index)
	}
	if _, duplicate := operations[removal.OperationID]; duplicate || operationIDExists(before, removal.OperationID) {
		return -1, fmt.Errorf("mutation operation id %q was already used", removal.OperationID)
	}
	operations[removal.OperationID] = struct{}{}
	if _, duplicate := paths[removal.ActivePath]; duplicate {
		return -1, fmt.Errorf("physical backend %q occurs more than once in removal group", removal.ActivePath)
	}
	paths[removal.ActivePath] = struct{}{}
	if removal.Standalone {
		return -1, nil
	}
	installationIndex, _, err := loadClientFromState(before, removal.InstallationID, removal.ClientBindingID)
	return installationIndex, err
}
