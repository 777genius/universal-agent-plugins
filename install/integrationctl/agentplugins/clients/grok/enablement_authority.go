package grok

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PluginEnablement = (*Adapter)(nil)

func (*Adapter) SupportsPluginEnablement(scope domain.InstallScope) bool {
	return scope == domain.ScopeProject && runtime.GOOS == "linux"
}

func enablementAuthority(env clients.Env, project, userRoot string) (*EnablementRecovery, error) {
	recovery, ok := env.NativeTransitions.(*EnablementRecovery)
	if !ok || recovery == nil || env.UserHome == "" || !filepath.IsAbs(env.UserHome) || filepath.Clean(env.UserHome) != env.UserHome ||
		userRoot != filepath.Join(env.UserHome, ".grok") || recovery.UserRoot != userRoot || recovery.ProjectRoot != project ||
		recovery.OperationsDir == "" || recovery.State == nil || recovery.PackageVerifier == nil {
		return nil, fmt.Errorf("grok enablement requires the exact constructor home and native transition authority")
	}
	if err := recovery.Kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	return recovery, nil
}

func validateEnabledObject(binding domain.ClientBinding, object domain.NativeObjectOwnership) error {
	if pathpolicy.ValidateLeafID(object.LogicalName) != nil || object.Path != binding.TargetLocator ||
		object.Kind != enabledMemberKind || !enablementHash(object.BeforeDigest) || !enablementHash(object.ManagedDigest) {
		return fmt.Errorf("grok enabled member identity differs from the exact project artifact")
	}
	prefix := "grok-enabled:"
	if !strings.HasPrefix(object.ObjectID, prefix) || len(object.ObjectID) < len(prefix)+65 {
		return fmt.Errorf("grok enabled member has no profile identity")
	}
	profile := object.ObjectID[len(prefix) : len(prefix)+64]
	id := "project/" + enablementDigest([]byte(binding.TargetLocator))[7:15] + "/" + object.LogicalName
	expected := domain.NativeObjectOwnership{ObjectID: prefix + profile + ":" + id, Kind: enabledMemberKind, LogicalName: object.LogicalName,
		Path: binding.TargetLocator, BeforeDigest: object.BeforeDigest, ManagedDigest: object.ManagedDigest, ProtectionClass: "native_config_member"}
	if !enablementHash("sha256:"+profile) || !reflect.DeepEqual(object, expected) {
		return fmt.Errorf("grok enabled member carries foreign ownership fields")
	}
	return nil
}

func enabledObjectForProfile(objects []domain.NativeObjectOwnership, path, name, userRoot string) (*domain.NativeObjectOwnership, error) {
	var result *domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind != enabledMemberKind {
			continue
		}
		if result != nil || validateEnabledObject(domain.ClientBinding{TargetLocator: path}, object) != nil || object.LogicalName != name ||
			!strings.HasPrefix(object.ObjectID, "grok-enabled:"+enablementDigest([]byte(userRoot))[7:]+":") {
			return nil, fmt.Errorf("grok enabled member differs from the selected profile or plugin")
		}
		ownedCopy := object
		result = &ownedCopy
	}
	return result, nil
}

func (recovery EnablementRecovery) activationAuthority(request domain.ActivationRequest) (domain.StateFileV2, int, domain.ClientBinding, error) {
	state, err := recovery.State.Load()
	if err != nil {
		return state, -1, domain.ClientBinding{}, err
	}
	id := request.NativeAttempt
	index, installation, binding, err := enablementBinding(state, id.InstallationID, id.BindingID)
	if err != nil {
		return state, index, binding, err
	}
	if id.OperationID == "" || binding.NativeActivationAttempt != id.OperationID || id.NativeRoot != request.Plan.NativeRegistryRoot ||
		binding.TargetLocator != request.Delivery.ActivePath || binding.NativeProfileRoot != filepath.Join(recovery.ProjectRoot, ".grok") ||
		installation.DeclaredName != request.DeclaredName || binding.PhysicalArtifact != request.Plan.PhysicalArtifactID ||
		shared.ManagedPackageDigest(binding) != request.Delivery.ArtifactDigest || request.Delivery.ArtifactDigest == "" {
		return state, index, binding, fmt.Errorf("grok enablement differs from its durable selected native attempt")
	}
	return state, index, binding, New().ValidateBindingProfile(binding.NativeProfileRoot, binding)
}
