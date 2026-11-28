package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// InstallationPolicy admits one prospective install input and returns the
// fingerprint of the rules evaluated. Callers compare it across review/apply.
// A nil policy preserves unconfigured behavior; it is never a per-run bypass.
type InstallationPolicy interface {
	Check(context.Context, domain.InstallPolicyRequest) (string, error)
}
