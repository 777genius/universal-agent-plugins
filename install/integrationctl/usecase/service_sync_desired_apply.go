package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

func canReplaceDesiredSync(record domain.InstallationRecord, desiredPolicy domain.InstallPolicy) bool {
	return record.Policy.Scope == "project" && desiredPolicy.Scope == "project"
}
