package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

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
	if err := e.CheckProjectNamespace(); err != nil {
		return err
	}
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
		if b.ClientID != string(c.ClientID) || b.Scope != string(e.scope()) {
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
		expected, err := e.nativeRegistryRoot(c)
		if err != nil {
			return c, err
		}
		canonical, err := filepath.EvalSymlinks(expected)
		if err != nil || canonical != c.ProfileAuthority.Facts().CanonicalRoot {
			return c, fmt.Errorf("selected alias differs from frozen physical profile")
		}
		if e.scope() == domain.ScopeUser {
			c.ConfigRoot = canonical
		}
	}
	return c, nil
}

func (e *Engine) preparedClients(req Request) (domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	client, err := e.detectedClient(req)
	if err != nil {
		return domain.DetectedClient{}, nil, err
	}
	detected, err := e.detectedClients(req)
	if err != nil {
		return domain.DetectedClient{}, nil, err
	}
	client = physicalClient(req, client)
	detected = physicalDetected(req, detected)
	return client, detected, nil
}

// WithTemporaryRoot returns an engine sharing authoritative state but staging
// read-only preparation in a caller-owned disposable directory.
func (e *Engine) WithTemporaryRoot(root string) (*Engine, error) {
	cfg := e.cfg
	cfg.TempRoot = root
	cfg.temporaryRoot = true
	derived, err := New(cfg)
	if err != nil {
		return nil, err
	}
	// Derived preparation observes the same selected physical namespace. A new
	// staging directory must not reset the original engine's identity pin.
	derived.projectGuard = e.projectGuard
	if err := derived.CheckProjectNamespace(); err != nil {
		return nil, err
	}
	// Prepare always uses DryRun; preserve the observation policy for an
	// eventual confirmed Apply without enabling writes during preparation.
	derived.persistObservations = e.persistObservations
	return derived, nil
}
