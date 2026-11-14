package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type resumeObservationActivator struct {
	observedActivator
	available bool
	requests  []domain.ActivationRequest
}

func (a *resumeObservationActivator) VerifierAvailable(domain.DetectedClient, domain.DeliveryPlan, string) bool {
	return a.available
}

func (a *resumeObservationActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.requests = append(a.requests, request)
	return a.observedActivator.Activate(ctx, request)
}

func TestResumeUncheckedAuthenticationNoChangeRequiresVerifiedObservation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		auth           domain.AuthenticationState
		plannedAuth    domain.AuthenticationState
		available      bool
		action         string
		observationErr error
		wantNoChange   bool
	}{
		{name: "verified unchecked", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, wantNoChange: true},
		{name: "no auth action required", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotRequired, available: true, wantNoChange: true},
		{name: "no verifier", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked},
		{name: "pending auth", auth: domain.AuthenticationPending, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "failed auth", auth: domain.AuthenticationFailed, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "new auth requirement", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationPending, available: true},
		{name: "outstanding action", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, action: "authenticate"},
		{name: "unknown observation", auth: domain.AuthenticationNotChecked, plannedAuth: domain.AuthenticationNotChecked, available: true, observationErr: errors.New("client observation unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/unchanged-auth")
			input.Confirmed = true
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(state.Installations[0])
			binding.Activation = domain.ActivationActive
			binding.Verification = domain.VerificationInstalled
			binding.Authentication = tc.auth
			state.Installations[0].Clients[binding.ClientBindingID] = binding
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			outcome := lifecycleOutcome(binding)
			// Client registration success must not attest authentication.
			outcome.Authentication = domain.AuthenticationNotChecked
			if tc.action != "" {
				outcome.UserActions = []string{tc.action}
			}
			activator := &resumeObservationActivator{
				observedActivator: observedActivator{outcome: outcome, err: tc.observationErr},
				available:         tc.available,
			}
			service.Activator = activator
			plan := installed.Plan
			plan.Authentication = tc.plannedAuth
			result, err := service.resume(context.Background(), input, AddResult{Plan: plan}, installed.InstallationID, binding.ClientBindingID, binding)
			if !errors.Is(err, tc.observationErr) || result.NoChange != tc.wantNoChange || result.Mutated {
				t.Fatalf("resume: %+v %v", result, err)
			}
			if len(activator.requests) != 1 || !activator.requests[0].VerifyOnly {
				t.Fatalf("resume requests: %+v", activator.requests)
			}
			if result.Activation.Authentication != tc.auth || result.Activation.AuthenticationAttested {
				t.Fatalf("authentication escalated: %+v", result.Activation)
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(state, after) {
				t.Fatalf("unchanged observation wrote state: %v", err)
			}
		})
	}
}

type antigravityObservationRunner struct {
	active   string
	version  string
	disabled bool
	absent   bool
	failed   bool
	calls    int
}

func (runner *antigravityObservationRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls++
	body := []byte("1.5.3\n")
	if len(command.Argv) == 4 {
		var err error
		body, err = json.Marshal([]any{map[string]any{"name": filepath.Base(runner.active), "path": runner.active,
			"version": runner.version, "enabled": !runner.disabled, "skills": []string{}, "mcpServers": []string{}}})
		if err != nil {
			return legacyports.CommandResult{}, err
		}
		if runner.absent {
			body = []byte("[]")
		}
	}
	return legacyports.CommandResult{CleanupComplete: !runner.failed, Stdout: body}, nil
}

type antigravityObservationFixture struct {
	service   Service
	store     transaction.StateStore
	input     AddInput
	runner    *antigravityObservationRunner
	installed AddResult
}

func antigravityReadFixture(t *testing.T) antigravityObservationFixture {
	t.Helper()
	service, store, client := serviceFixture(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	client.ClientID = domain.ClientAntigravity
	client.ConfigRoot = filepath.Join(home, ".gemini", "config")
	client.NativeConfigRoot = client.ConfigRoot
	client.ExecutablePath = filepath.Join(home, "agy")
	if err = os.WriteFile(client.ExecutablePath, []byte("inert; no native process"), 0700); err != nil {
		t.Fatal(err)
	}
	input := addInput(t, client, "https://example.com/antigravity-observation")
	input.Confirmed, input.BackendExecutable = true, client.ExecutablePath
	runner := &antigravityObservationRunner{version: "1.0.0"}
	service.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	installed, err := service.Add(context.Background(), input)
	if err != nil || installed.Activation.Activation != domain.ActivationPrepared || runner.calls != 0 {
		t.Fatal("initial file delivery required native bootstrap", installed, err, runner.calls)
	}
	config, err := json.Marshal(map[string]any{"plugins": map[string]any{filepath.Base(installed.Plan.ActivePath): map[string]any{"enabled": true}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(client.ConfigRoot, "config.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	runner.active = installed.Plan.ActivePath
	return antigravityObservationFixture{service: service, store: store, input: input, runner: runner, installed: installed}
}

func TestAntigravityRealProviderResumeReobservesHistoricalActiveAndPreservesReceipts(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(map[bool]string{false: "observed", true: "unavailable"}[failure], func(t *testing.T) {
			fixture := antigravityReadFixture(t)
			before, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(before.Installations[0])
			binding.Activation = domain.ActivationActive
			binding.Authentication = domain.AuthenticationPending
			before.Installations[0].Clients[binding.ClientBindingID] = binding
			if err = fixture.store.Save(before); err != nil {
				t.Fatal(err)
			}
			fixture.runner.failed = failure
			result, err := fixture.service.Add(context.Background(), fixture.input)
			if (err != nil) != failure || result.Activation.Activation == domain.ActivationActive || result.Activation.AuthenticationAttested {
				t.Fatal("historical Active became false native evidence", result, err)
			}
			after, loadErr := fixture.store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			updated := onlyBinding(after.Installations[0])
			if !reflect.DeepEqual(binding.Receipts, updated.Receipts) || !reflect.DeepEqual(binding.PackageRevision, updated.PackageRevision) ||
				!reflect.DeepEqual(before.TransactionReceipts, after.TransactionReceipts) || updated.Authentication != domain.AuthenticationPending {
				t.Fatal("read-only resume rewrote package/authentication ownership", after)
			}
			want := 2
			if failure {
				want = 1
			}
			if fixture.runner.calls != want {
				t.Fatal("wrong metadata command count", fixture.runner.calls)
			}
		})
	}
}

func TestAntigravityRealProviderUpdateAndAbsentBundleRepairRetainPrepared(t *testing.T) {
	fixture := antigravityReadFixture(t)
	repeated, err := fixture.service.Add(context.Background(), fixture.input)
	if err != nil || repeated.Activation.Activation != domain.ActivationPrepared {
		t.Fatal(repeated, err)
	}
	fixture.input.OperationID = "antigravity-update"
	setEnvelopeVersion(t, &fixture.input.Envelope, "2.0.0", "sha256:source-two", "sha256:manifest-two")
	fixture.runner.version = "2.0.0"
	updated, err := fixture.service.Update(context.Background(), fixture.input)
	if err != nil || updated.Activation.Activation != domain.ActivationPrepared {
		t.Fatal(updated, err)
	}
	if err = os.RemoveAll(updated.Plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	fixture.input.OperationID = "antigravity-repair"
	repaired, err := fixture.service.Repair(context.Background(), fixture.input)
	if err != nil || repaired.Activation.Activation != domain.ActivationPrepared || repaired.Activation.ActivationAttested {
		t.Fatal("repair promoted runtime activation", repaired, err)
	}
}

func TestAntigravityRealProviderDryRepairRemainsFileOnly(t *testing.T) {
	fixture := antigravityReadFixture(t)
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	fixture.input.DryRun, fixture.input.Confirmed = true, false
	fixture.input.OperationID = "antigravity-dry-repair"
	result, err := fixture.service.Repair(t.Context(), fixture.input)
	if err != nil || fixture.runner.calls != 0 || result.Activation.Activation != domain.ActivationPrepared {
		t.Fatal("dry repair launched native inventory or lost Prepared materialization", result, err, fixture.runner.calls)
	}
	after, err := fixture.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("dry repair changed recorded ownership", err)
	}
}

func TestAntigravityRealProviderRepeatRefusesForeignManifestBeforeNativeCommand(t *testing.T) {
	fixture := antigravityReadFixture(t)
	fixture.service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: fixture.service.Stager, Runner: fixture.runner})
	foreign := filepath.Join(fixture.installed.Plan.TargetRoot, "foreign-same-name")
	if err := os.MkdirAll(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(foreign, "plugin.json")
	body := []byte(`{"$schema":"https://antigravity.google/schemas/v1/plugin.json","name":"demo","version":"1.0.0"}`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyBinding(before.Installations[0])
	observation, err := fixture.service.NativeObserver.ObserveNativeIdentity(t.Context(), fixture.input.Client, fixture.installed.Plan, &binding)
	if err != nil || observation.State != domain.NativeIdentityUnmanaged || fixture.runner.calls != 0 {
		t.Fatal("fixture must prove an existing filesystem collision", observation, err, fixture.runner.calls)
	}
	result, err := fixture.service.Add(t.Context(), fixture.input)
	if err == nil || fixture.runner.calls != 0 || result.Activation.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatal("known collision reached native commands", result, err, fixture.runner.calls)
	}
	after, loadErr := fixture.store.Load()
	current, readErr := os.ReadFile(path)
	if loadErr != nil || readErr != nil || string(current) != string(body) || !reflect.DeepEqual(before, after) {
		t.Fatal("collision refusal changed foreign bytes or recorded ownership", loadErr, readErr)
	}
}

func TestAntigravityPreparedIntactRepairObservesOnlyConfirmedQualifiedUser(t *testing.T) {
	for _, condition := range []string{"confirmed", "preview", "unconfirmed", "missing-provenance", "foreign-provenance", "missing-config", "cleanup-unproved"} {
		t.Run(condition, func(t *testing.T) {
			fixture := antigravityReadFixture(t)
			fixture.input.Scope = domain.ScopeUser
			fixture.input.OperationID = "antigravity-intact-repair"
			wantCalls, wantError := 0, false
			switch condition {
			case "confirmed":
				wantCalls = 2
			case "preview":
				fixture.input.DryRun = true
			case "unconfirmed":
				fixture.input.Confirmed = false
			case "missing-provenance":
				fixture.input.Client.NativeConfigRoot = ""
			case "foreign-provenance":
				fixture.input.Client.NativeConfigRoot = filepath.Join(t.TempDir(), ".gemini", "config")
			case "missing-config":
				wantCalls = 2
				if err := os.Remove(filepath.Join(fixture.input.Client.ConfigRoot, "config.json")); err != nil {
					t.Fatal(err)
				}
			case "cleanup-unproved":
				fixture.runner.failed, wantCalls, wantError = true, 1, true
			}
			before, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			result, err := fixture.service.Repair(t.Context(), fixture.input)
			if (err != nil) != wantError || fixture.runner.calls != wantCalls || result.Mutated ||
				result.Activation.Activation == domain.ActivationActive || result.Activation.ActivationAttested || result.Activation.AuthenticationAttested {
				t.Fatal("intact repair crossed consent, original-profile or cleanup boundary", result, err, fixture.runner.calls)
			}
			if (condition == "confirmed" || condition == "missing-config") && (result.Activation.Activation != domain.ActivationPrepared ||
				!reflect.DeepEqual(result.Activation.UserActions, []string{"The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading"})) {
				t.Fatal("confirmed intact repair skipped the real scoped verifier", result)
			}
			after, err := fixture.store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("read-only intact repair changed recorded state or receipts", err)
			}
		})
	}
}

func TestAntigravityMissingConfigResumeAndRepairPreserveOriginalAuthority(t *testing.T) {
	for _, operation := range []string{"resume", "repair"} {
		for _, native := range []string{"enabled", "disabled", "absent"} {
			t.Run(operation+"/"+native, func(t *testing.T) {
				fixture := antigravityReadFixture(t)
				fixture.input.Scope = domain.ScopeUser
				fixture.input.OperationID = "antigravity-missing-config-" + operation
				fixture.runner.disabled, fixture.runner.absent = native == "disabled", native == "absent"
				config := filepath.Join(fixture.input.Client.ConfigRoot, "config.json")
				if err := os.Remove(config); err != nil {
					t.Fatal(err)
				}
				before, err := fixture.store.Load()
				if err != nil {
					t.Fatal(err)
				}
				parent, err := os.Lstat(fixture.input.Client.ConfigRoot)
				if err != nil {
					t.Fatal(err)
				}
				manifest := filepath.Join(fixture.installed.Plan.ActivePath, "plugin.json")
				packageBefore, err := os.ReadFile(manifest)
				if err != nil {
					t.Fatal(err)
				}
				var result AddResult
				if operation == "repair" {
					result, err = fixture.service.Repair(t.Context(), fixture.input)
				} else {
					result, err = fixture.service.Add(t.Context(), fixture.input)
				}
				guidance := map[string]string{
					"enabled":  "The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading",
					"disabled": "The selected User plugin remains disabled; decide whether to enable it in Antigravity",
					"absent":   "The exact UAP-owned plugin is absent from native User inventory; inspect it in Antigravity",
				}[native]
				if err != nil || fixture.runner.calls != 2 || result.Mutated ||
					result.Activation.Activation != domain.ActivationPrepared || result.Activation.Verification != domain.VerificationInstalled ||
					result.Activation.ActivationAttested || result.Activation.AuthenticationAttested ||
					!reflect.DeepEqual(result.Activation.UserActions, []string{guidance}) {
					t.Fatal("real provider lost native missing-config evidence or escalated qualification", result, err, fixture.runner.calls)
				}
				after, loadErr := fixture.store.Load()
				packageAfter, readErr := os.ReadFile(manifest)
				current, statErr := os.Lstat(fixture.input.Client.ConfigRoot)
				if loadErr != nil || readErr != nil || statErr != nil || !reflect.DeepEqual(before, after) ||
					string(packageBefore) != string(packageAfter) || !os.SameFile(parent, current) ||
					parent.Mode() != current.Mode() || parent.Size() != current.Size() || !parent.ModTime().Equal(current.ModTime()) {
					t.Fatal("read-only observation changed owner, binding, source, DATA receipts or original parent", loadErr, readErr, statErr)
				}
				if _, err := os.Lstat(config); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("provider observation created or adopted a missing config", err)
				}
			})
		}
	}
}
