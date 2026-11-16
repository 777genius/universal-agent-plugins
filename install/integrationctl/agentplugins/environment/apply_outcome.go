package environment

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// BindingStatus is the terminal state of one planned binding in ApplySync.
type BindingStatus string

const (
	BindingApplied      BindingStatus = "applied"
	BindingUnchanged    BindingStatus = "unchanged"
	BindingFailed       BindingStatus = "failed"
	BindingNotAttempted BindingStatus = "not_attempted"
)

// BindingOutcome is the observed result for exactly one planned binding. It
// keeps the binding next to its result so callers never realign positions.
type BindingOutcome struct {
	Binding PreparedBinding
	Status  BindingStatus
	// Result is set when the installer ran far enough to report one, even when
	// the binding failed or only partly happened.
	Result *installer.Result
	// Err is the cause of a failed binding, or the reason a later binding was
	// not attempted after the whole sync stopped.
	Err error
}

// Ok reports a binding that now matches its plan.
func (outcome BindingOutcome) Ok() bool {
	return outcome.Status == BindingApplied || outcome.Status == BindingUnchanged
}

// Results lists the installer results in plan order, omitting bindings that
// never produced one. It matches what callers received before outcomes existed.
func Results(outcomes []BindingOutcome) []installer.Result {
	results := make([]installer.Result, 0, len(outcomes))
	for _, outcome := range outcomes {
		if outcome.Result != nil {
			results = append(results, *outcome.Result)
		}
	}
	return results
}

// SyncError reports a sync where at least one binding did not succeed. It
// unwraps to every cause so errors.Is and errors.As keep classifying failures.
type SyncError struct {
	Outcomes []BindingOutcome
	// Stopped is set when the sync stopped early instead of reaching every binding.
	Stopped error
	// stoppedAt is the outcome that carries Stopped, so Unwrap lists it once.
	stoppedAt int
}

func (err *SyncError) Error() string {
	failed, notAttempted, total := 0, 0, 0
	var first error
	for _, outcome := range err.Outcomes {
		switch outcome.Status {
		case BindingFailed:
			failed++
			if first == nil {
				first = outcome.Err
			}
		case BindingNotAttempted:
			notAttempted++
		}
		if outcome.Status != BindingUnchanged {
			total++
		}
	}
	if err.Stopped != nil {
		return fmt.Sprintf("dependency sync stopped, %d of %d bindings not attempted: %v", notAttempted, total, err.Stopped)
	}
	if first == nil {
		return fmt.Sprintf("%d of %d bindings failed", failed, total)
	}
	return fmt.Sprintf("%d of %d bindings failed: %v", failed, total, first)
}

func (err *SyncError) Unwrap() []error {
	causes := make([]error, 0, len(err.Outcomes)+1)
	if err.Stopped != nil {
		causes = append(causes, err.Stopped)
	}
	for index, outcome := range err.Outcomes {
		if outcome.Status != BindingFailed || outcome.Err == nil || (err.Stopped != nil && index == err.stoppedAt) {
			continue
		}
		causes = append(causes, outcome.Err)
	}
	return causes
}

// stopError marks a failure that must end the whole sync because continuing
// could act on stale reviewed authority or unsafe state.
type stopError struct{ cause error }

func (err stopError) Error() string { return err.cause.Error() }
func (err stopError) Unwrap() error { return err.cause }

// Stop marks err as ending the whole sync. Host guards that protect reviewed
// authority use it; ordinary client failures must not.
func Stop(err error) error {
	if err == nil {
		return nil
	}
	return stopError{cause: err}
}

// stopsSync separates failures scoped to one binding from those that make later
// bindings unsafe or meaningless. Every Prepare re-reads state and refuses while
// recovery is required, so continuing past a binding failure stays fail-safe.
func stopsSync(ctx context.Context, err error, result *installer.Result) bool {
	if ctx.Err() != nil {
		return true
	}
	if result != nil && (result.Outcome == installer.OutcomeRecovery || result.Outcome == installer.OutcomeCancelled) {
		return true
	}
	var marked stopError
	switch {
	case errors.As(err, &marked), domain.IsUnclosedClientProbe(err),
		errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, installer.ErrCancelled), errors.Is(err, installer.ErrRecoveryRequired),
		errors.Is(err, installer.ErrPlanChanged), errors.Is(err, ErrSourceChange),
		errors.Is(err, installer.ErrStateCorrupt), errors.Is(err, installer.ErrMigrationRequired),
		errors.Is(err, installer.ErrUnsupportedStateSchema), errors.Is(err, installer.ErrInvalidConfig):
		return true
	}
	return false
}
