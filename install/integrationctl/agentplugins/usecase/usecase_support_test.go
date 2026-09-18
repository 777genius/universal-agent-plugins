package usecase

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

// testService completes a service literal with the production path policy. The
// tests outside this package use usecasetest.NewService; an import of that
// package from here would be a cycle.
func testService(base Service) Service {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	return base
}
