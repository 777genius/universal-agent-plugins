package opencode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateOpenCodeNativeWithKernel(ctx context.Context, request domain.ActivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeWithKernel(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects, kernel)
}

func DeactivateOpenCodeNativeWithKernel(ctx context.Context, request domain.DeactivationRequest, kernel nativeconfig.Kernel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyOpenCodeNativeWithKernel(request.Client.ConfigRoot, "", request.NativeObjects, nil, kernel)
}

func ApplyOpenCodeNative(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyOpenCodeNativeWithKernel(configRoot, activePath, previous, desired, nativeconfig.New())
}

func applyOpenCodeNativeWithKernel(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, kernel, shared.RenameDirectoryExclusive, os.RemoveAll)
}

func ApplyOpenCodeNativeWithRename(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc) (resultErr error) {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, nativeconfig.New(), rename, os.RemoveAll)
}

func ApplyOpenCodeNativeWithOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename openCodeRenameFunc, removeAll func(string) error) (resultErr error) {
	return applyOpenCodeNativeWithKernelAndOps(configRoot, activePath, previous, desired, nativeconfig.New(), rename, removeAll)
}

func applyOpenCodeNativeWithKernelAndOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename openCodeRenameFunc, removeAll func(string) error) (resultErr error) {
	prepared, err := prepareOpenCodeNativeApply(configRoot, activePath, previous, desired, kernel, rename, removeAll)
	if err != nil {
		return err
	}
	requests, err := openCodeMCPRequests(prepared.kernel, prepared.projection, prepared.previous, prepared.desired)
	if err != nil {
		return err
	}
	skills, err := installOpenCodeSkillsWithOps(configRoot, activePath, prepared.previous, prepared.desired, prepared.rename, prepared.removeAll)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			if err := skills.rollback(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("rollback OpenCode skills: %w", err))
			}
		}
	}()
	return commitOpenCodeNative(prepared.kernel, requests, skills, prepared.desired, &committed)
}

type openCodeNativeApply struct {
	rename     openCodeRenameFunc
	removeAll  func(string) error
	kernel     nativeconfig.Kernel
	projection OpenCodeProjection
	previous   []domain.NativeObjectOwnership
	desired    []domain.NativeObjectOwnership
}

func prepareOpenCodeNativeApply(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename openCodeRenameFunc, removeAll func(string) error) (*openCodeNativeApply, error) {
	if err := kernel.RequireFileIO(); err != nil {
		return nil, err
	}
	if rename == nil {
		return nil, fmt.Errorf("OpenCode rename operation is unavailable")
	}
	if removeAll == nil {
		return nil, fmt.Errorf("OpenCode cleanup operation is unavailable")
	}
	if strings.TrimSpace(configRoot) == "" || !filepath.IsAbs(configRoot) {
		return nil, fmt.Errorf("OpenCode config root is unavailable")
	}
	previous, desired = OpenCodeObjects(previous), OpenCodeObjects(desired)
	projection := OpenCodeProjection{}
	var err error
	if len(desired) > 0 {
		projection, err = ReadOpenCodeProjection(activePath)
		if err != nil {
			return nil, err
		}
		if err := validateOpenCodeProjection(configRoot, activePath, projection); err != nil {
			return nil, err
		}
	}
	if err := preflightOpenCodeObjects(configRoot, activePath, projection, previous, desired); err != nil {
		return nil, err
	}
	if err := confirmOpenCodeConfigSelection(configRoot, projection, previous, desired); err != nil {
		return nil, err
	}
	return &openCodeNativeApply{rename: rename, removeAll: removeAll, kernel: kernel, projection: projection, previous: previous, desired: desired}, nil
}

func confirmOpenCodeConfigSelection(configRoot string, projection OpenCodeProjection, previous, desired []domain.NativeObjectOwnership) error {
	expectedConfig := expectedOpenCodeConfig(projection, previous)
	if expectedConfig == "" {
		return nil
	}
	selected, err := selectOpenCodeConfig(filepath.Join(configRoot, "opencode.json"), filepath.Join(configRoot, "opencode.jsonc"))
	if err != nil {
		return err
	}
	jsonExists, jsoncExists, err := openCodeConfigPresence(configRoot)
	if err != nil {
		return err
	}
	if !shared.SameCleanPath(selected, expectedConfig) {
		if len(desired) > 0 || jsonExists || jsoncExists {
			return fmt.Errorf("OpenCode config selection changed after staging; rerun the operation")
		}
		// Removal is already complete when both exact config variants are
		// absent. Do not recreate either path just to remove an absent entry.
	}
	return nil
}

func expectedOpenCodeConfig(projection OpenCodeProjection, previous []domain.NativeObjectOwnership) string {
	if projection.ConfigPath != "" {
		return projection.ConfigPath
	}
	for _, object := range previous {
		if object.Kind == OpenCodeMCPObjectKind {
			return object.Path
		}
	}
	return ""
}

func commitOpenCodeNative(kernel nativeconfig.Kernel, requests []nativeconfig.Request, skills *openCodeSkillTxn, desired []domain.NativeObjectOwnership, committed *bool) error {
	if len(requests) == 0 {
		*committed = true
		// Config and skills are now externally committed. Transaction-root cleanup
		// is best effort and cannot truthfully turn this into a failed activation:
		// the lifecycle must persist the desired native receipts. A failed cleanup
		// leaves a committed marker in the private transaction root so the residue
		// is distinguishable from rollback recovery and safe to remove later.
		skills.commit()
		return nil
	}
	receipts, applyErr := kernel.ApplyBatch(requests)
	if applyErr != nil && !nativeconfig.IsCommittedCleanup(applyErr) {
		return applyErr
	}
	if len(receipts) != len(requests) {
		if applyErr != nil {
			return errors.Join(applyErr, fmt.Errorf("OpenCode native config committed without complete receipts"))
		}
		return fmt.Errorf("OpenCode native config returned incomplete receipts")
	}
	// ApplyBatch promises that typed committed-cleanup failures include the
	// receipts for bytes already written. Preserve both those bytes and the
	// skills installed in the same provider transaction.
	*committed = true
	if err := matchOpenCodeMCPReceipts(requests, receipts, desired); err != nil {
		return err
	}
	skills.commit()
	return applyErr
}

func matchOpenCodeMCPReceipts(requests []nativeconfig.Request, receipts []nativeconfig.Receipt, desired []domain.NativeObjectOwnership) error {
	desiredByID := shared.ObjectMap(desired)
	for index, request := range requests {
		if request.Action == nativeconfig.ActionRemove {
			continue
		}
		expected := desiredByID["opencode-mcp:"+request.Name]
		if receipts[index].Digest != expected.ManagedDigest || receipts[index].Path != expected.Path {
			return fmt.Errorf("OpenCode native receipt differs from staged ownership")
		}
	}
	return nil
}
