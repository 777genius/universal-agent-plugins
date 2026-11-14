package contracttest

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type lifecycleAdapter struct {
	exampleAdapter
	activate   func(clients.Env, domain.ActivationRequest) (domain.ActivationOutcome, error)
	deactivate func(clients.Env, domain.DeactivationRequest) (domain.DeactivationOutcome, error)
}

func (a lifecycleAdapter) Activate(_ context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	return a.activate(env, request)
}

func (a lifecycleAdapter) Deactivate(_ context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return a.deactivate(env, request)
}

func TestRunLifecycleAcceptsAWellBehavedAdapter(t *testing.T) {
	RunLifecycle(t, lifecycleAdapter{
		exampleAdapter: exampleAdapter{id: domain.ClientCursor},
		activate: func(_ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
			if err := identityMismatch(request); err != nil {
				return domain.ActivationOutcome{}, err
			}
			return domain.ActivationOutcome{Activation: domain.ActivationManual}, nil
		},
		deactivate: func(clients.Env, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
			return domain.DeactivationOutcome{}, nil
		},
	})
}

func TestLifecycleViolationsRejectABrokenAdapter(t *testing.T) {
	cases := map[string]clients.Adapter{
		"mutating verify-only": lifecycleAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			activate: func(env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
				if env.Runner == nil {
					return domain.ActivationOutcome{}, nil
				}
				_, _ = env.Runner.Run(context.Background(), legacyports.Command{Argv: []string{"/bin/copilot", "plugin", "install", "demo"}})
				return domain.ActivationOutcome{}, nil
			},
			deactivate: func(clients.Env, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
				return domain.DeactivationOutcome{}, nil
			},
		},
		"accepts mismatched ids": lifecycleAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			activate: func(clients.Env, domain.ActivationRequest) (domain.ActivationOutcome, error) {
				return domain.ActivationOutcome{}, nil
			},
			deactivate: func(clients.Env, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
				return domain.DeactivationOutcome{}, nil
			},
		},
		"unconfirmed deactivate runs commands": lifecycleAdapter{
			exampleAdapter: exampleAdapter{id: domain.ClientCursor},
			activate: func(_ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
				return domain.ActivationOutcome{}, identityMismatch(request)
			},
			deactivate: func(env clients.Env, _ domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
				if env.Runner == nil {
					return domain.DeactivationOutcome{}, nil
				}
				_, _ = env.Runner.Run(context.Background(), legacyports.Command{Argv: []string{"/bin/copilot", "plugin", "uninstall", "demo"}})
				return domain.DeactivationOutcome{}, nil
			},
		},
	}
	for name, adapter := range cases {
		lifecycle, ok := adapter.(clients.Lifecycle)
		if !ok {
			t.Fatalf("%s: adapter is not Lifecycle", name)
		}
		if violations := lifecycleViolations(t, lifecycle, adapter.ID()); len(violations) == 0 {
			t.Errorf("lifecycleViolations accepted the %q adapter", name)
		}
	}
}

func identityMismatch(request domain.ActivationRequest) error {
	if request.Plan.ClientID != request.Client.ClientID || request.Delivery.ClientID != request.Client.ClientID {
		return errMismatch
	}
	return nil
}

var errMismatch = errString("activation client identity mismatch")

type errString string

func (e errString) Error() string { return string(e) }
