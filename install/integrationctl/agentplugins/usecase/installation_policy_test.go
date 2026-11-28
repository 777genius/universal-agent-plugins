package usecase

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type installationPolicyFunc func(context.Context, domain.InstallPolicyRequest) (string, error)

func (f installationPolicyFunc) Check(ctx context.Context, request domain.InstallPolicyRequest) (string, error) {
	return f(ctx, request)
}

type policyHostPreparation struct {
	calls  int
	change func()
}

func (h *policyHostPreparation) PrepareOpenCodeHost(_ context.Context, client domain.DetectedClient, _ domain.PackageEnvelope) (domain.DetectedClient, error) {
	h.calls++
	if h.change != nil {
		h.change()
	}
	return client, nil
}

func (*policyHostPreparation) RevalidateOpenCodeHost(context.Context, domain.DetectedClient) error {
	return nil
}

type policyStager struct {
	ports.PackageStager
	calls int
}

func (s *policyStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, id string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	s.calls++
	return s.PackageStager.Stage(ctx, envelope, plan, id, hints)
}

type policyMutationLock struct {
	ports.MutationLock
	calls, releases int
	change          func()
}

func (l *policyMutationLock) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	l.calls++
	release, err := l.MutationLock.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	if l.change != nil {
		l.change()
	}
	return func() error { l.releases++; return release() }, nil
}

type policyChangingPlanner struct {
	ports.DeliveryPlanner
	change func()
}

func (p policyChangingPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	plan, err := p.DeliveryPlanner.Plan(ctx, request)
	p.change()
	return plan, err
}

func policyDenied() error {
	return &domain.InstallPolicyError{Violations: []domain.InstallPolicyViolation{{Code: "install.allowed_targets", Message: "target denied", Subject: "fixture"}}}
}

func TestInstallationPolicyRejectsEverySingleMutationBeforeHostOrLock(t *testing.T) {
	for _, operation := range []string{"add", "update", "repair", "refresh"} {
		t.Run(operation, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/policy")
			input.Confirmed = true
			if _, err := service.Add(t.Context(), input); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			host := &policyHostPreparation{}
			lock := &policyMutationLock{MutationLock: service.Lock}
			stager := &policyStager{PackageStager: service.Stager}
			service.OpenCodeHosts, service.Lock, service.Stager = host, lock, stager
			denied := policyDenied()
			service.InstallationPolicy = installationPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
				if request.Envelope == nil || request.Envelope.Manifest.Name != "demo" || request.Source.CanonicalSource != input.Envelope.Source.CanonicalSource || request.Scope != input.Scope || !reflect.DeepEqual(request.Targets, []domain.ClientID{client.ClientID}) {
					t.Fatalf("incomplete policy facts: %+v", request)
				}
				if request.GitPinned != nil || request.ArchivePinned != nil {
					t.Fatal("resolved revision invented requested pin facts")
				}
				return "", denied
			})
			var result AddResult
			switch operation {
			case "add":
				result, err = service.Add(t.Context(), input)
			case "update":
				result, err = service.Update(t.Context(), input)
			case "repair":
				result, err = service.Repair(t.Context(), input)
			case "refresh":
				result, err = service.RefreshProjection(t.Context(), input)
			}
			if !errors.Is(err, denied) || result.Mutated || host.calls != 0 || lock.calls != 0 || stager.calls != 0 {
				t.Fatalf("denial crossed effects: result=%+v err=%v host=%d lock=%d stage=%d", result, err, host.calls, lock.calls, stager.calls)
			}
			after, err := os.ReadFile(store.Path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("denial changed installed receipts: %v", err)
			}
		})
	}
}

func TestInstallationPolicyAdmitsWholeGroupBeforePreparingAnyHost(t *testing.T) {
	for _, compatibility := range []bool{false, true} {
		t.Run(map[bool]string{false: "targets", true: "compatibility"}[compatibility], func(t *testing.T) {
			service, _, client := serviceFixture(t)
			first := addInput(t, client, "https://example.com/policy-group")
			second := first
			second.Client = domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			host := &policyHostPreparation{}
			service.OpenCodeHosts = host
			denied := policyDenied()
			service.InstallationPolicy = installationPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
				if request.Targets[0] == second.Client.ClientID {
					return "", denied
				}
				return "reviewed", nil
			})
			group := GroupInput{Targets: []AddInput{first, second}, Confirmed: true, OperationGroupID: "policy-group"}
			if compatibility {
				group.Targets, group.CompatibilityChecks = []AddInput{first}, []AddInput{second}
			}
			result, err := service.AddGroup(t.Context(), group)
			if !errors.Is(err, denied) || host.calls != 0 || result.Mutated {
				t.Fatalf("later target denial was not atomic: result=%+v err=%v host=%d", result, err, host.calls)
			}
		})
	}
}

func TestInstallationPolicyChangesStopBeforeRecoveryOrStaging(t *testing.T) {
	for _, boundary := range []string{"host", "lock", "plan"} {
		t.Run(boundary, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/policy-changed")
			input.Confirmed = true
			fingerprint := "reviewed"
			service.InstallationPolicy = installationPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return fingerprint, nil })
			change := func() { fingerprint = "new-rules-still-allowing-this-package" }
			lock := &policyMutationLock{MutationLock: service.Lock}
			stager := &policyStager{PackageStager: service.Stager}
			service.Lock, service.Stager = lock, stager
			switch boundary {
			case "host":
				service.OpenCodeHosts = &policyHostPreparation{change: change}
			case "lock":
				lock.change = change
			case "plan":
				service.Planner = policyChangingPlanner{DeliveryPlanner: service.Planner, change: change}
			}
			result, err := service.Add(t.Context(), input)
			if !errors.Is(err, domain.ErrInstallPolicyChanged) || result.Mutated || stager.calls != 0 || lock.calls != lock.releases {
				t.Fatalf("stale review applied: result=%+v err=%v stage=%d lock=%+v", result, err, stager.calls, lock)
			}
			if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
				t.Fatalf("changed policy published state: %v", err)
			}
		})
	}
}

func TestInstallationPolicyRejectsMixedGroupSnapshotsBeforeHosts(t *testing.T) {
	service, store, client := serviceFixture(t)
	first := addInput(t, client, "https://example.com/policy-group-change")
	second := first
	second.Client = domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
	host := &policyHostPreparation{}
	service.OpenCodeHosts = host
	service.InstallationPolicy = installationPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
		if request.Targets[0] == second.Client.ClientID {
			return "changed-but-allowing", nil
		}
		return "reviewed", nil
	})
	result, err := service.AddGroup(t.Context(), GroupInput{Targets: []AddInput{first, second}, Confirmed: true, OperationGroupID: "policy-group-change"})
	if !errors.Is(err, domain.ErrInstallPolicyChanged) || result.Mutated || host.calls != 0 {
		t.Fatalf("mixed policy snapshots reached preparation: result=%+v err=%v host=%d", result, err, host.calls)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatalf("mixed policy snapshots published state: %v", err)
	}
}

func TestInstallationPolicyChecksAllUpdateTargetsBeforeProjectionRefresh(t *testing.T) {
	f := newGroupProjectionFixture(t, "opencode_v1")
	f.inputs[1].Client.OpenCodeHost = groupProjectionHost{dialect: "opencode_v2"}
	state, config := f.state(t), f.config(t)
	denied := policyDenied()
	f.service.InstallationPolicy = installationPolicyFunc(func(_ context.Context, request domain.InstallPolicyRequest) (string, error) {
		if request.Targets[0] == domain.ClientCursor {
			return "", denied
		}
		return "reviewed", nil
	})
	result, err := f.service.UpdateGroup(t.Context(), GroupInput{
		Targets: []AddInput{f.inputs[1], f.inputs[0]}, Confirmed: true, OperationGroupID: "policy-refresh",
	})
	if !errors.Is(err, denied) || result.Mutated || !reflect.DeepEqual(state, f.state(t)) || !bytes.Equal(config, f.config(t)) {
		t.Fatalf("earlier projection changed before later target denial: result=%+v err=%v", result, err)
	}
}

func TestInstallationPolicyBlocksRetainedProvenanceButAllowsRemoval(t *testing.T) {
	denied := policyDenied()
	gate := installationPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) { return "", denied })
	for _, mode := range []string{"rebind", "migrate", "switch"} {
		t.Run(mode, func(t *testing.T) {
			f := newRetainedRevisionFixture(t, "direct")
			before := f.snapshot(t)
			f.service.InstallationPolicy = gate
			input := BindingChangeInput{Selector: f.input.InstallationID, Envelope: f.input.Envelope, Confirmed: true}
			var result BindingChangeResult
			var err error
			switch mode {
			case "rebind":
				result, err = f.service.Rebind(t.Context(), input)
			case "migrate":
				result, err = f.service.MigrateFormat(t.Context(), input)
			case "switch":
				result, err = f.service.SwitchRetained(t.Context(), input, domain.OriginModeDirect, nil)
			}
			if !errors.Is(err, denied) || result.Mutated || !reflect.DeepEqual(before, f.snapshot(t)) {
				t.Fatalf("retained provenance bypassed policy: result=%+v err=%v", result, err)
			}
		})
	}
	f := newRetainedRevisionFixture(t, "active")
	f.service.InstallationPolicy = installationPolicyFunc(func(context.Context, domain.InstallPolicyRequest) (string, error) {
		t.Fatal("removal consulted installation policy")
		return "", errors.New("invalid configured policy")
	})
	f.remove(t)
}
