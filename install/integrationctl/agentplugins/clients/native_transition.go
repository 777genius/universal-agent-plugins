package clients

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// NativeTransitionContext is explicit composition authority. Factories must
// only capture it; construction never discovers, recovers, or mutates state.
type NativeTransitionContext struct {
	ProjectRoot, UserHome, OperationsDir string
	State                                ports.NativeTransitionState
	Kernel                               nativeconfig.Kernel
	PackageVerifier                      ports.NativePackageVerifier
}

type NativeTransitionProvider interface {
	NewNativeTransitions(NativeTransitionContext) ports.ObservedNativeTransitionRecovery
}
