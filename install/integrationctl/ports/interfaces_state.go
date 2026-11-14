package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

type ResolvedSource struct {
	Kind      string
	Requested domain.RequestedSourceRef
	Resolved  domain.ResolvedSourceRef
	LocalPath string
	// Structured provenance fields let standard-first consumers preserve a
	// stable source identity while tracking the immutable resolved revision.
	CanonicalSource  string
	Repository       string
	PackageSubpath   string
	ResolvedRevision string
	// Cleanup is an opaque, resolver-owned capability. Callers must never
	// construct cleanup paths from package or persisted state data.
	Cleanup         func() error
	SourceDigest    string
	ExecutableFiles []string
	ImportRoots     []string
	FailureClass    string
}

type SourceResolver interface {
	Resolve(context.Context, domain.IntegrationRef) (ResolvedSource, error)
}

type ManifestLoader interface {
	Load(context.Context, ResolvedSource) (domain.IntegrationManifest, error)
}

type StateFile struct {
	SchemaVersion int                         `json:"schema_version"`
	Installations []domain.InstallationRecord `json:"installations"`
}

type StateStore interface {
	Load(context.Context) (StateFile, error)
	Save(context.Context, StateFile) error
}

type WorkspaceLockStore interface {
	Load(context.Context) (domain.WorkspaceLock, error)
	Save(context.Context, domain.WorkspaceLock) error
	Path() string
}

type UnlockFunc func() error

type LockManager interface {
	Acquire(context.Context, string) (UnlockFunc, error)
}

type OperationJournal interface {
	Start(context.Context, domain.OperationRecord) error
	AppendStep(context.Context, string, domain.JournalStep) error
	Finish(context.Context, string, string) error
	ListOpen(context.Context) ([]domain.OperationRecord, error)
}

type EvidenceEntry struct {
	Key           string   `json:"key"`
	Claim         string   `json:"claim"`
	EvidenceClass string   `json:"evidence_class"`
	URLs          []string `json:"urls"`
}

type EvidenceRegistry interface {
	Get(context.Context, string) (EvidenceEntry, error)
	List(context.Context) ([]EvidenceEntry, error)
}
