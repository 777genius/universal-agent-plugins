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

// ActiveNativeProjector reconstructs the desired native ownership from an
// already committed package without writing into its tree. The stager verifies
// that tree before invoking this client-owned capability.
type ActiveNativeProjector interface {
	ProjectActiveNative(context.Context, string, domain.PackageEnvelope, domain.DeliveryPlan, string) ([]domain.NativeObjectOwnership, error)
}

// SkillsLocator is implemented by a client whose projection moves the
// package's skills/ elsewhere in the delivered tree. It returns that directory
// for plan, relative to the tree and slash-separated. Without it the skills
// stay in skills/. Reading command skills back from an installed tree relies
// on it, so it must agree with the client's Projector.
type SkillsLocator interface {
	InstalledSkillsDirectory(plan domain.DeliveryPlan) string
}

// ExplicitSkillDecorator adds the client's mechanism for explicit-only
// invocation to a skill rendered from a command: header fields and files, never
// other changes. A client whose definition declares
// EnforcesExplicitSkillInvocation must implement it; the stager refuses to
// deliver an explicit-only command otherwise, and also when the decorated
// header no longer says domain.ExplicitInvocationField: true.
type ExplicitSkillDecorator interface {
	DecorateExplicitSkill(domain.RenderedSkill) (domain.RenderedSkill, error)
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
