package shared

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AppendUnique appends value unless the slice already carries it. Plan warnings
// and user actions are a set in practice: the same sentence twice is noise, and
// a refiner has to stay idempotent when the same plan is refined again.
func AppendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// PromoteBackendReady marks a plan ready and prepared when the CLI that
// installs on this client's behalf was found. Copilot and VS Code share that
// CLI - VS Code is installed through the Copilot one - so both adapters promote
// through this rule and differ only in which executable they resolve for it.
func PromoteBackendReady(plan *domain.DeliveryPlan, backendExecutable string) {
	PromoteNativeReady(plan, backendExecutable, true)
}
