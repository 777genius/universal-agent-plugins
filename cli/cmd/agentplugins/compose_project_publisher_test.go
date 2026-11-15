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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func TestProjectPublisherRetainsScopedKernelAuthority(t *testing.T) {
	for _, project := range []bool{false, true} {
		name := "user"
		root := ""
		registry := clientregistry.Default()
		if project {
			name = "project"
			root = t.TempDir()
			registry = clientregistry.Project()
		}
		t.Run(name, func(t *testing.T) {
			dataRoot := t.TempDir()
			store := statev2.Store{Path: filepath.Join(dataRoot, "state-v2.json"), ProjectRoot: root}
			paths := pathpolicy.Policy{}
			plan := planner.Planner{ProjectRoot: root, ManagedRoot: filepath.Join(dataRoot, "managed"), Paths: paths, Registry: registry}
			service := newAgentpluginsLifecycle(dataRoot, store, paths, registry, providers.Stager{},
				processadapter.OS{}, plan, dirswap.Manager{JournalDir: filepath.Join(dataRoot, "operations")},
				processlock.Lock{Path: filepath.Join(dataRoot, "mutation.lock")}, nativeconfig.New())
			activator, ok := service.Activator.(providers.Activator)
			if !ok {
				t.Fatal("expected concrete native activator")
			}
			observer, ok := service.NativeObserver.(providers.NativeIdentityObserver)
			if !ok {
				t.Fatal("expected concrete native identity observer")
			}
			publishers := []struct {
				name  string
				value any
			}{{"activator", activator.ProjectRoots}, {"observer", observer.ProjectRoots}}
			for _, item := range publishers {
				publisher, ok := item.value.(transaction.ProjectRootPublisher)
				if !ok {
					t.Fatalf("%s has no root publisher", item.name)
				}
				if publisher.Kernel.Namespace != service.Kernel.Namespace ||
					publisher.Kernel.Directory.Namespace != service.Kernel.Directory.Namespace ||
					(publisher.Kernel.PhysicalAuthority == nil) != (service.Kernel.PhysicalAuthority == nil) {
					t.Fatalf("%s copied an unscoped kernel", item.name)
				}
			}
			if project {
				if service.Kernel.Namespace != dataRoot || service.Kernel.Directory.Namespace != dataRoot ||
					service.Kernel.PhysicalAuthority == nil {
					t.Fatal("Project publisher lacks scoped physical authority")
				}
			} else if service.Kernel.Namespace != "" || service.Kernel.Directory.Namespace != "" ||
				service.Kernel.PhysicalAuthority != nil {
				t.Fatal("User publisher acquired Project authority")
			}
		})
	}
}
