package transaction

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// ProjectRootPublisher uses the installation's existing kernel and namespace.
// A committed standalone receipt owns the root after any one binding is removed.
// It grants no authority to replace or delete the runtime's mutable contents.
type ProjectRootPublisher struct{ Kernel Kernel }

var _ ports.NativeProjectRootPublisher = ProjectRootPublisher{}

func (publisher ProjectRootPublisher) PublishProjectRoot(ctx context.Context, request domain.ActivationRequest, spec ports.NativeProjectRoot) (resultErr error) {
	state, binding, err := publisher.projectRootRequest(ctx, request, spec)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(spec.Path); err == nil {
		return publisher.VerifyProjectRoot(ctx, spec.Path, spec.Marker)
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, found, err := rootReceipt(state, spec.Path); err != nil || found {
		return errors.Join(err, fmt.Errorf("recorded native runtime root is missing; refusing recreation"))
	}
	prepared, err := prepareNativeRoot(ctx, spec)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, prepared.close()) }()
	parentAuthority, err := recordedPublicationParent(prepared.baseRoot.ancestry)
	if err != nil {
		return errors.Join(err, prepared.cleanup())
	}
	mutation := DirectoryMutation{
		PublicationParentAuthority: parentAuthority,
		Standalone:                 true, RequireAbsent: true,
		OperationID: request.NativeAttempt.OperationID + "-root", Sequence: 1,
		InstallationID: request.NativeAttempt.InstallationID, ClientBindingID: binding.ClientBindingID,
		OwnedBase: prepared.base, ActivePath: prepared.active, StagingPath: prepared.staging,
		AfterDigest: prepared.digest, DesiredState: state,
		Verify: func(ctx context.Context, _ string) error {
			return verifyNativeRoot(ctx, spec.Path, spec.Marker, prepared.digest, parentAuthority)
		},
	}
	_, err = publisher.Kernel.ApplyDirectory(ctx, mutation)
	if err != nil {
		err = errors.Join(err, publisher.cleanupUnusedRoot(state, mutation.OperationID, prepared))
	}
	return err
}

// An open journal or a state decision retains kernel ownership. Only an absent
// journal together with exact old state releases a preflight-refused staging.
func (publisher ProjectRootPublisher) cleanupUnusedRoot(before domain.StateFileV2, operation string, prepared *preparedNativeRoot) error {
	if _, err := publisher.Kernel.Directory.Load(operation); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("unpublished root retained: operation journal is indeterminate: %w", err)
	}
	after, err := publisher.Kernel.StateStore.Load()
	if err != nil {
		return fmt.Errorf("unpublished root retained: state decision is indeterminate: %w", err)
	}
	oldJSON, oldErr := marshalComparableState(before)
	newJSON, newErr := marshalComparableState(after)
	if oldErr != nil || newErr != nil {
		return errors.Join(oldErr, newErr)
	}
	if string(oldJSON) != string(newJSON) {
		return nil
	}
	if _, err := prepared.baseRoot.root.Lstat(filepath.Base(prepared.staging)); os.IsNotExist(err) {
		return nil // Existing kernel rollback already removed its staging.
	} else if err != nil {
		return err
	}
	return prepared.cleanup()
}

func (publisher ProjectRootPublisher) projectRootRequest(ctx context.Context, request domain.ActivationRequest, spec ports.NativeProjectRoot) (domain.StateFileV2, domain.ClientBinding, error) {
	var state domain.StateFileV2
	var binding domain.ClientBinding
	if err := ctx.Err(); err != nil {
		return state, binding, err
	}
	if publisher.Kernel.StateStore == nil {
		return state, binding, fmt.Errorf("native root state unavailable")
	}
	if err := validateNativeRootRequest(request, spec); err != nil {
		return state, binding, err
	}
	state, err := publisher.Kernel.StateStore.Load()
	if err != nil {
		return state, binding, err
	}
	for _, installation := range state.Installations {
		if installation.InstallationID == request.NativeAttempt.InstallationID {
			binding = installation.Clients[request.NativeAttempt.BindingID]
		}
	}
	if binding.ClientBindingID != request.NativeAttempt.BindingID || binding.ClientID != string(request.Client.ClientID) ||
		binding.Scope != string(domain.ScopeProject) || binding.NativeActivationAttempt != request.NativeAttempt.OperationID ||
		binding.NativeProfileRoot != request.Plan.NativeRegistryRoot || binding.TargetLocator != request.Plan.ActivePath {
		return state, binding, fmt.Errorf("native root publication differs from recorded binding")
	}
	return state, binding, nil
}

func validateNativeRootRequest(request domain.ActivationRequest, spec ports.NativeProjectRoot) error {
	if request.Plan.Scope != domain.ScopeProject ||
		request.Client.ClientID != request.Plan.ClientID || request.NativeAttempt.OperationID == "" ||
		request.NativeAttempt.NativeRoot != request.Plan.NativeRegistryRoot ||
		!request.Plan.SelectedDelivery.IsZero() || request.VerifyOnly || request.ActivationComplete {
		return fmt.Errorf("native root publication lacks exact Project attempt")
	}
	if filepath.Dir(spec.Path) != request.Plan.NativeRegistryRoot || filepath.Dir(spec.Marker.Path) != spec.Path ||
		len(spec.MarkerBody) == 0 || len(spec.MarkerBody) > 4096 || spec.Marker.ManagedDigest != fmt.Sprintf("%x", sha256.Sum256(spec.MarkerBody)) {
		return fmt.Errorf("native root marker or layout differs")
	}
	if err := pathpolicy.RequireContainedChild(request.Plan.TargetContext.ProjectRoot, spec.Path); err != nil {
		return err
	}
	return nil
}

func (publisher ProjectRootPublisher) VerifyProjectRoot(ctx context.Context, root string, marker domain.NativeObjectOwnership) error {
	if publisher.Kernel.StateStore == nil {
		return fmt.Errorf("native root state unavailable")
	}
	state, err := publisher.Kernel.StateStore.Load()
	if err != nil {
		return err
	}
	receipt, found, err := rootReceipt(state, root)
	if err != nil || !found {
		return errors.Join(err, fmt.Errorf("native root has no committed publication receipt"))
	}
	return verifyNativeRoot(ctx, root, marker, receipt.AfterDigest, receipt.PublicationParentAuthority)
}

func rootReceipt(state domain.StateFileV2, root string) (domain.MutationReceipt, bool, error) {
	var selected domain.MutationReceipt
	for _, receipt := range state.TransactionReceipts {
		if receipt.ActivePath != root && receipt.ActivePath != filepath.Dir(root) {
			continue
		}
		if !strings.HasPrefix(receipt.AfterDigest, "native-root-v1:") {
			continue
		}
		if selected.OperationID != "" || receipt.MutationType != "directory_swap" || receipt.Phase != ReceiptPhaseCommitted {
			return selected, false, fmt.Errorf("native root publication receipt is ambiguous or pending")
		}
		if err := receipt.ValidatePublicationParent(true); err != nil {
			return selected, false, err
		}
		selected = receipt
	}
	return selected, selected.OperationID != "", nil
}

func verifyNativeRoot(ctx context.Context, root string, marker domain.NativeObjectOwnership, expected string, authority *domain.ProfileAuthority) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if authority == nil {
		return fmt.Errorf("native root original parent is required")
	}
	if err := profileauthority.Revalidate(ctx, *authority); err != nil {
		return err
	}
	if filepath.Dir(marker.Path) != root || marker.ManagedDigest == "" || marker.UserModified {
		return fmt.Errorf("native root marker authority differs")
	}
	parent, err := holdNativeRoot(filepath.Dir(root))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, parent.root.Close()) }()
	child, err := parent.child(filepath.Base(root))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, child.root.Close()) }()
	if digestNativeRoot(child, parent, root, marker.ManagedDigest) != expected {
		return fmt.Errorf("native root physical publication changed")
	}
	body, err := child.readMarker(filepath.Base(marker.Path), 4096)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(body)) != marker.ManagedDigest {
		return errors.Join(err, fmt.Errorf("native root marker changed"))
	}
	return errors.Join(ctx.Err(), parent.verifyNamed(), child.verifyNamed(), profileauthority.Revalidate(ctx, *authority))
}
