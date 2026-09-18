package kiro

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
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

func applyKiroNativeMutation(configRoot, activePath string, previous, desired []domain.NativeObjectOwnership) (resultErr error) {
	configRoot = strings.TrimSpace(configRoot)
	if configRoot == "" {
		return kiroErrorf("Kiro config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	if err := validateKiroDesiredIdentities(configRoot, previousByID, desiredByID); err != nil {
		return err
	}
	transactionRoot, err := createKiroTransactionRoot(configRoot, previous, desired)
	if err != nil {
		return err
	}
	if transactionRoot != "" {
		defer func() { _ = os.RemoveAll(transactionRoot) }()
	}
	staged, err := stageKiroSkills(activePath, transactionRoot, desiredByID)
	if err != nil {
		return err
	}
	mcpMutation, err := planKiroMCPMutation(configRoot, activePath, previousByID, desiredByID)
	if err != nil {
		return err
	}
	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	mcpWritten := false
	defer func() {
		if resultErr == nil {
			return
		}
		if rollbackErr := rollbackKiroNative(mcpMutation, previousByID, backups, installed, mcpWritten); rollbackErr != nil {
			// Keep the mutation cause out of the unwrap chain so errors.Is on
			// the original resultErr still matches after a failed rollback.
			resultErr = fmt.Errorf("%s; Kiro native rollback failed: %w", resultErr.Error(), rollbackErr)
		}
	}()
	if err := backupPreviousKiroSkills(transactionRoot, previousByID, backups); err != nil {
		return err
	}
	if err := installStagedKiroSkills(staged, desiredByID, installed); err != nil {
		return err
	}
	if mcpMutation.active {
		if err := atomicfile.Write(mcpMutation.path, mcpMutation.body, mcpMutation.mode); err != nil {
			return fmt.Errorf("write Kiro MCP configuration: %w", err)
		}
		mcpWritten = true
	}
	return VerifyNativeObjects(configRoot, desired, false)
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
	active         bool
	path           string
	body           []byte
	original       []byte
	mode           os.FileMode
	originalExists bool
}

func planKiroMCPMutation(configRoot, activePath string, previousByID, desiredByID map[string]domain.NativeObjectOwnership) (kiroMCPMutation, error) {
	mutation := kiroMCPMutation{path: filepath.Join(configRoot, "settings", "mcp.json"), mode: 0o600}
	previousMCP := previousMCPObjects(valuesOf(previousByID))
	desiredMCP := previousMCPObjects(valuesOf(desiredByID))
	if len(previousMCP)+len(desiredMCP) == 0 {
		return mutation, nil
	}
	mcp, original, mode, exists, err := ReadMCPConfig(mutation.path)
	if err != nil {
		return kiroMCPMutation{}, err
	}
	mutation.original, mutation.mode, mutation.originalExists = original, mode, exists
	for _, object := range previousByID {
		if object.Kind == kiroMCPObjectKind {
			delete(mcp, object.LogicalName)
		}
	}
	for _, object := range desiredByID {
		if object.Kind != kiroMCPObjectKind {
			continue
		}
		server, err := projectedKiroMCPServer(activePath, object.LogicalName)
		if err != nil {
			return kiroMCPMutation{}, err
		}
		if shared.DigestJSONObject(server) != object.ManagedDigest {
			return kiroMCPMutation{}, fmt.Errorf("projected Kiro MCP server %q does not match its ownership digest", object.LogicalName)
		}
		mcp[object.LogicalName] = server
	}
	body, err := encodeKiroMCPConfig(original, mcp)
	if err != nil {
		return kiroMCPMutation{}, err
	}
	mutation.active, mutation.body = true, body
	return mutation, nil
}

func valuesOf(objects map[string]domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	result := make([]domain.NativeObjectOwnership, 0, len(objects))
	for _, object := range objects {
		result = append(result, object)
	}
	return result
}

func backupPreviousKiroSkills(transactionRoot string, previousByID map[string]domain.NativeObjectOwnership, backups map[string]string) error {
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
		if err := os.Rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Kiro skill %q: %w", object.LogicalName, err)
		}
		backups[id] = backup
	}
	return nil
}

func installStagedKiroSkills(staged map[string]string, desiredByID map[string]domain.NativeObjectOwnership, installed map[string]domain.NativeObjectOwnership) error {
	for id, source := range staged {
		object := desiredByID[id]
		if err := os.Rename(source, object.Path); err != nil {
			return fmt.Errorf("activate Kiro skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}
	return nil
}

func rollbackKiroNative(mutation kiroMCPMutation, previousByID map[string]domain.NativeObjectOwnership, backups map[string]string, installed map[string]domain.NativeObjectOwnership, mcpWritten bool) error {
	var rollbackErr error
	if mcpWritten {
		if mutation.originalExists {
			rollbackErr = atomicfile.Write(mutation.path, mutation.original, mutation.mode)
		} else if body, readErr := os.ReadFile(mutation.path); readErr == nil && bytes.Equal(body, mutation.body) {
			rollbackErr = os.Remove(mutation.path)
		}
	}
	for id, object := range installed {
		if digest, digestErr := shared.DigestSkillDirectory(object.Path); digestErr == nil && digest == object.ManagedDigest {
			if err := os.RemoveAll(object.Path); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
		}
		if backup := backups[id]; backup != "" {
			if err := os.Rename(backup, object.Path); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
			delete(backups, id)
		}
	}
	for id, backup := range backups {
		if err := os.Rename(backup, previousByID[id].Path); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}
