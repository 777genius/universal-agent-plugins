package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type projectRepairObserver struct {
	observation      domain.NativeIdentityObservation
	err              error
	prepared, native int
}

func (o *projectRepairObserver) ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	o.native++
	return domain.NativeIdentityObservation{}, errors.New("native discovery forbidden")
}
func (o *projectRepairObserver) ObservePreparedIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	o.prepared++
	return o.observation, o.err
}

type projectRepairNativeOnly struct{}

func (projectRepairNativeOnly) ObserveNativeIdentity(context.Context, domain.DetectedClient, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return domain.NativeIdentityObservation{}, errors.New("native discovery forbidden")
}

func TestProjectRepairAdmissionMatchesIndependentManagedEvidence(t *testing.T) {
	const owned = "sha256:owned"
	intact := domain.NativeIdentityObservation{State: domain.NativeIdentityManaged, Digest: owned, ReceiptReconciled: true, NativeDiscoveryState: domain.NativeIdentityAbsent}
	absent := domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent, NativeDiscoveryState: domain.NativeIdentityAbsent}
	drift := domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, Digest: "sha256:drift", NativeDiscoveryState: domain.NativeIdentityAbsent}
	for _, tc := range []struct {
		name        string
		verify      error
		observation domain.NativeIdentityObservation
		observeErr  error
		change      func(*domain.NativeIdentityObservation)
		want        bool
	}{
		{name: "intact", observation: intact, want: true},
		{name: "known-absent", verify: &ports.VerificationError{Kind: ports.VerificationAbsent}, observation: absent, want: true},
		{name: "known-drift", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:drift"}, observation: drift, want: true},
		{name: "unreconciled-receipt", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.ReceiptReconciled = false }},
		{name: "missing-receipt-digest", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.Digest = "" }},
		{name: "different-receipt-digest", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.Digest = "sha256:other" }},
		{name: "discovery-attempt", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.NativeDiscoveryAttempted = true }},
		{name: "discovery-reconciliation", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.NativeDiscoveryReconciled = true }},
		{name: "package-only-fallback", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.NativeDiscoveryState = "" }},
		{name: "unknown-state", observation: intact, change: func(o *domain.NativeIdentityObservation) { o.State = "unknown" }},
		{name: "foreign-local", observation: domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged, NativeDiscoveryState: domain.NativeIdentityAbsent}},
		{name: "observation-error", observation: intact, observeErr: errors.New("read refused")},
		{name: "unknown-integrity-error", verify: errors.New("permission denied"), observation: drift},
		{name: "changed-between-measurements", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:first"}, observation: drift},
		{name: "mismatch-without-actual", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch}, observation: drift},
		{name: "mismatch-without-observed", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:drift"}, observation: drift, change: func(o *domain.NativeIdentityObservation) { o.Digest = "" }},
		{name: "mismatch-contradictory-receipt", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:drift"}, observation: drift, change: func(o *domain.NativeIdentityObservation) { o.ReceiptReconciled = true }},
		{name: "absent-with-digest", verify: &ports.VerificationError{Kind: ports.VerificationAbsent}, observation: absent, change: func(o *domain.NativeIdentityObservation) { o.Digest = owned }},
		{name: "absent-with-foreign-local", verify: &ports.VerificationError{Kind: ports.VerificationAbsent}, observation: domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged, NativeDiscoveryState: domain.NativeIdentityAbsent}},
		{name: "drift-with-unknown-local", verify: &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: "sha256:drift"}, observation: domain.NativeIdentityObservation{State: domain.NativeIdentityIndeterminate, NativeDiscoveryState: domain.NativeIdentityAbsent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observer := &projectRepairObserver{observation: tc.observation, err: tc.observeErr}
			if tc.change != nil {
				tc.change(&observer.observation)
			}
			session := repairSession{ctx: t.Context(), service: Service{NativeObserver: observer}, input: AddInput{Scope: domain.ScopeProject, Client: domain.DetectedClient{ClientID: domain.ClientCline}}, client: domain.ClientBinding{NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: owned}}}, verifyErr: tc.verify}
			err := session.verifyProjectRepairOwnership()
			if (err == nil) != tc.want || observer.prepared != 1 || observer.native != 0 {
				t.Fatalf("admission=%v want=%t prepared=%d native=%d", err, tc.want, observer.prepared, observer.native)
			}
		})
	}
}
func TestProjectRepairAdmissionRequiresPreparedCapability(t *testing.T) {
	for _, kind := range []string{"nil", "native-only", "profile-wrapped-fallback"} {
		t.Run(kind, func(t *testing.T) {
			session := repairSession{ctx: t.Context(), input: AddInput{Scope: domain.ScopeProject, Client: domain.DetectedClient{ClientID: domain.ClientCline}}, verifyErr: &ports.VerificationError{Kind: ports.VerificationAbsent}}
			switch kind {
			case "native-only":
				session.service.NativeObserver = projectRepairNativeOnly{}
			case "profile-wrapped-fallback":
				session.service.NativeObserver = profileObserver{NativeIdentityObserver: projectRepairNativeOnly{}, check: func() error { return nil }, fallback: func(context.Context, domain.DeliveryPlan, *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
					return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
				}}
			}
			if err := session.verifyProjectRepairOwnership(); err == nil {
				t.Fatal("package-only compatibility fallback admitted project component repair")
			}
		})
	}
}
