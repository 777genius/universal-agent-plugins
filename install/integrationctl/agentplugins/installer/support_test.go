package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A missing executable must refuse CLI/probe authority before effects, while
// native config clients must not acquire an artificial process dependency.
func TestLocalClientSupportAndExecutableAuthority(t *testing.T) {
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engine, err := New(Config{StateRoot: filepath.Join(root, "state"), HelperExecutable: helper, Registry: all.Default(), TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		id         string
		executable bool
	}{
		{"codex", true}, {"claude", true}, {"cursor", true}, {"copilot", true}, {"kiro", true}, {"grok", true}, {"opencode", true},
		{"gemini", false}, {"cline", false}, {"windsurf", false}, {"kimi", false},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			if !engine.SupportsClient(test.id) || !ClientSupported(all.Default(), test.id) {
				t.Fatal("local adapter excluded")
			}
			request := Request{Operation: OpInstall, ClientID: test.id, ClientConfigRoot: filepath.Join(root, test.id)}
			_, err := engine.detectedClient(request)
			if test.executable && !errors.Is(err, ErrInvalidRequest) || !test.executable && err != nil {
				t.Fatalf("missing executable: %v", err)
			}
			facts := []TargetFacts{{ClientID: test.id, BindingID: "binding", ConfigRoot: request.ClientConfigRoot}}
			_, err = engine.knownTargetIndex(facts)
			if test.executable && !errors.Is(err, ErrTargetFactsUnavailable) || !test.executable && err != nil {
				t.Fatalf("known target missing executable: %v", err)
			}
			for _, operation := range []Operation{OpInstall, OpUpdate, OpRepair, OpRemove} {
				request.Operation, request.ClientExecutable = operation, "relative/probe"
				if _, err := engine.detectedClient(request); !errors.Is(err, ErrInvalidRequest) {
					t.Fatalf("relative executable accepted for %s: %v", operation, err)
				}
			}
			facts[0].Executable = "relative/probe"
			if _, err := engine.knownTargetIndex(facts); !errors.Is(err, ErrTargetFactsUnavailable) {
				t.Fatalf("relative known target executable: %v", err)
			}
		})
	}
	for _, id := range []string{"chatgpt", "vscode", "unknown", ""} {
		if engine.SupportsClient(id) || ClientSupported(all.Default(), id) {
			t.Fatalf("unsupported %s admitted", id)
		}
	}
	if ClientSupported(nil, "codex") {
		t.Fatal("nil registry broadened")
	}
	metadata := engine.Discover()
	if len(metadata) != 11 {
		t.Fatalf("Discover clients=%d", len(metadata))
	}
	for _, meta := range metadata {
		if meta.Capabilities.ClientID != domain.ClientID(meta.ClientID) {
			t.Fatalf("missing capabilities: %+v", meta)
		}
	}
	if _, err := os.Stat(engine.cfg.StateFile); !os.IsNotExist(err) {
		t.Fatalf("read-only authority checks wrote state: %v", err)
	}
}
