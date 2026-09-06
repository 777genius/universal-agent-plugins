// Package nativeconfig applies ownership-aware MCP server patches to native
// JSON and JSONC client configuration files.
package nativeconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Codec string

const (
	// CodecMCPServers is the explicit generic mcpServers profile: stdio uses
	// command/args/env/cwd and remote uses url/headers with no transport tag.
	// Clients with different native transport keys require their own codec.
	CodecMCPServers Codec = "mcpServers"
	CodecGemini     Codec = "gemini-mcpServers"
	CodecOpenCode   Codec = "opencode-mcp"
	CodecWindsurf   Codec = "windsurf-mcpServers"
	CodecCline      Codec = "cline-mcpServers"
)

type Action string

const (
	ActionAdd    Action = "add"
	ActionUpdate Action = "update"
	ActionRemove Action = "remove"
)

var (
	ErrAmbiguousConfig  = errors.New("both JSON and JSONC native config files exist")
	ErrCollision        = errors.New("native MCP entry already exists")
	ErrNotOwned         = errors.New("native MCP entry is not exactly owned")
	ErrMalformed        = errors.New("native config is malformed")
	ErrConcurrentChange = errors.New("native config changed during patch")
)

// CommittedCleanupError reports that the requested native config bytes and
// receipts were committed, but releasing the cooperating writer lock did not
// complete cleanly. Callers must keep the returned receipts and must not roll
// back other state that was committed in the same provider transaction. The
// wrapped error remains available so the cleanup degradation is observable.
type CommittedCleanupError struct{ Err error }

func (err *CommittedCleanupError) Error() string {
	return "native config committed but lock cleanup degraded: " + err.Err.Error()
}

func (err *CommittedCleanupError) Unwrap() error { return err.Err }

func IsCommittedCleanup(err error) bool {
	var committed *CommittedCleanupError
	return errors.As(err, &committed)
}

// Server is the transport-aware neutral MCP shape accepted by the supported
// codecs. Type must be "stdio" or "remote". Portable placeholders are resolved
// once in raw args, env values, and cwd. RemoteTransport is codec-specific and is
// rejected unless the selected codec explicitly defines it.
type Server struct {
	// Trusted adapters set these only after portable expansion. They are not
	// native fields or persisted payload flags; projection readers restore them.
	StdioValuesResolved bool              `json:"-"`
	CWDResolved         bool              `json:"-"`
	Type                string            `json:"type"`
	Command             string            `json:"command,omitempty"`
	Args                []string          `json:"args,omitempty"`
	Env                 map[string]string `json:"env,omitempty"`
	CWD                 string            `json:"cwd,omitempty"`
	URL                 string            `json:"url,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	// RemoteTransport distinguishes Gemini's streamable HTTP and legacy SSE
	// native keys. It is ignored by codecs whose native shape uses one URL key.
	RemoteTransport string `json:"remote_transport,omitempty"`
}

// Paths names the mutually exclusive client config variants. If neither
// exists, JSON is created. The paths must be explicit and must not use HOME.
type Paths struct {
	JSON  string
	JSONC string
}

type Placeholders struct {
	PackageRoot string
	DataRoot    string
}

// Receipt identifies one exact projected entry. Ownership intentionally covers
// the complete entry: if another writer adds even one foreign field inside the
// entry, the receipt becomes stale and update/remove fail closed. It is safe to
// persist because it contains no original file bytes or unrelated config.
type Receipt struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Codec   Codec  `json:"codec"`
	Name    string `json:"name"`
	Digest  string `json:"digest"`
}

type Request struct {
	Paths        Paths
	Codec        Codec
	Action       Action
	Name         string
	Server       Server
	Placeholders Placeholders
	Owned        *Receipt
	// Desired optionally binds add/update to the exact projected receipt the
	// caller staged. ApplyBatch compares it before any config write, including
	// the selected JSON/JSONC path, so selection drift cannot commit first and
	// fail only during a provider postcondition.
	Desired *Receipt
}

// FileIO abstracts exact no-follow reads and atomic replacement. WriteAtomic
// preserves the requested file mode in the default implementation; platform
// metadata beyond the mode (for example ACLs and extended attributes) is not
// guaranteed to survive replacement.
type FileIO interface {
	ReadNoFollow(path string) (body []byte, mode os.FileMode, exists bool, err error)
	WriteAtomic(path string, body []byte, mode os.FileMode) error
	RemoveNoFollow(path string) error
}

// conditionalFileIO moves the final precondition check inside the mutation
// boundary. This prevents test seams and cooperating implementations from
// inserting work between validation and replacement. Ordinary filesystems do
// not expose a portable linearizable content-CAS, so external clients that do
// not honor our locks can still win the final syscall-sized race; post-write
// verification remains mandatory. Rollback repeats the same immediate
// precondition check, but it has the same residual syscall-sized limitation.
type conditionalFileIO interface {
	CompareAndSwap(path string, expected []byte, expectedExists bool, body []byte, mode os.FileMode) error
	RemoveIfUnchanged(path string, expected []byte) error
}

type lockAcquirer func(Paths, Codec) (func() error, error)

type Kernel struct {
	files        FileIO
	acquireLocks lockAcquirer
}

func New() Kernel { return Kernel{files: conditionalOSFiles{}} }

func NewWithFileIO(files FileIO) Kernel { return Kernel{files: files} }

func NewWithLockAcquirer(acquireLocks func(Paths, Codec) (func() error, error)) Kernel {
	return Kernel{files: conditionalOSFiles{}, acquireLocks: acquireLocks}
}

func validateRequest(req Request) error {
	if err := validateExactPath(req.Paths.JSON, "JSON native config path"); err != nil {
		return err
	}
	if req.Paths.JSONC != "" {
		if err := validateExactPath(req.Paths.JSONC, "JSONC native config path"); err != nil {
			return err
		}
	}
	if req.Paths.JSONC != "" && filepath.Clean(req.Paths.JSON) == filepath.Clean(req.Paths.JSONC) {
		return fmt.Errorf("JSON and JSONC native config paths must differ")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("MCP entry name is required")
	}
	if !supportedCodec(req.Codec) {
		return fmt.Errorf("unsupported native config codec %q", req.Codec)
	}
	if req.Action != ActionAdd && req.Action != ActionUpdate && req.Action != ActionRemove {
		return fmt.Errorf("unsupported native config action %q", req.Action)
	}
	if req.Action != ActionAdd && req.Owned == nil {
		return fmt.Errorf("%s requires an ownership receipt: %w", req.Action, ErrNotOwned)
	}
	if req.Action == ActionRemove && req.Desired != nil {
		return fmt.Errorf("remove must not declare a desired receipt")
	}
	return nil
}

func supportedCodec(codec Codec) bool {
	return codec == CodecMCPServers || codec == CodecGemini || codec == CodecOpenCode || codec == CodecWindsurf || codec == CodecCline
}

func codecCollectionKey(codec Codec) string {
	if codec == CodecOpenCode {
		return "mcp"
	}
	return "mcpServers"
}

func validateExactPath(path, label string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%s is required", label)
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be absolute", label)
	}
	if filepath.Clean(path) != path {
		return fmt.Errorf("%s must be clean", label)
	}
	return nil
}
