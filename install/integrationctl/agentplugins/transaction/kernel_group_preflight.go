package transaction

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// validateDirectoryGroupItem is used in two distinct checkpoints:
// first for the entire group before any filesystem effect, then per item as
// earlier items update desired state. Do not remove either checkpoint.
func validateDirectoryGroupItem(before, state domain.StateFileV2, mutation DirectoryMutation, index int, operations, paths map[string]struct{}) (int, domain.ClientBinding, error) {
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
