package usecase

import (
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func sortSyncReportTargets(report *domain.Report) {
	sort.Slice(report.Targets, func(i, j int) bool {
		return lessSyncTargetReport(report.Targets[i], report.Targets[j])
	})
}

func lessSyncTargetReport(left, right domain.TargetReport) bool {
	if left.TargetID == right.TargetID {
		return left.DeliveryKind < right.DeliveryKind
	}
	return left.TargetID < right.TargetID
}
