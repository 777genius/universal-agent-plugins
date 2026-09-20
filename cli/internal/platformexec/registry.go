package platformexec

import (
	"fmt"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/sdk/platformmeta"
)

type DiagnosticSeverity string

const (
	SeverityFailure DiagnosticSeverity = "failure"
	SeverityWarning DiagnosticSeverity = "warning"
)

const (
	CodeManifestInvalid          = "manifest_invalid"
	CodeGeneratedContractInvalid = "generated_contract_invalid"
	CodeEntrypointMismatch       = "entrypoint_mismatch"
	CodeGeminiDirNameMismatch    = "gemini_dir_name_mismatch"
	CodeGeminiMCPCommandStyle    = "gemini_mcp_command_style"
	CodeGeminiPolicyIgnored      = "gemini_policy_ignored"
)

type Diagnostic struct {
	Severity DiagnosticSeverity
	Code     string
	Path     string
	Target   string
	Message  string
}

type ImportSeed struct {
	Manifest         pluginmodel.Manifest
	Launcher         *pluginmodel.Launcher
	Explicit         bool
	IncludeUserScope bool
}

type ImportResult struct {
	Manifest     pluginmodel.Manifest
	Launcher     *pluginmodel.Launcher
	Artifacts    []pluginmodel.Artifact
	Warnings     []pluginmodel.Warning
	DroppedKinds []string
}

type Adapter interface {
	ID() string
	DetectNative(root string) bool
	RefineDiscovery(root string, state *pluginmodel.TargetState) error
	Import(root string, seed ImportSeed) (ImportResult, error)
	Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error)
	ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error)
	Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error)
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry() *Registry {
	return &Registry{adapters: map[string]Adapter{}}
}

func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("platform adapter: nil adapter")
	}
	id := strings.ToLower(strings.TrimSpace(adapter.ID()))
	if id == "" {
		return fmt.Errorf("platform adapter: empty id")
	}
	if _, ok := platformmeta.Lookup(id); !ok {
		return fmt.Errorf("platform adapter %q: missing platformmeta profile", id)
	}
	if _, exists := r.adapters[id]; exists {
		return fmt.Errorf("platform adapter %q: duplicate registration", id)
	}
	r.adapters[id] = adapter
	return nil
}

func (r *Registry) Lookup(id string) (Adapter, bool) {
	adapter, ok := r.adapters[strings.ToLower(strings.TrimSpace(id))]
	return adapter, ok
}

func (r *Registry) DetectImport(root string) []Adapter {
	var matches []Adapter
	for _, id := range sortedKeys(r.adapters) {
		adapter := r.adapters[id]
		if adapter.DetectNative(root) {
			matches = append(matches, adapter)
		}
	}
	return matches
}

func (r *Registry) ValidateCoverage() error {
	for _, profile := range platformmeta.All() {
		if !profile.Contract.ImportSupport && !profile.Contract.GenerateSupport && !profile.Contract.ValidateSupport {
			continue
		}
		if _, ok := r.adapters[profile.ID]; !ok {
			return fmt.Errorf("platform adapter %q: missing registration for executable profile", profile.ID)
		}
	}
	return nil
}

var defaultRegistry = mustDefaultRegistry()

func DefaultRegistry() *Registry {
	return defaultRegistry
}

func Lookup(id string) (Adapter, bool) {
	return DefaultRegistry().Lookup(id)
}

func DetectImport(root string) []Adapter {
	return DefaultRegistry().DetectImport(root)
}

func ValidateCoverage() error {
	return DefaultRegistry().ValidateCoverage()
}

func mustDefaultRegistry() *Registry {
	registry := NewRegistry()
	for _, adapter := range []Adapter{
		claudeAdapter{},
		codexPackageAdapter{},
		codexRuntimeAdapter{},
		cursorAdapter{},
		cursorWorkspaceAdapter{},
		geminiAdapter{},
		opencodeAdapter{},
	} {
		if err := registry.Register(adapter); err != nil {
			panic(err)
		}
	}
	if err := registry.ValidateCoverage(); err != nil {
		panic(err)
	}
	return registry
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}
