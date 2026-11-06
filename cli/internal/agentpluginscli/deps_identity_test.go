package agentpluginscli

import (
	"errors"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDependencyPinEnvelopeComparison(t *testing.T) {
	envelope := domain.PackageEnvelope{LoaderKind: domain.LoaderKindAgentPlugins, TreeDigest: "tree", ManifestDigest: "manifest", Manifest: domain.PluginManifest{Version: "1.0.0"}, ExecutableFiles: []string{"bin/alpha", "bin/omega"}}
	pin := environment.LockedPackage{TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest, Version: envelope.Manifest.Version, ExecutableFiles: []string{"bin/omega", "bin/alpha"}}
	before := append([]string{}, pin.ExecutableFiles...)
	if err := validateDependencyPinEnvelope(pin, envelope); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pin.ExecutableFiles, before) {
		t.Fatal("comparison rewrote the lock's inventory")
	}
	for _, tc := range []struct {
		name string
		edit func(*environment.LockedPackage)
	}{
		{"tree", func(p *environment.LockedPackage) { p.TreeDigest = "other" }},
		{"manifest", func(p *environment.LockedPackage) { p.ManifestDigest = "other" }},
		{"version", func(p *environment.LockedPackage) { p.Version = "2.0.0" }},
		{"absent inventory", func(p *environment.LockedPackage) { p.ExecutableFiles = nil }},
		{"empty inventory", func(p *environment.LockedPackage) { p.ExecutableFiles = []string{} }},
		{"ghost", func(p *environment.LockedPackage) { p.ExecutableFiles = []string{"bin/alpha", "bin/ghost"} }},
		{"duplicate", func(p *environment.LockedPackage) {
			p.ExecutableFiles = []string{"bin/alpha", "bin/alpha", "bin/omega"}
		}},
		{"unsafe", func(p *environment.LockedPackage) { p.ExecutableFiles = []string{"../outside"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := pin
			tc.edit(&changed)
			if err := validateDependencyPinEnvelope(changed, envelope); !errors.Is(err, errDependencyPinMismatch) {
				t.Fatalf("mismatch lacks typed identity: %v", err)
			}
		})
	}
	pin.Version = ""
	if err := validateDependencyPinEnvelope(pin, envelope); err != nil {
		t.Fatalf("legacy absent version cannot be hydrated: %v", err)
	}
	pin.ExecutableFiles, envelope.ExecutableFiles = nil, []string{}
	if err := validateDependencyPinEnvelope(pin, envelope); err != nil {
		t.Fatalf("empty inventory encodings disagree: %v", err)
	}
}
