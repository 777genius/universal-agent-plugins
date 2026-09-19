package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// geminiNativeApply carries the locals ApplyGeminiNativeMutationWithKernelRenameAndCapacity
// used to thread through one native mutation. Extract-method keeps call order; the
// struct is the phase context, not a new policy layer.
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
	descriptor   GeminiDescriptor
}

func ApplyGeminiNativeMutationWithKernelRenameAndCapacity(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename geminiRenameFunc, capacity shared.CombinedCapacityFunc) (resultErr error) {
	prepared, err := prepareGeminiNativeApply(configRoot, activePath, previous, desired, kernel, rename, capacity)
	if err != nil {
		return err
	}
	txn, err := newGeminiSkillTxn(prepared)
	if err != nil {
		return err
	}
	if err := txn.stageDesired(); err != nil {
		return err
	}
	defer func() {
		if txn.cleanup && txn.transactionRoot != "" {
			_ = os.RemoveAll(txn.transactionRoot)
		}
	}()
	// A failed restore leaves the only recoverable copy inside transactionRoot.
	// Keep that directory intact and report its exact location to the caller.
	defer func() {
		if resultErr == nil || nativeconfig.IsCommittedCleanup(resultErr) {
			return
		}
		if rollbackErr := txn.rollback(); rollbackErr != nil {
			txn.cleanup = false
			resultErr = fmt.Errorf("%w; Gemini skill rollback failed: %w; recovery retained at %q", resultErr, rollbackErr, txn.transactionRoot)
		}
	}()
	if err := txn.backupPrevious(); err != nil {
		return err
	}
	if err := txn.installStaged(); err != nil {
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

func prepareGeminiNativeApply(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename geminiRenameFunc, capacity shared.CombinedCapacityFunc) (*geminiNativeApply, error) {
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
	previous, desired = GeminiObjects(previous), GeminiObjects(desired)
	if err := VerifyGeminiNativeObjects(configRoot, previous, true, kernel); err != nil {
		return nil, err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	idsCapacity, capacityErr := capacity(len(previousByID), len(desiredByID))
	if capacityErr != nil {
		return nil, fmt.Errorf("prepare managed Gemini object set: %w", capacityErr)
	}
	if err := validateGeminiDesiredIdentity(configRoot, previousByID, desiredByID, kernel); err != nil {
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

func validateGeminiDesiredIdentity(configRoot string, previousByID, desiredByID map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel) error {
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("the Gemini native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireGeminiObjectAbsent(configRoot, object, kernel); err != nil {
			return err
		}
	}
	return nil
}

func loadGeminiDescriptor(activePath string, desired []domain.NativeObjectOwnership) (GeminiDescriptor, error) {
	if len(desired) == 0 {
		return GeminiDescriptor{}, nil
	}
	if strings.TrimSpace(activePath) == "" {
		return GeminiDescriptor{}, fmt.Errorf("active package path is required for Gemini native installation")
	}
	body, err := os.ReadFile(filepath.Join(activePath, GeminiDescriptorName))
	descriptor := GeminiDescriptor{}
	if err != nil || json.Unmarshal(body, &descriptor) != nil || strings.TrimSpace(descriptor.DataRoot) == "" || !filepath.IsAbs(descriptor.DataRoot) {
		return GeminiDescriptor{}, fmt.Errorf("read Gemini projection descriptor: invalid or missing descriptor")
	}
	return descriptor, nil
}
