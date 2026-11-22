package sourceacquisition

import (
	"fmt"
	"strings"
)

// gitTransportExitError marks a nonzero Git result after process supervision
// itself succeeded. A generic process ExitCode cannot establish that fact.
type gitTransportExitError struct {
	cause error
	code  int
	// connect marks a read that failed only to reach its remote.
	connect bool
}

func (err *gitTransportExitError) Error() string             { return err.cause.Error() }
func (err *gitTransportExitError) Unwrap() error             { return err.cause }
func (err *gitTransportExitError) GitTransportExitCode() int { return err.code }
func (err *gitTransportExitError) Is(target error) bool {
	return target == ErrGitConnect && err.connect
}

// gitExitResult is the error of a supervised Git process that exited with
// code; it is connect-class when stdout and stderr show only that.
func gitExitResult(command Command, code int, stdout, stderr []byte) error {
	failure := gitTransportExitFailure(command, code, append(append([]byte(nil), stdout...), stderr...))
	failure.connect = isGitConnectFailure(code, stdout, stderr)
	return failure
}

func gitTransportExitFailure(command Command, code int, output []byte) *gitTransportExitError {
	var cause error
	if command.NativeCredentials {
		cause = fmt.Errorf("native Git authentication or transport failed (exit %d); configure Git credentials or SSH access and retry", code)
	} else {
		cause = fmt.Errorf("git %s failed with exit code %d: %s", strings.Join(command.Args, " "), code, strings.TrimSpace(string(output)))
	}
	return &gitTransportExitError{cause: cause, code: code}
}
