package agentpluginscli

import (
	"errors"
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

var errDependencyPinMismatch = errors.New("dependency pin does not match verified package")

// A lock's empty inventory means no executable members, including for historical
// omitted fields. Only receipt-based repair may fill its missing inventory before
// reaching this strict boundary. Historical absent versions may be hydrated by
// the caller after validation; a recorded version is always authoritative.
func validateDependencyPinEnvelope(pin environment.LockedPackage, envelope domain.PackageEnvelope) error {
	if envelope.LoaderKind != domain.LoaderKindAgentPlugins || envelope.TreeDigest != pin.TreeDigest || envelope.ManifestDigest != pin.ManifestDigest {
		return fmt.Errorf("%w: strict standard-loader identity checks failed", errDependencyPinMismatch)
	}
	if pin.Version != "" && pin.Version != envelope.Manifest.Version {
		return fmt.Errorf("%w: package version differs from uap.lock", errDependencyPinMismatch)
	}
	if err := environment.ValidateExecutableInventory(pin.ExecutableFiles); err != nil {
		return fmt.Errorf("%w: %w", errDependencyPinMismatch, err)
	}
	if err := environment.ValidateExecutableInventory(envelope.ExecutableFiles); err != nil {
		return fmt.Errorf("%w: verified inventory: %w", errDependencyPinMismatch, err)
	}
	if !slices.Equal(canonicalDependencyExecutables(pin.ExecutableFiles), canonicalDependencyExecutables(envelope.ExecutableFiles)) {
		return fmt.Errorf("%w: executable inventory differs from uap.lock", errDependencyPinMismatch)
	}
	return nil
}

// Operational copies use the canonical order required by snapshot/SDK contracts.
// Keep the original lock order intact so healthy historical files do not churn.
func canonicalDependencyExecutables(files []string) []string {
	out := append([]string{}, files...)
	slices.Sort(out)
	return out
}
