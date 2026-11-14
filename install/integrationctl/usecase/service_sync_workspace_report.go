package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) newWorkspaceSyncReport(desiredCount int) domain.Report {
	return domain.Report{
		OperationID: operationID("sync", "workspace", s.now()),
		Summary:     fmt.Sprintf("Processed workspace sync for %d desired integration(s).", desiredCount),
	}
}

func finalizeWorkspaceSyncReport(report *domain.Report) {
	if !workspaceSyncHasNoChanges(*report) {
		return
	}
	report.Summary = "Workspace sync found no changes."
}

func workspaceSyncHasNoChanges(report domain.Report) bool {
	return len(report.Targets) == 0 && len(report.Warnings) == 0
}
