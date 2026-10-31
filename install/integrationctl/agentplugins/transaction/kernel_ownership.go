package transaction

// Profile ownership and mutation authority checks.

import (
	"context"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func copyOwners(owners []domain.PhysicalProfileOwner) []domain.PhysicalProfileOwner {
	out := append([]domain.PhysicalProfileOwner(nil), owners...)
	for i := range out {
		out[i].Authority = domain.CloneProfileAuthority(out[i].Authority)
	}
	return out
}
func neutralOwners(owners []domain.PhysicalProfileOwner) []dirswap.ProfileOwner {
	out := make([]dirswap.ProfileOwner, 0, len(owners))
	for _, o := range owners {
		token, err := profileauthority.ToNeutral(*o.Authority)
		if err != nil {
			panic("validated physical owner cannot convert")
		}
		out = append(out, dirswap.ProfileOwner{Namespace: o.Namespace, InstallationID: o.InstallationID, ClientID: o.ClientID, ClientBindingID: o.ClientBindingID, Authority: &token})
	}
	return out
}
func sameReceiptOwners(owners []domain.PhysicalProfileOwner, journal []dirswap.ProfileOwner) bool {
	if len(owners) != len(journal) {
		return false
	}
	for i, o := range owners {
		j := journal[i]
		if o.Namespace != j.Namespace || o.InstallationID != j.InstallationID || o.ClientID != j.ClientID || o.ClientBindingID != j.ClientBindingID || o.Authority == nil || j.Authority == nil {
			return false
		}
		token, err := profileauthority.FromNeutral(*j.Authority)
		if err != nil || !token.Equal(*o.Authority) {
			return false
		}
	}
	return true
}
func (kernel Kernel) validateOwners(ctx context.Context, owners []domain.PhysicalProfileOwner) error {
	if len(owners) > 256 {
		return fmt.Errorf("profile owner scope limit")
	}
	seen := map[[4]string]bool{}
	for _, o := range owners {
		key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
		if kernel.Namespace == "" || o.Namespace != kernel.Namespace || o.InstallationID == "" || o.ClientID == "" || o.ClientBindingID == "" || o.Authority == nil || o.Authority.IsZero() || seen[key] {
			return fmt.Errorf("invalid exact physical profile scope")
		}
		seen[key] = true
		if kernel.PhysicalAuthority == nil {
			return fmt.Errorf("physical profile verifier is required")
		}
		if err := kernel.PhysicalAuthority.RevalidateProfileAuthority(ctx, domain.ClientID(o.ClientID), *o.Authority); err != nil {
			return err
		}
	}
	return nil
}
func (kernel Kernel) bindingOwner(ctx context.Context, installationID, key string, b domain.ClientBinding) ([]domain.PhysicalProfileOwner, error) {
	if b.ProfileAuthority == nil {
		if kernel.PhysicalAuthority != nil {
			if err := kernel.PhysicalAuthority.RevalidateProfileAuthority(ctx, domain.ClientID(b.ClientID), domain.ProfileAuthority{}); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if key != b.ClientBindingID || b.ProfileAuthority.IsZero() || b.NativeProfileRoot != b.ProfileAuthority.Facts().CanonicalRoot {
		return nil, fmt.Errorf("physical binding key or root differs from owner")
	}
	return []domain.PhysicalProfileOwner{{Namespace: b.ProfileNamespace, InstallationID: installationID, ClientID: b.ClientID, ClientBindingID: key, Authority: domain.CloneProfileAuthority(b.ProfileAuthority)}}, nil
}
func (kernel Kernel) mutationOwners(ctx context.Context, state domain.StateFileV2, mutations []DirectoryMutation) ([]domain.PhysicalProfileOwner, error) {
	before, err := kernel.StateStore.Load()
	if err != nil {
		return nil, err
	}
	var owners []domain.PhysicalProfileOwner
	for _, m := range mutations {
		_, b, err := loadClientFromState(state, m.InstallationID, m.ClientBindingID)
		if err != nil {
			return nil, err
		}
		if _, old, err := loadClientFromState(before, m.InstallationID, m.ClientBindingID); err == nil && (old.ProfileAuthority != nil || b.ProfileAuthority != nil) {
			if old.ClientID != b.ClientID || old.ProfileNamespace != b.ProfileNamespace || !domain.SameProfileAuthority(old.ProfileAuthority, b.ProfileAuthority) {
				return nil, fmt.Errorf("candidate differs from recorded physical owner")
			}
		}
		scope, err := kernel.bindingOwner(ctx, m.InstallationID, m.ClientBindingID, b)
		if err != nil {
			return nil, err
		}
		owners = append(owners, scope...)
		if b.DataReceiptID != "" {
			peers, err := kernel.sharedDataReceiptOwners(ctx, state, m, b.DataReceiptID)
			if err != nil {
				return nil, err
			}
			owners = append(owners, peers...)
		}
	}
	unique := map[[4]string]domain.PhysicalProfileOwner{}
	for _, owner := range owners {
		unique[[4]string{owner.Namespace, owner.InstallationID, owner.ClientID, owner.ClientBindingID}] = owner
	}
	owners = nil
	for _, owner := range unique {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].ClientBindingID < owners[j].ClientBindingID })
	return owners, kernel.validateOwners(ctx, owners)
}
// Shared data receipts retain every physical profile in the same installation,
// excluding the binding already added by the caller.
func (kernel Kernel) sharedDataReceiptOwners(ctx context.Context, state domain.StateFileV2, mutation DirectoryMutation, receiptID string) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	for _, installation := range state.Installations {
		if installation.InstallationID != mutation.InstallationID {
			continue
		}
		for key, peer := range installation.Clients {
			if key == mutation.ClientBindingID || peer.DataReceiptID != receiptID {
				continue
			}
			relevant, err := kernel.bindingOwner(ctx, installation.InstallationID, key, peer)
			if err != nil {
				return nil, err
			}
			owners = append(owners, relevant...)
		}
	}
	return owners, nil
}

func (kernel Kernel) removalOwners(ctx context.Context, removals []DirectoryRemoval) ([]domain.PhysicalProfileOwner, error) {
	if kernel.StateStore == nil {
		return nil, fmt.Errorf("transaction state store is required")
	}
	state, err := kernel.StateStore.Load()
	if err != nil {
		return nil, err
	}
	var owners []domain.PhysicalProfileOwner
	seen := map[[4]string]bool{}
	add := func(items []domain.PhysicalProfileOwner) {
		for _, o := range items {
			key := [4]string{o.Namespace, o.InstallationID, o.ClientID, o.ClientBindingID}
			if !seen[key] {
				seen[key] = true
				owners = append(owners, o)
			}
		}
	}
	for _, r := range removals {
		if err := validateRemovalScopes(state, r); err != nil {
			return nil, err
		}
		add(copyOwners(r.ProfileOwners))
		scopes, err := kernel.removalBindingScopes(ctx, state, r)
		if err != nil {
			return nil, err
		}
		add(scopes)
	}
	sort.Slice(owners, func(i, j int) bool { return owners[i].ClientBindingID < owners[j].ClientBindingID })
	return owners, kernel.validateOwners(ctx, owners)
}

// Compare requested physical receipt owners with the recorded state before
// mutating any managed or native directory.
func validateRemovalScopes(state domain.StateFileV2, removal DirectoryRemoval) error {
	for _, owner := range removal.ProfileOwners {
		_, binding, err := loadClientFromState(state, owner.InstallationID, owner.ClientBindingID)
		if err != nil || owner.Namespace != binding.ProfileNamespace || owner.ClientID != binding.ClientID || !domain.SameProfileAuthority(owner.Authority, binding.ProfileAuthority) {
			return fmt.Errorf("removal scope differs from recorded owner")
		}
	}
	return nil
}

func (kernel Kernel) removalBindingScopes(ctx context.Context, state domain.StateFileV2, removal DirectoryRemoval) ([]domain.PhysicalProfileOwner, error) {
	var owners []domain.PhysicalProfileOwner
	for _, installation := range state.Installations {
		if installation.InstallationID != removal.InstallationID {
			continue
		}
		for key, binding := range installation.Clients {
			if (removal.DataReceiptID == "" && key != removal.ClientBindingID) || (removal.DataReceiptID != "" && binding.DataReceiptID != removal.DataReceiptID) {
				continue
			}
			scope, err := kernel.bindingOwner(ctx, installation.InstallationID, key, binding)
			if err != nil {
				return nil, err
			}
			owners = append(owners, scope...)
		}
	}
	return owners, nil
}

type guardedStateStore struct {
	StateStore
	check func() error
}

func (s guardedStateStore) Save(state domain.StateFileV2) error {
	if err := s.check(); err != nil {
		return err
	}
	return s.StateStore.Save(state)
}
func (s guardedStateStore) RequireMutationReady() error { return requireMutationReady(s.StateStore) }
func (kernel Kernel) guardOwners(ctx context.Context, owners []domain.PhysicalProfileOwner) Kernel {
	frozen := copyOwners(owners)
	kernel.Directory.Namespace = kernel.Namespace
	kernel.Directory.RequiredOwners = neutralOwners(frozen)
	if len(frozen) > 0 {
		kernel.Directory.CheckOpen = true
		base := kernel
		kernel.StateStore = guardedStateStore{StateStore: kernel.StateStore, check: func() error {
			if err := base.validateOwners(ctx, frozen); err != nil {
				return err
			}
			return base.PrevalidateRecovery(ctx)
		}}
	}
	return kernel
}
