package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

type PlatformProfile struct {
	Platform        runtime.PlatformID
	Status          runtime.SupportStatus
	PublicPackage   string
	InternalPackage string
	InternalImport  string
	TransportModes  []runtime.TransportMode
	LiveTestProfile string
	// RuntimeOnly excludes optional observers from authoring/install qualification.
	RuntimeOnly bool
	Scaffold    ScaffoldMeta
	Validate    ValidateMeta
}

type EventDescriptor struct {
	Platform           runtime.PlatformID
	Event              runtime.EventID
	Invocation         InvocationBinding
	Carrier            runtime.CarrierKind
	Contract           ContractMeta
	DecodeFunc         string
	EncodeFunc         string
	Registrar          RegistrarMeta
	Docs               DocsMeta
	Capabilities       []runtime.CapabilityID
	CapabilityMappings []CapabilityMapping
}

type ContractMeta struct {
	Maturity runtime.MaturityLevel
	V1Target bool
}

type InvocationBinding struct {
	Kind        runtime.InvocationKind
	Name        string
	ResolverRef string
}

type RegistrarMeta struct {
	MethodName   string
	EventType    string
	ResponseType string
	WrapFunc     string
}

type CapabilityMapping struct {
	Unified  string
	Platform runtime.CapabilityID
}

type TemplateFile struct {
	Path     string
	Template string
	Extra    bool
}

type ScaffoldMeta struct {
	RequiredFiles  []string
	OptionalFiles  []string
	ForbiddenFiles []string
	TemplateFiles  []TemplateFile
}

type ValidateMeta struct {
	RequiredFiles  []string
	ForbiddenFiles []string
	BuildTargets   []string
}

type DocsMeta struct {
	SnippetKey string
	TableGroup string
	Summary    string
}
