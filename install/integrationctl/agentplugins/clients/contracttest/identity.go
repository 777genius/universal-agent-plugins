package contracttest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// RunRegistryInspector asserts the identity-inspection half of the contract
// when the adapter implements it. A client without RegistryInspector is
// observed only through the generic prepared-root walk.
func RunRegistryInspector(t *testing.T, adapter clients.Adapter) {
	t.Helper()
	RunAdapter(t, adapter)
	inspector, ok := adapter.(clients.RegistryInspector)
	if !ok {
		return
	}
	for _, violation := range registryInspectorViolations(t, inspector, adapter.ID()) {
		t.Error(violation)
	}
}

func registryInspectorViolations(t *testing.T, inspector clients.RegistryInspector, id domain.ClientID) []string {
	t.Helper()
	violations := make([]string, 0, 4)
	violations = append(violations, registryInspectorNilRunnerViolations(inspector, id)...)
	violations = append(violations, registryInspectorCanceledContextViolations(inspector, id)...)
	violations = append(violations, registryInspectorUnmanagedFindingViolations(inspector, id)...)
	return violations
}

func registryInspectorNilRunnerViolations(inspector clients.RegistryInspector, id domain.ClientID) []string {
	var panicked any
	var finding clients.RegistryFinding
	var err error
	func() {
		defer func() { panicked = recover() }()
		finding, err = inspector.InspectNativeRegistry(context.Background(), clients.Env{}, registryInspectorClient(id), registryInspectorPlan(id), nil)
	}()
	if panicked != nil {
		return []string{fmt.Sprintf("InspectNativeRegistry panicked with a nil runner: %v", panicked)}
	}
	if inspector.UsesNativeRegistryExecutable() && finding != clients.RegistryIndeterminate {
		return []string{fmt.Sprintf("nil runner produced finding %d, want Indeterminate", finding)}
	}
	_ = err
	return nil
}

func registryInspectorCanceledContextViolations(inspector clients.RegistryInspector, id domain.ClientID) []string {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var panicked any
	var err error
	func() {
		defer func() { panicked = recover() }()
		_, err = inspector.InspectNativeRegistry(ctx, clients.Env{}, registryInspectorClient(id), registryInspectorPlan(id), nil)
	}()
	if panicked != nil {
		return []string{fmt.Sprintf("InspectNativeRegistry panicked on a canceled context: %v", panicked)}
	}
	if !errors.Is(err, context.Canceled) {
		return []string{fmt.Sprintf("canceled context returned %v, want context.Canceled", err)}
	}
	return nil
}

func registryInspectorUnmanagedFindingViolations(inspector clients.RegistryInspector, id domain.ClientID) []string {
	var panicked any
	var finding clients.RegistryFinding
	var err error
	func() {
		defer func() { panicked = recover() }()
		finding, err = inspector.InspectNativeRegistry(context.Background(), clients.Env{}, registryInspectorClient(id), registryInspectorPlan(id), nil)
	}()
	if panicked != nil {
		return []string{fmt.Sprintf("InspectNativeRegistry panicked: %v", panicked)}
	}
	if err != nil {
		return nil
	}
	if finding == clients.RegistryExpected {
		return []string{"InspectNativeRegistry reported Expected without a managed binding"}
	}
	return nil
}

func registryInspectorClient(id domain.ClientID) domain.DetectedClient {
	return domain.DetectedClient{ClientID: id, ConfigRoot: "/agentplugins-contract/config"}
}

func registryInspectorPlan(id domain.ClientID) domain.DeliveryPlan {
	return domain.DeliveryPlan{
		ClientID:                 id,
		DeclaredName:             "demo",
		DeclaredVersion:          "1.0.0",
		PhysicalArtifactID:       "demo-0123456789ab",
		NativeRegistryExecutable: "/agentplugins-contract/bin/client",
		NativeRegistryRoot:       "/agentplugins-contract/config",
		TargetAnchor:             "/agentplugins-contract/config",
		TargetRoot:               "/agentplugins-contract/config/root",
		ActivePath:               "/agentplugins-contract/config/root/demo-0123456789ab",
	}
}
