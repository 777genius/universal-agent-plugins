package codex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type closureDiscoveryRunner struct {
	listing  []byte
	failure  error
	cancel   context.CancelFunc
	unclosed bool
}

func (runner *closureDiscoveryRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{Stdout: runner.listing}, nil
}
func (runner *closureDiscoveryRunner) RunDuplexToCompletion(ctx context.Context, _ legacyports.Command, _ func(io.Writer, io.Reader) error) error {
	if runner.cancel != nil {
		runner.cancel()
	}
	cause := errors.Join(runner.failure, ctx.Err())
	if runner.unclosed {
		return &legacyports.ProcessClosureError{Cause: cause}
	}
	return cause
}

// The core stops a client group on IsUnclosedClientProbe. Losing that typed
// cause here would let the next client run while native cleanup is uncertain.
func TestCodexCompletionClosureUncertaintyStopsBothScopeObservers(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeProject, domain.ScopeUser} {
		for _, scenario := range []string{"closed-failure", "unclosed", "unclosed-canceled"} {
			t.Run(string(scope)+"/"+scenario, func(t *testing.T) {
				env, request, _ := projectDiscoveryFixture(t)
				request.DeclaredName = "demo"
				request.Plan.PhysicalArtifactID = "owned"
				market := shared.ManagedMarketplaceName("owned")
				listing, err := json.Marshal(map[string]any{"installed": []any{map[string]any{"pluginId": "demo@" + market, "name": "demo", "marketplaceName": market, "installed": true, "enabled": true, "version": "1.0.0", "source": map[string]any{"source": "local", "path": request.Delivery.ActivePath}}}})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				failure := errors.New("injected native completion failure")
				runner := &closureDiscoveryRunner{listing: listing, failure: failure, unclosed: scenario != "closed-failure"}
				if scenario == "unclosed-canceled" {
					runner.cancel = cancel
				}
				env.Runner = runner
				if scope == domain.ScopeUser {
					request.VerifyOnly = true
					request.Plan.Scope = scope
					request.Plan.NativeRegistryRoot = request.Client.ConfigRoot
					request.Plan.TargetContext = domain.TargetContext{Scope: scope, UserConfigRoot: request.Client.ConfigRoot}
				}
				outcome, err := New().Activate(ctx, env, request)
				if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled {
					t.Fatalf("installed state lost: %+v %v", outcome, err)
				}
				if runner.unclosed {
					if !domain.IsUnclosedClientProbe(err) || !errors.Is(err, failure) || outcome.NativeEffect != domain.NativeEffectUncertain {
						t.Fatalf("closure uncertainty swallowed: %+v %v", outcome, err)
					}
				} else if err != nil || outcome.NativeEffect == domain.NativeEffectUncertain {
					t.Fatalf("closed failure incorrectly became unsafe: %+v %v", outcome, err)
				}
			})
		}
	}
}
