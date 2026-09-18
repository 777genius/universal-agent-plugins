package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The interfaces below are optional capabilities of a required port. The use
// case discovers them with a type assertion, the same way the standard library
// discovers io.WriterTo, and falls back to the base contract when a provider
// does not implement one. Declaring them here keeps the contract readable in a
// single place instead of hiding it in unexported use case declarations.

// ActivationPreflighter lets an activator reject a request before any client or
// managed file is touched.
type ActivationPreflighter interface {
	PreflightActivation(domain.ActivationRequest) error
}

// AutomaticActivationClassifier reports that activation completes without a
// user step, so no manual follow-up action is recorded.
type AutomaticActivationClassifier interface {
	AutomaticallyActivates(domain.ActivationRequest) bool
}

// ActivationVerifierClassifier reports whether an exact client-side verifier
// can observe this plan, which decides if prior activation evidence may be
// retained instead of re-derived.
type ActivationVerifierClassifier interface {
	VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool
}

// ManagedStdioPreflighter resolves a declared stdio helper against the managed
// runtime before the plan promises to deliver it.
type ManagedStdioPreflighter interface {
	PreflightManagedStdio(command string) error
}

// DataPathPreflighter reports the effective plugin data path for a component
// and whether it is available on this host.
type DataPathPreflighter interface {
	PreflightDataPath(path string) (string, bool, error)
}

// PluginDataAwareStager stages a package that also owns a plugin data
// directory, so staging and data provisioning commit as one unit.
type PluginDataAwareStager interface {
	StageWithPluginData(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints, string) (domain.StagedDelivery, error)
}
