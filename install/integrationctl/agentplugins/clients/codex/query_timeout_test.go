package codex

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type codexQueryDeadlineRunner struct {
	t               *testing.T
	deadline        time.Time
	listing         []byte
	failure         error
	cleanupComplete bool
	cancel          context.CancelFunc
	wait            bool
	calls           int
}

func (runner *codexQueryDeadlineRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.t.Helper()
	runner.calls++
	if !reflect.DeepEqual(command.Argv[1:], []string{"plugin", "list", "--json"}) {
		runner.t.Fatalf("read-only query changed: %v", command.Argv)
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		runner.t.Error("native plugin query received an unbounded context")
		return legacyports.CommandResult{NoChildStarted: true}, errors.New("missing query deadline")
	}
	if time.Until(deadline) > codexProjectDiscoveryTimeout {
		runner.t.Error("query exceeds the existing native discovery budget")
	}
	if !runner.deadline.IsZero() && !deadline.Equal(runner.deadline) {
		runner.t.Errorf("earlier caller deadline changed: got %v want %v", deadline, runner.deadline)
	}
	if runner.cancel != nil {
		runner.cancel()
	}
	if runner.wait {
		<-ctx.Done()
		return legacyports.CommandResult{CleanupComplete: true}, ctx.Err()
	}
	return legacyports.CommandResult{Stdout: runner.listing, CleanupComplete: runner.failure == nil || runner.cleanupComplete}, runner.failure
}

func codexQueryDeadlineFixture(t *testing.T) (domain.ActivationRequest, []byte) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profile, artifact := tempRoot(t), tempRoot(t)
	request := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: profile},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientCodex, Scope: domain.ScopeUser,
			PhysicalArtifactID: "query-owned", NativeRegistryRoot: profile},
		Delivery:     domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: artifact},
		DeclaredName: "demo", BackendExecutable: executable, VerifyOnly: true,
	}
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	listing, err := json.Marshal(map[string]any{"installed": []any{map[string]any{
		"pluginId": "demo@" + marketplace, "name": "demo", "marketplaceName": marketplace,
		"installed": true, "enabled": true, "version": "1.0.0",
		"source": map[string]any{"source": "local", "path": artifact},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return request, listing
}

func invokeCodexQuery(ctx context.Context, kind string, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	switch kind {
	case "verification":
		return New().Activate(ctx, env, request)
	case "cache version":
		_, err := codexNativeUserVersion(ctx, env, request)
		return domain.ActivationOutcome{}, err
	default:
		err := verifyCodexNativeRemoval(ctx, env, domain.DeactivationRequest{
			Client: request.Client, DeclaredName: request.DeclaredName,
			BackendExecutable: request.BackendExecutable,
		}, shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID))
		return domain.ActivationOutcome{}, err
	}
}

func TestCodexPluginQueriesHaveFiniteDeadline(t *testing.T) {
	for _, kind := range []string{"verification", "cache version", "removal"} {
		t.Run(kind, func(t *testing.T) {
			request, _ := codexQueryDeadlineFixture(t)
			runner := &codexQueryDeadlineRunner{t: t, failure: context.DeadlineExceeded, cleanupComplete: true}
			env := clients.Env{Runner: runner}
			outcome, err := invokeCodexQuery(context.Background(), kind, env, request)
			if runner.calls != 1 || !errors.Is(err, context.DeadlineExceeded) || outcome.Activation == domain.ActivationActive {
				t.Fatalf("timed-out query qualified or lost its cause: calls=%d outcome=%+v err=%v", runner.calls, outcome, err)
			}
		})
	}
}

func TestCodexPluginQueryPreservesCallerCancellation(t *testing.T) {
	for _, scenario := range []string{"earlier deadline", "already canceled", "canceled with valid output"} {
		t.Run(scenario, func(t *testing.T) {
			request, listing := codexQueryDeadlineFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			runner := &codexQueryDeadlineRunner{t: t, listing: listing, wait: true}
			want := context.Canceled
			switch scenario {
			case "earlier deadline":
				var stop context.CancelFunc
				ctx, stop = context.WithTimeout(ctx, 20*time.Millisecond)
				defer stop()
				runner.deadline, _ = ctx.Deadline()
				want = context.DeadlineExceeded
			case "already canceled":
				cancel()
			case "canceled with valid output":
				runner.cancel, runner.wait = cancel, false
			}
			env := clients.Env{Runner: runner}
			outcome, err := New().Activate(ctx, env, request)
			if runner.calls != 1 || !errors.Is(err, want) || outcome.Activation == domain.ActivationActive {
				t.Fatalf("caller cancellation qualified a query: calls=%d outcome=%+v err=%v", runner.calls, outcome, err)
			}
		})
	}
}

func TestCodexPluginQueryPreservesClosureFailure(t *testing.T) {
	for _, kind := range []string{"verification", "cache version", "removal"} {
		t.Run(kind, func(t *testing.T) {
			request, listing := codexQueryDeadlineFixture(t)
			cause := errors.New("native child closure unproved")
			closure := &legacyports.ProcessClosureError{Cause: cause}
			runner := &codexQueryDeadlineRunner{t: t, listing: listing, failure: closure}
			env := clients.Env{Runner: runner}
			outcome, err := invokeCodexQuery(context.Background(), kind, env, request)
			var got *legacyports.ProcessClosureError
			if !errors.As(err, &got) || got != closure || !errors.Is(err, cause) || outcome.Activation == domain.ActivationActive || runner.calls != 1 {
				t.Fatalf("query lost closure failure: calls=%d outcome=%+v err=%v", runner.calls, outcome, err)
			}
			if !domain.IsUnclosedClientProbe(err) {
				t.Fatalf("query lost unsafe process marker: %v", err)
			}
		})
	}
}

func TestCodexPluginQueryClosedFailuresStayOrdinary(t *testing.T) {
	for _, kind := range []string{"verification", "cache version", "removal"} {
		t.Run(kind, func(t *testing.T) {
			for _, test := range []struct {
				name string
				err  error
			}{
				{name: "ordinary error", err: errors.New("native listing failed after cleanup")},
				{name: "deadline", err: context.DeadlineExceeded},
			} {
				t.Run(test.name, func(t *testing.T) {
					request, listing := codexQueryDeadlineFixture(t)
					runner := &codexQueryDeadlineRunner{t: t, listing: listing, failure: test.err, cleanupComplete: true}
					outcome, err := invokeCodexQuery(context.Background(), kind, clients.Env{Runner: runner}, request)
					if runner.calls != 1 || !errors.Is(err, test.err) || outcome.Activation == domain.ActivationActive || domain.IsUnclosedClientProbe(err) {
						t.Fatalf("closed failure lost its ordinary cause: calls=%d outcome=%+v err=%v", runner.calls, outcome, err)
					}
				})
			}
		})
	}
}

func TestCodexPluginQueryAcceptsRecognizedListing(t *testing.T) {
	request, listing := codexQueryDeadlineFixture(t)
	runner := &codexQueryDeadlineRunner{t: t, listing: listing}
	env := clients.Env{Runner: runner}
	outcome, err := New().Activate(context.Background(), env, request)
	if err != nil || outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled || runner.calls != 1 {
		t.Fatalf("recognized listing refused: calls=%d outcome=%+v err=%v", runner.calls, outcome, err)
	}
}
