package installer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type hostedPreparationRunner struct {
	available     bool
	checks, calls int
	onCheck       func(int)
	failure       error
}

func (r *hostedPreparationRunner) DuplexCapability() error {
	r.checks++
	if r.onCheck != nil {
		r.onCheck(r.checks)
	}
	if r.available {
		return nil
	}
	if r.failure != nil {
		return r.failure
	}
	return errors.New("fixture duplex unavailable")
}

func (r *hostedPreparationRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	r.calls++
	return legacyports.CommandResult{}, errors.New("native execution is forbidden")
}

func (r *hostedPreparationRunner) RunDuplexWithPlannedShutdown(context.Context, legacyports.Command, func(io.Writer, io.Reader) error) error {
	r.calls++
	return errors.New("native execution is forbidden")
}

func hostedPreparationFixture(t *testing.T) (*Engine, Request, *hostedPreparationRunner) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkg, user := filepath.Join(base, "package"), filepath.Join(base, "home", ".kiro")
	deathWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"hosted-preparation","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(pkg, "skills", "owned", "SKILL.md"), deathSkill("owned", "original"))
	deathWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"owned":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	deathWrite(t, filepath.Join(user, "settings", "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp","disabled":true}}}`)
	registry, err := clients.NewRegistry(kiro.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	runner := &hostedPreparationRunner{}
	engine, err := New(Config{StateRoot: filepath.Join(base, "uap"), Registry: registry, Runner: runner, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Operation: OpInstall, PackageRoot: pkg, ClientID: "kiro", ClientConfigRoot: user,
		ClientExecutable: filepath.Join(base, "kiro-cli"), OperationID: "hosted-preparation", RequiredComponents: []string{"mcp", "skills"}}
	return engine, req, runner
}

func TestFacadeHostedPreparationPinsIntentAndSnapshot(t *testing.T) {
	engine, req, runner := hostedPreparationFixture(t)
	handle, err := engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	plan := handle.Plan()
	if plan.InstallationID == "" || plan.Delivery.InstallIntent != string(domain.InstallIntentPrepare) || len(handle.deliveries) != 1 || handle.deliveries[0].request.InstallIntent != domain.InstallIntentPrepare || runner.calls != 0 {
		t.Fatalf("preparation was not frozen: %+v, captures=%d, calls=%d", plan, len(handle.deliveries), runner.calls)
	}
	original := handle.snapshot.TreeDigest
	// Availability and mutable source changes after consent cannot silently
	// replace the frozen preparation with automatic activation or new bytes.
	runner.available = true
	deathWrite(t, filepath.Join(req.PackageRoot, "skills", "owned", "SKILL.md"), deathSkill("owned", "changed after preparation"))
	result, err := engine.Apply(t.Context(), handle, Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted || result.Client.Activation != string(domain.ActivationPrepared) || result.Client.Verification != string(domain.VerificationPackageValid) || runner.calls != 0 {
		t.Fatalf("apply changed agreed preparation: %+v, %v, calls=%d", result, err, runner.calls)
	}
	state, err := engine.store.Load()
	if err != nil || len(state.Installations) != 1 || state.Installations[0].InstallationID != plan.InstallationID || state.Installations[0].Source.TreeDigest != original {
		t.Fatalf("source or installation changed: %+v, %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.InstallIntent != domain.InstallIntentPrepare || binding.Activation != domain.ActivationPrepared || binding.Authentication == domain.AuthenticationComplete {
			t.Fatal("preparation became automatic or attested")
		}
	}
}

func TestFacadeHostedPreparationPreservesAutomaticAndOtherFailures(t *testing.T) {
	for _, scenario := range []string{"available", "wrapped-preflight", "second-check-available", "canceled", "invalid-source", "project"} {
		t.Run(scenario, func(t *testing.T) {
			engine, req, runner := hostedPreparationFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch scenario {
			case "available":
				runner.available = true
			case "second-check-available":
				runner.onCheck = func(n int) { runner.available = n > 1 }
			case "wrapped-preflight":
				runner.onCheck = func(n int) {
					if n == 2 {
						runner.failure = errors.Join(errors.New("additional context"), errors.New("fixture duplex unavailable"))
					}
				}
			case "canceled":
				runner.onCheck = func(int) { cancel() }
			case "invalid-source":
				deathWrite(t, filepath.Join(req.PackageRoot, "plugin.json"), "not JSON")
			case "project":
				req.Scope, req.ProjectRoot = domain.ScopeProject, filepath.Join(filepath.Dir(req.PackageRoot), "project")
			}
			before, err := os.ReadFile(filepath.Join(req.ClientConfigRoot, "settings", "mcp.json"))
			if err != nil {
				t.Fatal(err)
			}
			handle, prepareErr := engine.Prepare(ctx, req)
			if handle != nil {
				defer func() {
					if err := handle.Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			if scenario == "available" || scenario == "wrapped-preflight" {
				intent, checks := domain.InstallIntentAutomatic, 1
				if scenario == "wrapped-preflight" {
					intent, checks = domain.InstallIntentPrepare, 2
				}
				if prepareErr != nil || handle.Plan().Delivery.InstallIntent != string(intent) || runner.checks != checks {
					t.Fatalf("available automatic plan was changed: %v", prepareErr)
				}
			} else if prepareErr == nil || handle != nil {
				t.Fatalf("unrelated failure converted to preparation: %v", prepareErr)
			}
			if scenario == "canceled" && !errors.Is(prepareErr, context.Canceled) {
				t.Fatalf("cancellation lost: %v", prepareErr)
			}
			if scenario == "second-check-available" && (runner.checks != 2 || !strings.Contains(prepareErr.Error(), "fixture duplex unavailable")) {
				t.Fatalf("original preflight failure lost: %v", prepareErr)
			}
			after, err := os.ReadFile(filepath.Join(req.ClientConfigRoot, "settings", "mcp.json"))
			state, stateErr := engine.store.Load()
			if err != nil || stateErr != nil || !reflect.DeepEqual(before, after) || len(state.Installations) != 0 || runner.calls != 0 {
				t.Fatal("preview changed state, native configuration or executed a process")
			}
		})
	}
}

func applyHostedFixture(t *testing.T, engine *Engine, req Request) Result {
	t.Helper()
	handle, err := engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := engine.Apply(t.Context(), handle, Decision{Confirmed: true})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestFacadeHostedPreparationRefusesAppearingRetainedInstallation(t *testing.T) {
	for _, phase := range []string{"retry", "apply"} {
		t.Run(phase, func(t *testing.T) {
			engine, req, runner := hostedPreparationFixture(t)
			installed := applyHostedFixture(t, engine, req)
			req.InstallationID, req.Operation, req.OperationID = installed.InstallationID, OpRemove, "retain-data"
			applyHostedFixture(t, engine, req)
			retained, err := engine.store.Load()
			if err != nil || len(retained.Installations) != 1 || len(retained.Installations[0].Clients) != 0 || len(retained.Installations[0].DataReceipts) != 1 {
				t.Fatalf("real retained fixture missing: %+v, %v", retained, err)
			}
			// Model a concurrent same-IID retained state arriving after the
			// original empty capture. Its receipts come from real install/remove.
			if err := engine.store.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}); err != nil {
				t.Fatal(err)
			}
			req.Operation, req.OperationID = OpInstall, "fresh-after-capture"
			runner.checks = 0
			var captured []byte
			inject := func() {
				if err := engine.store.Save(retained); err != nil {
					t.Fatal(err)
				}
				captured, err = os.ReadFile(engine.cfg.StateFile)
				if err != nil {
					t.Fatal(err)
				}
			}
			if phase == "retry" {
				runner.onCheck = func(n int) {
					if n == 2 {
						inject()
					}
				}
			}
			handle, refused := engine.Prepare(t.Context(), req)
			if phase == "apply" {
				if refused != nil {
					t.Fatal(refused)
				}
				inject()
				_, refused = engine.Apply(t.Context(), handle, Decision{Confirmed: true})
			}
			if handle != nil {
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			}
			after, readErr := os.ReadFile(engine.cfg.StateFile)
			if !errors.Is(refused, ErrPlanChanged) || readErr != nil || len(captured) == 0 || !reflect.DeepEqual(captured, after) || runner.calls != 0 {
				t.Fatalf("appearing retained authority was overwritten: %v, %v, calls=%d", refused, readErr, runner.calls)
			}
		})
	}
}

func TestFacadeHostedPreparationPreservesRecordedModesAndOwnership(t *testing.T) {
	for _, scenario := range []string{"recorded-automatic", "retained-preference", "owned-drift"} {
		t.Run(scenario, func(t *testing.T) {
			engine, req, runner := hostedPreparationFixture(t)
			installed := applyHostedFixture(t, engine, req)
			req.InstallationID, req.OperationID = installed.InstallationID, "existing-installation"
			state, err := engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "recorded-automatic":
				for key, binding := range state.Installations[0].Clients {
					binding.InstallIntent = domain.InstallIntentAutomatic
					state.Installations[0].Clients[key] = binding
				}
				if err := engine.store.Save(state); err != nil {
					t.Fatal(err)
				}
			case "retained-preference":
				req.Operation = OpRemove
				applyHostedFixture(t, engine, req)
				req.Operation, runner.available = OpInstall, true
			case "owned-drift":
				deathWrite(t, filepath.Join(installed.Binding.TargetPath, "skills", "owned", "SKILL.md"), "changed owned bytes")
			}
			before, err := os.ReadFile(engine.cfg.StateFile)
			if err != nil {
				t.Fatal(err)
			}
			handle, prepareErr := engine.Prepare(t.Context(), req)
			if scenario == "retained-preference" {
				if prepareErr != nil || handle.Plan().Delivery.InstallIntent != string(domain.InstallIntentPrepare) {
					t.Fatalf("retained preparation changed: %v", prepareErr)
				}
			} else if prepareErr == nil {
				t.Fatal("recorded mode or ownership refusal was converted to preparation")
			}
			if handle != nil {
				if err := handle.Close(); err != nil {
					t.Fatal(err)
				}
			}
			after, readErr := os.ReadFile(engine.cfg.StateFile)
			if readErr != nil || !reflect.DeepEqual(before, after) || runner.calls != 0 {
				t.Fatal("recorded identity, receipts or mode were overwritten")
			}
		})
	}
}

func TestFacadeHostedPreparationEligibilityDoesNotBroadenScopeOrConsent(t *testing.T) {
	engine, req, _ := hostedPreparationFixture(t)
	handle, err := engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, scenario := range []string{"supported", "unsupported", "project", "personal-mapping", "attested", "update", "recorded"} {
		t.Run(scenario, func(t *testing.T) {
			candidate := &PreparedOperation{req: handle.req, recorded: handle.recorded}
			input := usecase.AddInput{Client: handle.client, Scope: domain.ScopeUser}
			preview := usecase.AddResult{InstallationID: handle.plan.InstallationID, Plan: handle.observationPlan}
			preview.Plan.InstallIntent = domain.InstallIntentAutomatic
			switch scenario {
			case "unsupported":
				preview.Plan.Status = domain.PlanUnsupported
			case "project":
				preview.Plan.TargetContext.ProjectRoot = filepath.Join(t.TempDir(), "project")
			case "personal-mapping":
				preview.Plan.ClientID, input.Client.ClientID = domain.ClientChatGPT, domain.ClientChatGPT
			case "attested":
				input.AuthComplete = true
			case "update":
				candidate.req.Operation = OpUpdate
			case "recorded":
				candidate.recorded.Installations = []domain.Installation{{InstallationID: preview.InstallationID}}
			}
			if freshHostedPreparation(candidate, input, preview) != (scenario == "supported") {
				t.Fatal("fresh hosted preparation eligibility broadened scope or consent")
			}
		})
	}
}

type hostedPreparationBarrierLock struct {
	inner         ports.MutationLock
	write         func() error
	held, entered bool
}

func (lock *hostedPreparationBarrierLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	writerRelease, err := lock.inner.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	writeErr := lock.write()
	if err := errors.Join(writeErr, writerRelease()); err != nil {
		return nil, err
	}
	lock.entered = true
	release, err := lock.inner.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	lock.held = true
	return func() error { lock.held = false; return release() }, nil
}

type hostedPreparationPlanWitness struct {
	inner       ports.DeliveryPlanner
	lock        *hostedPreparationBarrierLock
	lockedCalls int
}

func (w *hostedPreparationPlanWitness) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	if w.lock.held {
		w.lockedCalls++
	}
	return w.inner.Plan(ctx, request)
}

func TestFacadeHostedPreparationRejectsRetainedStateAtMutationLock(t *testing.T) {
	engine, req, runner := hostedPreparationFixture(t)
	installed := applyHostedFixture(t, engine, req)
	req.InstallationID, req.Operation, req.OperationID = installed.InstallationID, OpRemove, "retain-before-race"
	applyHostedFixture(t, engine, req)
	retained, err := engine.store.Load()
	if err != nil || len(retained.Installations) != 1 || len(retained.Installations[0].Clients) != 0 || !retained.Installations[0].DataRetained {
		t.Fatalf("real retained installation missing: %+v, %v", retained, err)
	}
	if err := engine.store.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}); err != nil {
		t.Fatal(err)
	}
	req.Operation, req.OperationID = OpInstall, "fresh-before-race"
	handle, err := engine.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := engine.preflightApply(t.Context(), handle, OpInstall); err != nil {
		t.Fatal(err)
	}
	// The facade's outer checks see no installation. A cooperating writer then
	// publishes real retained receipts under the same lock before core Add locks.
	nativeBefore, dataBefore := deathTree(t, req.ClientConfigRoot), deathTree(t, engine.cfg.PluginDataBase)
	var retainedBytes []byte
	barrier := &hostedPreparationBarrierLock{write: func() error {
		if err := engine.store.Save(retained); err != nil {
			return err
		}
		retainedBytes, err = os.ReadFile(engine.cfg.StateFile)
		return err
	}}
	witness := &hostedPreparationPlanWitness{lock: barrier}
	_, applyErr := engine.applyMutatingPackage(t.Context(), handle, func(svc usecase.Service, input usecase.AddInput) (usecase.AddResult, error) {
		barrier.inner, witness.inner = svc.Lock, engine.planner()
		svc.Lock = barrier
		svc.Planner = confirmationPlanner{inner: witness, prepared: handle, store: svc.StateStore}
		if input.InstallIntent != domain.InstallIntentPrepare {
			t.Fatal("facade lost frozen intent before the lock")
		}
		return svc.Add(t.Context(), input)
	})
	final, loadErr := engine.store.Load()
	finalBytes, readErr := os.ReadFile(engine.cfg.StateFile)
	if !barrier.entered || barrier.held || witness.lockedCalls != 0 || !errors.Is(applyErr, ErrPlanChanged) || loadErr != nil || readErr != nil ||
		!reflect.DeepEqual(final, retained) || len(retainedBytes) == 0 || !reflect.DeepEqual(finalBytes, retainedBytes) || runner.calls != 0 {
		t.Fatalf("locked fresh-only refusal failed: entered=%v held=%v inner=%d err=%v load=%v calls=%d", barrier.entered, barrier.held, witness.lockedCalls, applyErr, loadErr, runner.calls)
	}
	deathAssertTree(t, req.ClientConfigRoot, nativeBefore)
	deathAssertTree(t, engine.cfg.PluginDataBase, dataBefore)
}
