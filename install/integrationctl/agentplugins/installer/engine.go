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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Engine is the process-local installer. It does not export Store or Kernel.
type Engine struct {
	cfg   Config
	store transaction.StateStore
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
	return &Engine{cfg: resolved, store: statev2.Store{Path: resolved.StateFile}}, nil
}

// SupportsClient reports whether the composition root registered the client
// for this engine. The facade never broadens a caller's explicit registry.
func (e *Engine) SupportsClient(clientID string) bool {
	if e == nil || e.cfg.Registry == nil || clientID == "" {
		return false
	}
	_, ok := e.cfg.Registry.Lookup(domain.ClientID(clientID))
	return ok
}

func (e *Engine) lifecycle(helper *managedstdio.Source, facts BindingFacts, detected map[domain.ClientID]domain.DetectedClient) usecase.Service {
	paths := pathpolicy.Policy{}
	plan := e.planner()
	nativeKernel := nativeconfig.New()
	frozenProfiles := profileClients(detected)
	profileCheck := func(ctx context.Context) error {
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
	inner := providers.Activator{Runner: e.cfg.Runner, Registry: e.cfg.Registry, NativeConfig: &nativeKernel}
	var observer usecase.NativeIdentityObserver
	if e.cfg.EnableNativeObserver && e.cfg.Runner != nil {
		observer = providers.NativeIdentityObserver{
			Stager: stager, Runner: e.cfg.Runner, NativeConfig: &nativeKernel, Registry: e.cfg.Registry,
		}
	}
	return usecase.Service{
		StateStore: e.store, Paths: paths, Planner: plan, Targets: plan, Stager: stager,
		Detected:          detected,
		Activator:         seamActivator{profileCheck: profileCheck, inner: inner, onCommitted: e.cfg.OnCommittedBinding, store: e.store, facts: facts},
		PluginData:        providers.PluginDataManager{Base: e.cfg.PluginDataBase},
		Lock:              processlock.Lock{Path: e.cfg.LockFile},
		Kernel:            transaction.Kernel{Namespace: e.cfg.StateRoot, PhysicalAuthority: plan, StateStore: e.store, Directory: dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}},
		PhysicalAuthority: plan, PhysicalProfiles: profileClients(detected),
		NativeObserver:     observer,
		NamespacePreflight: providers.OpenCodeNamespacePreflight{Kernel: nativeKernel},
	}
}

func (e *Engine) planner() planner.Planner {
	return planner.Planner{
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
	for _, dir := range []string{
		filepath.Dir(e.cfg.StateFile), filepath.Dir(e.cfg.LockFile),
		e.cfg.OperationsDir, e.cfg.PluginDataBase, e.cfg.ManagedRoot, e.cfg.TempRoot,
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func newLoader() (loader.Loader, error) {
	reg, err := specregistry.New()
	if err != nil {
		return loader.Loader{}, err
	}
	return loader.Loader{Registry: reg}, nil
}

func profileClients(detected map[domain.ClientID]domain.DetectedClient) []domain.DetectedClient {
	out := make([]domain.DetectedClient, 0, len(detected))
	for _, c := range detected {
		c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
		out = append(out, c)
	}
	return out
}

// VerifyProfileAuthority addresses only the recorded owner in this engine's explicit namespace.
func (e *Engine) VerifyProfileAuthority(ctx context.Context, installationID, bindingID string) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	state, err := e.store.Load()
	if err != nil {
		return err
	}
	installation, ok := findInstall(state, installationID)
	if !ok || installation.InstallationID != installationID {
		return ErrNotInstalled
	}
	binding, ok := installation.Clients[bindingID]
	if !ok || binding.ClientBindingID != bindingID || binding.ClientID == "" {
		return ErrNotInstalled
	}
	token := domain.ProfileAuthority{}
	if binding.ProfileAuthority != nil {
		token = *binding.ProfileAuthority
		if token.IsZero() || binding.ProfileNamespace != e.cfg.StateRoot || binding.NativeProfileRoot != token.Facts().CanonicalRoot || bindingID != domain.ComputeClientBindingID(installationID, binding.ClientID, binding.Scope, binding.TargetLocator) {
			return fmt.Errorf("recorded physical owner is malformed")
		}
	}
	return e.planner().RevalidateProfileAuthority(ctx, domain.ClientID(binding.ClientID), token)
}
func (e *Engine) captureRequestProfiles(ctx context.Context, req Request) (map[domain.ClientID]domain.DetectedClient, error) {
	detected, err := e.detectedClients(req)
	if err != nil {
		return nil, err
	}
	state, err := e.store.Load()
	if err != nil {
		return nil, err
	}
	installationID := firstNonEmpty(req.InstallationID, req.Selector)
	if installationID == "" {
		for _, i := range state.Installations {
			if i.Source.CanonicalSource == firstNonEmpty(req.SourceRoot, req.PackageRoot) {
				if installationID != "" {
					return nil, ErrAmbiguousInstallations
				}
				installationID = i.InstallationID
			}
		}
	}
	installation, _ := findInstall(state, installationID)
	for id, c := range detected {
		current, err := e.captureClientProfile(ctx, installation, c)
		if err != nil {
			return nil, err
		}
		detected[id] = current
	}
	return detected, e.prevalidatePhysical(ctx)
}
func (e *Engine) checkPreparedProfiles(ctx context.Context, p *PreparedOperation) error {
	return e.checkRequestProfiles(ctx, p.req)
}
func (e *Engine) checkRequestProfiles(ctx context.Context, req Request) error {
	for _, c := range req.physical {
		token := domain.ProfileAuthority{}
		if c.ProfileAuthority != nil {
			token = *c.ProfileAuthority
			if c.ProfileNamespace != e.cfg.StateRoot {
				return ErrPlanChanged
			}
		}
		if err := e.planner().RevalidateProfileAuthority(ctx, c.ClientID, token); err != nil {
			return err
		}
	}
	return e.prevalidatePhysical(ctx)
}
func (e *Engine) prevalidatePhysical(ctx context.Context) error {
	if err := (transaction.Kernel{Namespace: e.cfg.StateRoot, PhysicalAuthority: e.planner(), StateStore: e.store, Directory: dirswap.Manager{Namespace: e.cfg.StateRoot, JournalDir: e.cfg.OperationsDir}}).PrevalidateRecovery(ctx); err != nil {
		return fmt.Errorf("read installation state and pending authority: %w", err)
	}
	return nil
}
func physicalClient(req Request, c domain.DetectedClient) domain.DetectedClient {
	if frozen, ok := req.physical[c.ClientID]; ok {
		frozen.ProfileAuthority = domain.CloneProfileAuthority(frozen.ProfileAuthority)
		return frozen
	}
	return c
}
func physicalDetected(req Request, detected map[domain.ClientID]domain.DetectedClient) map[domain.ClientID]domain.DetectedClient {
	for id, c := range detected {
		detected[id] = physicalClient(req, c)
	}
	return detected
}

func (e *Engine) captureClientProfile(ctx context.Context, installation domain.Installation, c domain.DetectedClient) (domain.DetectedClient, error) {
	var recorded *domain.ClientBinding
	for key, b := range installation.Clients {
		if b.ClientID != string(c.ClientID) || b.Scope != string(domain.ScopeUser) {
			continue
		}
		if recorded != nil || key != b.ClientBindingID {
			return c, fmt.Errorf("physical binding owner is ambiguous")
		}
		bindingCopy := b
		recorded = &bindingCopy
	}
	if recorded != nil {
		if err := e.VerifyProfileAuthority(ctx, installation.InstallationID, recorded.ClientBindingID); err != nil {
			return c, err
		}
		c.ProfileAuthority = domain.CloneProfileAuthority(recorded.ProfileAuthority)
		c.ProfileNamespace = recorded.ProfileNamespace
	} else {
		token, err := e.planner().CaptureProfileAuthority(ctx, c)
		if err != nil {
			return c, err
		}
		if !token.IsZero() {
			c.ProfileAuthority = &token
			c.ProfileNamespace = e.cfg.StateRoot
		}
	}
	if c.ProfileAuthority != nil {
		canonical, err := filepath.EvalSymlinks(c.ConfigRoot)
		if err != nil || canonical != c.ProfileAuthority.Facts().CanonicalRoot {
			return c, fmt.Errorf("selected alias differs from frozen physical profile")
		}
		c.ConfigRoot = canonical
	}
	return c, nil
}
