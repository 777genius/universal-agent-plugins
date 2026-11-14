package runtime

import "context"

type PlatformID string
type EventID string
type CapabilityID string
type CarrierKind string
type SupportStatus string
type MaturityLevel string
type TransportMode string
type InvocationKind string

const (
	CarrierStdinJSON CarrierKind = "stdin_json"
	CarrierArgvJSON  CarrierKind = "argv_json"
)

const (
	StatusRuntimeSupported SupportStatus = "runtime_supported"
	StatusScaffoldOnly     SupportStatus = "scaffold_only"
	StatusDeferred         SupportStatus = "deferred"
)

const (
	MaturityStable       MaturityLevel = "stable"
	MaturityBeta         MaturityLevel = "beta"
	MaturityExperimental MaturityLevel = "experimental"
)

const (
	ProcessMode TransportMode = "process"
	HybridMode  TransportMode = "hybrid"
	DaemonMode  TransportMode = "daemon"
)

const (
	InvocationArgvCommand         InvocationKind = "argv_command"
	InvocationArgvCommandCaseFold InvocationKind = "argv_command_casefold"
	InvocationArgvPayload         InvocationKind = "argv_payload"
	InvocationEnvMarker           InvocationKind = "env_marker"
	InvocationCustomResolver      InvocationKind = "custom_resolver"
)

type IO interface {
	ReadStdin(ctx context.Context) ([]byte, error)
	WriteStdout([]byte) error
	WriteStderr(string) error
}

type Env interface {
	LookupEnv(string) (string, bool)
}

type Logger interface {
	Info(string)
	Error(string)
}

type NopLogger struct{}

func (NopLogger) Info(string)  {}
func (NopLogger) Error(string) {}

type Invocation struct {
	Platform PlatformID
	Event    EventID
	RawName  string
}

type Envelope struct {
	Invocation Invocation
	Args       []string
	Stdin      []byte
	Env        Env
}

type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   string
}

type SupportEntry struct {
	Platform        PlatformID
	Event           EventID
	Status          SupportStatus
	Maturity        MaturityLevel
	V1Target        bool
	InvocationKind  InvocationKind
	Carrier         CarrierKind
	TransportModes  []TransportMode
	ScaffoldSupport bool
	ValidateSupport bool
	Capabilities    []CapabilityID
	Summary         string
	LiveTestProfile string
}

type Descriptor struct {
	Platform PlatformID
	Event    EventID
	Carrier  CarrierKind
	Decode   func(Envelope) (any, string, error)
	Encode   func(any) Result
}

type InvocationContext struct {
	Context    context.Context
	Invocation Invocation
	Descriptor Descriptor
	Env        Env
	Logger     Logger
}

type Handled struct {
	Value any
	Err   error
}

type TypedHandler func(InvocationContext, any) Handled

type Next func(InvocationContext) Handled
type Middleware func(Next) Next

type Resolver func(args []string, env Env) (Invocation, error)
type DescriptorLookup func(platform PlatformID, event EventID) (Descriptor, bool)
type EnvelopeBuilder func(ctx context.Context, inv Invocation, carrier CarrierKind, args []string, io IO, env Env) (Envelope, error)

type RegistrarBackend interface {
	Register(platform PlatformID, event EventID, handler TypedHandler)
	RegisterCustom(rawName string, desc Descriptor, handler TypedHandler) error
}
