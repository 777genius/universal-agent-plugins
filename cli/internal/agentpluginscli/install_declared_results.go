package agentpluginscli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Successful declared repairs precede normal convergence. Retain only their
// typed presentation so the command emits one JSON result for both phases.
// These snapshots never authorize effects or replace installer state.
type declaredRepairPresentation struct {
	actions  []dependencyActionDTO
	results  []dependencyResultDTO
	consumed bool
}

func retainDeclaredRepairPresentation(opts *options, actions []dependencyAction, outcomes []environment.BindingOutcome) {
	if opts.declaredRepairs == nil {
		opts.declaredRepairs = &declaredRepairPresentation{}
	}
	pending := opts.declaredRepairs
	pending.actions = append(pending.actions, dependencyActionOutput(actions, opts.dependencyComponents)...)
	pending.results = append(pending.results, dependencyBindingResultOutput(outcomes)...)
}

// Consume before writing: even a partial write must never trigger a second
// document or a later attempt to flush these same committed repair facts.
func consumeDeclaredRepairPresentation(opts *options) *declaredRepairPresentation {
	pending := opts.declaredRepairs
	if pending == nil || pending.consumed {
		return nil
	}
	pending.consumed = true
	return pending
}

// An error after repair can occur before the ordinary sync renderer. Complete
// that command's single envelope here, retaining committed repair evidence and
// the original typed cause. A renderer that already attempted output wins.
func finishDeclaredRepairPresentation(cmd *cobra.Command, opts *options, cause error) error {
	pending := opts.declaredRepairs
	if pending == nil {
		return cause
	}
	if !pending.consumed {
		renderErr := renderDependencyResultOutput(cmd, opts, nil, nil, cause)
		cause = errors.Join(cause, renderErr)
	}
	if cause != nil {
		return &declaredInstallResultError{cause: cause}
	}
	return nil
}

// The process entrypoint must not append its early-refusal envelope after this
// command has already attempted a result document, including a partial write.
type declaredInstallResultError struct{ cause error }

func (*declaredInstallResultError) Error() string {
	return "Declared installation did not complete after repairing installed bindings."
}

func (failure *declaredInstallResultError) Unwrap() error { return failure.cause }
