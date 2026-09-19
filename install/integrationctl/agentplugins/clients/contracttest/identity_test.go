package contracttest

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type identityAdapter struct {
	exampleAdapter
	usesExecutable bool
	inspect        func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error)
}

func (a identityAdapter) UsesNativeRegistryExecutable() bool { return a.usesExecutable }

func (a identityAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, _ domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	return a.inspect(ctx, env, managed)
}

func TestRunRegistryInspectorAcceptsAWellBehavedAdapter(t *testing.T) {
	RunRegistryInspector(t, identityAdapter{
		exampleAdapter: exampleAdapter{id: domain.ClientCursor},
		inspect: func(ctx context.Context, env clients.Env, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
			if err := ctx.Err(); err != nil {
				return clients.RegistryIndeterminate, err
			}
			if env.Runner == nil {
				return clients.RegistryIndeterminate, nil
			}
			if managed == nil {
				return clients.RegistryClear, nil
			}
			return clients.RegistryExpected, nil
		},
	})
}

func TestRegistryInspectorViolationsRejectABrokenAdapter(t *testing.T) {
	cases := map[string]clients.RegistryInspector{
		"nil runner panics": identityAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			usesExecutable: true,
			inspect: func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error) {
				panic("nil runner")
			},
		},
		"nil runner reports expected": identityAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			usesExecutable: true,
			inspect: func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error) {
				return clients.RegistryExpected, nil
			},
		},
		"ignores canceled context": identityAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			inspect: func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error) {
				return clients.RegistryClear, nil
			},
		},
		"expected without managed": identityAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			inspect: func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error) {
				return clients.RegistryExpected, nil
			},
		},
		"native config empty root is clear": identityAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCline},
			inspect: func(context.Context, clients.Env, *domain.ClientBinding) (clients.RegistryFinding, error) {
				return clients.RegistryClear, nil
			},
		},
	}
	for name, inspector := range cases {
		if violations := registryInspectorViolations(t, inspector, domain.ClientCursor); len(violations) == 0 {
			t.Errorf("registryInspectorViolations accepted the %q adapter", name)
		}
	}
}
