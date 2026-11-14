// Package usecasetest constructs install core services for tests outside the
// usecase package. It exists so that a new required dependency is wired in one
// place instead of in every test that builds a service.
package usecasetest

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// NewService completes a service literal with the production path policy. It
// deliberately offers no way to substitute a different one: a permissive path
// policy in a test would make the test prove nothing about containment.
func NewService(base usecase.Service) usecase.Service {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	return base
}
