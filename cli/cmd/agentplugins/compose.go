package main

import (
	"context"
	"fmt"
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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statemigration"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	clientregistry "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func migrationPendingError(list func() ([]dirswap.Receipt, error), load func() (domain.StateFileV2, error)) error {
	journals, err := list()
	if err != nil {
		return err
	}
	if len(journals) != 0 {
		return fmt.Errorf("%d open transaction journals require the matching previous SDK or binary", len(journals))
	}
	state, err := load()
	if err != nil {
		return err
	}
	for _, receipt := range state.TransactionReceipts {
		if receipt.Phase != "committed" {
			return fmt.Errorf("receipt %s phase %s requires the matching previous SDK or binary", receipt.OperationID, receipt.Phase)
		}
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.PendingNativeIntent != nil {
				return fmt.Errorf("native intent for binding %s requires the matching previous SDK or binary", binding.ClientBindingID)
			}
			for _, receipt := range binding.Receipts {
				if receipt.Phase != "committed" {
					return fmt.Errorf("receipt %s phase %s requires the matching previous SDK or binary", receipt.OperationID, receipt.Phase)
				}
			}
		}
	}
	return nil
}

func composeAgentpluginsApp(home, dataRoot string) (agentpluginscli.App, error) {
	directoryClient, discoveryClient, securityClient, err := newProductionFeedClients(dataRoot)
	if err != nil {
		return agentpluginscli.App{}, err
	}
	registry, err := specregistry.New()
	if err != nil {
		return agentpluginscli.App{}, err
	}
	return newAgentpluginsCLIApp(home, dataRoot, directoryClient, discoveryClient, securityClient, registry)
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

func newAgentpluginsCLIApp(home, dataRoot string, directoryClient *directoryv1.Client, discoveryClient *discoveryv1.Client, securityClient *securityv1.Client, registry *specregistry.Registry) (agentpluginscli.App, error) {
	return newScopedAgentpluginsCLIApp(home, dataRoot, "", directoryClient, discoveryClient, securityClient, registry)
}

func newScopedAgentpluginsCLIApp(home, cacheRoot, projectRoot string, directoryClient *directoryv1.Client, discoveryClient *discoveryv1.Client, securityClient *securityv1.Client, registry *specregistry.Registry) (agentpluginscli.App, error) {
	dataRoot := cacheRoot
	if projectRoot != "" {
		dataRoot = filepath.Join(projectRoot, ".uap")
	}
	packageLoader := loader.Loader{Registry: registry}
	runner := processadapter.OS{}
	v2Store := statev2.Store{Path: filepath.Join(dataRoot, "state-v2.json"), ProjectRoot: projectRoot}
	directoryManager := dirswap.Manager{JournalDir: filepath.Join(dataRoot, "operations-v2")}
	mutationLock := processlock.Lock{Path: filepath.Join(dataRoot, "mutation.lock")}
	clientRegistry := clientregistry.Default()
	if projectRoot != "" {
		clientRegistry = clientregistry.Project()
	}
	paths := pathpolicy.Policy{}
	nativeKernel := nativeconfig.New()
	helperExecutable, err := os.Executable()
	if err != nil {
		return agentpluginscli.App{}, err
	}
	stager := newManagedStager(clientRegistry, paths, helperExecutable)
	planner := clientplanner.Planner{ProjectRoot: projectRoot, ManagedRoot: filepath.Join(dataRoot, "managed"), Paths: paths, Registry: clientRegistry}
	lifecycle := newAgentpluginsLifecycle(dataRoot, v2Store, paths, clientRegistry, stager, runner, planner, directoryManager, mutationLock, nativeKernel)
	facade, err := installer.New(installer.Config{
		ProjectRoot: projectRoot, StateRoot: dataRoot, StateFile: v2Store.Path, LockFile: filepath.Join(dataRoot, "mutation.lock"),
		OperationsDir: filepath.Join(dataRoot, "operations-v2"), PluginDataBase: filepath.Join(dataRoot, "plugin-data"),
		ManagedRoot: filepath.Join(dataRoot, "managed"), TempRoot: filepath.Join(dataRoot, "installer-tmp"),
		HelperExecutable: helperExecutable, HelperVersion: version, Registry: clientRegistry, Runner: runner,
		EnableNativeObserver: true,
	})
	if err != nil {
		return agentpluginscli.App{}, err
	}
	lifecycle.ProjectRoot = projectRoot
	lifecycle.CheckNamespace = facade.CheckProjectNamespace
	if projectRoot != "" {
		lifecycle.Kernel.Namespace = dataRoot
		lifecycle.Kernel.PhysicalAuthority = planner
		lifecycle.Kernel.Directory.Namespace = dataRoot
	}
	app := assembleAgentpluginsApp(home, dataRoot, cacheRoot, v2Store, mutationLock, lifecycle, directoryClient, discoveryClient, securityClient, packageLoader, clientRegistry, planner)
	app.Installer = facade
	app.ManagementFactory = managementAppFactory(home, dataRoot)
	setAgentpluginsProjectContext(&app, projectRoot)
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		if scope != domain.ScopeProject {
			root = ""
		}
		return newScopedAgentpluginsCLIApp(home, cacheRoot, root, directoryClient, discoveryClient, securityClient, registry)
	}
	return app, nil
}

func newManagedStager(clientRegistry *clients.Registry, paths pathpolicy.Policy, helperExecutable string) providers.Stager {
	// The composition root is the one place that decides which clients this
	// binary knows about, so it is also the only place that names the full set.
	stager := providers.Stager{Registry: clientRegistry, Paths: paths}
	stager.LauncherSource, _ = managedstdio.NewSource(helperExecutable, version)
	return stager
}

func newAgentpluginsLifecycle(dataRoot string, v2Store statev2.Store, paths pathpolicy.Policy, clientRegistry *clients.Registry, stager providers.Stager, runner processadapter.OS, planner clientplanner.Planner, directoryManager dirswap.Manager, mutationLock processlock.Lock, nativeKernel nativeconfig.Kernel) usecase.Service {
	transitions := opencode.NativeTransitions{ProjectRoot: v2Store.ProjectRoot, PackageVerifier: stager, State: transaction.Kernel{StateStore: v2Store}, Kernel: nativeKernel}
	return usecase.Service{
		ClientPreparation: providers.NewOpenCodeClientPreparation(clientRegistry),
		StateStore:        v2Store, Paths: paths, Planner: planner, Targets: planner, Stager: stager,
		Activator: providers.Activator{OpenCodeTransitions: transitions, Runner: runner, Registry: clientRegistry, NativeConfig: &nativeKernel},
		Lock:      mutationLock, Kernel: transaction.Kernel{NativeRecovery: transitions, StateStore: v2Store, Directory: directoryManager},
		NativeObserver:     providers.NativeIdentityObserver{Stager: stager, Runner: runner, Registry: clientRegistry, NativeConfig: &nativeKernel},
		NamespacePreflight: providers.OpenCodeNamespacePreflight{Kernel: nativeKernel},
		PluginData:         providers.PluginDataManager{Base: filepath.Join(dataRoot, "plugin-data")},
	}
}

func assembleAgentpluginsApp(home, dataRoot, cacheRoot string, v2Store statev2.Store, mutationLock processlock.Lock, lifecycle usecase.Service, directoryClient *directoryv1.Client, discoveryClient *discoveryv1.Client, securityClient *securityv1.Client, packageLoader loader.Loader, clientRegistry *clients.Registry, planner clientplanner.Planner) agentpluginscli.App {
	detector := clientdetect.NewOS(home)
	detector.Registry = clientRegistry
	legacyStatePath := filepath.Join(home, ".plugin-kit-ai", "state.json")
	migrator := statemigration.Migrator{
		LegacyPath: legacyStatePath,
		V2Store:    v2Store,
		Lock:       mutationLock,
		PendingCheck: func(_ context.Context) error {
			return migrationPendingError(lifecycle.Kernel.Directory.ListOpen, v2Store.Load)
		},
	}
	return agentpluginscli.App{
		Version:              version,
		UserHome:             home,
		ManagedRoot:          filepath.Join(dataRoot, "managed"),
		DependenciesRoot:     dataRoot,
		EnvironmentStoreRoot: filepath.Join(cacheRoot, "store"),
		EnvironmentLockPath:  filepath.Join(dataRoot, "environment.lock"),
		StateStore:           v2Store,
		StateMigrator:        &migrator,
		LegacyLifecycle:      agentpluginscli.NewLegacyLifecycle(legacyStatePath),
		LegacyStateLock:      locks.FileLock{BaseDir: filepath.Join(home, ".plugin-kit-ai", "locks")},
		Detector:             detector,
		ClientRegistry:       clientRegistry,
		Planner:              planner,
		Targets:              planner,
		DirectoryClient:      directoryClient,
		DirectoryClientForDryRun: func(root string) (agentpluginscli.DirectoryClient, error) {
			return cloneDependencyDirectoryClient(directoryClient, root)
		},
		CommitDirectoryPreview: func(preview agentpluginscli.DirectoryClient) error {
			return commitDependencyDirectoryPreview(directoryClient, preview)
		},
		DiscoveryClient:     discoveryClient,
		SourceAcquirer:      lazySourceAcquirer{dataRoot: cacheRoot, acquirer: sourceacquisition.Acquirer{TempRoot: cacheRoot}},
		PackageLoader:       packageLoader,
		NativePackageLoader: loader.OpenAILoader{Loader: packageLoader},
		SecurityIndex:       securityClient,
		SecurityEvaluator: securityscan.Evaluator{
			Scanner: securityscan.ReleaseScanner{Root: filepath.Join(cacheRoot, "security", "lintai"), HTTPClient: lintaiReleaseHTTPClient()},
			Cache:   securityscan.FileCache{Root: filepath.Join(cacheRoot, "security", "assessments")}, Requirement: securityscan.DefaultRequirement(),
		},
		SourceAcquirerForDryRun: func(root string) agentpluginscli.SourceAcquirer {
			return skillsource.Acquirer{Acquirer: sourceacquisition.Acquirer{TempRoot: root}}
		},
		SecurityEvaluatorForDryRun: func(root string) ports.PackageSecurityEvaluator {
			return securityscan.Evaluator{
				Scanner: securityscan.ReleaseScanner{Root: filepath.Join(root, "lintai"), HTTPClient: lintaiReleaseHTTPClient()},
				Cache:   nil, Requirement: securityscan.DefaultRequirement(),
			}
		},
		Lifecycle:     lifecycle,
		Input:         os.Stdin,
		Output:        os.Stdout,
		ErrorOutput:   os.Stderr,
		PromptFactory: terminalprompts.New,
		Terminal:      term.IsTerminal(int(os.Stdin.Fd())),
	}
}

func cloneDependencyDirectoryClient(directoryClient *directoryv1.Client, root string) (agentpluginscli.DirectoryClient, error) {
	copied := *directoryClient
	copied.Cache.Path = filepath.Join(root, "directory-cache.json")
	if bundle, err := directoryClient.Cache.Load(directoryClient.Trust); err == nil {
		if err := copied.Cache.Store(bundle, directoryClient.Trust); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) && directoryClient.Cache.Path != "" {
		return nil, err
	}
	return &copied, nil
}

// Persist only a signature-verified staged observation, using the cache's
// existing concurrent sequence/conflict checks after agreement confirmation.
func commitDependencyDirectoryPreview(original *directoryv1.Client, preview agentpluginscli.DirectoryClient) error {
	staged, ok := preview.(*directoryv1.Client)
	if !ok {
		return fmt.Errorf("unexpected dependency Directory preview client")
	}
	bundle, err := staged.Cache.Load(staged.Trust)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = original.Cache.Reconcile(bundle, original.Trust)
	return err
}

func setAgentpluginsProjectContext(app *agentpluginscli.App, projectRoot string) {
	app.Scope, app.ProjectRoot = domain.ScopeUser, projectRoot
	if projectRoot == "" {
		return
	}
	app.Scope = domain.ScopeProject
	app.DependenciesRoot = projectRoot
	app.StateMigrator = nil
	app.LegacyLifecycle = nil
	app.LegacyStateLock = nil
}
