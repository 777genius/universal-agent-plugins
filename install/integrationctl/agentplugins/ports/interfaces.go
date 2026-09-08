package ports

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type VerificationKind string

const (
	VerificationAbsent         VerificationKind = "absent"
	VerificationDigestMismatch VerificationKind = "digest_mismatch"
	VerificationIndeterminate  VerificationKind = "indeterminate"
	VerificationExcludedMarker VerificationKind = "excluded_ownership_marker"
)

// VerificationError classifies integrity failures without converting an
// infrastructure error into evidence that a managed directory drifted.
type VerificationError struct {
	Kind         VerificationKind
	ActualDigest string
	Err          error
}

func (err *VerificationError) Error() string {
	if err.Err != nil {
		return err.Err.Error()
	}
	return fmt.Sprintf("package verification failed: %s", err.Kind)
}

func (err *VerificationError) Unwrap() error { return err.Err }

type SchemaRegistry interface {
	Supports(schemaURI string) bool
	Validate(schemaURI string, value any) error
	Digest(schemaURI string) (string, bool)
}

type PackageLoader interface {
	Load(context.Context, domain.LoadInput) (domain.PackageEnvelope, error)
}

// PackageSecurityEvaluator inspects the already acquired immutable snapshot.
// It must complete before any client or managed package file is changed.
type PackageSecurityEvaluator interface {
	Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error)
}

type ClientDetector interface {
	Detect(context.Context) ([]domain.DetectedClient, error)
}

// VersionProbingClientDetector is an opt-in capability for mutating lifecycle
// resolution that must bind signed evidence to an exact installed version.
type VersionProbingClientDetector interface {
	DetectWithVersionProbe(context.Context) ([]domain.DetectedClient, error)
}

// TargetedVersionProbingClientDetector observes versions only for explicitly
// selected clients while retaining read-only surface detection for all clients.
type TargetedVersionProbingClientDetector interface {
	DetectTargetsWithVersionProbe(context.Context, []domain.ClientID) ([]domain.DetectedClient, error)
}

type DeliveryPlanner interface {
	Plan(context.Context, domain.PackageEnvelope, domain.DetectedClient, domain.InstallScope, string) (domain.DeliveryPlan, error)
}

type DeliveryTargetResolver interface {
	ResolveTarget(context.Context, domain.DetectedClient, domain.InstallScope, string) (domain.DeliveryTarget, error)
}

type PackageStager interface {
	Stage(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints) (domain.StagedDelivery, error)
	Discard(context.Context, domain.StagedDelivery) error
	Verify(context.Context, string, string) error
}

type ClientActivator interface {
	Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error)
	Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error)
}
