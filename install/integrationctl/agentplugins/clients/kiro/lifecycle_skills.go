package kiro

import (
	"bytes"
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

type kiroNativeTxn struct {
	prepared        *kiroNativeApply
	transactionRoot string
	staged          map[string]string
	mcpPath         string
	mcp             map[string]any
	originalMCP     []byte
	originalMode    os.FileMode
	originalExists  bool
	hasMCPMutation  bool
	newMCP          []byte
	backups         map[string]string
	installed       map[string]domain.NativeObjectOwnership
	mcpWritten      bool
}

func newKiroNativeTxn(prepared *kiroNativeApply) (*kiroNativeTxn, error) {
	txn := &kiroNativeTxn{
		prepared: prepared, staged: map[string]string{}, mcp: map[string]any{},
		originalMode: 0o600, backups: map[string]string{},
		installed:      map[string]domain.NativeObjectOwnership{},
		mcpPath:        filepath.Join(prepared.configRoot, "settings", "mcp.json"),
		hasMCPMutation: len(previousMCPObjects(prepared.previous))+len(previousMCPObjects(prepared.desired)) > 0,
	}
	if !hasKiroSkillObjects(prepared.previous) && !hasKiroSkillObjects(prepared.desired) {
		return txn, nil
	}
	skillsRoot := filepath.Join(prepared.configRoot, "skills")
	if err := ValidateNativePath(prepared.configRoot, skillsRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(skillsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create Kiro skills root: %w", err)
	}
	root, err := os.MkdirTemp(skillsRoot, ".agentplugins-native-")
	if err != nil {
		return nil, fmt.Errorf("create Kiro native transaction: %w", err)
	}
	txn.transactionRoot = root
	return txn, nil
}

func (txn *kiroNativeTxn) stageSkills() error {
	for id, object := range txn.prepared.desiredByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if strings.TrimSpace(txn.prepared.activePath) == "" {
			return fmt.Errorf("active package path is required for Kiro skill installation")
		}
		source := filepath.Join(txn.prepared.activePath, filepath.FromSlash(object.SourceRelative))
		if err := pathpolicy.RequireContainedChild(txn.prepared.activePath, source); err != nil {
			return fmt.Errorf("unsafe Kiro skill source for %q: %w", object.LogicalName, err)
		}
		target := filepath.Join(txn.transactionRoot, "new-"+object.LogicalName)
		if err := filetree.CopyDir(source, target); err != nil {
			return fmt.Errorf("stage Kiro skill %q: %w", object.LogicalName, err)
		}
		digest, err := shared.DigestSkillDirectory(target)
		if err != nil || digest != object.ManagedDigest {
			return fmt.Errorf("staged Kiro skill %q does not match its ownership digest", object.LogicalName)
		}
		txn.staged[id] = target
	}
	return nil
}

func (txn *kiroNativeTxn) prepareMCP() error {
	if !txn.hasMCPMutation {
		return nil
	}
	var err error
	txn.mcp, txn.originalMCP, txn.originalMode, txn.originalExists, err = ReadMCPConfig(txn.mcpPath)
	if err != nil {
		return err
	}
	for _, object := range txn.prepared.previousByID {
		if object.Kind == kiroMCPObjectKind {
			delete(txn.mcp, object.LogicalName)
		}
	}
	for _, object := range txn.prepared.desiredByID {
		if object.Kind != kiroMCPObjectKind {
			continue
		}
		server, err := projectedKiroMCPServer(txn.prepared.activePath, object.LogicalName)
		if err != nil {
			return err
		}
		if shared.DigestJSONObject(server) != object.ManagedDigest {
			return fmt.Errorf("projected Kiro MCP server %q does not match its ownership digest", object.LogicalName)
		}
		txn.mcp[object.LogicalName] = server
	}
	txn.newMCP, err = encodeKiroMCPConfig(txn.originalMCP, txn.mcp)
	return err
}

func (txn *kiroNativeTxn) applySkillsAndMCP() error {
	for id, object := range txn.prepared.previousByID {
		if object.Kind != kiroSkillObjectKind {
			continue
		}
		if _, err := os.Lstat(object.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		backup := filepath.Join(txn.transactionRoot, "old-"+object.LogicalName)
		if err := os.Rename(object.Path, backup); err != nil {
			return fmt.Errorf("backup Kiro skill %q: %w", object.LogicalName, err)
		}
		txn.backups[id] = backup
	}
	for id, source := range txn.staged {
		object := txn.prepared.desiredByID[id]
		if err := os.Rename(source, object.Path); err != nil {
			return fmt.Errorf("activate Kiro skill %q: %w", object.LogicalName, err)
		}
		txn.installed[id] = object
	}
	if txn.hasMCPMutation {
		if err := atomicfile.Write(txn.mcpPath, txn.newMCP, txn.originalMode); err != nil {
			return fmt.Errorf("write Kiro MCP configuration: %w", err)
		}
		txn.mcpWritten = true
	}
	return nil
}

func (txn *kiroNativeTxn) rollback() error {
	var rollbackErr error
	if txn.mcpWritten {
		if txn.originalExists {
			rollbackErr = atomicfile.Write(txn.mcpPath, txn.originalMCP, txn.originalMode)
		} else if body, readErr := os.ReadFile(txn.mcpPath); readErr == nil && bytes.Equal(body, txn.newMCP) {
			rollbackErr = os.Remove(txn.mcpPath)
		}
	}
	for id, object := range txn.installed {
		if digest, digestErr := shared.DigestSkillDirectory(object.Path); digestErr == nil && digest == object.ManagedDigest {
			if err := os.RemoveAll(object.Path); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
		}
		if backup := txn.backups[id]; backup != "" {
			if err := os.Rename(backup, object.Path); err != nil && rollbackErr == nil {
				rollbackErr = err
			}
			delete(txn.backups, id)
		}
	}
	for id, backup := range txn.backups {
		if err := os.Rename(backup, txn.prepared.previousByID[id].Path); err != nil && rollbackErr == nil {
			rollbackErr = err
		}
	}
	return rollbackErr
}
