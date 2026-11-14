package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Engine is the process-local installer. It does not export Store or Kernel.
type Engine struct {
	projectGuard *projectNamespaceGuard
	cfg          Config
	store        transaction.StateStore
	// persistObservations enables the §5.4 PersistAuthoritativeObservations
	// seam. Prepare still uses DryRun and does not persist.
	persistObservations bool
}

// New validates Config and copies it. It does not create directories, open a
// journal, or execute a helper or client.
func New(cfg Config) (*Engine, error) {
	resolved, err := cfg.resolved()
	if err != nil {
		return nil, err
	}
	engine := &Engine{cfg: resolved, store: statev2.Store{Path: resolved.StateFile, ProjectRoot: resolved.ProjectRoot}, projectGuard: &projectNamespaceGuard{}}
	if err := engine.CheckProjectNamespace(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	return engine, nil
}

// ClientSupported reports saved-dependency qualification for an explicit
// registry. It is separate from existing SDK adapter registration.
func ClientSupported(registry *clients.Registry, clientID string) bool {
	return ClientSupportExclusionReason(registry, clientID) == ""
}

// ClientScopeSupported is the read-only saved-admission gate before source
// acquisition. A qualified project declaration does not reopen deferred user
// dependency support or admit remote/manual-only clients.
func ClientScopeSupported(registry *clients.Registry, clientID string, scope domain.InstallScope) bool {
	if scope == "" {
		scope = domain.ScopeUser
	}
	if !clients.SupportsScope(registry, domain.ClientID(clientID), scope) {
		return false
	}
	if scope != domain.ScopeProject {
		return ClientSupported(registry, clientID)
	}
	definition, _ := domain.ClientDefinitionFor(domain.ClientID(clientID))
	return !definition.PlansWithoutHostPresence && !definition.Traits.RequiresPersonalMappingForPrepare
}

// ClientSupportExclusionReason is the shared qualification policy for the SDK,
// CLI and generated public metadata. It never removes a registered adapter.
func ClientSupportExclusionReason(registry *clients.Registry, clientID string) string {
	definition, defined := domain.ClientDefinitionFor(domain.ClientID(clientID))
	if !defined {
		return "not_registered"
	}
	if definition.Traits.SavedDependenciesDeferred {
		// Owner qualification scope does not remove existing SDK/imperative capabilities.
		return "dependency_management_deferred"
	}
	if definition.PlansWithoutHostPresence || definition.Traits.RequiresPersonalMappingForPrepare {
		return "manual_preparation_only"
	}
	if _, registered := registry.Lookup(definition.ID); !registered {
		return "not_registered"
	}
	return ""
}

// SupportsClient preserves the composition root's explicit registry boundary.
func (e *Engine) SupportsClient(clientID string) bool {
	if e == nil {
		return false
	}
	_, registered := e.cfg.Registry.Lookup(domain.ClientID(clientID))
	return registered
}

// RequiresClientExecutable exposes the same explicit executable authority policy
// used by install preparation, including native host-profile consumers.
func (e *Engine) RequiresClientExecutable(clientID string) bool {
	return e.SupportsClient(clientID) && requiresExecutable(e.cfg.Registry, domain.ClientID(clientID), e.scope())
}

func (e *Engine) lifecycle(helper *managedstdio.Source, facts BindingFacts, detected map[domain.ClientID]domain.DetectedClient) usecase.Service {
	paths := pathpolicy.Policy{}
	plan := e.planner()
	nativeKernel := nativeconfig.New()
	frozenProfiles := profileClients(detected)
	profileCheck := func(ctx context.Context) error {
		if err := e.CheckProjectNamespace(); err != nil {
			return err
		}
		for _, c := range frozenProfiles {
			token := domain.ProfileAuthority{}
			if c.ProfileAuthority != nil {
				token = *c.ProfileAuthority
				if c.ProfileNamespace != e.cfg.StateRoot {
					return ErrPlanChanged
				}
			}
			if err := plan.RevalidateProfileAuthority(ctx, c.ClientID, token); err != nil {
				return err
			}
		}
		return e.prevalidatePhysical(ctx)
	}
	stager := seamStager{
		profileCheck: profileCheck,
		Stager:       providers.Stager{LauncherSource: helper, Registry: e.cfg.Registry, Paths: paths},
		serverName:   e.cfg.ServerName,
		projectArgs:  e.cfg.ProjectArgs,
		facts:        facts,
	}
	legacyTransitions := opencode.NativeTransitions{ProjectRoot: e.cfg.ProjectRoot, PackageVerifier: stager, State: transaction.Kernel{StateStore: e.store}, Kernel: nativeKernel}
	transitions := providers.NewNativeTransitions(e.cfg.Registry, clients.NativeTransitionContext{ProjectRoot: e.cfg.ProjectRoot, UserHome: e.cfg.UserHome, OperationsDir: e.cfg.OperationsDir, State: transaction.Kernel{StateStore: e.store}, Kernel: nativeKernel, PackageVerifier: stager}, legacyTransitions)
	directoryKernel := transaction.Kernel{Namespace: e.cfg.StateRoot, PhysicalAuthority: plan, NativeRecovery: transitions, StateStore: e.store, Directory: dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}}
	projectRoots := transaction.ProjectRootPublisher{Kernel: directoryKernel}
	inner := providers.Activator{ProjectRoots: projectRoots, UserHome: e.cfg.UserHome, NativeTransitions: transitions, OpenCodeTransitions: legacyTransitions, Runner: e.cfg.Runner, Registry: e.cfg.Registry, NativeConfig: &nativeKernel}
	var observer usecase.NativeIdentityObserver
	identity := providers.NativeIdentityObserver{ProjectRoots: projectRoots, Stager: stager, NativeConfig: &nativeKernel, Registry: e.cfg.Registry}
	if e.cfg.EnableNativeObserver && e.cfg.Runner != nil {
		identity.Runner = e.cfg.Runner
		observer = identity
	} else if e.cfg.ProjectRoot != "" {
		observer = preparedProjectIdentityObserver{identity: identity}
	}
	return usecase.Service{
		CheckNamespace: e.CheckProjectNamespace,
		StateStore:     e.store, Paths: paths, Planner: plan, Targets: plan, Stager: stager, ProjectRoot: e.cfg.ProjectRoot,
		Detected:          detected,
		Activator:         seamActivator{independent: e.cfg.IndependentActivations, profileCheck: profileCheck, inner: inner, onCommitted: e.cfg.OnCommittedBinding, store: e.store, facts: facts},
		PluginData:        providers.PluginDataManager{Base: e.cfg.PluginDataBase},
		Lock:              processlock.Lock{Path: e.cfg.LockFile},
		Kernel:            directoryKernel,
		PhysicalAuthority: plan, PhysicalProfiles: profileClients(detected),
		NativeObserver:     observer,
		NamespacePreflight: providers.OpenCodeNamespacePreflight{Kernel: nativeKernel},
	}
}

func (e *Engine) planner() planner.Planner {
	return planner.Planner{
		ProjectRoot: e.cfg.ProjectRoot,
		ManagedRoot: e.cfg.ManagedRoot,
		Paths:       pathpolicy.Policy{},
		Registry:    e.cfg.Registry,
	}
}

func (e *Engine) report(phase ProgressPhase) {
	if e.cfg.Progress == nil {
		return
	}
	e.cfg.Progress(ProgressEvent{Phase: phase})
}

func (e *Engine) helper() (*managedstdio.Source, error) {
	if e.cfg.HelperExecutable == "" {
		return nil, fmt.Errorf("%w: HelperExecutable is required for this operation", ErrInvalidConfig)
	}
	return managedstdio.NewSource(e.cfg.HelperExecutable, e.cfg.HelperVersion)
}

// helperIdentity is the version plus SHA-256 of the helper bytes. UAP
// managedstdio.Source stores the same digest; Prepare reports it without
// requiring execute bits that Go Windows FileMode omits on regular files.
func (e *Engine) helperIdentity() (version, digest string) {
	version = e.cfg.HelperVersion
	if e.cfg.HelperExecutable == "" {
		return version, ""
	}
	body, err := os.ReadFile(e.cfg.HelperExecutable)
	if err != nil {
		return version, ""
	}
	sum := sha256.Sum256(body)
	return version, hex.EncodeToString(sum[:])
}

func (e *Engine) ensureDirs() error {
	if err := e.CheckProjectNamespace(); err != nil {
		return err
	}
	for _, dir := range []string{
		filepath.Dir(e.cfg.StateFile), filepath.Dir(e.cfg.LockFile),
		e.cfg.OperationsDir, e.cfg.PluginDataBase, e.cfg.ManagedRoot, e.cfg.TempRoot,
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return e.CheckProjectNamespace()
}

func newLoader() (loader.Loader, error) {
	reg, err := specregistry.New()
	if err != nil {
		return loader.Loader{}, err
	}
	return loader.Loader{Registry: reg}, nil
}
