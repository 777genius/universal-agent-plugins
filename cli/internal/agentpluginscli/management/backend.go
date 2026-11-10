package management

import (
	"context"
	"errors"
)

// Backend is the application boundary. Implementations retain installer authority
// in Go; none of these wire DTOs are sufficient authority for native effects.
type Backend interface {
	Session(context.Context) (Session, error)
	Catalog(context.Context, string) (Catalog, error)
	CatalogDetail(context.Context, string) (CatalogEntry, error)
	SkillsCatalog(context.Context, SourceRequest) (SkillCatalog, error)
	Inspect(context.Context) (Environment, error)
	CheckUpdates(context.Context, UpdateCheckRequest) (UpdateCheck, error)
	Prepare(context.Context, ChangeRequest, string) (Prepared, error)
}

type Prepared interface {
	Review() Plan
	Check(context.Context) error
	Apply(context.Context, func(Progress)) (Outcome, error)
	OperationIDs() []string
	Close()
}

type Outcome struct {
	RecoverySettled        bool
	ReconciledOperationIDs []string
	State                  OperationState
	Result                 OperationResult
	Diagnostics            []Diagnostic
}

var ErrPlanStale = errors.New("installer facts changed; prepare a new review")

// Refusal is application-selected public guidance. Raw provider errors, paths
// containing credentials, or subprocess output must not enter this message.
type Refusal struct {
	Code      string
	Message   string
	Retryable bool
	Status    int
}

func (r *Refusal) Error() string { return r.Message }
