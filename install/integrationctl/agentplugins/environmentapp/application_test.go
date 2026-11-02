package environmentapp_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

var errInterrupted = errors.New("injected interruption")

type applicationHost struct {
	files                     map[string][]byte
	trace                     []string
	readyCalls, failReady     int
	lockErr, materializeErr   error
	writeFailure              string
	writeThenFail             bool
	writes, prepares, applies int
	materialized              environment.Lock
	afterAcquire, afterEnsure func()
	afterPrepare, afterApply  func()
	failApply                 int
	current                   environment.PreparedBinding
}

func (host *applicationHost) RequireMutationReady() error {
	host.trace = append(host.trace, "ready")
	host.readyCalls++
	if host.readyCalls == host.failReady {
		return errInterrupted
	}
	return nil
}
func (host *applicationHost) Acquire(context.Context) (func() error, error) {
	host.trace = append(host.trace, "acquire")
	if host.lockErr != nil {
		return nil, host.lockErr
	}
	if host.afterAcquire != nil {
		host.afterAcquire()
	}
	return func() error { host.trace = append(host.trace, "release"); return nil }, nil
}
func (host *applicationHost) Ensure(_ context.Context, lock environment.Lock) error {
	host.trace = append(host.trace, "materialize")
	host.materialized = lock
	if host.afterEnsure != nil {
		host.afterEnsure()
	}
	return host.materializeErr
}
func (host *applicationHost) Read(path string) ([]byte, error) {
	body, exists := host.files[path]
	if !exists {
		return nil, fs.ErrNotExist
	}
	return bytes.Clone(body), nil
}
func (host *applicationHost) Write(path string, body []byte) error {
	host.trace = append(host.trace, "write "+path)
	host.writes++
	if path == host.writeFailure && !host.writeThenFail {
		return errInterrupted
	}
	host.files[path] = bytes.Clone(body)
	if path == host.writeFailure {
		return errInterrupted
	}
	return nil
}
func (host *applicationHost) Prepare(_ context.Context, action environment.PreparedBinding) (*installer.PreparedOperation, error) {
	host.trace = append(host.trace, "prepare "+action.Alias)
	host.prepares++
	host.current = action
	if host.afterPrepare != nil {
		host.afterPrepare()
	}
	return &installer.PreparedOperation{}, nil
}
func (host *applicationHost) Apply(context.Context, *installer.PreparedOperation) (installer.Result, error) {
	host.trace = append(host.trace, "apply "+host.current.Alias)
	host.applies++
	if host.afterApply != nil {
		host.afterApply()
	}
	if host.applies == host.failApply {
		return installer.Result{Outcome: installer.OutcomeIncomplete, Mutated: true, Reason: "native interruption"}, errInterrupted
	}
	return installer.Result{Outcome: installer.OutcomeCompleted, Mutated: true}, nil
}

func applicationFixture() (*applicationHost, environmentapp.Service, environmentapp.Change) {
	host := &applicationHost{files: map[string][]byte{"manifest": []byte("before manifest"), "lock": []byte("before lock")}}
	intent := &environmentapp.Intent{ManifestPath: "manifest", LockPath: "lock",
		ManifestBytes: []byte("before manifest"), LockBytes: []byte("before lock"),
		CandidateManifest: []byte("after manifest"), CandidateLock: []byte("after lock")}
	change := environmentapp.Change{Intent: intent,
		Plan: environment.PreparedSync{EnvironmentID: "user:default", Bindings: []environment.PreparedBinding{
			{Alias: "a", ClientID: "codex", Action: environment.ActionInstall},
			{Alias: "intact", ClientID: "codex", Action: environment.ActionUnchanged},
			{Alias: "b", ClientID: "claude", Action: environment.ActionInstall},
		}},
		Lock:    environment.Lock{LockVersion: 1, ManifestDigest: "retained digest", Packages: []environment.LockedPackage{{Alias: "a"}, {Alias: "b"}, {Alias: "unselected"}}},
		Aliases: []string{"b", "a"},
		Recheck: func(context.Context) error {
			host.trace = append(host.trace, "review")
			return intent.Check(host)
		},
	}
	return host, environmentapp.Service{Readiness: host, Lock: host, Packages: host, Files: host, Mutator: host}, change
}

func TestApplyRetainsPublicationAndBindingGuardOrdering(t *testing.T) {
	host, service, change := applicationFixture()
	guards := 0
	change.CheckInputs = func() error {
		guards++
		return change.Intent.Check(host)
	}
	result, err := service.Apply(t.Context(), change)
	if err != nil || !result.Published || len(result.Results) != 2 || guards != 5 {
		t.Fatalf("application lost advanced preimages/custom guard: result=%+v guards=%d err=%v", result, guards, err)
	}
	want := []string{"ready", "acquire", "ready", "review", "materialize", "write manifest", "write lock", "prepare a", "apply a", "prepare b", "apply b", "release"}
	if !reflect.DeepEqual(host.trace, want) {
		t.Fatalf("effects escaped application ordering: %v", host.trace)
	}
	if !reflect.DeepEqual(host.materialized.Packages, change.Lock.Packages[:2]) || host.materialized.ManifestDigest != change.Lock.ManifestDigest || len(change.Lock.Packages) != 3 {
		t.Fatalf("materialization changed graph metadata or included an unselected pin: %+v", host.materialized)
	}
	if err := change.Intent.Check(host); err != nil || change.Intent.Pending() {
		t.Fatalf("successful publication did not advance retained intent: %v", err)
	}
}

func TestApplyRefusalsNeverPublishOrApply(t *testing.T) {
	for _, name := range []string{"initial readiness", "lock", "canceled after acquire", "locked readiness", "stale review", "materialization", "canceled after materialization", "changed inputs", "custom guard"} {
		t.Run(name, func(t *testing.T) {
			host, service, change := applicationFixture()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			wantErr := errInterrupted
			switch name {
			case "initial readiness":
				host.failReady = 1
			case "lock":
				host.lockErr = errInterrupted
			case "canceled after acquire":
				host.afterAcquire = cancel
				wantErr = context.Canceled
			case "locked readiness":
				host.failReady = 2
			case "stale review":
				change.Recheck = func(context.Context) error { return installer.ErrPlanChanged }
				wantErr = installer.ErrPlanChanged
			case "materialization":
				host.materializeErr = errInterrupted
			case "canceled after materialization":
				host.afterEnsure = cancel
				wantErr = context.Canceled
			case "changed inputs":
				host.afterEnsure = func() { host.files["lock"] = []byte("externally changed") }
				wantErr = installer.ErrPlanChanged
			case "custom guard":
				change.CheckInputs = func() error { return errInterrupted }
			}
			result, err := service.Apply(ctx, change)
			if !errors.Is(err, wantErr) || result.Published || host.writes != 0 || host.prepares != 0 || host.applies != 0 {
				t.Fatalf("refusal escaped before-publication gate: result=%+v trace=%v err=%v", result, host.trace, err)
			}
			acquired := name != "initial readiness" && name != "lock"
			released := host.trace[len(host.trace)-1] == "release"
			if acquired != released {
				t.Fatalf("environment lock was not released: %v", host.trace)
			}
		})
	}
}

func TestApplyPublicationFailureReportsObservedDurableIntent(t *testing.T) {
	for _, tc := range []struct {
		name, path       string
		after, published bool
	}{
		{"manifest refused", "manifest", false, false},
		{"manifest written then error", "manifest", true, true},
		{"lock refused", "lock", false, true},
		{"lock written then error", "lock", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, service, change := applicationFixture()
			host.writeFailure, host.writeThenFail = tc.path, tc.after
			result, err := service.Apply(t.Context(), change)
			if !errors.Is(err, errInterrupted) || result.Published != tc.published || host.prepares != 0 || host.applies != 0 {
				t.Fatalf("publication failure lost actual effects: result=%+v trace=%v err=%v", result, host.trace, err)
			}
			if tc.path == "lock" && (!strings.Contains(err.Error(), "uap.toml saved, uap.lock not written") || !strings.Contains(err.Error(), "run agentplugins deps lock")) {
				t.Fatalf("partial publication lost existing recovery guidance: %v", err)
			}
			wantManifest := "before manifest"
			if tc.path == "lock" {
				wantManifest = "after manifest"
			}
			if string(change.Intent.ManifestBytes) != wantManifest || string(change.Intent.LockBytes) != "before lock" {
				t.Fatal("failed write incorrectly advanced a retained preimage", change.Intent)
			}
			if host.trace[len(host.trace)-1] != "release" {
				t.Fatal("publication failure retained environment lock")
			}
		})
	}
}

func TestApplyChecksInputsBeforeEachPrepareAndApply(t *testing.T) {
	for _, during := range []string{"prepare", "apply"} {
		t.Run(during, func(t *testing.T) {
			host, service, change := applicationFixture()
			if during == "prepare" {
				host.afterPrepare = func() { host.files["manifest"] = []byte("changed after prepare") }
			} else {
				host.afterApply = func() { host.files["lock"] = []byte("changed after apply") }
			}
			result, err := service.Apply(t.Context(), change)
			wantApplies := 0
			if during == "apply" {
				wantApplies = 1
			}
			if !errors.Is(err, installer.ErrPlanChanged) || !result.Published || host.prepares != 1 || host.applies != wantApplies {
				t.Fatalf("stale binding crossed input guard: result=%+v trace=%v err=%v", result, host.trace, err)
			}
			if during == "apply" && (len(result.Results) != 1 || result.Results[0].Outcome != installer.OutcomeCompleted) {
				t.Fatal("later stale-input refusal lost completed binding result", result)
			}
		})
	}
}

func TestApplyRepairAndUnchangedIntent(t *testing.T) {
	host, service, change := applicationFixture()
	change.RepairOnly = true
	host.materializeErr = errInterrupted
	change.Intent.CandidateManifest, change.Intent.CandidateLock = nil, nil
	result, err := service.Apply(t.Context(), change)
	if err != nil || result.Published || host.writes != 0 || host.materialized.LockVersion != 0 || len(result.Results) != 2 {
		t.Fatalf("repair changed intent or materialized a new graph: result=%+v trace=%v err=%v", result, host.trace, err)
	}
}

func TestApplyPreservesPartialInstallerResults(t *testing.T) {
	host, service, change := applicationFixture()
	host.failApply = 2
	change.Plan.Bindings = append(change.Plan.Bindings, environment.PreparedBinding{Alias: "never attempted", ClientID: "codex", Action: environment.ActionInstall})
	result, err := service.Apply(t.Context(), change)
	if !errors.Is(err, errInterrupted) || !result.Published || len(result.Results) != 2 || result.Results[0].Outcome != installer.OutcomeCompleted || result.Results[1].Outcome != installer.OutcomeIncomplete || host.prepares != 2 {
		t.Fatalf("binding loop lost partial result or continued after failure: result=%+v trace=%v err=%v", result, host.trace, err)
	}
}

func TestApplyRequiresRetainedReview(t *testing.T) {
	host, service, change := applicationFixture()
	change.Recheck = nil
	if _, err := service.Apply(t.Context(), change); err == nil || len(host.trace) != 0 {
		t.Fatal("missing retained review reached application effects", host.trace, err)
	}
}
