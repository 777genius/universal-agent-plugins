//go:build linux

package grok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Reuse the finite readonly native fixture. Its ordinary duplex route remains
// forbidden, so admission cannot accidentally fall back to writable execution.
type admittedEnablementFixture struct {
	*enablementNativeFixture
	unavailable bool
	probes      int
}

func (native *admittedEnablementFixture) DuplexCapability() error {
	native.probes++
	if native.unavailable {
		return errors.New("inert kernel containment unavailable")
	}
	return nil
}

func (*admittedEnablementFixture) RunDuplexWithPlannedShutdown(context.Context, legacyports.Command, func(io.Writer, io.Reader) error) error {
	return errors.New("ordinary writable duplex must never be selected")
}

func TestEnablementAdmissionRequiresPinnedContainedProjectRuntime(t *testing.T) {
	for _, condition := range []string{"qualified", "version", "executable", "user-scope", "unavailable", "no-readonly"} {
		t.Run(condition, func(t *testing.T) {
			fixture := newLifecycleEnablementFixture(t)
			env, request, original := lifecycleEnablementInput(t, fixture)
			native := &admittedEnablementFixture{enablementNativeFixture: original}
			env.Runner = native
			switch condition {
			case "version":
				request.Client.Version = "1.0.67"
			case "executable":
				request.BackendExecutable += "-different"
			case "user-scope":
				request.Plan.Scope = domain.ScopeUser
			case "unavailable":
				native.unavailable = true
			case "no-readonly":
				env.Runner = ordinaryEnablementRunner{}
			}
			before := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
			err := New().PreflightActivation(env, request)
			if condition == "qualified" && err != nil || condition != "qualified" && err == nil {
				t.Fatalf("runtime admission differs: %v", err)
			}
			if original.calls != 0 {
				t.Fatal("preflight executed native ACP")
			}
			assertPrototypeFilesUnchanged(t, before)
		})
	}
}

type ordinaryEnablementRunner struct{}

func (ordinaryEnablementRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected ordinary client command")
}

func TestEnablementProducerPublishesRecoverableObservedV3(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	native.wrongReadback = true
	outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged})
	if err == nil || outcome.NativeEffect != domain.NativeEffectUncertain {
		t.Fatal("failed readback did not retain uncertain production record", err, outcome)
	}
	record, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil || record.Version != 3 {
		t.Fatal("native enablement did not produce v3", record.Version, err)
	}
	observed := recordedObservation(t, fixture.recovery)
	cold := coldEnablementRecovery(fixture)
	if err := cold.ReconcileObserved(t.Context(), observed, cold.State); err != nil {
		t.Fatal("produced v3 cannot recover from exact public observation", err)
	}
	body, err := os.ReadFile(fixture.config)
	if err != nil || string(body) != string(fixture.original) || enablementStateDigest(fixture.loaded(t)) != record.BeforeStateDigest {
		t.Fatal("observed v3 did not restore complete predecessor", err)
	}
}

type observedStateGuard struct {
	base                        ports.NativeTransitionState
	loadFailure, persistFailure error
	loads, persists             int
	alternate                   *domain.StateFileV2
	alternateAfter              int
}

func (guard *observedStateGuard) Load() (domain.StateFileV2, error) {
	guard.loads++
	if guard.loadFailure != nil {
		return domain.StateFileV2{}, guard.loadFailure
	}
	if guard.alternate != nil && guard.loads > guard.alternateAfter {
		return *guard.alternate, nil
	}
	return guard.base.Load()
}
func (guard *observedStateGuard) PersistStateDecisionWithDisposition(before, desired domain.StateFileV2) (domain.StateDecisionDisposition, error) {
	guard.persists++
	if guard.persistFailure != nil {
		return domain.StateDecisionUnknown, guard.persistFailure
	}
	return guard.base.PersistStateDecisionWithDisposition(before, desired)
}

func TestObservedEnablementRecoveryUsesSuppliedStateFence(t *testing.T) {
	for _, phase := range []string{"load", "persist"} {
		t.Run(phase, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			fixture.attempt(t, true, false)
			observed := recordedObservation(t, fixture.recovery)
			path, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			before := capturePrototypeFiles(t, fixture.state.Path, path)
			refused := errors.New("inert observed state fence refusal")
			guard := &observedStateGuard{base: fixture.recovery.State}
			if phase == "load" {
				guard.loadFailure = refused
			} else {
				guard.persistFailure = refused
			}
			err := coldEnablementRecovery(fixture).ReconcileObserved(t.Context(), observed, guard)
			if !errors.Is(err, refused) || guard.loads == 0 || phase == "persist" && guard.persists == 0 {
				t.Fatal("observed recovery bypassed caller state fence", err, guard.loads, guard.persists)
			}
			assertPrototypeFilesUnchanged(t, before)
		})
	}
}

func TestOrdinaryRecoveryCompletesOnlyDurableRemoval(t *testing.T) {
	for _, item := range []struct {
		version       int
		footer, state string
		applied, pass bool
	}{
		{3, "complete", "target", true, true}, {3, "complete", "attempt", true, false}, {3, "none", "attempt", true, false}, {3, "partial", "attempt", true, false}, {2, "complete", "target", true, false},
	} {
		t.Run(fmt.Sprint(item.version)+"/"+item.footer+"/"+item.state, func(t *testing.T) {
			fixture, desired := recordedRemovalBoundary(t, item.footer, item.state, item.applied, item.version)
			journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			before := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			err := coldEnablementRecovery(fixture).Recover(t.Context())
			if (err == nil) != item.pass {
				t.Fatal("ordinary removal cleanup result differs", err)
			}
			if item.pass {
				assertRecordedSettlement(t, fixture, fixture.next, enablementStateDigest(desired))
			} else {
				assertPrototypeFilesUnchanged(t, before)
			}
		})
	}
}

func TestOrdinaryRemovalCleanupCannotDispatchRollbackAfterStateChange(t *testing.T) {
	fixture, desired := removalEnablementFixture(t, 3)
	attempted := fixture.loaded(t)
	applyRemovalFixture(t, fixture)
	tail, err := decisionTail(fixture.record, enablementStateDigest(desired))
	if err != nil {
		t.Fatal(err)
	}
	appendFixtureFooter(t, fixture, tail)
	if err := fixture.state.Save(desired); err != nil {
		t.Fatal(err)
	}
	observed := recordedObservation(t, fixture.recovery)
	journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
	before := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
	cold := coldEnablementRecovery(fixture)
	// Pending and whole-set preflight see the durable target; execution sees the
	// exact older attempt. Cleanup must refuse rather than restore config bytes.
	guard := &observedStateGuard{base: cold.State, alternate: &attempted, alternateAfter: 2}
	cold.State = guard
	if err := cold.Recover(t.Context()); err == nil || guard.persists != 0 {
		t.Fatal("automatic cleanup dispatched rollback", err, guard.persists)
	}
	assertPrototypeFilesUnchanged(t, before)
	if err := cold.finishSettledRemoval(t.Context(), observed); err == nil {
		t.Fatal("completion entrypoint admitted an attempt")
	}
	assertPrototypeFilesUnchanged(t, before)
}
