package grok

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Projection carries only previously acknowledged ownership. This does not
// assert native loading or enable a plugin while replacing its package.
func retainedEnabledObjects(plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	if plan.Scope != domain.ScopeProject {
		return nil, nil
	}
	owned, err := enabledObjectForProfile(plan.PreviousNativeObjects, plan.ActivePath, plan.DeclaredName, plan.TargetContext.UserConfigRoot)
	if err != nil || owned == nil {
		return nil, err
	}
	if _, err := grokPlanRoot(domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return nil, err
	}
	return []domain.NativeObjectOwnership{*owned}, nil
}

// Existing enabled membership is checked before any package transaction.
// Preserve unrelated current config members; a missing, disabled or ambiguous
// owned member needs an explicit native resolution before update or repair.
func preflightRetainedEnablement(env clients.Env, request domain.ActivationRequest) (resultErr error) {
	objects, err := retainedEnabledObjects(request.Plan)
	if err != nil || len(objects) == 0 {
		return err
	}
	recovery, err := enablementAuthority(env, request.Plan.TargetContext.ProjectRoot, request.Client.ConfigRoot)
	if err != nil {
		return err
	}
	if err := recovery.requireRetainedOwner(request.Plan, objects[0]); err != nil {
		return err
	}
	unlock, err := lockNativeEnablement(recovery.UserRoot)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, unlock()) }()
	file, err := recovery.Kernel.ReadExactFile(filepath.Join(recovery.UserRoot, "config.toml"))
	if err != nil {
		return err
	}
	if !file.Exists || !file.Mode.IsRegular() || file.Mode.Perm() != 0600 && file.Mode.Perm() != 0644 {
		return fmt.Errorf("grok retained enablement requires an existing ordinary native config")
	}
	id := "project/" + enablementDigest([]byte(request.Plan.ActivePath))[7:15] + "/" + request.Plan.DeclaredName
	return requireEnabledMembership(file.Body, id, request.Plan.DeclaredName)
}

func (recovery EnablementRecovery) requireRetainedOwner(plan domain.DeliveryPlan, expected domain.NativeObjectOwnership) error {
	state, err := recovery.State.Load()
	if err != nil {
		return err
	}
	found := false
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.TargetLocator != plan.ActivePath {
				continue
			}
			if found || binding.ClientID != "grok" || binding.Scope != "project" || installation.DeclaredName != plan.DeclaredName || binding.PhysicalArtifact != plan.PhysicalArtifactID ||
				New().ValidateBindingProfile(plan.NativeRegistryRoot, binding) != nil {
				return fmt.Errorf("grok retained enablement differs from its selected binding")
			}
			owned, err := enabledObjectForProfile(binding.NativeObjects, plan.ActivePath, plan.DeclaredName, recovery.UserRoot)
			if err != nil || owned == nil || !reflect.DeepEqual(*owned, expected) {
				return fmt.Errorf("grok retained enablement differs from its acknowledged receipt")
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("grok retained enablement has no current binding")
	}
	return nil
}
