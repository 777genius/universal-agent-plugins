package clients

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// StagingLayout overrides where the transaction staging directory is created
// and what the generic path validation additionally requires. Clients that
// stage directly under the plan target root do not implement it; the stager
// falls back to shared.DefaultStagingLayout.
type StagingLayout interface {
	StagingBase(plan domain.DeliveryPlan) string
	ValidateTargetLayout(plan domain.DeliveryPlan) error
}

// Projector writes the client-specific projection into an already sanitized
// staging tree and returns the native objects it now owns. The generic stager
// owns the snapshot copy, the sanitizing pass and the digest; a projector only
// adds what its client needs to read the package.
type Projector interface {
	Project(ctx context.Context, in ProjectionInput) ([]domain.NativeObjectOwnership, error)
}

// ProjectionInput is everything a projector is allowed to see. StagingPath is
// the tree to write into; Plan.ActivePath is the future location the projection
// has to encode, and the two are deliberately different.
type ProjectionInput struct {
	StagingPath    string
	Envelope       domain.PackageEnvelope
	Plan           domain.DeliveryPlan
	Hints          domain.CompatibilityHints
	PluginDataPath string
	Launcher       StdioLauncherDeliverer
}

// DeliverLauncher returns the injected launcher copy function, or nil when the
// composition root did not supply one. Projectors must use this instead of
// calling Launcher.Deliver directly: a typed-nil *managedstdio.Source inside
// the interface would panic.
func (in ProjectionInput) DeliverLauncher() func(string) error {
	if in.Launcher == nil {
		return nil
	}
	return in.Launcher.Deliver
}
