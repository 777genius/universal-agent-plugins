package domain

// ExitCode is returned to the shell by plugin-kit-ai install.
type ExitCode int

const (
	ExitSuccess   ExitCode = 0
	ExitUsage     ExitCode = 1
	ExitRelease   ExitCode = 2 // no release, tag, or matching asset
	ExitNetwork   ExitCode = 3
	ExitChecksum  ExitCode = 4
	ExitFS        ExitCode = 5
	ExitAmbiguous ExitCode = 6 // multiple tar.gz or binaries in archive
)

// Error carries a stable exit code for the CLI.
type Error struct {
	Code    ExitCode
	Message string
}

func (e *Error) Error() string { return e.Message }

func NewError(code ExitCode, msg string) *Error {
	return &Error{Code: code, Message: msg}
}
