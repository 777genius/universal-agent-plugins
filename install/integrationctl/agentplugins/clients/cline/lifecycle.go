package cline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ApplyClineNativeMutationWithKernelRenameAndCapacity(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename clineRenameFunc, capacity shared.CombinedCapacityFunc) (resultErr error) {
	prepared, err := prepareClineNativeApply(configRoot, activePath, previous, desired, kernel, rename, capacity)
	if err != nil {
		return err
	}
	txn, err := newClineSkillTxn(prepared)
	if err != nil {
		return err
	}
	defer func() {
		if txn.cleanup {
			_ = os.RemoveAll(txn.transactionRoot)
		}
	}()
	defer func() {
		if resultErr != nil && !nativeconfig.IsCommittedCleanup(resultErr) {
			if rollbackErr := txn.rollback(); rollbackErr != nil {
				txn.cleanup = false
				resultErr = fmt.Errorf("%w; Cline skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, txn.transactionRoot)
			}
		}
	}()
	if err := txn.stageDesired(); err != nil {
		return err
	}
	if err := txn.backupAndInstall(); err != nil {
		return err
	}
	if err := VerifyClineNativeObjects(prepared.configRoot, clineSkillObjects(prepared.desired), false, prepared.kernel); err != nil {
		return err
	}
	return mutateClineMCPWithKernelAndCapacity(prepared.configRoot, prepared.activePath, prepared.previousByID, prepared.desiredByID, prepared.kernel, prepared.idsCapacity)
}

type clineNativeApply struct {
	rename       clineRenameFunc
	kernel       nativeconfig.Kernel
	configRoot   string
	activePath   string
	previous     []domain.NativeObjectOwnership
	desired      []domain.NativeObjectOwnership
	previousByID map[string]domain.NativeObjectOwnership
	desiredByID  map[string]domain.NativeObjectOwnership
	idsCapacity  int
}

func prepareClineNativeApply(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename clineRenameFunc, capacity shared.CombinedCapacityFunc) (*clineNativeApply, error) {
	if err := kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	if rename == nil {
		return nil, fmt.Errorf("the Cline rename operation is unavailable")
	}
	if capacity == nil {
		return nil, fmt.Errorf("the Cline capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("the Cline config root is unavailable")
	}
	previous, desired = ClineObjects(previous), ClineObjects(desired)
	if err := VerifyClineNativeObjects(configRoot, previous, true, kernel); err != nil {
		return nil, err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return nil, fmt.Errorf("prepare managed Cline MCP server set: %w", capacityErr)
	}
	if err := validateClineDesiredIdentity(previousByID, desiredByID); err != nil {
		return nil, err
	}
	return &clineNativeApply{
		rename: rename, kernel: kernel, configRoot: configRoot, activePath: activePath,
		previous: previous, desired: desired, previousByID: previousByID, desiredByID: desiredByID,
		idsCapacity: idsCapacity,
	}, nil
}

func validateClineDesiredIdentity(previousByID, desiredByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("the Cline native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if object.Kind == ClineSkillObjectKind {
			if _, err := os.Lstat(object.Path); err == nil {
				return fmt.Errorf("the Cline skill %q already exists without agentplugins ownership", object.LogicalName)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}
