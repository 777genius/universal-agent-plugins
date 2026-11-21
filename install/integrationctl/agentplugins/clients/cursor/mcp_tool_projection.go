package cursor

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectCursorToolApprovals(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	approved, err := selectedCursorToolApprovals(envelope, plan)
	if err != nil || len(approved) == 0 {
		return err
	}
	return shared.WriteJSON(filepath.Join(root, cursorToolApprovalFile), map[string]any{
		"version": 1, "mcp_auto_approve": approved,
	})
}

func verifyCursorToolApprovals(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	expected, err := selectedCursorToolApprovals(envelope, plan)
	if err != nil || len(expected) == 0 {
		return err
	}
	original, err := nativeconfig.New().ReadExactFile(filepath.Join(root, cursorToolApprovalFile))
	if err != nil {
		return err
	}
	if !original.Exists {
		return fmt.Errorf("%w: Cursor tool permission projection is missing", domain.ErrProjectionRefreshRequired)
	}
	actual, err := decodeCursorToolApprovals(original.Body)
	if err != nil || !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("%w: Cursor tool permission projection changed", domain.ErrProjectionRefreshRequired)
	}
	return nil
}

// The activation request deliberately carries no untrusted publisher manifest.
// The selected, verified package carries the exact approved names instead.
func readCursorToolApprovalsForActivation(request domain.ActivationRequest, kernel nativeconfig.Kernel) (cursorToolApprovals, error) {
	if !cursorHasToolApproval(request.Plan.Components) {
		return nil, nil
	}
	if request.Plan.Scope != domain.ScopeProject || request.Delivery.ActivePath != request.Plan.ActivePath {
		return nil, fmt.Errorf("Cursor approval package or project differs from the confirmed plan")
	}
	snapshot, err := kernel.ReadExactFile(filepath.Join(request.Delivery.ActivePath, cursorToolApprovalFile))
	if err != nil {
		return nil, err
	}
	if !snapshot.Exists {
		return nil, fmt.Errorf("Cursor selected MCP tool approval projection is missing")
	}
	approved, err := decodeCursorToolApprovals(snapshot.Body)
	if err != nil {
		return nil, err
	}
	selected, _ := cursorSelectedMCP(request.Plan.Components)
	for server := range approved {
		found := false
		for _, name := range selected {
			if name == server {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("Cursor selected approval server is not in the plan")
		}
	}
	return approved, nil
}

