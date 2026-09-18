// Package plannertest constructs planners for tests outside the planner
// package. It exists so that a new required dependency is wired in one place
// instead of in every test that builds a planner.
package plannertest

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

// NewPlanner completes a planner literal with the production path policy. It
// deliberately offers no way to substitute a different one: a permissive path
// policy in a test would make the test prove nothing about containment.
func NewPlanner(base planner.Planner) planner.Planner {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	return base
}
