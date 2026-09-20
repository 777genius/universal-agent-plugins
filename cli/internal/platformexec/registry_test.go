package platformexec

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type testAdapter struct {
	id string
}

func (a testAdapter) ID() string                                             { return a.id }
func (a testAdapter) DetectNative(string) bool                               { return false }
func (a testAdapter) RefineDiscovery(string, *pluginmodel.TargetState) error { return nil }
func (a testAdapter) Import(string, ImportSeed) (ImportResult, error)        { return ImportResult{}, nil }
func (a testAdapter) Generate(string, pluginmodel.PackageGraph, pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	return nil, nil
}
func (a testAdapter) ManagedPaths(string, pluginmodel.PackageGraph, pluginmodel.TargetState) ([]string, error) {
	return nil, nil
}
func (a testAdapter) Validate(string, pluginmodel.PackageGraph, pluginmodel.TargetState) ([]Diagnostic, error) {
	return nil, nil
}

func TestDefaultRegistry_CoversExecutableProfiles(t *testing.T) {
	if err := ValidateCoverage(); err != nil {
		t.Fatalf("ValidateCoverage error = %v", err)
	}
}

func TestRegistry_RejectsDuplicateRegistration(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(testAdapter{id: "codex-runtime"}); err != nil {
		t.Fatalf("first Register error = %v", err)
	}
	err := registry.Register(testAdapter{id: "codex-runtime"})
	if err == nil || !strings.Contains(err.Error(), "duplicate registration") {
		t.Fatalf("second Register error = %v", err)
	}
}
