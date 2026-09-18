package main

import (
	"os"
	"path/filepath"

	"golang.org/x/term"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminalprompts"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func composeAgentpluginsApp(home, dataRoot string) (agentpluginscli.App, error) {
	directoryClient, discoveryClient, securityClient, err := newProductionFeedClients(dataRoot)
	if err != nil {
		return agentpluginscli.App{}, err
	}
	registry, err := specregistry.New()
	if err != nil {
		return agentpluginscli.App{}, err
	}
	return newAgentpluginsCLIApp(home, dataRoot, directoryClient, discoveryClient, securityClient, registry), nil
}

func newProductionFeedClients(dataRoot string) (*directoryv1.Client, *discoveryv1.Client, *securityv1.Client, error) {
	directoryClient, err := directoryClientFactory(dataRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	discoveryClient, err := discoveryClientFactory(dataRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	securityClient, err := securityClientFactory(dataRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	return directoryClient, discoveryClient, securityClient, nil
}

func newAgentpluginsCLIApp(home, dataRoot string, directoryClient *directoryv1.Client, discoveryClient *discoveryv1.Client, securityClient *securityv1.Client, registry *specregistry.Registry) agentpluginscli.App {
	packageLoader := loader.Loader{Registry: registry}
	runner := processadapter.OS{}
	v2Store := statev2.Store{Path: filepath.Join(dataRoot, "state-v2.json")}
	directoryManager := dirswap.Manager{JournalDir: filepath.Join(dataRoot, "operations-v2")}
	mutationLock := processlock.Lock{Path: filepath.Join(dataRoot, "mutation.lock")}
	clientRegistry := clientregistry.Default()
	paths := pathpolicy.Policy{}
	stager := newManagedStager(clientRegistry, paths)
	planner := clientplanner.Planner{ManagedRoot: filepath.Join(dataRoot, "managed"), Paths: paths, Registry: clientRegistry}
	lifecycle := newAgentpluginsLifecycle(dataRoot, v2Store, paths, clientRegistry, stager, runner, planner, directoryManager, mutationLock)
	return assembleAgentpluginsApp(home, dataRoot, v2Store, mutationLock, lifecycle, directoryClient, discoveryClient, securityClient, packageLoader, clientRegistry, planner)
}

func newManagedStager(clientRegistry *clients.Registry, paths pathpolicy.Policy) providers.Stager {
	// The composition root is the one place that decides which clients this
	// binary knows about, so it is also the only place that names the full set.
	stager := providers.Stager{Registry: clientRegistry, Paths: paths}
	if executable, err := os.Executable(); err == nil {
		stager.LauncherSource, _ = managedstdio.NewSource(executable, version)
	}
	return stager
}

func newAgentpluginsLifecycle(dataRoot string, v2Store statev2.Store, paths pathpolicy.Policy, clientRegistry *clients.Registry, stager providers.Stager, runner processadapter.OS, planner clientplanner.Planner, directoryManager dirswap.Manager, mutationLock processlock.Lock) usecase.Service {
	return usecase.Service{
		StateStore: v2Store, Paths: paths, Planner: planner, Targets: planner, Stager: stager,
		Activator: providers.Activator{Runner: runner, Registry: clientRegistry},
		Lock:      mutationLock, Kernel: transaction.Kernel{StateStore: v2Store, Directory: directoryManager},
		NativeObserver: providers.NativeIdentityObserver{Stager: stager, Runner: runner, Registry: clientRegistry},
		PluginData:     providers.PluginDataManager{Base: filepath.Join(dataRoot, "plugin-data")},
	}
}

func assembleAgentpluginsApp(home, dataRoot string, v2Store statev2.Store, mutationLock processlock.Lock, lifecycle usecase.Service, directoryClient *directoryv1.Client, discoveryClient *discoveryv1.Client, securityClient *securityv1.Client, packageLoader loader.Loader, clientRegistry *clients.Registry, planner clientplanner.Planner) agentpluginscli.App {
	detector := clientdetect.NewOS(home)
	detector.Registry = clientRegistry
	legacyStatePath := filepath.Join(home, ".plugin-kit-ai", "state.json")
	migrator := statemigration.Migrator{
		LegacyPath:     legacyStatePath,
		V2Store:        v2Store,
		Lock:           mutationLock,
		RecoverJournal: lifecycle.Kernel.Recover,
	}
	return agentpluginscli.App{
		Version:             version,
		UserHome:            home,
		ManagedRoot:         filepath.Join(dataRoot, "managed"),
		StateStore:          v2Store,
		StateMigrator:       &migrator,
		LegacyLifecycle:     agentpluginscli.NewLegacyLifecycle(legacyStatePath),
		LegacyStateLock:     locks.FileLock{BaseDir: filepath.Join(home, ".plugin-kit-ai", "locks")},
		Detector:            detector,
		ClientRegistry:      clientRegistry,
		Planner:             planner,
		Targets:             planner,
		DirectoryClient:     directoryClient,
		DiscoveryClient:     discoveryClient,
		SourceAcquirer:      lazySourceAcquirer{dataRoot: dataRoot, acquirer: sourceacquisition.Acquirer{TempRoot: dataRoot}},
		PackageLoader:       packageLoader,
		NativePackageLoader: loader.OpenAILoader{Loader: packageLoader},
		SecurityIndex:       securityClient,
		SecurityEvaluator: securityscan.Evaluator{
			Scanner: securityscan.ReleaseScanner{Root: filepath.Join(dataRoot, "security", "lintai"), HTTPClient: lintaiReleaseHTTPClient()},
			Cache:   securityscan.FileCache{Root: filepath.Join(dataRoot, "security", "assessments")}, Requirement: securityscan.DefaultRequirement(),
		},
		Lifecycle:     lifecycle,
		Input:         os.Stdin,
		Output:        os.Stdout,
		ErrorOutput:   os.Stderr,
		PromptFactory: terminalprompts.New,
		Terminal:      term.IsTerminal(int(os.Stdin.Fd())),
	}
}
