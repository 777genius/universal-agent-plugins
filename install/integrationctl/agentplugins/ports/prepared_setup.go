package ports

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// PreparedSetupClassifier admits a client-owned setup retry after successful
// read-only verification. Absence of this optional capability means no retry.
type PreparedSetupClassifier interface {
	CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool
}
