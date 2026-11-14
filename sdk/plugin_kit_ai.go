package pluginkitai

import (
	"os"
	"sync"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/gen"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime/process"
)

// IO aliases the runtime I/O contract used by the SDK app host.
type IO = runtime.IO

// Env aliases the runtime environment reader used by invocation resolution.
type Env = runtime.Env

// Logger aliases the structured logger interface accepted by the SDK app host.
type Logger = runtime.Logger

// Middleware aliases the SDK middleware function signature.
type Middleware = runtime.Middleware

// Next aliases the middleware continuation function.
type Next = runtime.Next

// Handled aliases the typed handler result container.
type Handled = runtime.Handled

// InvocationContext aliases the metadata that accompanies a decoded invocation.
type InvocationContext = runtime.InvocationContext

// Result aliases the low-level runtime result written back to the host process.
type Result = runtime.Result

// NopLogger aliases the logger implementation that drops all log records.
type NopLogger = runtime.NopLogger

// CapabilityID aliases the normalized cross-platform capability identifier.
type CapabilityID = runtime.CapabilityID

// SupportStatus aliases the support-level enum used by generated support entries.
type SupportStatus = runtime.SupportStatus

// MaturityLevel aliases the API maturity enum exposed by support metadata.
type MaturityLevel = runtime.MaturityLevel

// TransportMode aliases the runtime transport mode enum for supported hooks.
type TransportMode = runtime.TransportMode

// SupportEntry aliases a generated public support-matrix row.
type SupportEntry = runtime.SupportEntry

// MaxPayloadBytes is the single wire limit for stdin and argv JSON payloads
// accepted by runtime decoders. Consumers should reference this constant
// instead of duplicating the number.
const MaxPayloadBytes = runtime.MaxPayloadBytes

// Config configures a root SDK app instance before handlers are registered.
type Config struct {
	// Name is the human-readable app label used in diagnostics and examples.
	Name string
	// Args overrides the process argv used to resolve the current invocation.
	Args []string
	// IO overrides the stdin/stdout/stderr implementation used by Run.
	IO IO
	// Env overrides environment lookups used during invocation resolution.
	Env Env
	// Logger overrides structured logging emitted by the runtime engine.
	Logger Logger
}

// App owns middleware, handler registration, and invocation dispatch.
type App struct {
	mu      sync.Mutex
	runDone bool
	name    string
	args    []string
	io      runtime.IO
	env     runtime.Env
	logger  runtime.Logger

	handlers *runtime.HandlerRegistry
	mws      []runtime.Middleware
	custom   *customRegistry
}

// New builds an App with sane defaults for argv, process I/O, env, and logging.
func New(cfg Config) *App {
	args := cfg.Args
	if len(args) == 0 {
		args = os.Args
	}
	io := cfg.IO
	if io == nil {
		io = process.IO{}
	}
	env := cfg.Env
	if env == nil {
		env = process.Env{}
	}
	logger := cfg.Logger
	if logger == nil {
		logger = runtime.NopLogger{}
	}
	return &App{
		name:     cfg.Name,
		args:     append([]string(nil), args...),
		io:       io,
		env:      env,
		logger:   logger,
		handlers: runtime.NewHandlerRegistry(),
		custom:   newCustomRegistry(),
	}
}

// Supported returns a copy of the generated public support matrix entries.
func Supported() []SupportEntry {
	entries := gen.AllSupportEntries()
	out := make([]SupportEntry, len(entries))
	copy(out, entries)
	return out
}
