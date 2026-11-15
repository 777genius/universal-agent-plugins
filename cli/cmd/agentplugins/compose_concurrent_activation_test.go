package main

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// The CLI is the one composition root that qualified its process runner for
// simultaneous client commands, so it must be the one that opts in.
func TestCLICompositionRunsAttestedClientsConcurrently(t *testing.T) {
	dataRoot := t.TempDir()
	registry := clientregistry.Default()
	paths := pathpolicy.Policy{}
	store := statev2.Store{Path: filepath.Join(dataRoot, "state-v2.json")}
	plan := planner.Planner{ManagedRoot: filepath.Join(dataRoot, "managed"), Paths: paths, Registry: registry}
	service := newAgentpluginsLifecycle(dataRoot, store, paths, registry, providers.Stager{},
		processadapter.OS{}, plan, dirswap.Manager{JournalDir: filepath.Join(dataRoot, "operations")},
		processlock.Lock{Path: filepath.Join(dataRoot, "mutation.lock")}, nativeconfig.New())
	activator, ok := service.Activator.(providers.Activator)
	if !ok || !activator.Concurrent {
		t.Fatalf("the CLI must enable concurrent client activation: %#v", service.Activator)
	}
}
