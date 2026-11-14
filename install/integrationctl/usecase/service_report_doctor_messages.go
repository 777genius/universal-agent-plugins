package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func doctorWarningForOperation(op domain.OperationRecord) string {
	switch op.Status {
	case "degraded":
		return fmt.Sprintf("Operation %s for %s ended degraded - run plugin-kit-ai integrations repair %s.", op.OperationID, op.IntegrationID, op.IntegrationID)
	case "in_progress":
		return fmt.Sprintf("Operation %s for %s is still marked in_progress - inspect the journal and rerun repair if the process was interrupted.", op.OperationID, op.IntegrationID)
	case "failed":
		return fmt.Sprintf("Operation %s for %s failed before commit - inspect the journal and rerun the desired lifecycle command.", op.OperationID, op.IntegrationID)
	default:
		return fmt.Sprintf("Open operation %s for %s is still marked %s.", op.OperationID, op.IntegrationID, op.Status)
	}
}
