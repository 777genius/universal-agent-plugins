package kiro

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func ActivateNative(ctx context.Context, request domain.ActivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
}

func DeactivateNative(ctx context.Context, request domain.DeactivationRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return applyKiroNativeMutation(request.Client.ConfigRoot, "", request.NativeObjects, nil)
}

func applyKiroNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) error {
	return applyKiroNativeMutationWithOps(configRoot, activePath, previous, desired, shared.RenameDirectoryExclusive, os.RemoveAll)
}

func applyKiroNativeMutationWithOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, rename func(string, string) error, removeAll func(string) error) error {
	_, err := applyKiroNativeMutationWithKernelAndOps(configRoot, activePath, previous, desired, nativeconfig.New(), rename, removeAll)
	return err
}

func applyKiroNativeMutationWithKernelAndOps(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, rename func(string, string) error, removeAll func(string) error) (effect domain.NativeEffectState, resultErr error) {
	effect = domain.NativeEffectUnchanged
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" {
		return effect, kiroErrorf("Kiro config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return effect, err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	if err := validateKiroDesiredIdentities(configRoot, previousByID, desiredByID); err != nil {
		return effect, err
	}
	transactionRoot, err := createKiroTransactionRoot(configRoot, previous, desired)
	if err != nil {
		return effect, err
	}
	if transactionRoot != "" {
		defer func() {
			if effect != domain.NativeEffectUncertain {
				_ = removeAll(transactionRoot)
			}
		}()
	}
	staged, err := stageKiroSkills(activePath, transactionRoot, desiredByID)
	if err != nil {
		return effect, err
	}
	return commitKiroNativeMutation(configRoot, activePath, transactionRoot, previousByID, desiredByID, staged, kernel, rename, removeAll)
}

func commitKiroNativeMutation(configRoot, activePath, transactionRoot string, previousByID, desiredByID map[string]domain.NativeObjectOwnership, staged map[string]string, kernel nativeconfig.Kernel, rename func(string, string) error, removeAll func(string) error) (effect domain.NativeEffectState, resultErr error) {
	effect = domain.NativeEffectUnchanged
	mcpMutation, err := planKiroMCPMutationWithKernel(configRoot, activePath, previousByID, desiredByID, kernel)
	if err != nil {
		return effect, err
	}
	if mcpMutation.file != nil {
		defer func() {
			cleanupErr := mcpMutation.file.Close()
			if cleanupErr != nil && resultErr == nil {
				resultErr = &nativeconfig.CommittedCleanupError{Err: cleanupErr}
			} else {
				resultErr = errors.Join(resultErr, cleanupErr)
			}
		}()
	}
	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	mcpAttempted := false
	defer func() {
		if resultErr == nil {
			return
		}
		uncertain, rollbackErr := rollbackKiroNativeWithOps(mcpMutation, previousByID, backups, installed, mcpAttempted, rename, removeAll)
		effect, resultErr = kiroRollbackOutcome(transactionRoot, uncertain, resultErr, rollbackErr)
	}()
	if err := backupPreviousKiroSkills(transactionRoot, previousByID, backups, rename); err != nil {
		return effect, err
	}
	if err := installStagedKiroSkills(staged, desiredByID, installed, rename); err != nil {
		return effect, err
	}
	if mcpMutation.active {
		mcpAttempted = true
		if err := mcpMutation.file.Apply(mcpMutation.body); err != nil {
			return effect, err
		}
	}
	if err := VerifyNativeObjects(configRoot, valuesOf(desiredByID), false); err != nil {
		return effect, err
	}
	return domain.NativeEffectCommitted, nil
}

func kiroRollbackOutcome(transactionRoot string, uncertain bool, primary, rollbackErr error) (domain.NativeEffectState, error) {
	effect := domain.NativeEffectUnchanged
	if uncertain {
		effect = domain.NativeEffectUncertain
	}
	if rollbackErr != nil {
		if uncertain && transactionRoot != "" {
			rollbackErr = fmt.Errorf("recovery retained at %q: %w", transactionRoot, rollbackErr)
		}
		primary = errors.Join(primary, kiroErrorf("Kiro native rollback failed: %w", rollbackErr))
	}
	return effect, primary
}

func validateKiroDesiredIdentities(configRoot string, previousByID, desiredByID map[string]domain.NativeObjectOwnership) error {
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return kiroErrorf("Kiro native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireKiroObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}
	return nil
}

func createKiroTransactionRoot(configRoot string, previous, desired []domain.NativeObjectOwnership) (string, error) {
	if !hasKiroSkillObjects(previous) && !hasKiroSkillObjects(desired) {
		return "", nil
	}
	skillsRoot := filepath.Join(configRoot, "skills")
	if err := ValidateNativePath(configRoot, skillsRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return "", fmt.Errorf("create Kiro skills root: %w", err)
	}
	transactionRoot, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return "", fmt.Errorf("create Kiro native transaction: %w", err)
	}
	return transactionRoot, nil
}

func stageKiroSkills(activePath, transactionRoot string, desiredByID map[string]domain.NativeObjectOwnership) (map[string]string, error) {
	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if strings.TrimSpace(activePath) == "" {
			return nil, fmt.Errorf("active package path is required for Kiro skill installation")
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return nil, fmt.Errorf("unsafe Kiro skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return nil, fmt.Errorf("stage Kiro skill %q: %w", object.LogicalName, err)
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return nil, fmt.Errorf("staged Kiro skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}
	return staged, nil
}

type kiroMCPMutation struct {
	active bool
	body   []byte
	file   *nativeconfig.ExactFile
}

func planKiroMCPMutationWithKernel(configRoot, activePath string, previousByID, desiredByID map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel) (mutation kiroMCPMutation, resultErr error) {
	previousMCP := previousMCPObjects(valuesOf(previousByID))
	desiredMCP := previousMCPObjects(valuesOf(desiredByID))
	if len(previousMCP)+len(desiredMCP) == 0 {
		return mutation, nil
	}
	path := filepath.Join(configRoot, "settings", "mcp.json")
	if err := ValidateNativePath(configRoot, path); err != nil {
		return mutation, err
	}
	file, err := kernel.BeginExactFile(path)
	if err != nil {
		return mutation, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, file.Close())
		}
	}()
	body, err := prepareKiroMCPBody(file.Original(), activePath, previousByID, previousMCP, desiredMCP)
	if err != nil {
		return mutation, err
	}
	return kiroMCPMutation{active: true, body: body, file: file}, nil
}

func prepareKiroMCPBody(snapshot nativeconfig.FileSnapshot, activePath string, previousByID map[string]domain.NativeObjectOwnership, previousMCP, desiredMCP []domain.NativeObjectOwnership) ([]byte, error) {
	mcp := map[string]any{}
	if snapshot.Exists {
		var err error
		mcp, err = decodeKiroMCPConfig(snapshot.Body)
		if err != nil {
			return nil, err
		}
	}
	// Recheck ownership against the locked snapshot that will be compared at
	// replacement. An earlier registry inspection cannot authorize newer bytes.
	for _, object := range previousMCP {
		if err := verifyKiroMCPDigest(mcp, object, true); err != nil {
			return nil, err
		}
	}
	for _, object := range desiredMCP {
		if _, owned := previousByID[object.ObjectID]; !owned {
			if _, exists := mcp[object.LogicalName]; exists {
				return nil, kiroErrorf("Kiro MCP server %q already exists without agentplugins ownership", object.LogicalName)
			}
		}
	}
	for _, object := range previousMCP {
		delete(mcp, object.LogicalName)
	}
	for _, object := range desiredMCP {
		server, err := projectedKiroMCPServer(activePath, object.LogicalName)
		if err != nil {
			return nil, err
		}
		if shared.DigestJSONObject(server) != object.ManagedDigest {
			return nil, fmt.Errorf("projected Kiro MCP server %q does not match its ownership digest", object.LogicalName)
		}
		mcp[object.LogicalName] = server
	}
	return encodeKiroMCPConfig(snapshot.Body, mcp)
}

func valuesOf(objects map[string]domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		result = append(result, object)
	}
	return result
}

func backupPreviousKiroSkills(transactionRoot string, previousByID map[string]domain.NativeObjectOwnership, backups map[string]string, rename func(string, string) error) error {
	for id, object := range previousByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(transactionRoot, "old-"+object.LogicalName)
		if err := renameKiroDirectoryExclusive(object.Path, backup, rename); err != nil {
			return fmt.Errorf("backup Kiro skill %q: %w", object.LogicalName, err)
		}
		backups[id] = backup
	}
	return nil
}

func installStagedKiroSkills(staged map[string]string, desiredByID map[string]domain.NativeObjectOwnership, installed map[string]domain.NativeObjectOwnership, rename func(string, string) error) error {
	for id, source := range staged {
		object := desiredByID[id]
		if err := renameKiroDirectoryExclusive(source, object.Path, rename); err != nil {
			return fmt.Errorf("activate Kiro skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}
	return nil
}

func rollbackKiroNative(mutation kiroMCPMutation, previousByID map[string]domain.NativeObjectOwnership, backups map[string]string, installed map[string]domain.NativeObjectOwnership, mcpWritten bool) error {
	_, err := rollbackKiroNativeWithOps(mutation, previousByID, backups, installed, mcpWritten, shared.RenameDirectoryExclusive, os.RemoveAll)
	return err
}

func rollbackKiroNativeWithOps(mutation kiroMCPMutation, previousByID map[string]domain.NativeObjectOwnership, backups map[string]string, installed map[string]domain.NativeObjectOwnership, mcpWritten bool, rename func(string, string) error, removeAll func(string) error) (uncertain bool, rollbackErr error) {
	if mcpWritten {
		rollbackErr = mutation.file.Rollback()
		uncertain = mutation.file.Effect() != nativeconfig.FileUnchanged
	}
	for _, object := range installed {
		digest, err := shared.DigestSkillDirectory(object.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err == nil && digest == object.ManagedDigest {
			err = removeAll(object.Path)
		} else if err == nil {
			err = fmt.Errorf("installed skill changed outside agentplugins")
		}
		if err != nil {
			uncertain = true
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("retain Kiro skill at %q: %w", object.Path, err))
		}
	}
	// Each backup gets exactly one restore attempt. Keep failed backups in the
	// transaction directory, including when removing an installed skill failed.
	for id, backup := range backups {
		if err := renameKiroDirectoryExclusive(backup, previousByID[id].Path, rename); err != nil {
			uncertain = true
			rollbackErr = errors.Join(rollbackErr, kiroErrorf("Kiro backup retained at %q; restore to %q: %w", backup, previousByID[id].Path, err))
		} else {
			delete(backups, id)
		}
	}
	return uncertain, rollbackErr
}

func renameKiroDirectoryExclusive(source, target string, rename func(string, string) error) error {
	if _, err := os.Lstat(target); err == nil {
		return kiroErrorf("Kiro target already exists at %q: %w", target, os.ErrExist)
	} else if !os.IsNotExist(err) {
		return err
	}
	return rename(source, target)
}
