package providers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Project component files are an ownership boundary even when live discovery
// is disabled. Their findings stay on the prepared side: a file match cannot
// authorize native recovery or claim that a client loaded the component.
func (observer NativeIdentityObserver) inspectPreparedProjectFiles(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (registryFinding, error) {
	target := plan.TargetContext
	if plan.Scope != domain.ScopeProject && target.Scope != domain.ScopeProject && target.ProjectRoot == "" {
		return registryClear, nil
	}
	if client.ClientID != plan.ClientID || plan.Scope != domain.ScopeProject || target.Scope != domain.ScopeProject ||
		target.UserConfigRoot != client.ConfigRoot {
		return registryIndeterminate, fmt.Errorf("project file observation requires matching client, scope and original user profile")
	}
	if _, err := domain.NewTargetContext(target.Scope, target.ProjectRoot, target.UserConfigRoot); err != nil {
		return registryIndeterminate, err
	}
	canonical, err := filepath.EvalSymlinks(target.ProjectRoot)
	if err != nil {
		return registryIndeterminate, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return registryIndeterminate, err
	}
	if canonical != target.ProjectRoot || !info.IsDir() {
		return registryIndeterminate, fmt.Errorf("project file observation requires the exact canonical project directory")
	}
	switch plan.SelectedDelivery.EffectiveTraitsForScope(client.ClientID, plan.Scope).LifecycleKind {
	case domain.LifecycleNativeConfig:
	case domain.LifecycleCLIRegistry, domain.LifecyclePrepared:
		return registryClear, nil
	default:
		return registryIndeterminate, fmt.Errorf("project file observation requires a qualified scoped lifecycle")
	}
	// The adapter receives the same injected kernel and complete ownership
	// receipts, but no runner regardless of the caller's live-discovery policy.
	observer.Runner = nil
	return observer.inspectNativeRegistry(ctx, client, plan, managed)
}

func combinePreparedFindings(first, second registryFinding) registryFinding {
	valid := func(finding registryFinding) registryFinding {
		switch finding {
		case registryCollision, registryIndeterminate, registryExpected, registryClear:
			return finding
		default:
			return registryIndeterminate
		}
	}
	first, second = valid(first), valid(second)
	for _, finding := range []registryFinding{registryCollision, registryIndeterminate, registryExpected, registryClear} {
		if first == finding || second == finding {
			return finding
		}
	}
	return registryIndeterminate
}
