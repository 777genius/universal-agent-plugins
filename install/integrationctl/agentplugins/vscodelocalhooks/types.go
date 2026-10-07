// Package vscodelocalhooks renders fixed executable invocations for VS Code's
// Local hooks. It performs no IO, expansion, consent, or client activation.
package vscodelocalhooks

import "errors"

// PluginPath is the native Local namespace, relative to the plugin root.
// Portable package projection and namespace preservation belong to the caller.
const PluginPath = "com.github.copilot/hooks/hooks.json"

type Event string

const (
	Stop         Event = "Stop"
	SubagentStop Event = "SubagentStop"
)

type Shell string

const (
	// LinuxSH targets Node shell:true on Linux: /bin/sh -c, irrespective of SHELL.
	LinuxSH Shell = "linux-/bin/sh"
	// MacOSSH targets Node shell:true on macOS: /bin/sh -c, irrespective of SHELL.
	MacOSSH Shell = "macos-/bin/sh"
	// WindowsPowerShell51 prepares the source-pinned ComSpec=system cmd.exe
	// branch. Native Windows execution remains a separate qualification gate.
	WindowsPowerShell51 Shell = "windows-system-powershell-5.1"
)

// Target is a trusted snapshot supplied by the host, never read from ambient
// environment here. ComSpec and SystemRoot must be empty for LinuxSH/MacOSSH. Windows
// requires an absolute SystemRoot and its exact System32\cmd.exe ComSpec.
// No pwsh, cmd script, alternate ComSpec or default-shell fallback is supported.
type Target struct {
	Shell      Shell
	ComSpec    string
	SystemRoot string
}

// Spec contains only fixed, already projected literals. Executable must be a
// trusted absolute executable path; Args exclude argv[0]. Native input, secrets,
// shell programs and unresolved root/data placeholders must not be passed here.
// TimeoutSeconds must be 5; zero does not request an implicit default.
type Spec struct {
	Event          Event
	Executable     string
	Args           []string
	TimeoutSeconds int
}

// Bounds keep authored commands and ownership verification small. These are
// library limits, not statements about the native parser's broader acceptance.
const (
	TimeoutSeconds   = 5
	MaxArgs          = 64
	MaxLiteralBytes  = 2048
	MaxCommandBytes  = 16 * 1024
	MaxDocumentBytes = 64 * 1024
)

var (
	ErrInvalid     = errors.New("invalid Local hook input")
	ErrUnsupported = errors.New("unsupported Local hook representation")
	ErrNotOwned    = errors.New("local hook file differs from fixed authored entries")
)
