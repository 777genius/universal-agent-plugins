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
		return fmt.Errorf("Kiro config root is unavailable")
	}
	previous, desired = NativeObjects(previous), NativeObjects(desired)
	if err := VerifyNativeObjects(configRoot, previous, true); err != nil {
		return err
	}
	previousByID, desiredByID := shared.ObjectMap(previous), shared.ObjectMap(desired)
	for id, object := range desiredByID {
		if prior, replacing := previousByID[id]; replacing {
			if prior.Kind != object.Kind || prior.LogicalName != object.LogicalName || !shared.SameCleanPath(prior.Path, object.Path) {
				return fmt.Errorf("Kiro native object identity changed unexpectedly for %s", id)
			}
			continue
		}
		if err := requireKiroObjectAbsent(configRoot, object); err != nil {
			return err
		}
	}

	transactionRoot := ""
	if hasKiroSkillObjects(previous) || hasKiroSkillObjects(desired) {
		skillsRoot := filepath.Join(configRoot, "skills")
		if err := ValidateNativePath(configRoot, skillsRoot); err != nil {
			return err
		}
		if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
			return fmt.Errorf("create Kiro skills root: %w", err)
		}
		var err error
		transactionRoot, err = os.MkdirTemp(skillsRoot, ".agentplugins-native-")
		if err != nil {
			return fmt.Errorf("create Kiro native transaction: %w", err)
		}
		defer os.RemoveAll(transactionRoot)
	}

	staged := map[string]string{}
	for id, object := range desiredByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if strings.TrimSpace(activePath) == "" {
			return fmt.Errorf("active package path is required for Kiro skill installation")
		}
		source := filepath.Join(activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(activePath, source); err != nil {
			return fmt.Errorf("unsafe Kiro skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Kiro skill %q: %w", object.LogicalName, err)
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Kiro skill %q does not match its ownership digest", object.LogicalName)
		}
		staged[id] = target
	}

	mcpPath := filepath.Join(configRoot, "settings", "mcp.json")
	mcp, originalMCP, originalMode, originalExists := map[string]any{}, []byte(nil), os.FileMode(0o600), false
	hasMCPMutation := len(previousMCPObjects(previous))+len(previousMCPObjects(desired)) > 0
	if hasMCPMutation {
		var err error
		mcp, originalMCP, originalMode, originalExists, err = ReadMCPConfig(mcpPath)
		if err != nil {
			return err
		}
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
				return err
			}
			if shared.DigestJSONObject(server) != object.ManagedDigest {
				return fmt.Errorf("projected Kiro MCP server %q does not match its ownership digest", object.LogicalName)
			}
			mcp[object.LogicalName] = server
		}
	}
	newMCP := []byte(nil)
	if hasMCPMutation {
		var err error
		newMCP, err = encodeKiroMCPConfig(originalMCP, mcp)
		if err != nil {
			return err
		}
	}

	backups, installed := map[string]string{}, map[string]domain.NativeObjectOwnership{}
	mcpWritten := false
	rollback := func() error {
		var rollbackErr error
		if mcpWritten {
			if originalExists {
				rollbackErr = atomicfile.Write(mcpPath, originalMCP, originalMode)
			} else if body, readErr := os.ReadFile(mcpPath); readErr == nil && bytes.Equal(body, newMCP) {
				rollbackErr = os.Remove(mcpPath)
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
	defer func() {
		if resultErr != nil {
			if rollbackErr := rollback(); rollbackErr != nil {
				resultErr = fmt.Errorf("%v; Kiro native rollback failed: %w", resultErr, rollbackErr)
			}
		}
	}()

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
	for id, source := range staged {
		object := desiredByID[id]
		if err := os.Rename(source, object.Path); err != nil {
			return fmt.Errorf("activate Kiro skill %q: %w", object.LogicalName, err)
		}
		installed[id] = object
	}
	if hasMCPMutation {
		if err := atomicfile.Write(mcpPath, newMCP, originalMode); err != nil {
			return fmt.Errorf("write Kiro MCP configuration: %w", err)
		}
		mcpWritten = true
	}
	if err := VerifyNativeObjects(configRoot, desired, false); err != nil {
		return err
	}
	return nil
}
