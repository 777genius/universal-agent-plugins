package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPrepareRejectsCrossEnvironmentContextBeforePackageRead(t *testing.T) {
	root := t.TempDir()
	for _, test := range []struct {
		name, engineRoot string
		request          Request
	}{
		{"project-on-user-engine", "", Request{Scope: domain.ScopeProject, ProjectRoot: root}},
		{"user-on-project-engine", root, Request{Scope: domain.ScopeUser}},
		{"other-project", root, Request{Scope: domain.ScopeProject, ProjectRoot: filepath.Join(root, "other")}},
		{"unknown-scope", "", Request{Scope: "unknown"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateRoot := filepath.Join(root, test.name, ".uap")
			if test.engineRoot != "" {
				stateRoot = filepath.Join(test.engineRoot, ".uap")
			}
			engine, err := New(testConfig(t, Config{StateRoot: stateRoot, ProjectRoot: test.engineRoot}))
			if err != nil {
				t.Fatal(err)
			}
			test.request.PackageRoot = filepath.Join(root, "missing-package")
			test.request.ClientID = "codex"
			if _, err := engine.Prepare(context.Background(), test.request); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("context was not rejected before package read: %v", err)
			}
			if _, err := os.Lstat(stateRoot); !os.IsNotExist(err) {
				t.Fatalf("invalid context created mutable namespace: %v", err)
			}
		})
	}
}
