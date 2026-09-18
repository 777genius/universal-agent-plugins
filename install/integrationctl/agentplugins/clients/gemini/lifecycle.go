package gemini

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNative(env, clients.Ops{}, request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, nil)
}

func DeactivateNative(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyGeminiNative(env, clients.Ops{}, request.Client.ConfigRoot, "", request.NativeObjects, nil, nil)
}

type geminiRenameFunc func(string, string) error

func renameGeminiDirectoryNoReplace(oldPath, newPath string, rename geminiRenameFunc) error {
	if _, err := os.Lstat(newPath); err == nil {
		return fmt.Errorf("destination already exists: %s", newPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(oldPath, newPath)
}

func applyGeminiNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyGeminiNative(clients.Env{NativeConfig: nativeconfig.New()}, clients.Ops{}, configRoot, activePath, previous, desired, nil)
}

func applyGeminiNative(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, capacity shared.CombinedCapacityFunc) (resultErr error) {
	prepared, err := prepareGeminiNativeApply(env, ops, configRoot, activePath, previous, desired, capacity)
	if err != nil {
		return err
	}
	txn, err := newGeminiSkillTxn(prepared)
	if err != nil {
		return err
	}
	defer func() {
		if txn.cleanup && txn.transactionRoot != "" {
			_ = os.RemoveAll(txn.transactionRoot)
		}
	}()
	defer func() {
		if resultErr == nil || nativeconfig.IsCommittedCleanup(resultErr) {
			return
		}
		if rollbackErr := txn.rollback(); rollbackErr != nil {
			txn.cleanup = false
			resultErr = fmt.Errorf("%w; Gemini skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, txn.transactionRoot)
		}
	}()
	if err := txn.stageDesired(); err != nil {
		return err
	}
	if err := txn.backupAndInstall(); err != nil {
		return err
	}
	requests, err := geminiMCPRequests(prepared)
	if err != nil {
		return err
	}
	if err := verifyGeminiMCPReceipts(prepared, requests); err != nil {
		return err
	}
	if _, err := prepared.kernel.ApplyBatch(requests); err != nil {
		return err
	}
	txn.backups = map[string]string{}
	return nil
}

type geminiNativeApply struct {
	rename       geminiRenameFunc
	kernel       nativeconfig.Kernel
	configRoot   string
	activePath   string
	previous     []domain.NativeObjectOwnership
	desired      []domain.NativeObjectOwnership
	previousByID map[string]domain.NativeObjectOwnership
	desiredByID  map[string]domain.NativeObjectOwnership
	idsCapacity  int
	descriptor   geminiDescriptor
}

func prepareGeminiNativeApply(env clients.Env, ops clients.Ops, configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, capacity shared.CombinedCapacityFunc) (*geminiNativeApply, error) {
	rename := ops.Rename
	if rename == nil {
		rename = shared.RenameDirectoryExclusive
	}
	if capacity == nil {
		capacity = shared.CheckedCombinedCapacity
	}
	kernel := env.NativeConfig
	if rename == nil {
		return nil, fmt.Errorf("the Gemini rename operation is unavailable")
	}
	if capacity == nil {
		return nil, fmt.Errorf("the Gemini capacity checker is unavailable")
	}
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("the Gemini config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return nil, err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return nil, fmt.Errorf("prepare managed Gemini object set: %w", capacityErr)
	}
	if err := validateGeminiDesiredIdentity(configRoot, previousByID, desiredByID); err != nil {
		return nil, err
	}
	descriptor, err := loadGeminiDescriptor(activePath, desired)
	if err != nil {
		return nil, err
	}
	return &geminiNativeApply{
		rename: rename, kernel: kernel, configRoot: configRoot, activePath: activePath,
		previous: previous, desired: desired, previousByID: previousByID, desiredByID: desiredByID,
		idsCapacity: idsCapacity, descriptor: descriptor,
	}, nil
}

func validateGeminiDesiredIdentity(configRoot string, previousByID, desiredByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("the Gemini native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireGeminiObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}
	return nil
}
