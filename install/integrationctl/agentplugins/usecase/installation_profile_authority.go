package usecase

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func (service Service) authorityPort() ports.PhysicalProfileAuthority {
	if service.PhysicalAuthority != nil {
		return service.PhysicalAuthority
	}
	port, _ := service.Planner.(ports.PhysicalProfileAuthority)
	return port
}
func (service Service) checkProfiles(ctx context.Context) error {
	if service.CheckNamespace != nil {
		if err := service.CheckNamespace(); err != nil {
			return err
		}
	} else if service.ProjectRoot != "" {
		return fmt.Errorf("project namespace guard is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if service.profileCheck != nil {
		return service.profileCheck()
	}
	for _, c := range service.PhysicalProfiles {
		if err := service.checkProfile(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (service Service) checkProfile(ctx context.Context, c domain.DetectedClient) error {
	token := domain.ProfileAuthority{}
	if c.ProfileAuthority != nil {
		token = *c.ProfileAuthority
		if token.IsZero() {
			return fmt.Errorf("opted profile token is missing")
		}
		if c.ProfileNamespace == "" || c.ProfileNamespace != service.Kernel.Namespace {
			return fmt.Errorf("physical profile namespace differs")
		}
	}
	port := service.authorityPort()
	if port == nil {
		if !token.IsZero() {
			return fmt.Errorf("physical profile verifier is required")
		}
		return nil
	}
	return port.RevalidateProfileAuthority(ctx, c.ClientID, token)
}
func (service Service) freezeProfiles(ctx context.Context, installationID string, selected []domain.DetectedClient, capture bool) (Service, []domain.DetectedClient, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return service, nil, err
	}
	frozen := append([]domain.DetectedClient(nil), selected...)
	opted := false
	shared, err := service.sharedProfiles(ctx, state, installationID)
	if err != nil {
		return service, nil, err
	}
	for _, c := range shared {
		opted = opted || c.ProfileAuthority != nil
	}
	for i, c := range frozen {
		current, err := service.freezeProfile(ctx, state, installationID, c, capture)
		if err != nil {
			return service, nil, err
		}
		frozen[i] = current
		opted = opted || current.ProfileAuthority != nil
	}
	if !opted {
		return service, frozen, nil
	}
	return service.guardFrozenProfiles(ctx, frozen, shared), frozen, nil
}

func (service Service) dataProfileOwners(dataID string) []domain.PhysicalProfileOwner {
	state, err := service.StateStore.Load()
	if err != nil {
		return nil
	}
	var owners []domain.PhysicalProfileOwner
	for _, i := range state.Installations {
		for key, b := range i.Clients {
			if b.DataReceiptID == dataID && b.ProfileAuthority != nil {
				owners = append(owners, domain.PhysicalProfileOwner{Namespace: b.ProfileNamespace, InstallationID: i.InstallationID, ClientID: b.ClientID, ClientBindingID: key, Authority: domain.CloneProfileAuthority(b.ProfileAuthority)})
			}
		}
	}
	return owners
}

func (service Service) sharedProfiles(ctx context.Context, state domain.StateFileV2, installationID string) ([]domain.DetectedClient, error) {
	var shared []domain.DetectedClient
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, b := range installation.Clients {
			if b.PhysicalArtifact != domain.ComputePhysicalArtifactID(installation.DeclaredName, installationID) {
				continue
			}
			if key != b.ClientBindingID {
				return nil, fmt.Errorf("shared data binding key differs")
			}
			c := domain.DetectedClient{ClientID: domain.ClientID(b.ClientID), ConfigRoot: b.NativeProfileRoot, ProfileAuthority: domain.CloneProfileAuthority(b.ProfileAuthority), ProfileNamespace: b.ProfileNamespace}
			if service.ProjectRoot != "" {
				c.ConfigRoot = service.Detected[c.ClientID].ConfigRoot
			}
			if err := service.checkProfile(ctx, c); err != nil {
				return nil, err
			}
			shared = append(shared, c)
		}
	}
	return shared, nil
}
func recordedProfile(state domain.StateFileV2, installationID string, c domain.DetectedClient, scopes ...domain.InstallScope) (*domain.ClientBinding, error) {
	var recorded *domain.ClientBinding
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, b := range installation.Clients {
			if b.ClientID != string(c.ClientID) || b.Scope != string(selectedScope(scopes)) {
				continue
			}
			if key != b.ClientBindingID || recorded != nil {
				return nil, fmt.Errorf("physical owner binding is ambiguous or malformed")
			}
			bindingCopy := b
			recorded = &bindingCopy
		}
	}
	return recorded, nil
}
func (service Service) freezeProfile(ctx context.Context, state domain.StateFileV2, installationID string, c domain.DetectedClient, capture bool) (domain.DetectedClient, error) {
	recorded, err := recordedProfile(state, installationID, c, service.scope())
	if err != nil {
		return c, err
	}
	if recorded != nil {
		if c.ProfileAuthority != nil && !domain.SameProfileAuthority(c.ProfileAuthority, recorded.ProfileAuthority) {
			return c, fmt.Errorf("caller profile differs from recorded owner")
		}
		c.ProfileAuthority = domain.CloneProfileAuthority(recorded.ProfileAuthority)
		c.ProfileNamespace = recorded.ProfileNamespace
	} else if c.ProfileAuthority == nil && capture && service.authorityPort() != nil {
		token, err := service.authorityPort().CaptureProfileAuthority(ctx, c)
		if err != nil {
			return c, err
		}
		if !token.IsZero() {
			c.ProfileAuthority = &token
			c.ProfileNamespace = service.Kernel.Namespace
		}
	}
	if c.ProfileAuthority != nil {
		root := c.ProfileAuthority.Facts().CanonicalRoot
		expected := c.ConfigRoot
		if service.ProjectRoot != "" {
			target, err := service.Targets.ResolveTarget(ctx, c, domain.ScopeProject, "project-context")
			if err != nil {
				return c, err
			}
			expected = target.NativeRegistryRoot
		}
		spelling, err := filepath.EvalSymlinks(expected)
		if err != nil || spelling != root {
			return c, fmt.Errorf("selected profile differs from recorded canonical root")
		}
		if service.ProjectRoot == "" {
			c.ConfigRoot = root
		}
	}
	if err := service.checkProfile(ctx, c); err != nil {
		return c, err
	}
	c.ProfileAuthority = domain.CloneProfileAuthority(c.ProfileAuthority)
	return c, nil
}

// A moved Skill retains the environment's explicit stable source authority;
// ordinary direct packages keep their historical repository/path rule.
func managedDirectSource(installation domain.Installation, input AddInput) bool {
	if input.EnvironmentAuthority == nil || normalizedOriginMode(input.OriginMode) != domain.OriginModeDirect || installation.Source.Repository != input.Envelope.Source.Repository {
		return false
	}
	if installation.Source.StableSourceKey != "" {
		hint := input.Envelope.Source.SourceBindingHint
		return (hint == "uap-dependency-skill" || hint == "uap-dependency-web-skill") && installation.Source.StableSourceKey == input.Envelope.Source.StableSourceKey && input.EnvironmentAuthority.SourceKey == installation.Source.StableSourceKey
	}
	return installation.Source.PackageSubpath == input.Envelope.Source.PackageSubpath
}
