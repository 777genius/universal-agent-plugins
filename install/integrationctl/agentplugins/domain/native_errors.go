package domain

import "errors"

// ErrManagedSkillChanged identifies an existing owned Skill directory whose
// current bytes no longer match its receipt. Recovery must preserve those bytes
// until the owner explicitly resolves the conflict.
var ErrManagedSkillChanged = errors.New("managed skill changed outside agentplugins")

// ErrForeignSkillDestination identifies an existing project Skill target with
// no matching UAP ownership. It must be left untouched.
var ErrForeignSkillDestination = errors.New("foreign project Skill destination")

// ErrManagedPackageChanged identifies an owned plugin package or native
// identity whose bytes or ownership no longer match its receipt. It stops the
// whole operation until the owner repairs or reviews it.
var ErrManagedPackageChanged = errors.New("managed package changed outside agentplugins")

// ErrForeignNativeIdentity identifies a native plugin identity at a target that
// agentplugins does not own. It must be left untouched.
var ErrForeignNativeIdentity = errors.New("foreign native identity at the install location")

// ErrTargetUnsupported identifies a selected target that cannot install the
// package.
var ErrTargetUnsupported = errors.New("target cannot install the package")

// TypedError keeps an existing message unchanged and adds a classifiable
// sentinel for callers that use errors.Is.
type TypedError struct {
	Text  string
	Kind  error
	Cause error
}

func (err *TypedError) Error() string { return err.Text }
func (err *TypedError) Unwrap() []error {
	if err.Cause == nil {
		return []error{err.Kind}
	}
	return []error{err.Kind, err.Cause}
}

// ForeignSkillDestinationError carries a project-relative path from a verified
// native project layout. Error() never exposes paths or provider diagnostics.
type ForeignSkillDestinationError struct {
	RelativePath string
}

func (*ForeignSkillDestinationError) Error() string { return ErrForeignSkillDestination.Error() }
func (*ForeignSkillDestinationError) Unwrap() error { return ErrForeignSkillDestination }
